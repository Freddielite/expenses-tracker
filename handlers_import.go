package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
	"github.com/xuri/excelize/v2"
)

// HandleImportCSV handles POST /api/import/csv. Kept as a dedicated
// endpoint (in addition to the newer /api/import/file below) for anyone
// scripting a raw CSV upload directly. Accepts the same shape
// HandleExportCSV produces — a header row followed by one row per
// transaction — but matches columns by name (case-insensitive) rather than
// position, so a spreadsheet export with reordered or extra columns still
// works. Required columns: date, type, category, amount. Optional:
// description.
//
// Accepts either a raw CSV request body, or a multipart/form-data upload
// with the file under the "file" field, so the frontend can just build a
// FormData without worrying about which one to send.
func (a *API) HandleImportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var body io.Reader = r.Body
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		file, _, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, `missing "file" field in upload`)
			return
		}
		defer file.Close()
		body = file
	}

	header, rows, err := readCSVRows(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := a.importTabularRows(tenantFrom(r), header, rows)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// HandleImportFile handles POST /api/import/file — the general-purpose
// import endpoint that accepts a CSV, Excel (.xlsx/.xls), or PDF upload
// under the multipart "file" field and picks a parser based on the
// uploaded filename's extension. This is what the frontend's "Import
// transactions" button now points at, so a user can drop in whatever
// format their bank or card issuer happens to export.
func (a *API) HandleImportFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		writeError(w, http.StatusBadRequest, "expected a multipart/form-data upload with a \"file\" field")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, `missing "file" field in upload`)
		return
	}
	defer file.Close()

	kind := fileKind(header.Filename, header.Header.Get("Content-Type"))

	var result ImportResult
	switch kind {
	case "csv":
		hdr, rows, err := readCSVRows(file)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		result, err = a.importTabularRows(tenantFrom(r), hdr, rows)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	case "xlsx":
		hdr, rows, err := readXLSXRows(file)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		result, err = a.importTabularRows(tenantFrom(r), hdr, rows)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	case "pdf":
		buf, err := io.ReadAll(file)
		if err != nil {
			writeError(w, http.StatusBadRequest, "could not read the uploaded PDF")
			return
		}
		result, err = a.importPDF(tenantFrom(r), buf)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "unsupported file type — upload a .csv, .xlsx, or .pdf file")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// fileKind guesses the upload format from its filename extension, falling
// back to the browser-supplied Content-Type when the extension is missing
// or ambiguous.
func fileKind(filename, contentType string) string {
	name := strings.ToLower(strings.TrimSpace(filename))
	switch {
	case strings.HasSuffix(name, ".csv"):
		return "csv"
	case strings.HasSuffix(name, ".xlsx"), strings.HasSuffix(name, ".xls"):
		return "xlsx"
	case strings.HasSuffix(name, ".pdf"):
		return "pdf"
	}

	mediaType, _, _ := mime.ParseMediaType(contentType)
	switch mediaType {
	case "text/csv":
		return "csv"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "application/vnd.ms-excel":
		return "xlsx"
	case "application/pdf":
		return "pdf"
	}
	return ""
}

// readCSVRows parses a CSV body into a header row and the remaining data
// rows, both as plain [][]string, so CSV and XLSX uploads can share the
// same column-matching logic below.
func readCSVRows(body io.Reader) (header []string, rows [][]string, err error) {
	reader := csv.NewReader(body)
	reader.FieldsPerRecord = -1 // tolerate ragged rows, e.g. a blank trailing description

	header, err = reader.Read()
	if err == io.EOF {
		return nil, nil, fmt.Errorf("empty CSV file")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("could not parse CSV header row")
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Keep going with a placeholder row rather than aborting the
			// whole import — importTabularRows reports it as a skipped
			// row with a clear reason, same as a bad value would be.
			rows = append(rows, nil)
			continue
		}
		rows = append(rows, record)
	}
	return header, rows, nil
}

// readXLSXRows reads the first sheet of an uploaded workbook into the same
// header + rows shape readCSVRows produces. Only the first sheet is read —
// bank and card issuer exports are almost always single-sheet, and reading
// just one keeps the behavior predictable rather than guessing which of
// several sheets holds the transactions.
func readXLSXRows(body io.Reader) (header []string, rows [][]string, err error) {
	buf, err := io.ReadAll(body)
	if err != nil {
		return nil, nil, fmt.Errorf("could not read the uploaded file")
	}
	f, err := excelize.OpenReader(bytes.NewReader(buf))
	if err != nil {
		return nil, nil, fmt.Errorf("could not open the Excel file — is it a valid .xlsx?")
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	if sheet == "" {
		return nil, nil, fmt.Errorf("the workbook has no sheets")
	}
	all, err := f.GetRows(sheet)
	if err != nil {
		return nil, nil, fmt.Errorf("could not read the %q sheet", sheet)
	}
	if len(all) == 0 {
		return nil, nil, fmt.Errorf("the first sheet is empty")
	}
	return all[0], all[1:], nil
}

// importTabularRows matches columns by name (case-insensitive) against a
// header row and creates one transaction per data row — shared by the CSV
// and XLSX paths, which both reduce to the same header + [][]string shape.
func (a *API) importTabularRows(tenantID string, header []string, rows [][]string) (ImportResult, error) {
	col := make(map[string]int, len(header))
	for i, name := range header {
		col[strings.ToLower(strings.TrimSpace(name))] = i
	}
	for _, name := range []string{"date", "type", "category", "amount"} {
		if _, ok := col[name]; !ok {
			return ImportResult{}, fmt.Errorf("missing required column %q", name)
		}
	}
	descIdx, hasDesc := col["description"]

	result := ImportResult{Errors: []string{}}
	rowNum := 1 // header is row 1, so the first data row is row 2 — matches what a user sees in a spreadsheet

	for _, record := range rows {
		rowNum++
		if record == nil {
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: could not parse row", rowNum))
			continue
		}

		field := func(idx int) string {
			if idx < 0 || idx >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[idx])
		}

		if isBlankRow(record) {
			// XLSX sheets routinely have trailing blank rows after the
			// real data — skip them silently rather than reporting a
			// wall of "row N: invalid amount" errors.
			continue
		}

		amount, err := parseAmount(field(col["amount"]))
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: invalid amount %q", rowNum, field(col["amount"])))
			continue
		}

		t := &Transaction{
			Type:     TransactionType(strings.ToLower(field(col["type"]))),
			Category: field(col["category"]),
			Date:     normalizeDate(field(col["date"])),
			Amount:   amount,
		}
		if hasDesc {
			t.Description = field(descIdx)
		}

		if err := validateTransaction(t); err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: %v", rowNum, err))
			continue
		}
		if err := a.store.Create(tenantID, t); err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: failed to save", rowNum))
			continue
		}
		result.Imported++
	}

	capImportErrors(&result)
	return result, nil
}

// importPDF extracts the plain text of an uploaded PDF and looks for
// lines that read like a statement transaction: a date, a description,
// and a trailing amount. Unlike the CSV/XLSX paths this is a best-effort
// heuristic — PDF has no notion of columns once the text is pulled out,
// so a "type" isn't stated outright and is inferred from the amount's
// sign and a few common keywords. The result always carries a Note
// telling the caller to double-check what came in.
func (a *API) importPDF(tenantID string, fileBytes []byte) (ImportResult, error) {
	r, err := pdf.NewReader(bytes.NewReader(fileBytes), int64(len(fileBytes)))
	if err != nil {
		return ImportResult{}, fmt.Errorf("could not open the PDF — is it a valid, non-scanned PDF?")
	}
	textReader, err := r.GetPlainText()
	if err != nil {
		return ImportResult{}, fmt.Errorf("could not extract text from the PDF — it may be a scanned image rather than real text")
	}
	var textBuf bytes.Buffer
	if _, err := textBuf.ReadFrom(textReader); err != nil {
		return ImportResult{}, fmt.Errorf("could not read the PDF's extracted text")
	}

	result := ImportResult{Errors: []string{}}
	result.Note = "PDF import reads text heuristically (date, description, and amount per line) since a PDF has no real columns — double-check the imported entries, especially income vs. expense."

	lineNum := 0
	for _, line := range strings.Split(textBuf.String(), "\n") {
		lineNum++
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		match := pdfTransactionLine.FindStringSubmatch(line)
		if match == nil {
			continue // most lines in a statement (headers, page numbers, balances) simply aren't transactions
		}

		date := normalizeDate(match[1])
		desc := strings.TrimSpace(match[2])
		amount, err := parseAmount(match[3])
		if err != nil {
			continue
		}

		t := &Transaction{
			Type:        inferTransactionType(match[3], desc),
			Category:    "Uncategorized",
			Description: desc,
			Date:        date,
			Amount:      amount,
		}
		if err := validateTransaction(t); err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf("line %d: %v", lineNum, err))
			continue
		}
		if err := a.store.Create(tenantID, t); err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf("line %d: failed to save", lineNum))
			continue
		}
		result.Imported++
	}

	if result.Imported == 0 {
		result.Errors = append(result.Errors, "no transaction-shaped lines were found — this PDF's layout may not be supported yet")
	}

	capImportErrors(&result)
	return result, nil
}

// pdfTransactionLine matches a line of the shape:
//
//	07/14/2026  Coffee shop purchase   -4.85
//	2026-07-14  Paycheck deposit       1,200.00
//
// i.e. a leading date, a description, and a trailing signed/parenthesized
// dollar amount. Statement layouts vary a lot, so this intentionally
// covers the common case rather than trying to be exhaustive.
var pdfTransactionLine = regexp.MustCompile(
	`^(\d{1,4}[/-]\d{1,2}[/-]\d{1,4})\s+(.+?)\s+(\(?-?\$?[\d,]+\.\d{2}\)?)$`,
)

// inferTransactionType guesses income vs. expense from the raw amount
// token (parentheses or a leading minus sign both conventionally mean a
// debit) and, failing that, from common keywords in the description.
func inferTransactionType(rawAmount, description string) TransactionType {
	if strings.HasPrefix(rawAmount, "(") || strings.HasPrefix(rawAmount, "-") {
		return TypeExpense
	}
	lower := strings.ToLower(description)
	for _, kw := range []string{"deposit", "payroll", "payment received", "refund", "interest paid", "credit"} {
		if strings.Contains(lower, kw) {
			return TypeIncome
		}
	}
	return TypeExpense
}

// parseAmount accepts the amount formats commonly seen in exports and
// statements — a plain "12.34", a signed "-12.34", one with thousands
// separators "1,200.00", a leading currency symbol "$12.34", or
// accounting-style parentheses for a negative "(12.34)" — and returns an
// unsigned magnitude, since Transaction.Type (not the amount's sign)
// carries income vs. expense.
func parseAmount(raw string) (float64, error) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = s[1 : len(s)-1]
	}
	s = strings.TrimPrefix(s, "$")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)

	amount, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	if amount < 0 {
		amount = -amount
	}
	return amount, nil
}

// normalizeDate tries a handful of common spreadsheet/statement date
// formats and rewrites them to the app's ISO "2026-07-14" shape. If none
// match, the original string is returned as-is — validateTransaction only
// requires a non-empty date, so an unrecognized format still imports, it
// just may not sort/filter as expected until edited.
func normalizeDate(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}
	layouts := []string{
		"2006-01-02",
		"01/02/2006",
		"1/2/2006",
		"01-02-2006",
		"1-2-2006",
		"01/02/06",
		"1/2/06",
		"Jan 2, 2006",
		"January 2, 2006",
		"2 Jan 2006",
		"02-Jan-2006",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return s
}

func isBlankRow(record []string) bool {
	for _, cell := range record {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// capImportErrors caps how many error strings are sent back so one badly
// formed thousand-row file doesn't blow up the response body — Skipped
// already reflects the true total regardless of this cap.
func capImportErrors(result *ImportResult) {
	const maxErrors = 20
	if len(result.Errors) > maxErrors {
		result.Errors = append(result.Errors[:maxErrors], fmt.Sprintf("...and %d more", len(result.Errors)-maxErrors))
	}
}
