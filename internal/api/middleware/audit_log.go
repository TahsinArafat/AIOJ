package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// AuditRecorder inserts admin audit entries. Nil-safe.
type AuditRecorder struct {
	Store store.AuditStore
}

// Audit logs state-changing admin requests (POST/PUT/PATCH/DELETE under /api/admin).
// GET is ignored. Bodies are not stored (PII / size); only method, path, actor.
func (a *AuditRecorder) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a == nil || a.Store == nil || !isAuditable(r) {
			next.ServeHTTP(w, r)
			return
		}
		// Drain body for handlers that re-read it? We do NOT consume the body —
		// action is derived from method+path only.
		next.ServeHTTP(w, r)

		claims := GetUserClaims(r)
		entry := &model.AuditEntry{
			Action:     r.Method + " " + r.URL.Path,
			TargetType: "http",
			TargetID:   r.URL.Path,
			IP:         clientIP(r),
			Detail:     json.RawMessage(`{}`),
		}
		if claims != nil {
			entry.ActorID = claims.UserID
			entry.ActorName = claims.Username
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := a.Store.Insert(ctx, entry); err != nil {
				slog.Warn("audit insert failed", "action", entry.Action, "error", err)
			}
		}()
	})
}

func isAuditable(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return false
	}
	return strings.HasPrefix(r.URL.Path, "/api/admin/")
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		return host[:i]
	}
	return host
}
