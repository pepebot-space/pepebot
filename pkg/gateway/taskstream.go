package gateway

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pepebot-space/pepebot/pkg/logger"
	"github.com/pepebot-space/pepebot/pkg/task"
)

// TaskEvent represents a real-time task event pushed to WebSocket clients.
type TaskEvent struct {
	Type string     `json:"type"` // task.created, task.moved, task.completed, task.failed, task.approval_needed
	Task *task.Task `json:"task"`
	Time string     `json:"time"`
}

// TaskStreamHub manages WebSocket connections for task events.
type TaskStreamHub struct {
	clients map[*websocket.Conn]bool
	mu      sync.RWMutex
}

// NewTaskStreamHub creates a new hub.
func NewTaskStreamHub() *TaskStreamHub {
	return &TaskStreamHub{
		clients: make(map[*websocket.Conn]bool),
	}
}

var taskUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// HandleWebSocket handles new WebSocket connections for /v1/tasks/stream.
func (h *TaskStreamHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := taskUpgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.WarnCF("taskstream", "WebSocket upgrade failed", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()

	logger.DebugC("taskstream", "Client connected")

	// Keep connection alive — read messages (client can send ping/close)
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, conn)
			h.mu.Unlock()
			conn.Close()
			logger.DebugC("taskstream", "Client disconnected")
		}()

		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	// Ping ticker to keep connection alive
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			h.mu.RLock()
			_, exists := h.clients[conn]
			h.mu.RUnlock()
			if !exists {
				return
			}
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				return
			}
		}
	}()
}

// Broadcast sends a task event to all connected clients.
func (h *TaskStreamHub) Broadcast(event TaskEvent) {
	if h == nil {
		return
	}

	event.Time = time.Now().Format(time.RFC3339)

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	h.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, conn := range clients {
		err := conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			h.mu.Lock()
			delete(h.clients, conn)
			h.mu.Unlock()
			conn.Close()
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *TaskStreamHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
