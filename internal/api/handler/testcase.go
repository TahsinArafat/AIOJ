package handler

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/fps"
	"github.com/tahsinarafat/aioj/internal/model"
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
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "failed to open file: "+err.Error(), http.StatusBadRequest)
			return
		}

		filename := fileHeader.Filename

		if strings.HasSuffix(strings.ToLower(filename), ".zip") {
			zipBytes, err := io.ReadAll(file)
			file.Close()
			if err != nil {
				http.Error(w, "failed to read zip file: "+err.Error(), http.StatusBadRequest)
				return
			}

			_, err = fps.ExtractZip(zipBytes, probDir)
			if err != nil {
				http.Error(w, "failed to extract zip: "+err.Error(), http.StatusBadRequest)
				return
			}
		} else {
			err = h.saveFile(probDir, fileHeader)
			file.Close()
			if err != nil {
				http.Error(w, "failed to save file: "+fileHeader.Filename, http.StatusInternalServerError)
				return
			}
		}
	}

	scores := autoDiscoverTestCases(probDir)
	needsUpdate := p.TestdataPath != probDir
	if scores != nil {
		p.TestCaseScore = scores
		needsUpdate = true
	}

	if needsUpdate {
		p.TestdataPath = probDir
		if err := h.store.Update(r.Context(), p.ID, p); err != nil {
			http.Error(w, "failed to update problem", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *TestcaseHandler) saveFile(dir string, fileHeader *multipart.FileHeader) error {
	filename := fileHeader.Filename
	if strings.Contains(filename, "..") || strings.ContainsAny(filename, "/\\") {
		return fmt.Errorf("invalid filename")
	}
	baseName := filepath.Base(filename)

	src, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dstPath := filepath.Join(dir, baseName)
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(dst, src)
	dst.Close()

	if err != nil {
		os.Remove(dstPath)
	}

	return err
}

// autoDiscoverTestCases scans targetDir for paired input/output test case files
// and returns TestCaseScore entries with a default score of 10 per pair.
// Input extensions: .in, .input, .txt
// Output extensions: .out, .output, .ans, .sol
// Matching is by filename stem (prefix before extension).
func autoDiscoverTestCases(targetDir string) []model.TestCaseScore {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil
	}

	inputExts := map[string]bool{
		".in":    true,
		".input": true,
		".txt":   true,
	}
	outputExts := map[string]bool{
		".out":   true,
		".output": true,
		".ans":   true,
		".sol":   true,
	}

	inputFiles := make(map[string]string)
	outputFiles := make(map[string]string)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		stem := strings.TrimSuffix(name, filepath.Ext(name))

		if inputExts[ext] {
			inputFiles[stem] = name
		} else if outputExts[ext] {
			outputFiles[stem] = name
		}
	}

	var scores []model.TestCaseScore
	for stem, inName := range inputFiles {
		outName, ok := outputFiles[stem]
		if !ok {
			continue
		}
		scores = append(scores, model.TestCaseScore{
			InputName:  inName,
			OutputName: outName,
			Score:      10,
		})
	}

	if len(scores) == 0 {
		return nil
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].InputName < scores[j].InputName
	})

	return scores
}
