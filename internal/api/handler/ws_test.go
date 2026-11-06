package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tahsinarafat/aioj/internal/auth"
)

func newTestWSManager(t *testing.T) (*WSManager, string) {
	t.Helper()
	jwtMgr := auth.NewJWTManager(strings.Repeat("k", 32), time.Hour, 24*time.Hour)
	token, err := jwtMgr.GenerateAccessToken("user-1", "alice", "user")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return NewWSManager(jwtMgr), token
}

func TestWS_RejectsAnonymous(t *testing.T) {
	mgr, _ := newTestWSManager(t)
	srv := httptest.NewServer(http.HandlerFunc(mgr.Handle))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	_, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err == nil {
		t.Fatal("expected the handshake to fail without a token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		got := 0
		if resp != nil {
			got = resp.StatusCode
		}
		t.Fatalf("status = %d, want 401", got)
	}
}

func TestWS_RejectsInvalidToken(t *testing.T) {
	mgr, _ := newTestWSManager(t)
	srv := httptest.NewServer(http.HandlerFunc(mgr.Handle))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "?token=not-a-jwt"
	_, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err == nil {
		t.Fatal("expected the handshake to fail with a bad token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}

func TestWS_AcceptsTokenAndRejectsCrossOrigin(t *testing.T) {
	mgr, token := newTestWSManager(t)
	srv := httptest.NewServer(http.HandlerFunc(mgr.Handle))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "?token=" + token

	t.Run("same origin connects", func(t *testing.T) {
		header := http.Header{"Origin": {srv.URL}}
		conn, _, err := websocket.DefaultDialer.Dial(url, header)
		if err != nil {
			t.Fatalf("same-origin dial failed: %v", err)
		}
		conn.Close()
	})

	t.Run("foreign origin is refused", func(t *testing.T) {
		header := http.Header{"Origin": {"https://evil.example"}}
		conn, resp, err := websocket.DefaultDialer.Dial(url, header)
		if err == nil {
			conn.Close()
			t.Fatal("cross-origin handshake must be refused")
		}
		if resp == nil || resp.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %v, want 403", resp)
		}
	})
}

func TestWS_SubscriptionCap(t *testing.T) {
	mgr, token := newTestWSManager(t)
	srv := httptest.NewServer(http.HandlerFunc(mgr.Handle))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "?token=" + token
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	for i := 0; i < maxSubscriptionsPerConn+5; i++ {
		if err := conn.WriteJSON(WSMessage{Type: "subscribe", SubmissionID: "sub-" + string(rune('a'+i))}); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	// Give the server a moment to drain the frames, then confirm the cap held.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mgr.mu.RLock()
		total := 0
		for _, conns := range mgr.subs {
			total += len(conns)
		}
		mgr.mu.RUnlock()
		if total > maxSubscriptionsPerConn {
			t.Fatalf("connection holds %d subscriptions, want at most %d", total, maxSubscriptionsPerConn)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
