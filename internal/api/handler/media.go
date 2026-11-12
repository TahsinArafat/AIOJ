package handler

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// Upload limits. The previous handler validated only the filename extension,
// which says nothing about the bytes: a .png could hold an HTML page or a
// 4 GB blob.
const (
	// maxMediaUploadBytes is the hard ceiling for one image.
	maxMediaUploadBytes = 5 << 20 // 5 MB
	// maxMediaFormMemory is how much of the multipart body is buffered in RAM
	// before spilling to a temp file.
	maxMediaFormMemory = 1 << 20 // 1 MB
)

// allowedImageTypes maps the sniffed content type to the extension we store.
// http.DetectContentType is authoritative here: it inspects the magic bytes, so
// a renamed file is rejected.
var allowedImageTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
}

// storedMediaName matches the only filenames this handler ever writes. The
// static file handler refuses anything else, which keeps it from ever serving a
// path that did not come from an upload.
var storedMediaName = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(png|jpg|gif)$`)

type MediaHandler struct {
	dir string
	// problems resolves the {slug} the upload route carries, so the write can be
	// authorized against the problem it is for. Optional: the static serve path
	// and the handler tests do not need it.
	problems store.ProblemStore
}

func NewMediaHandler(dir string) *MediaHandler {
	os.MkdirAll(dir, 0o755)
	return &MediaHandler{dir: dir}
}

// WithProblems attaches the problem store used to authorize an upload against
// its problem. Returns the handler for call-site chaining.
func (h *MediaHandler) WithProblems(p store.ProblemStore) *MediaHandler {
	h.problems = p
	return h
}

// authorizedForProblem resolves {slug} and requires the caller to hold authoring
// rights on it, the same rule every other write in this route group uses.
//
// Without this the upload route was authenticated but not authorized: it sat
// inside the setter-workspace group but its handler never read the slug, so any
// signed-in user could write into the global media directory — and nothing ever
// removes those files. Returns the problem, or nil after writing the error.
func (h *MediaHandler) authorizedForProblem(w http.ResponseWriter, r *http.Request) *model.Problem {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil
	}
	// A handler built without a store cannot authorize anything, so it refuses
	// rather than falling open.
	if h.problems == nil {
		http.Error(w, "media upload is not configured", http.StatusServiceUnavailable)
		return nil
	}
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		http.Error(w, "problem slug is required", http.StatusBadRequest)
		return nil
	}
	p, err := h.problems.GetBySlug(r.Context(), slug)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return nil
	}
	if claims.Role != "admin" && !h.problems.HasAccess(r.Context(), p.ID, claims.UserID, "owner", "co_author") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return nil
	}
	return p
}

func (h *MediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
	// Authorize before touching the body: parsing a 5 MB multipart form for a
	// caller who may not upload here is work an anonymous request should not be
	// able to ask for.
	if h.authorizedForProblem(w, r) == nil {
		return
	}
	// Bound the body before parsing: ParseMultipartForm's argument is only a
	// memory hint, not a limit, so the request itself was unbounded.
	r.Body = http.MaxBytesReader(w, r.Body, maxMediaUploadBytes+maxMediaFormMemory)
	if err := r.ParseMultipartForm(maxMediaFormMemory); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "image too large (max 5 MB)", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	file, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing 'image' field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Sniff the first 512 bytes — the amount DetectContentType reads — and then
	// copy them back so the stored file is complete.
	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		http.Error(w, "failed to read upload", http.StatusBadRequest)
		return
	}
	head = head[:n]

	contentType := http.DetectContentType(head)
	ext, ok := allowedImageTypes[contentType]
	if !ok {
		http.Error(w, "invalid file type; allowed: png, jpg, gif", http.StatusBadRequest)
		return
	}

	filename := uuid.New().String() + ext
	destPath := filepath.Join(h.dir, filename)

	dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}

	written, err := io.Copy(dst, io.MultiReader(bytes.NewReader(head), io.LimitReader(file, maxMediaUploadBytes)))
	closeErr := dst.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(destPath)
		slog.Error("media upload write failed", "file", filename, "error", err)
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}
	if written > maxMediaUploadBytes {
		_ = os.Remove(destPath)
		http.Error(w, "image too large (max 5 MB)", http.StatusRequestEntityTooLarge)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"url": "/media/" + filename,
	})
}

// Serve streams a stored upload. It replaces http.FileServer, which listed the
// directory for /media/ and would happily serve any file that ever landed
// there.
func (h *MediaHandler) Serve(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/media/")
	if !storedMediaName.MatchString(name) {
		http.NotFound(w, r)
		return
	}

	f, err := os.Open(filepath.Join(h.dir, name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	// Immutable, content-addressed-by-UUID filenames: cache hard, and never let
	// the browser second-guess the type (the extension came from sniffing).
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	http.ServeContent(w, r, name, info.ModTime(), f)
}
