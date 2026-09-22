package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/tahsinarafat/aioj/internal/mail"
)

func TestDevMail_List(t *testing.T) {
	c := mail.NewMailCatcher()
	_ = c.Send(context.Background(), &mail.Message{To: []string{"x@x.com"}, Subject: "hi"})

	h := &DevMailHandler{Sender: c}
	req := httptest.NewRequest("GET", "/api/dev/mail", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status: %d", rec.Code)
	}
	var body struct {
		Data []*mail.Message `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 {
		t.Errorf("got %d messages, want 1", len(body.Data))
	}
}
