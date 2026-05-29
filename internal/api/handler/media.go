package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type MediaHandler struct {
	dir string
}

func NewMediaHandler(dir string) *MediaHandler {
	os.MkdirAll(dir, 0o755)
	return &MediaHandler{dir: dir}
}

func (h *MediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "file too large or invalid form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing 'image' field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif":
	default:
		http.Error(w, "invalid file type; allowed: .png, .jpg, .jpeg, .gif", http.StatusBadRequest)
		return
	}

	filename := uuid.New().String() + ext
	destPath := filepath.Join(h.dir, filename)

	dst, err := os.Create(destPath)
	if err != nil {
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := dst.ReadFrom(file); err != nil {
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"url": "/media/" + filename,
	})
}
