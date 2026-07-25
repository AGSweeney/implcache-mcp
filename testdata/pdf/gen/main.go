// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Generates synthetic PDF fixtures under testdata/pdf/.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	outDir := ".."
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal(err)
	}
	write(filepath.Join(outDir, "text_manual.pdf"), textManualPDF())
	write(filepath.Join(outDir, "image_only.pdf"), imageOnlyPDF())
	write(filepath.Join(outDir, "bookmarked.pdf"), bookmarkedPDF())
	fmt.Println("wrote fixtures to", outDir)
}

func write(path, body string) {
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func textManualPDF() string {
	// Multi-page text PDF with repeating header for suppression tests.
	p1 := contentStream(
		"BT /F1 12 Tf 72 720 Td (COMMON HEADER) Tj T* " +
			"(Widget reconnect backoff and RetryPolicy configuration.) Tj T* " +
			"(See chapter one for initialization sequence details.) Tj ET",
	)
	p2 := contentStream(
		"BT /F1 12 Tf 72 720 Td (COMMON HEADER) Tj T* " +
			"(Chapter two covers error handling and timeout budgets.) Tj T* " +
			"(Always validate the product version before calling RegisterHandler.) Tj ET",
	)
	return assemblePDF([]pageSpec{
		{contents: p1},
		{contents: p2},
	}, nil, "Synthetic Text Manual")
}

func imageOnlyPDF() string {
	// Empty content stream → no extractable text (OCR required).
	return assemblePDF([]pageSpec{
		{contents: contentStream("")},
	}, nil, "Image Only Stub")
}

func bookmarkedPDF() string {
	p1 := contentStream(
		"BT /F1 12 Tf 72 720 Td (Introduction to the Example Device SDK.) Tj T* " +
			"(This section explains bootstrapping the host adapter.) Tj ET",
	)
	p2 := contentStream(
		"BT /F1 12 Tf 72 720 Td (API Reference for RegisterHandler and callbacks.) Tj T* " +
			"(Pass a stable command name and validate the payload schema.) Tj ET",
	)
	outlines := &outlineSpec{
		title:    "Introduction",
		pageIndex: 0,
		nextTitle: "API Reference",
		nextPage:  1,
	}
	return assemblePDF([]pageSpec{
		{contents: p1},
		{contents: p2},
	}, outlines, "Bookmarked Manual")
}

type pageSpec struct {
	contents string
}

type outlineSpec struct {
	title     string
	pageIndex int
	nextTitle string
	nextPage  int
}

func contentStream(ops string) string {
	body := strings.TrimSpace(ops)
	return fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(body)+1, body+"\n")
}

func assemblePDF(pages []pageSpec, outlines *outlineSpec, title string) string {
	// Object layout:
	// 1 Catalog, 2 Pages, 3 Font, then for each page: PageN, ContentN
	// optional Outlines at end.
	var b strings.Builder
	b.WriteString("%PDF-1.4\n")
	offsets := map[int]int{}
	nextID := 1

	writeObj := func(id int, body string) {
		offsets[id] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", id, body)
	}

	catalogID := nextID
	nextID++
	pagesID := nextID
	nextID++
	fontID := nextID
	nextID++

	pageIDs := make([]int, len(pages))
	contentIDs := make([]int, len(pages))
	for i := range pages {
		pageIDs[i] = nextID
		nextID++
		contentIDs[i] = nextID
		nextID++
	}

	var outlinesID, outline1ID, outline2ID int
	if outlines != nil {
		outlinesID = nextID
		nextID++
		outline1ID = nextID
		nextID++
		outline2ID = nextID
		nextID++
	}

	infoID := nextID
	nextID++

	catalogBody := fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R", pagesID)
	if outlinesID != 0 {
		catalogBody += fmt.Sprintf(" /Outlines %d 0 R /PageMode /UseOutlines", outlinesID)
	}
	catalogBody += " >>"
	writeObj(catalogID, catalogBody)

	kids := make([]string, len(pageIDs))
	for i, id := range pageIDs {
		kids[i] = fmt.Sprintf("%d 0 R", id)
	}
	writeObj(pagesID, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages)))
	writeObj(fontID, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")

	for i, p := range pages {
		writeObj(pageIDs[i], fmt.Sprintf(
			"<< /Type /Page /Parent %d 0 R /MediaBox [0 0 612 792] /Contents %d 0 R /Resources << /Font << /F1 %d 0 R >> >> >>",
			pagesID, contentIDs[i], fontID,
		))
		writeObj(contentIDs[i], p.contents)
	}

	if outlines != nil {
		writeObj(outlinesID, fmt.Sprintf(
			"<< /Type /Outlines /First %d 0 R /Last %d 0 R /Count 2 >>", outline1ID, outline2ID,
		))
		writeObj(outline1ID, fmt.Sprintf(
			"<< /Title (%s) /Parent %d 0 R /Next %d 0 R /Dest [%d 0 R /XYZ null 720 null] >>",
			pdfString(outlines.title), outlinesID, outline2ID, pageIDs[outlines.pageIndex],
		))
		writeObj(outline2ID, fmt.Sprintf(
			"<< /Title (%s) /Parent %d 0 R /Prev %d 0 R /Dest [%d 0 R /XYZ null 720 null] >>",
			pdfString(outlines.nextTitle), outlinesID, outline1ID, pageIDs[outlines.nextPage],
		))
	}

	writeObj(infoID, fmt.Sprintf("<< /Title (%s) /Author (ImplCache Fixtures) /Producer (testdata/pdf/gen) >>", pdfString(title)))

	xrefStart := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n", nextID)
	b.WriteString("0000000000 65535 f \n")
	for i := 1; i < nextID; i++ {
		fmt.Fprintf(&b, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root %d 0 R /Info %d 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		nextID, catalogID, infoID, xrefStart)
	return b.String()
}

func pdfString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")
	return s
}
