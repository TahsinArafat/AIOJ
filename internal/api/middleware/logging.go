package middleware

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// rwProxy records the response status for logging.
//
// It must forward every optional interface the real ResponseWriter implements.
// Wrapping without Hijack support silently broke WebSocket upgrades: gorilla's
// Upgrader needs http.Hijacker and returns 500 when the wrapper does not
// provide it.
type rwProxy struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *rwProxy) WriteHeader(c int) {
	if r.wroteHeader {
		// httptest and net/http both warn on duplicate WriteHeader calls;
		// forward only the first one.
		return
	}
	r.status = c
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(c)
}

func (r *rwProxy) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.wroteHeader = true
	}
	return r.ResponseWriter.Write(b)
}

// Hijack lets WebSocket and other protocol upgrades take over the connection.
func (r *rwProxy) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("logging: underlying ResponseWriter does not support hijacking")
	}
	// The upgrade writes its own status line, so record the honest code.
	r.status = http.StatusSwitchingProtocols
	r.wroteHeader = true
	return h.Hijack()
}

// Flush forwards streaming flushes (SSE, chunked responses).
func (r *rwProxy) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap exposes the original writer to http.ResponseController.
func (r *rwProxy) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		p := &rwProxy{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(p, r)
		slog.Info("req", "method", r.Method, "path", r.URL.Path, "status", p.status, "dur", time.Since(start))
	})
}
