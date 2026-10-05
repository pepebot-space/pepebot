// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package providers

import "testing"

// Chat attachments are signed URLs. filepath.Ext on one of those returns
// ".pdf?ex=...&hm=..." — matching no MIME type — so every Discord attachment
// was typed application/octet-stream, and Anthropic answered
// "document.source.base64.media_type: Input should be 'application/pdf'".
func TestDetectFileTypeIgnoresQueryAndFragment(t *testing.T) {
	cases := []struct {
		url  string
		mime string
		kind FileType
	}{
		{"https://cdn.discordapp.com/attachments/1/2/laporan.pdf?ex=68a&hm=9f3a&", "application/pdf", FileTypeDocument},
		{"https://cdn.discordapp.com/attachments/1/2/foto.png?ex=68a&hm=9f3a", "image/png", FileTypeImage},
		{"https://example.com/a/b/catatan.txt#bagian-2", "text/plain", FileTypeDocument},
		{"https://example.com/laporan.pdf", "application/pdf", FileTypeDocument},
		{"/tmp/laporan.pdf", "application/pdf", FileTypeDocument},
	}

	for _, tc := range cases {
		kind, mime := DetectFileType(tc.url)
		// text/plain arrives with a charset on some platforms; compare the type.
		if len(mime) < len(tc.mime) || mime[:len(tc.mime)] != tc.mime {
			t.Errorf("DetectFileType(%q) mime = %q, want %q", tc.url, mime, tc.mime)
		}
		if kind != tc.kind {
			t.Errorf("DetectFileType(%q) kind = %v, want %v", tc.url, kind, tc.kind)
		}
	}
}
