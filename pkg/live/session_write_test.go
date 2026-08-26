// Pepebot - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package live

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

// The client connection has more than one writer: the upstream pump forwards frames to
// it while a client tool call is written from its own goroutine. gorilla panics with
// "concurrent write to websocket connection" rather than corrupting the stream, which
// took down the whole gateway in testing — systemd restarted it mid-session.
//
// Every write after setup must go through writeClient. Run with -race.
func TestSessionWriteClientIsSerialized(t *testing.T) {
	upgrader := websocket.Upgrader{}
	served := make(chan *websocket.Conn, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		served <- conn
		for { // drain, so writes are not blocked by a full buffer
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()
	defer func() { (<-served).Close() }()

	session := &LiveSession{clientConn: client}

	// Mirrors the real shape: the forwarding pump and several tool calls at once.
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 25; j++ {
				if err := session.writeClient(websocket.TextMessage, []byte(`{"type":"tool_call"}`)); err != nil {
					return // the socket closed under us; nothing to assert
				}
			}
		}(i)
	}
	wg.Wait()
}
