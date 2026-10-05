// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package agent

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// buildOOXML writes a minimal archive of the given parts — the same shape Word,
// PowerPoint and Excel produce.
func buildOOXML(t *testing.T, parts map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range parts {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return buf.Bytes()
}

func TestExtractDocxText(t *testing.T) {
	doc := buildOOXML(t, map[string]string{
		"word/document.xml": `<?xml version="1.0"?><w:document xmlns:w="x"><w:body>` +
			`<w:p><w:r><w:t>Laporan rapat</w:t></w:r></w:p>` +
			`<w:p><w:r><w:t>Kode: RAPAT-7731</w:t></w:r></w:p>` +
			`</w:body></w:document>`,
		"[Content_Types].xml": `<Types/>`,
	})

	text, err := extractOfficeText(mimeDocx, doc)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	for _, want := range []string{"Laporan rapat", "RAPAT-7731"} {
		if !strings.Contains(text, want) {
			t.Errorf("text is missing %q:\n%s", want, text)
		}
	}
}

func TestExtractPptxTextAcrossSlides(t *testing.T) {
	deck := buildOOXML(t, map[string]string{
		"ppt/slides/slide1.xml": `<p:sld xmlns:p="x"><a:t xmlns:a="y">Slide satu</a:t></p:sld>`,
		"ppt/slides/slide2.xml": `<p:sld xmlns:p="x"><a:t xmlns:a="y">Slide dua</a:t></p:sld>`,
	})

	text, err := extractOfficeText(mimePptx, deck)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !strings.Contains(text, "Slide satu") || !strings.Contains(text, "Slide dua") {
		t.Errorf("both slides should be present:\n%s", text)
	}
}

// Spreadsheet cells reference a shared string table by index. Resolve it wrong
// and every text cell comes out as a number.
func TestExtractXlsxResolvesSharedStrings(t *testing.T) {
	book := buildOOXML(t, map[string]string{
		"xl/sharedStrings.xml": `<sst xmlns="x"><si><t>Tayangan</t></si><si><t>Pengikut</t></si></sst>`,
		"xl/worksheets/sheet1.xml": `<worksheet xmlns="x"><sheetData>` +
			`<row><c t="s"><v>0</v></c><c><v>478307</v></c></row>` +
			`<row><c t="s"><v>1</v></c><c><v>124</v></c></row>` +
			`</sheetData></worksheet>`,
	})

	text, err := extractOfficeText(mimeXlsx, book)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !strings.Contains(text, "Tayangan | 478307") {
		t.Errorf("shared string not resolved into its row:\n%s", text)
	}
	if !strings.Contains(text, "Pengikut | 124") {
		t.Errorf("second row wrong:\n%s", text)
	}
}

func TestExtractOfficeRejectsNonArchive(t *testing.T) {
	if _, err := extractOfficeText(mimeDocx, []byte("bukan zip sama sekali")); err == nil {
		t.Error("a file that is not an archive should report an error, not empty text")
	}
}

func TestIsOfficeDocument(t *testing.T) {
	for _, m := range []string{mimeDocx, mimePptx, mimeXlsx} {
		if !isOfficeDocument(m) {
			t.Errorf("%s should be convertible", m)
		}
	}
	for _, m := range []string{"application/pdf", "text/plain", "image/png", "application/zip"} {
		if isOfficeDocument(m) {
			t.Errorf("%s must not be treated as an Office document", m)
		}
	}
}
