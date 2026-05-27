package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

type WSMessage struct {
	Type         string      `json:"type"`
	SubmissionID string      `json:"submission_id,omitempty"`
	Data         interface{} `json:"data,omitempty"`
}

type WSManager struct {
	mu   sync.RWMutex
	subs map[string]map[*websocket.Conn]bool
}

func NewWSManager() *WSManager {
	return &WSManager{subs: make(map[string]map[*websocket.Conn]bool)}
}

func (m *WSManager) Handle(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws upgrade", "error", err)
		return
	}
	defer func() {
		m.mu.Lock()
		for _, conns := range m.subs {
			delete(conns, conn)
		}
		m.mu.Unlock()
		conn.Close()
	}()
	for {
		var msg WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}
		if msg.Type == "subscribe" && msg.SubmissionID != "" {
			m.mu.Lock()
			if m.subs[msg.SubmissionID] == nil {
				m.subs[msg.SubmissionID] = make(map[*websocket.Conn]bool)
			}
			m.subs[msg.SubmissionID][conn] = true
			m.mu.Unlock()
		}
	}
}

func (m *WSManager) NotifySubmission(id string, msg WSMessage) {
	m.mu.RLock()
	conns := m.subs[id]
	m.mu.RUnlock()
	if len(conns) == 0 {
		return
	}
	data, _ := json.Marshal(msg)
	for c := range conns {
		c.WriteMessage(websocket.TextMessage, data)
	}
}
