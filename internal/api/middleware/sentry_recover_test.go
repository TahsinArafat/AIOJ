package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSentryRecover_RethrowsPanic(t *testing.T) {
	h := SentryRecover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	defer func() {
		if rec := recover(); rec == nil {
			t.Error("expected panic to rethrow")
		}
	}()
	h.ServeHTTP(rec, req)
}
