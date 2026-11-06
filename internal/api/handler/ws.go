package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tahsinarafat/aioj/internal/auth"
)

// maxSubscriptionsPerConn bounds the map a single socket can grow. Without it a
// client could subscribe to unbounded submission IDs and pin memory forever.
const maxSubscriptionsPerConn = 32

// upgrader only accepts same-origin browser handshakes. The previous
// CheckOrigin always returned true, which let any site open an authenticated
// socket on a visitor's behalf (cross-site WebSocket hijacking).
var upgrader = websocket.Upgrader{
	CheckOrigin:      sameOrigin,
	HandshakeTimeout: 10 * time.Second,
}

// sameOrigin reports whether the handshake Origin matches the request Host.
// Non-browser clients (no Origin header) are allowed and still need a token.
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

type WSMessage struct {
	Type         string      `json:"type"`
	SubmissionID string      `json:"submission_id,omitempty"`
	Data         interface{} `json:"data,omitempty"`
}

type WSManager struct {
	jwt  *auth.JWTManager
	mu   sync.RWMutex
	subs map[string]map[*websocket.Conn]bool
}

func NewWSManager(jwt *auth.JWTManager) *WSManager {
	return &WSManager{jwt: jwt, subs: make(map[string]map[*websocket.Conn]bool)}
}

// authorize validates the access token supplied as a query parameter (browsers
// cannot set headers on a WebSocket) or as a Bearer header.
func (m *WSManager) authorize(r *http.Request) *auth.Claims {
	if m.jwt == nil {
		return nil
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			token = strings.TrimPrefix(h, "Bearer ")
		}
	}
	if token == "" {
		return nil
	}
	claims, err := m.jwt.ValidateToken(token)
	if err != nil {
		return nil
	}
	return claims
}

func (m *WSManager) Handle(w http.ResponseWriter, r *http.Request) {
	// Authentication happens before the upgrade so an anonymous client gets a
	// clean 401 instead of an open socket.
	if m.authorize(r) == nil {
		http.Error(w, "missing or invalid auth token", http.StatusUnauthorized)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("ws upgrade rejected", "error", err)
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

	count := 0
	for {
		var msg WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}
		if msg.Type != "subscribe" || msg.SubmissionID == "" {
			continue
		}
		m.mu.Lock()
		if _, already := m.subs[msg.SubmissionID][conn]; !already {
			if count >= maxSubscriptionsPerConn {
				m.mu.Unlock()
				continue
			}
			if m.subs[msg.SubmissionID] == nil {
				m.subs[msg.SubmissionID] = make(map[*websocket.Conn]bool)
			}
			m.subs[msg.SubmissionID][conn] = true
			count++
		}
		m.mu.Unlock()
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
