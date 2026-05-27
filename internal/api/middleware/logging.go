package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

type rwProxy struct {
	http.ResponseWriter
	status int
}

func (r *rwProxy) WriteHeader(c int) {
	r.status = c
	r.ResponseWriter.WriteHeader(c)
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		p := &rwProxy{ResponseWriter: w, status: 200}
		next.ServeHTTP(p, r)
		slog.Info("req", "method", r.Method, "path", r.URL.Path, "status", p.status, "dur", time.Since(start))
	})
}
