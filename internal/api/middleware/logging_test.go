package middleware

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestLogging_PreservesHijack is the regression test for the WebSocket outage:
// Logging wrapped ResponseWriter without forwarding Hijack, so every /api/ws
// upgrade failed with "bad handshake" and status 500.
func TestLogging_PreservesHijack(t *testing.T) {
	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade through Logging failed: %v", err)
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("ok"))
	})

	srv := httptest.NewServer(Logging(handler))
	defer srv.Close()

	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	url := "ws" + srv.URL[len("http"):]
	conn, resp, err := dialer.Dial(url, nil)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("dial failed: %v (status %d)", err, status)
	}
	defer conn.Close()

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(msg) != "ok" {
		t.Fatalf("message = %q, want %q", msg, "ok")
	}
}

func TestRwProxy_FlushAndUnwrap(t *testing.T) {
	inner := httptest.NewRecorder()
	p := &rwProxy{ResponseWriter: inner, status: http.StatusOK}

	if _, ok := any(p).(http.Flusher); !ok {
		t.Fatal("rwProxy must implement http.Flusher")
	}
	if p.Unwrap() != http.ResponseWriter(inner) {
		t.Fatal("Unwrap should return the wrapped writer")
	}

	p.WriteHeader(http.StatusTeapot)
	p.WriteHeader(http.StatusInternalServerError) // must be ignored
	if p.status != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", p.status, http.StatusTeapot)
	}
	if inner.Code != http.StatusTeapot {
		t.Fatalf("inner status = %d, want %d", inner.Code, http.StatusTeapot)
	}
}

func TestRwProxy_HijackUnsupported(t *testing.T) {
	// httptest.ResponseRecorder is not an http.Hijacker; the wrapper must report
	// that clearly instead of panicking.
	p := &rwProxy{ResponseWriter: httptest.NewRecorder()}
	if _, _, err := p.Hijack(); err == nil {
		t.Fatal("expected an error for a non-hijackable writer")
	}
}

// TestRwProxy_HijackForwarded checks the happy path against a real server-side
// writer, which is what the failing WebSocket upgrade needed.
func TestRwProxy_HijackForwarded(t *testing.T) {
	var hijacked bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := &rwProxy{ResponseWriter: w, status: http.StatusOK}
		conn, _, err := p.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		hijacked = true
		if p.status != http.StatusSwitchingProtocols {
			t.Errorf("logged status = %d, want 101", p.status)
		}
		brw := bufio.NewWriter(conn)
		_, _ = brw.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")
		_ = brw.Flush()
		conn.Close()
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()
	if !hijacked {
		t.Fatal("handler never hijacked the connection")
	}
}
