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

// A channel image arrives as a signed, expiring CDN link. Passing that URL to
// the provider means the provider has to fetch it, and z.ai answers every such
// request with "图片输入格式/解析错误" — proven against the live endpoint. So
// images are inlined like every other attachment.
func TestBuildUserMessageInlinesRemoteImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("\x89PNG\r\n\x1a\nfake"))
	}))
	defer srv.Close()

	cb := &ContextBuilder{}
	msg := cb.buildUserMessage("apa ini", []string{srv.URL + "/foto.png"})

	blocks, ok := msg.Content.([]providers.ContentBlock)
	if !ok || len(blocks) != 2 {
		t.Fatalf("want 2 content blocks, got %#v", msg.Content)
	}
	if blocks[1].Type != "image_url" || blocks[1].ImageURL == nil {
		t.Fatalf("want image_url block, got %#v", blocks[1])
	}
	if !strings.HasPrefix(blocks[1].ImageURL.URL, "data:image/png;base64,") {
		t.Fatalf("image was passed through as a URL instead of inlined: %q", blocks[1].ImageURL.URL)
	}
}

func TestBuildUserMessageUnreachableImageDegrades(t *testing.T) {
	cb := &ContextBuilder{}
	msg := cb.buildUserMessage("apa ini", []string{"http://127.0.0.1:1/foto.png"})

	blocks, ok := msg.Content.([]providers.ContentBlock)
	if !ok || len(blocks) != 2 {
		t.Fatalf("want 2 content blocks, got %#v", msg.Content)
	}
	// One dead image must not 400 the entire conversation.
	if blocks[1].Type != "text" || !strings.Contains(blocks[1].Text, "could not be read") {
		t.Fatalf("want a text placeholder, got %#v", blocks[1])
	}
}
