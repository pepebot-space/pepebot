package agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pepebot-space/pepebot/pkg/providers"
)

// A remote (non-image) attachment must be inlined as a base64 data URL —
// providers reject a bare http URL in file_data.
func TestBuildUserMessageInlinesRemoteFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("%PDF-1.4 hello"))
	}))
	defer srv.Close()

	cb := &ContextBuilder{}
	msg := cb.buildUserMessage("look", []string{srv.URL + "/doc.pdf"})

	blocks, ok := msg.Content.([]providers.ContentBlock)
	if !ok || len(blocks) != 2 {
		t.Fatalf("want 2 content blocks, got %#v", msg.Content)
	}
	if blocks[1].Type != "file" || blocks[1].File == nil {
		t.Fatalf("want file block, got %#v", blocks[1])
	}
	if !strings.HasPrefix(blocks[1].File.FileData, "data:application/pdf;base64,") {
		t.Fatalf("file_data not a pdf data URL: %q", blocks[1].File.FileData)
	}
}

// An unreachable attachment degrades to text instead of an empty file block
// (which the provider 400s on).
func TestBuildUserMessageUnreachableFileDegrades(t *testing.T) {
	cb := &ContextBuilder{}
	msg := cb.buildUserMessage("", []string{"http://127.0.0.1:1/nope.pdf"})

	blocks := msg.Content.([]providers.ContentBlock)
	if len(blocks) != 1 || blocks[0].Type != "text" {
		t.Fatalf("want single text block, got %#v", blocks)
	}
}
