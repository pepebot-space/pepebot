// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package agent

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Office formats no model reads directly. A .docx is a zip of XML, and so are
// .pptx and .xlsx — so pepebot unzips them and sends the words.
//
// Without this an attached Word file is a dead end on every provider: Anthropic
// has no document shape for it, and the OpenAI-shaped providers pass bytes the
// model cannot parse. Turning it into text costs a few hundred lines of nothing
// and works on every model, including the text-only ones.
const (
	mimeDocx = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	mimePptx = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	mimeXlsx = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

// officeTextLimit caps what one attachment can contribute to a prompt. A long
// report is still useful truncated; an unbounded one pushes the conversation out
// of its context window.
const officeTextLimit = 200_000

// isOfficeDocument reports whether extractOfficeText can read this media type.
func isOfficeDocument(mimeType string) bool {
	switch mimeType {
	case mimeDocx, mimePptx, mimeXlsx:
		return true
	}
	return false
}

// extractOfficeText pulls the readable text out of an OOXML file.
//
// Each format keeps its words in different parts of the archive, but all three
// store them as <t> elements, so one extractor serves all three once it knows
// which parts to open. Spreadsheets are the exception: cell values live in a
// shared string table that the sheets reference by index.
func extractOfficeText(mimeType string, data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("not a readable Office file: %w", err)
	}

	if mimeType == mimeXlsx {
		return extractSheetText(zr)
	}

	wanted := func(name string) bool { return name == "word/document.xml" }
	if mimeType == mimePptx {
		wanted = func(name string) bool {
			return strings.HasPrefix(name, "ppt/slides/slide") && strings.HasSuffix(name, ".xml")
		}
	}

	var out strings.Builder
	for _, f := range zr.File {
		if !wanted(f.Name) {
			continue
		}
		text, err := textNodes(f)
		if err != nil {
			continue
		}
		if out.Len() > 0 {
			out.WriteString("\n\n")
		}
		out.WriteString(text)
		if out.Len() >= officeTextLimit {
			break
		}
	}

	return finish(out.String())
}

// extractSheetText renders a workbook as "value | value" rows. Cell text is held
// once in sharedStrings.xml and referenced by index from each sheet, so the
// table has to be read first or every text cell comes out as a number.
func extractSheetText(zr *zip.Reader) (string, error) {
	var shared []string
	for _, f := range zr.File {
		if f.Name == "xl/sharedStrings.xml" {
			if text, err := textNodesSlice(f); err == nil {
				shared = text
			}
			break
		}
	}

	var out strings.Builder
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "xl/worksheets/sheet") || !strings.HasSuffix(f.Name, ".xml") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		rows, err := sheetRows(rc, shared)
		rc.Close()
		if err != nil {
			continue
		}
		if out.Len() > 0 {
			out.WriteString("\n\n")
		}
		out.WriteString(rows)
		if out.Len() >= officeTextLimit {
			break
		}
	}

	return finish(out.String())
}

// sheetRows walks one worksheet, substituting shared strings for the cells that
// reference them (t="s"), and joins each row on " | ".
func sheetRows(r io.Reader, shared []string) (string, error) {
	dec := xml.NewDecoder(r)
	var out strings.Builder
	var row []string
	var cellType string
	var inValue bool

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return out.String(), err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "row":
				row = row[:0]
			case "c":
				cellType = ""
				for _, a := range t.Attr {
					if a.Name.Local == "t" {
						cellType = a.Value
					}
				}
			case "v", "t":
				inValue = true
			}
		case xml.CharData:
			if !inValue {
				continue
			}
			value := strings.TrimSpace(string(t))
			if value == "" {
				continue
			}
			if cellType == "s" {
				if idx, ok := sharedIndex(value, len(shared)); ok {
					value = shared[idx]
				}
			}
			row = append(row, value)
		case xml.EndElement:
			switch t.Name.Local {
			case "v", "t":
				inValue = false
			case "row":
				if len(row) > 0 {
					out.WriteString(strings.Join(row, " | "))
					out.WriteByte('\n')
				}
			}
		}
		if out.Len() >= officeTextLimit {
			break
		}
	}
	return out.String(), nil
}

func sharedIndex(value string, n int) (int, bool) {
	idx := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		idx = idx*10 + int(r-'0')
		if idx >= n {
			return 0, false
		}
	}
	return idx, true
}

// textNodes joins every <t> element in one archive part.
func textNodes(f *zip.File) (string, error) {
	parts, err := textNodesSlice(f)
	if err != nil {
		return "", err
	}
	return strings.Join(parts, " "), nil
}

func textNodesSlice(f *zip.File) ([]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	var parts []string
	var inText bool
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return parts, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inText = true
			}
		case xml.CharData:
			if inText {
				parts = append(parts, string(t))
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inText = false
			}
		}
	}
	return parts, nil
}

func finish(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("no readable text in the document")
	}
	if len(text) > officeTextLimit {
		text = text[:officeTextLimit] + "\n\n[dipotong: dokumen lebih panjang dari batas yang dikirim ke model]"
	}
	return text, nil
}
