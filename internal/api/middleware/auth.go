package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/tahsinarafat/aioj/internal/auth"
)

type contextKey string

const UserContextKey contextKey = "user"

func AuthMiddleware(jwtManager *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			token := ""
			if strings.HasPrefix(header, "Bearer ") {
				token = strings.TrimPrefix(header, "Bearer ")
			} else {
				// Allow token in query parameter for file downloads
				token = r.URL.Query().Get("token")
			}

			if token == "" {
				http.Error(w, "missing auth token", http.StatusUnauthorized)
				return
			}
			claims, err := jwtManager.ValidateToken(token)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OptionalAuthMiddleware(jwtManager *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if strings.HasPrefix(header, "Bearer ") {
				claims, err := jwtManager.ValidateToken(strings.TrimPrefix(header, "Bearer "))
				if err == nil {
					ctx := context.WithValue(r.Context(), UserContextKey, claims)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func GetUserClaims(r *http.Request) *auth.Claims {
	c, _ := r.Context().Value(UserContextKey).(*auth.Claims)
	return c
}
