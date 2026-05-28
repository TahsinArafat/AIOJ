package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/fps"
	"github.com/tahsinarafat/aioj/internal/store"
)

type ImportHandler struct {
	probStore store.ProblemStore
	dataDir   string
}

func NewImportHandler(probStore store.ProblemStore, dataDir string) *ImportHandler {
	return &ImportHandler{
		probStore: probStore,
		dataDir:   dataDir,
	}
}

func (h *ImportHandler) Import(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if claims.Role != "admin" && claims.Role != "teacher" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, "failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file parameter", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var xmlBytes []byte
	var probDir string
	problemID := uuid.New().String()

	filename := strings.ToLower(header.Filename)
	if strings.HasSuffix(filename, ".zip") {
		probDir = filepath.Join(h.dataDir, problemID)
		xmlBytes, err = fps.ExtractZip(fileBytes, probDir)
		if err != nil {
			http.Error(w, "failed to extract zip: "+err.Error(), http.StatusBadRequest)
			return
		}
		if len(xmlBytes) == 0 {
			os.RemoveAll(probDir)
			http.Error(w, "missing problem.xml or fps.xml in root of zip archive", http.StatusBadRequest)
			return
		}
	} else if strings.HasSuffix(filename, ".xml") {
		xmlBytes = fileBytes
	} else {
		http.Error(w, "unsupported file format; must be .xml or .zip", http.StatusBadRequest)
		return
	}

	problems, err := fps.ParseXML(xmlBytes)
	if err != nil {
		if probDir != "" {
			os.RemoveAll(probDir)
		}
		http.Error(w, "failed to parse FPS XML: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(problems) == 0 {
		if probDir != "" {
			os.RemoveAll(probDir)
		}
		http.Error(w, "xml contains no problems", http.StatusBadRequest)
		return
	}

	p := problems[0]
	p.ID = problemID
	p.Slug = strings.ToLower(strings.ReplaceAll(p.Title, " ", "-"))

	if existing, _ := h.probStore.GetBySlug(r.Context(), p.Slug); existing != nil {
		p.Slug = p.Slug + "-" + uuid.New().String()[:8]
	}

	p.TestdataPath = probDir
	p.CreatedBy = claims.UserID

	if err := h.probStore.Create(r.Context(), p); err != nil {
		if probDir != "" {
			os.RemoveAll(probDir)
		}
		http.Error(w, "failed to save problem: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "success",
		"problem_id": p.ID,
		"slug":       p.Slug,
	})
}
