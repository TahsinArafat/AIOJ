package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

func newTestcaseRequest(slug, name, method, body string) *http.Request {
	target := "/api/problems/" + slug + "/testcases"
	if name != "" {
		target += "/" + name
	}
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", slug)
	if name != "" {
		rctx.URLParams.Add("name", name)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, middleware.UserContextKey, &auth.Claims{UserID: "u1", Role: "admin"})
	return req.WithContext(ctx)
}

func newTestcaseHandler(t *testing.T) (*TestcaseHandler, string) {
	t.Helper()
	dataDir := t.TempDir()
	prob := &model.Problem{ID: "p1", Slug: "two-sum", Title: "Two Sum"}
	return NewTestcaseHandler(&mockProblemStore{getBySlug: prob}, dataDir), dataDir
}

func TestTestcasePathRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"", "../secret.in", "nested/01.in", "nested\\01.in", ".hidden", strings.Repeat("a", 129)} {
		if _, err := testcasePath(dir, name); err == nil {
			t.Fatalf("testcasePath(%q) accepted an unsafe name", name)
		}
	}
	path, err := testcasePath(dir, "01.in")
	if err != nil {
		t.Fatalf("testcasePath rejected a valid name: %v", err)
	}
	if path != filepath.Join(dir, "01.in") {
		t.Fatalf("testcasePath = %q, want %q", path, filepath.Join(dir, "01.in"))
	}
}

func TestListTestcasesPairsFiles(t *testing.T) {
	h, dataDir := newTestcaseHandler(t)
	probDir := filepath.Join(dataDir, "p1")
	if err := os.MkdirAll(probDir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"01.in":         "1 2\n",
		"01.out":        "3\n",
		"notes.md":      "unpaired\n",
		".testcase-tmp": "leftover from a crashed save",
	} {
		if err := os.WriteFile(filepath.Join(probDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	rec := httptest.NewRecorder()
	h.List(rec, newTestcaseRequest("two-sum", "", http.MethodGet, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Files []testcaseFile `json:"files"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Files) != 3 {
		t.Fatalf("got %d files, want 3", len(resp.Files))
	}
	byName := make(map[string]testcaseFile, len(resp.Files))
	for _, f := range resp.Files {
		byName[f.Name] = f
	}
	if got := byName["01.in"]; got.Role != "input" || got.Pair != "01.out" {
		t.Fatalf("01.in = %+v, want input paired with 01.out", got)
	}
	if got := byName["01.out"]; got.Role != "output" || got.Pair != "01.in" {
		t.Fatalf("01.out = %+v, want output paired with 01.in", got)
	}
	if got := byName["notes.md"]; got.Role != "other" || got.Pair != "" {
		t.Fatalf("notes.md = %+v, want unpaired other", got)
	}
}

func TestListTestcasesEmptyDirectory(t *testing.T) {
	h, _ := newTestcaseHandler(t)
	rec := httptest.NewRecorder()
	h.List(rec, newTestcaseRequest("two-sum", "", http.MethodGet, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for a problem with no uploads yet", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"files":[]`) {
		t.Fatalf("body = %s, want an empty files array", rec.Body.String())
	}
}

func TestSaveAndGetTestcase(t *testing.T) {
	h, _ := newTestcaseHandler(t)

	rec := httptest.NewRecorder()
	h.Save(rec, newTestcaseRequest("two-sum", "01.in", http.MethodPut, `{"content":"4 5\n"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("save status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.Get(rec, newTestcaseRequest("two-sum", "01.in", http.MethodGet, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var content testcaseContent
	if err := json.NewDecoder(rec.Body).Decode(&content); err != nil {
		t.Fatal(err)
	}
	if content.Content != "4 5\n" || content.Binary {
		t.Fatalf("content = %+v, want the saved text", content)
	}
}

func TestGetTestcaseDownloadServesRawFile(t *testing.T) {
	h, dataDir := newTestcaseHandler(t)
	probDir := filepath.Join(dataDir, "p1")
	if err := os.MkdirAll(probDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(probDir, "01.ans"), []byte("42\n"), 0644); err != nil {
		t.Fatal(err)
	}

	req := newTestcaseRequest("two-sum", "01.ans", http.MethodGet, "")
	req.URL.RawQuery = "download=1"
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "42\n" {
		t.Fatalf("body = %q, want the raw file", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "attachment") || !strings.Contains(got, "01.ans") {
		t.Fatalf("Content-Disposition = %q, want an attachment named 01.ans", got)
	}
}

func TestGetTestcaseFlagsBinary(t *testing.T) {
	h, dataDir := newTestcaseHandler(t)
	probDir := filepath.Join(dataDir, "p1")
	if err := os.MkdirAll(probDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(probDir, "01.out"), []byte{0xff, 0xfe, 0x00}, 0644); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.Get(rec, newTestcaseRequest("two-sum", "01.out", http.MethodGet, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var content testcaseContent
	if err := json.NewDecoder(rec.Body).Decode(&content); err != nil {
		t.Fatal(err)
	}
	if !content.Binary {
		t.Fatalf("binary = false for non-UTF-8 bytes")
	}
}

func TestDeleteTestcase(t *testing.T) {
	h, dataDir := newTestcaseHandler(t)
	probDir := filepath.Join(dataDir, "p1")
	if err := os.MkdirAll(probDir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(probDir, "01.in")
	if err := os.WriteFile(path, []byte("1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.Delete(rec, newTestcaseRequest("two-sum", "01.in", http.MethodDelete, ""))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still exists after delete")
	}

	rec = httptest.NewRecorder()
	h.Delete(rec, newTestcaseRequest("two-sum", "01.in", http.MethodDelete, ""))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("second delete status = %d, want 404", rec.Code)
	}
}

func TestTestcaseHandlersRejectTraversal(t *testing.T) {
	h, _ := newTestcaseHandler(t)
	rec := httptest.NewRecorder()
	h.Save(rec, newTestcaseRequest("two-sum", "../escape.in", http.MethodPut, `{"content":"x"}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a traversal attempt", rec.Code)
	}
}

func TestTestcaseHandlersRequireAuthor(t *testing.T) {
	dataDir := t.TempDir()
	h := NewTestcaseHandler(
		&mockProblemStore{getBySlug: &model.Problem{ID: "p1", Slug: "two-sum"}},
		dataDir,
	)
	req := newTestcaseRequest("two-sum", "", http.MethodGet, "")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, &auth.Claims{UserID: "u2", Role: "user"}))
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for a non-author", rec.Code)
	}
}
