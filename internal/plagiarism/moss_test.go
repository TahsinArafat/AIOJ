package plagiarism

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMoss_NotConfigured(t *testing.T) {
	c := &MossClient{}
	if c.Enabled() {
		t.Fatal("empty client should be disabled")
	}
	_, err := c.Submit(context.Background(), []MossFile{{Name: "a.cpp", Code: "int main(){}"}})
	if err != ErrNotConfigured {
		t.Fatalf("err = %v", err)
	}
}

func TestMoss_SubmitOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req mossRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		if req.User != "12345" || len(req.Files) != 1 {
			w.WriteHeader(400)
			return
		}
		_ = json.NewEncoder(w).Encode(mossResponse{URL: "https://moss.example/report/1"})
	}))
	defer srv.Close()

	c := &MossClient{User: "12345", Lang: "cpp", Base: srv.URL, HTTP: srv.Client()}
	url, err := c.Submit(context.Background(), []MossFile{{Name: "a.cpp", Code: "int main(){}"}})
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://moss.example/report/1" {
		t.Fatalf("url = %q", url)
	}
}
