package oauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGoogleProvider_FetchUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"sub": "g-12345", "name": "Alice",
			"email": "alice@x.com", "email_verified": true,
			"picture": "https://example.com/p.png",
		})
	}))
	defer srv.Close()

	p := NewGoogleProvider(GoogleConfig{UserInfoURL: srv.URL})
	info, err := p.FetchUser(nil, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if info.ProviderUserID != "g-12345" {
		t.Errorf("got %q, want g-12345", info.ProviderUserID)
	}
	if info.Email != "alice@x.com" {
		t.Errorf("got %q, want alice@x.com", info.Email)
	}
}
