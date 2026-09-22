package oauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubProvider_FetchUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			json.NewEncoder(w).Encode(map[string]any{
				"id": 12345, "login": "octocat", "name": "Octo Cat",
				"avatar_url": "https://example.com/a.png",
			})
		case "/user/emails":
			json.NewEncoder(w).Encode([]map[string]any{
				{"email": "octo@cat.com", "primary": true, "verified": true},
			})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	p := NewGitHubProvider(GitHubConfig{
		UserURL: srv.URL + "/user", EmailsURL: srv.URL + "/user/emails",
	})
	info, err := p.FetchUser(nil, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if info.Username != "octocat" {
		t.Errorf("got username %q, want octocat", info.Username)
	}
	if info.Email != "octo@cat.com" {
		t.Errorf("got email %q, want octo@cat.com", info.Email)
	}
	if info.ProviderUserID != "12345" {
		t.Errorf("got provider id %q, want 12345", info.ProviderUserID)
	}
}
