package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
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

// pngBytes returns a real 1x1 PNG, so content sniffing sees image/png.
func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func multipartBody(t *testing.T, field, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("create part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return &body, w.FormDataContentType()
}

// newMediaTestHandler builds a handler wired to a problem the given user owns,
// which is what the route now requires before it will accept bytes.
func newMediaTestHandler(t *testing.T, dir string) *MediaHandler {
	t.Helper()
	p := &model.Problem{ID: "p1", Slug: "x", Title: "X", CreatedBy: "author1"}
	return NewMediaHandler(dir).WithProblems(&mockProblemStore{
		getBySlug: p,
		access:    map[string]string{"author1/p1": "owner"},
	})
}

func uploadRequest(t *testing.T, field, filename string, content []byte) *http.Request {
	t.Helper()
	body, contentType := multipartBody(t, field, filename, content)
	req := httptest.NewRequest(http.MethodPost, "/api/problems/x/media", body)
	req.Header.Set("Content-Type", contentType)
	// The handler resolves {slug} and the caller's identity before reading the
	// body.
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{Keys: []string{"slug"}, Values: []string{"x"}},
	}))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey,
		&auth.Claims{UserID: "author1", Role: "setter"}))
	return req
}

func TestMediaUpload_AcceptsRealImage(t *testing.T) {
	dir := t.TempDir()
	h := newMediaTestHandler(t, dir)

	rec := httptest.NewRecorder()
	h.Upload(rec, uploadRequest(t, "image", "avatar.png", pngBytes(t)))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	url, ok := body["url"]
	if !ok || !strings.HasPrefix(url, "/media/") {
		t.Fatalf("unexpected url %q", url)
	}
	name := strings.TrimPrefix(url, "/media/")
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		t.Fatalf("file not written: %v", err)
	}
	if !storedMediaName.MatchString(name) {
		t.Fatalf("stored name %q is not a UUID filename", name)
	}
}

// TestMediaUpload_RejectsRenamedNonImage is the core fix: the old handler only
// checked the extension, so any bytes could be stored as .png.
func TestMediaUpload_RejectsRenamedNonImage(t *testing.T) {
	dir := t.TempDir()
	h := newMediaTestHandler(t, dir)

	payload := []byte("<!DOCTYPE html><script>alert(document.cookie)</script>")
	rec := httptest.NewRecorder()
	h.Upload(rec, uploadRequest(t, "image", "payload.png", payload))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("a rejected upload must not leave a file behind: %v", entries)
	}
}

func TestMediaUpload_RejectsOversizedBody(t *testing.T) {
	dir := t.TempDir()
	h := newMediaTestHandler(t, dir)

	// A valid PNG header followed by filler past the limit.
	huge := append(pngBytes(t), bytes.Repeat([]byte{0}, maxMediaUploadBytes+1024)...)
	rec := httptest.NewRecorder()
	h.Upload(rec, uploadRequest(t, "image", "huge.png", huge))

	if rec.Code != http.StatusRequestEntityTooLarge && rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 413 (or 400 from the parser)", rec.Code)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("an oversized upload must not leave a file behind: %v", entries)
	}
}

func TestMediaUpload_RequiresImageField(t *testing.T) {
	h := newMediaTestHandler(t, t.TempDir())
	rec := httptest.NewRecorder()
	h.Upload(rec, uploadRequest(t, "avatar", "a.png", pngBytes(t)))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestMediaServe_ServesUploadedFile(t *testing.T) {
	dir := t.TempDir()
	h := newMediaTestHandler(t, dir)

	rec := httptest.NewRecorder()
	h.Upload(rec, uploadRequest(t, "image", "a.png", pngBytes(t)))
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)

	req := httptest.NewRequest(http.MethodGet, body["url"], nil)
	out := httptest.NewRecorder()
	h.Serve(out, req)

	if out.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", out.Code)
	}
	if out.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("nosniff header missing")
	}
	if out.Body.Len() == 0 {
		t.Error("empty body")
	}
}

// TestMediaServe_NoDirectoryListing: http.FileServer rendered an index of the
// media directory.
func TestMediaServe_NoDirectoryListing(t *testing.T) {
	dir := t.TempDir()
	h := NewMediaHandler(dir)
	// Put a file there that was not created by an upload.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	for _, target := range []string{"/media/", "/media", "/media/notes.txt", "/media/../go.mod"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		h.Serve(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", target, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "secret") {
			t.Errorf("%s: leaked file content", target)
		}
	}
}

// The upload route sat inside the setter-workspace group, but the handler never
// read the {slug} it was mounted under and never checked access, so any signed-in
// user could write into the global media directory. Nothing ever removes those
// files, so they accumulated forever.
func TestMediaUpload_RequiresAuthoringRights(t *testing.T) {
	dir := t.TempDir()
	p := &model.Problem{ID: "p1", Slug: "x", Title: "X", CreatedBy: "author1"}
	// Someone else's problem: no access entry for the caller.
	h := NewMediaHandler(dir).WithProblems(&mockProblemStore{
		getBySlug: p,
		access:    map[string]string{"someone-else/p1": "owner"},
	})

	rec := httptest.NewRecorder()
	h.Upload(rec, uploadRequest(t, "image", "a.png", pngBytes(t)))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for a non-author", rec.Code)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("a refused upload wrote files: %v", entries)
	}
}

func TestMediaUpload_RejectsUnknownProblem(t *testing.T) {
	dir := t.TempDir()
	// mockProblemStore returns a nil problem when getBySlug is unset.
	h := NewMediaHandler(dir).WithProblems(&mockProblemStore{})

	rec := httptest.NewRecorder()
	h.Upload(rec, uploadRequest(t, "image", "a.png", pngBytes(t)))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// A handler without a store cannot authorize, so it must refuse rather than
// silently accepting every upload the way the old handler did.
func TestMediaUpload_RefusesWithoutProblemStore(t *testing.T) {
	dir := t.TempDir()
	h := NewMediaHandler(dir)

	rec := httptest.NewRecorder()
	h.Upload(rec, uploadRequest(t, "image", "a.png", pngBytes(t)))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("an unauthorized handler wrote files: %v", entries)
	}
}

// An admin may upload to any problem.
func TestMediaUpload_AdminBypassesOwnership(t *testing.T) {
	dir := t.TempDir()
	p := &model.Problem{ID: "p1", Slug: "x", Title: "X", CreatedBy: "author1"}
	h := NewMediaHandler(dir).WithProblems(&mockProblemStore{getBySlug: p, access: map[string]string{}})

	req := uploadRequest(t, "image", "a.png", pngBytes(t))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey,
		&auth.Claims{UserID: "admin1", Role: "admin"}))
	rec := httptest.NewRecorder()
	h.Upload(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
}
