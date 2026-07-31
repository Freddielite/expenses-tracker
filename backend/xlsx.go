package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"html"
	"strconv"
	"strings"
)

// This file implements a minimal but valid .xlsx (OOXML spreadsheet) writer
// using only the standard library. An .xlsx is just a zip archive of XML
// parts, so there's no need to pull in a third-party package for something
// this small — it also keeps the backend dependency-free and buildable
// without network access.
//
// Supported: multiple named sheets, text cells, numeric cells, a bold style
// for header rows, and a two-decimal number format for money columns.
// Not supported (deliberately, to keep this simple): formulas, merged
// cells, cell colors. None of those are needed for a data export.

// xlsxCellKind distinguishes how a cell's value should be written and styled.
type xlsxCellKind int

const (
	xlsxText xlsxCellKind = iota
	xlsxTextBold
	xlsxNumber
	xlsxNumberBold
)

type xlsxCell struct {
	text   string // used when kind is xlsxText / xlsxTextBold
	number float64
	kind   xlsxCellKind
}

func textCell(v string) xlsxCell        { return xlsxCell{text: v, kind: xlsxText} }
func headerCell(v string) xlsxCell      { return xlsxCell{text: v, kind: xlsxTextBold} }
func numberCell(v float64) xlsxCell     { return xlsxCell{number: v, kind: xlsxNumber} }
func boldNumberCell(v float64) xlsxCell { return xlsxCell{number: v, kind: xlsxNumberBold} }

type xlsxSheet struct {
	name string
	rows [][]xlsxCell
}

// styleIndex maps a cell kind to its <xf> index in styles.xml (see stylesXML).
func (c xlsxCell) styleIndex() int {
	switch c.kind {
	case xlsxTextBold:
		return 1
	case xlsxNumber:
		return 2
	case xlsxNumberBold:
		return 3
	default:
		return 0
	}
}

// BuildXLSX assembles a complete .xlsx workbook (as raw bytes) from the
// given sheets, in order.
func BuildXLSX(sheets []xlsxSheet) ([]byte, error) {
	if len(sheets) == 0 {
		return nil, fmt.Errorf("xlsx: at least one sheet is required")
	}

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	write := func(name, content string) error {
		fw, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = fw.Write([]byte(content))
		return err
	}

	if err := write("[Content_Types].xml", xlsxContentTypes(len(sheets))); err != nil {
		return nil, err
	}
	if err := write("_rels/.rels", xlsxPackageRels()); err != nil {
		return nil, err
	}
	if err := write("xl/workbook.xml", xlsxWorkbook(sheets)); err != nil {
		return nil, err
	}
	if err := write("xl/_rels/workbook.xml.rels", xlsxWorkbookRels(len(sheets))); err != nil {
		return nil, err
	}
	if err := write("xl/styles.xml", xlsxStyles()); err != nil {
		return nil, err
	}
	for i, sheet := range sheets {
		if err := write(fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1), xlsxSheetXML(sheet)); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// colLetter converts a 1-indexed column number to its spreadsheet letter
// (1 -> A, 2 -> B, ... 27 -> AA).
func colLetter(n int) string {
	var s strings.Builder
	for n > 0 {
		n--
		s.WriteByte(byte('A' + n%26))
		n /= 26
	}
	// The loop above builds the letters most-significant-last; reverse them.
	letters := []byte(s.String())
	for i, j := 0, len(letters)-1; i < j; i, j = i+1, j-1 {
		letters[i], letters[j] = letters[j], letters[i]
	}
	return string(letters)
}

// safeSheetName trims a sheet name to Excel's 31-character limit and strips
// characters that aren't allowed in a sheet name.
func safeSheetName(name string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", "?", "", "*", "", "[", "(", "]", ")", ":", "-")
	name = replacer.Replace(name)
	if len(name) > 31 {
		name = name[:31]
	}
	return name
}

func xlsxContentTypes(sheetCount int) string {
	var overrides strings.Builder
	for i := 1; i <= sheetCount; i++ {
		fmt.Fprintf(&overrides, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i)
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
		`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>` +
		overrides.String() +
		`</Types>`
}

func xlsxPackageRels() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
		`</Relationships>`
}

func xlsxWorkbook(sheets []xlsxSheet) string {
	var sheetEls strings.Builder
	for i, sheet := range sheets {
		fmt.Fprintf(&sheetEls, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`,
			html.EscapeString(safeSheetName(sheet.name)), i+1, i+1)
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<sheets>` + sheetEls.String() + `</sheets>` +
		`</workbook>`
}

func xlsxWorkbookRels(sheetCount int) string {
	var rels strings.Builder
	for i := 1; i <= sheetCount; i++ {
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, i, i)
	}
	fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`, sheetCount+1)
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		rels.String() +
		`</Relationships>`
}

// xlsxStyles defines exactly four cell formats (xf entries), indices 0-3,
// matching xlsxCell.styleIndex() above:
//
//	0: default text        1: bold text (headers)
//	2: number, 2dp          3: bold number, 2dp
func xlsxStyles() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<numFmts count="1"><numFmt numFmtId="164" formatCode="#,##0.00"/></numFmts>` +
		`<fonts count="2">` +
		`<font><sz val="11"/><name val="Calibri"/></font>` +
		`<font><b/><sz val="11"/><name val="Calibri"/></font>` +
		`</fonts>` +
		`<fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills>` +
		`<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>` +
		`<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>` +
		`<cellXfs count="4">` +
		`<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>` +
		`<xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/>` +
		`<xf numFmtId="164" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>` +
		`<xf numFmtId="164" fontId="1" fillId="0" borderId="0" xfId="0" applyNumberFormat="1" applyFont="1"/>` +
		`</cellXfs>` +
		`<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>` +
		`</styleSheet>`
}

func xlsxSheetXML(sheet xlsxSheet) string {
	// Column widths: size each column to its widest cell, within sane bounds.
	colCount := 0
	for _, row := range sheet.rows {
		if len(row) > colCount {
			colCount = len(row)
		}
	}
	widths := make([]int, colCount)
	for _, row := range sheet.rows {
		for c, cell := range row {
			l := len(cell.text)
			if cell.kind == xlsxNumber || cell.kind == xlsxNumberBold {
				l = len(strconv.FormatFloat(cell.number, 'f', 2, 64))
			}
			if l > widths[c] {
				widths[c] = l
			}
		}
	}

	var cols strings.Builder
	if colCount > 0 {
		cols.WriteString("<cols>")
		for i, w := range widths {
			width := w + 3
			if width < 10 {
				width = 10
			}
			if width > 50 {
				width = 50
			}
			fmt.Fprintf(&cols, `<col min="%d" max="%d" width="%d" customWidth="1"/>`, i+1, i+1, width)
		}
		cols.WriteString("</cols>")
	}

	var body strings.Builder
	body.WriteString("<sheetData>")
	for r, row := range sheet.rows {
		fmt.Fprintf(&body, `<row r="%d">`, r+1)
		for c, cell := range row {
			ref := colLetter(c+1) + strconv.Itoa(r+1)
			style := cell.styleIndex()
			switch cell.kind {
			case xlsxNumber, xlsxNumberBold:
				fmt.Fprintf(&body, `<c r="%s" s="%d"><v>%s</v></c>`, ref, style, strconv.FormatFloat(cell.number, 'f', 2, 64))
			default:
				styleAttr := ""
				if style != 0 {
					styleAttr = fmt.Sprintf(` s="%d"`, style)
				}
				fmt.Fprintf(&body, `<c r="%s"%s t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, styleAttr, html.EscapeString(cell.text))
			}
		}
		body.WriteString("</row>")
	}
	body.WriteString("</sheetData>")

	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		cols.String() +
		body.String() +
		`</worksheet>`
}
