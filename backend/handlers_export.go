package main

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"
)

func (a *API) HandleExportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="transactions.csv"`)

	writer := csv.NewWriter(w)
	defer writer.Flush()

	_ = writer.Write([]string{"date", "type", "category", "description", "amount"})
	for _, t := range a.store.List(tenantFrom(r)) {
		_ = writer.Write([]string{
			t.Date,
			string(t.Type),
			t.Category,
			t.Description,
			fmt.Sprintf("%.2f", t.Amount),
		})
	}
}

// HandleExportXLSX serves a multi-sheet Excel workbook: a Summary sheet,
// the full transaction ledger, a by-category breakdown, and a by-month
// breakdown — the same underlying data as HandleExportCSV and the reports
// endpoint, just packaged as a workbook instead of a flat CSV.
func (a *API) HandleExportXLSX(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	transactions := a.store.List(tenantFrom(r))
	report := BuildReport(transactions)

	data, err := BuildTransactionsXLSX(transactions, report)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build export")
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="expense-tracker-export.xlsx"`)
	_, _ = w.Write(data)
}

// HandleExportHTML serves a single, self-contained interactive HTML
// dashboard — charts, filters, and the full ledger, all embedded inline
// with no external requests, so the downloaded file works offline.
func (a *API) HandleExportHTML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	transactions := a.store.List(tenantFrom(r))
	report := BuildReport(transactions)

	page, err := BuildDashboardHTML(transactions, report)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build export")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="expense-tracker-report.html"`)
	_, _ = w.Write(page)
}

// BuildTransactionsXLSX assembles the workbook served by HandleExportXLSX.
func BuildTransactionsXLSX(transactions []*Transaction, report ReportResponse) ([]byte, error) {
	summarySheet := xlsxSheet{
		name: "Summary",
		rows: [][]xlsxCell{
			{headerCell("Expense Tracker Export")},
			{textCell("Generated"), textCell(time.Now().Format("2 Jan 2006, 15:04"))},
			{},
			{headerCell("Metric"), headerCell("Value")},
			{textCell("Total income"), numberCell(report.TotalIncome)},
			{textCell("Total expense"), numberCell(report.TotalExpense)},
			{textCell("Net"), boldNumberCell(report.Net)},
			{textCell("Transaction count"), numberCell(float64(report.TransactionCount))},
		},
	}

	txnRows := [][]xlsxCell{
		{headerCell("Date"), headerCell("Type"), headerCell("Category"), headerCell("Description"), headerCell("Amount")},
	}
	for _, t := range transactions {
		txnRows = append(txnRows, []xlsxCell{
			textCell(t.Date),
			textCell(string(t.Type)),
			textCell(t.Category),
			textCell(t.Description),
			numberCell(t.Amount),
		})
	}
	transactionsSheet := xlsxSheet{name: "Transactions", rows: txnRows}

	catRows := [][]xlsxCell{
		{headerCell("Category"), headerCell("Total"), headerCell("Count")},
	}
	for _, c := range report.ByCategory {
		catRows = append(catRows, []xlsxCell{
			textCell(c.Category),
			numberCell(c.Total),
			numberCell(float64(c.Count)),
		})
	}
	categorySheet := xlsxSheet{name: "By Category", rows: catRows}

	monthRows := [][]xlsxCell{
		{headerCell("Month"), headerCell("Income"), headerCell("Expense"), headerCell("Net")},
	}
	for _, m := range report.ByMonth {
		monthRows = append(monthRows, []xlsxCell{
			textCell(m.Month),
			numberCell(m.TotalIncome),
			numberCell(m.TotalExpense),
			numberCell(m.Net),
		})
	}
	monthSheet := xlsxSheet{name: "By Month", rows: monthRows}

	return BuildXLSX([]xlsxSheet{summarySheet, transactionsSheet, categorySheet, monthSheet})
}
