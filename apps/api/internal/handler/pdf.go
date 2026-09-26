package handler

import (
	"bytes"
	"fmt"
	"strings"
)

// Page geometry: A4 portrait in PostScript points (72 per inch).
const (
	pdfPageWidth  = 595
	pdfPageHeight = 842
	pdfMarginX    = 50
	pdfLineHeight = 14
	// pdfLinesPerPage leaves room for the margin at the top and bottom.
	pdfLinesPerPage = 52
)

// pdfDoc is a minimal PDF writer.
//
// It exists so report export does not take on a PDF dependency: the reports
// this service emits are plain text tables, and a writer that lays out text
// lines is a fraction of the size of a general-purpose PDF library — and far
// less to audit. Only the features used below are implemented (one Helvetica
// font, text-only pages, an Info dictionary).
type pdfDoc struct {
	title      string
	keywords   string
	pages      [][]string
}

func newPDFDoc(title string) *pdfDoc {
	return &pdfDoc{title: title}
}

// AddLines paginates lines into pages, starting a new page every
// pdfLinesPerPage lines.
func (d *pdfDoc) AddLines(lines []string) {
	if len(lines) == 0 {
		d.pages = append(d.pages, []string{""})
		return
	}
	for i := 0; i < len(lines); i += pdfLinesPerPage {
		end := i + pdfLinesPerPage
		if end > len(lines) {
			end = len(lines)
		}
		page := make([]string, end-i)
		copy(page, lines[i:end])
		d.pages = append(d.pages, page)
	}
}

// SetKeywords stores the report signature in the document's Info dictionary,
// so the signature travels inside the file and not only in a response header.
func (d *pdfDoc) SetKeywords(kw string) { d.keywords = kw }

// Bytes serialises the document.
//
// Object layout (numbers are referenced by the pages' /Kids and the trailer):
//
//	1 catalog, 2 pages tree, 3 font, 4 info, then 5/6, 7/8, ... page/content
//
// A correct cross-reference table is required for a viewer to open the file, so
// byte offsets are recorded as each object is written rather than computed.
func (d *pdfDoc) Bytes() []byte {
	if len(d.pages) == 0 {
		d.AddLines(nil)
	}
	pageCount := len(d.pages)
	totalObjs := 4 + 2*pageCount // catalog, pages, font, info + 2 per page
	offsets := make([]int, totalObjs+1)

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")

	obj := func(num int, body string) {
		offsets[num] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", num, body)
	}

	obj(1, "<< /Type /Catalog /Pages 2 0 R >>")

	kids := make([]string, 0, pageCount)
	for i := 0; i < pageCount; i++ {
		kids = append(kids, fmt.Sprintf("%d 0 R", 5+i*2))
	}
	obj(2, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>",
		strings.Join(kids, " "), pageCount))

	obj(3, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")

	obj(4, fmt.Sprintf("<< /Title (%s) /Producer (Sorolens) /Keywords (%s) >>",
		escapePDFText(d.title), escapePDFText(d.keywords)))

	for i, lines := range d.pages {
		pageNum := 5 + i*2
		contentNum := pageNum + 1

		obj(pageNum, fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] "+
				"/Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>",
			pdfPageWidth, pdfPageHeight, contentNum))

		stream := renderTextStream(lines, i+1, pageCount)
		obj(contentNum, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	}

	xrefOffset := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", totalObjs+1)
	// The head of the table is the free object 0 and is always this exact line.
	buf.WriteString("0000000000 65535 f \n")
	for n := 1; n <= totalObjs; n++ {
		// %010d is required: the xref table is fixed-width.
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[n])
	}
	fmt.Fprintf(&buf,
		"trailer\n<< /Size %d /Root 1 0 R /Info 4 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		totalObjs+1, xrefOffset)

	return buf.Bytes()
}

// renderTextStream lays a page's lines out top-down in 10pt Helvetica. The
// leading (TL) and start position (Td) match pdfLineHeight so pagination is
// predictable.
func renderTextStream(lines []string, page, total int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "BT\n/F1 10 Tf\n%d TL\n%d %d Td\n",
		pdfLineHeight, pdfMarginX, pdfPageHeight-pdfMarginX)

	for _, line := range lines {
		fmt.Fprintf(&b, "(%s) Tj\nT*\n", escapePDFText(line))
	}

	// Page number footer, drawn without moving the text cursor for the body.
	footer := fmt.Sprintf("Page %d of %d", page, total)
	fmt.Fprintf(&b, "ET\nBT\n/F1 8 Tf\n%d 30 Td\n(%s) Tj\nET",
		pdfMarginX, escapePDFText(footer))

	return b.String()
}

// escapePDFText escapes the three characters that would otherwise terminate a
// PDF literal string or start an escape sequence.
func escapePDFText(s string) string {
	// Non-ASCII would need font encoding work this writer does not do; folding
	// it to '?' keeps the output a valid PDF rather than emitting mojibake.
	s = strings.Map(func(r rune) rune {
		if r < 32 || r > 126 {
			return '?'
		}
		return r
	}, s)

	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	return r.Replace(s)
}
