package handler

import (
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/store"
)

type TestcaseHandler struct {
	store   store.ProblemStore
	dataDir string
}

func NewTestcaseHandler(s store.ProblemStore, dataDir string) *TestcaseHandler {
	return &TestcaseHandler{store: s, dataDir: dataDir}
}

func (h *TestcaseHandler) Upload(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	slug := chi.URLParam(r, "slug")
	p, err := h.store.GetBySlug(r.Context(), slug)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), p.ID, claims.UserID, "owner", "co_author") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB max
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		http.Error(w, "no files provided", http.StatusBadRequest)
		return
	}

	probDir := filepath.Join(h.dataDir, p.ID)
	if err := os.MkdirAll(probDir, 0755); err != nil {
		http.Error(w, "failed to create directory", http.StatusInternalServerError)
		return
	}

	for _, fileHeader := range files {
		if err := h.saveFile(probDir, fileHeader); err != nil {
			http.Error(w, "failed to save file: "+fileHeader.Filename, http.StatusInternalServerError)
			return
		}
	}

	if p.TestdataPath != probDir {
		p.TestdataPath = probDir
		if err := h.store.Update(r.Context(), p.ID, p); err != nil {
			http.Error(w, "failed to update problem", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *TestcaseHandler) saveFile(dir string, fileHeader *multipart.FileHeader) error {
	src, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dstPath := filepath.Join(dir, fileHeader.Filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}
