package handler

import (
	"bytes"
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

func uploadRequest(t *testing.T, field, filename string, content []byte) *http.Request {
	t.Helper()
	body, contentType := multipartBody(t, field, filename, content)
	req := httptest.NewRequest(http.MethodPost, "/api/problems/x/media", body)
	req.Header.Set("Content-Type", contentType)
	return req
}

func TestMediaUpload_AcceptsRealImage(t *testing.T) {
	dir := t.TempDir()
	h := NewMediaHandler(dir)

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
	h := NewMediaHandler(dir)

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
	h := NewMediaHandler(dir)

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
	h := NewMediaHandler(t.TempDir())
	rec := httptest.NewRecorder()
	h.Upload(rec, uploadRequest(t, "avatar", "a.png", pngBytes(t)))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestMediaServe_ServesUploadedFile(t *testing.T) {
	dir := t.TempDir()
	h := NewMediaHandler(dir)

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
