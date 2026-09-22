package middleware

import "net/http"

// SecurityHeaders sets baseline hardening headers on every response.
// CSP is intentionally permissive for first-party assets; tighten per
// deployment once asset origins are known.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Frame-Options", "DENY")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			h.Set("Content-Security-Policy",
				"default-src 'self'; "+
					"img-src 'self' data: https:; "+
					"style-src 'self' 'unsafe-inline'; "+
					"script-src 'self' blob:; "+
					"worker-src 'self' blob:; "+
					"connect-src 'self' wss: https:; "+
					"frame-ancestors 'none';")
			next.ServeHTTP(w, r)
		})
	}
}
