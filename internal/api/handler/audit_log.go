package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// AuditLogHandler serves GET /api/admin/audit-log.
type AuditLogHandler struct {
	Audit store.AuditStore
}

func NewAuditLogHandler(a store.AuditStore) *AuditLogHandler {
	return &AuditLogHandler{Audit: a}
}

func (h *AuditLogHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	items, total, err := h.Audit.List(r.Context(), offset, limit)
	if err != nil {
		slog.Error("list audit log", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": items, "total": total})
}

// Record is a helper used by middleware; exported for handlers that want custom actions.
func (h *AuditLogHandler) Record(e *model.AuditEntry) {
	if e == nil || h.Audit == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.Audit.Insert(ctx, e); err != nil {
			slog.Warn("audit insert failed", "action", e.Action, "error", err)
		}
	}()
}
