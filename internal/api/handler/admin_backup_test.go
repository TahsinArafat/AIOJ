package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/config"
)

func TestAdminBackupHandler_List(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		"backup_db_20250101_120000.sql",
		"backup_files_20250101_120000.tar.gz",
		"backup_full_20250101_120000.tar.gz",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	handler := NewAdminBackupHandler(nil, config.DatabaseConfig{}, dir)

	req := httptest.NewRequest("GET", "/admin/backups", nil)
	rec := httptest.NewRecorder()

	handler.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Data []BackupMeta `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 backups, got %d", len(resp.Data))
	}

	names := make(map[string]bool)
	for _, b := range resp.Data {
		names[b.Filename] = true
	}
	for _, f := range files {
		if !names[f] {
			t.Errorf("expected backup %q in response, not found", f)
		}
	}
}

func TestAdminBackupHandler_Delete(t *testing.T) {
	dir := t.TempDir()

	filename := "backup_db_20250101_120000.sql"
	filePath := filepath.Join(dir, filename)
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := NewAdminBackupHandler(nil, config.DatabaseConfig{}, dir)

	req := httptest.NewRequest("DELETE", "/admin/backups/{filename}", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("filename", filename)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	rec := httptest.NewRecorder()

	handler.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("expected file %s to be deleted, but it still exists", filePath)
	}
}
