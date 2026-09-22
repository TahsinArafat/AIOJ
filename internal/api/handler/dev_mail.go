package handler

import (
	"net/http"

	"github.com/tahsinarafat/aioj/internal/mail"
)

// DevMailHandler exposes the in-memory MailCatcher over HTTP.
// Mount ONLY when mail.driver == "catcher" (dev/test).
type DevMailHandler struct {
	Sender mail.Sender
}

func (h *DevMailHandler) List(w http.ResponseWriter, _ *http.Request) {
	c, ok := h.Sender.(*mail.MailCatcher)
	if !ok {
		http.Error(w, "mail catcher not enabled", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": c.All()})
}

func (h *DevMailHandler) Clear(w http.ResponseWriter, _ *http.Request) {
	c, ok := h.Sender.(*mail.MailCatcher)
	if !ok {
		http.Error(w, "mail catcher not enabled", http.StatusNotFound)
		return
	}
	c.Clear()
	w.WriteHeader(http.StatusNoContent)
}
