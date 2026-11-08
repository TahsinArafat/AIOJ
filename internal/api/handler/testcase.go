package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/fps"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

const (
	maxTestcaseUpload = 100 << 20 // ZIP/bulk upload cap
	// A textarea is the editor, so viewing and inline editing stop well below
	// the size where a browser tab would choke. Larger files still download.
	maxTestcaseContent = 8 << 20
)

var (
	testcaseInputExts = map[string]bool{
		".in":    true,
		".input": true,
		".txt":   true,
	}
	testcaseOutputExts = map[string]bool{
		".out":    true,
		".output": true,
		".ans":    true,
		".sol":    true,
	}
)

type TestcaseHandler struct {
	store   store.ProblemStore
	dataDir string
}

func NewTestcaseHandler(s store.ProblemStore, dataDir string) *TestcaseHandler {
	return &TestcaseHandler{store: s, dataDir: dataDir}
}

// testcaseFile is one file in a problem's testdata directory.
type testcaseFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	// Role is "input", "output" or "other", derived from the extension.
	Role string `json:"role"`
	// Pair names the counterpart file sharing the stem, when one exists.
	Pair string `json:"pair,omitempty"`
}

type testcaseContent struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Content string `json:"content"`
	// Binary is true when the bytes are not valid UTF-8; such files are
	// download-only because the editor cannot represent them.
	Binary bool `json:"binary"`
}

// authorProblem resolves {slug} and checks the caller may author its data.
// It writes the error response itself and returns nil on failure.
func (h *TestcaseHandler) authorProblem(w http.ResponseWriter, r *http.Request) *model.Problem {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil
	}

	p, err := h.store.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return nil
	}
	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), p.ID, claims.UserID, "owner", "co_author") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return nil
	}
	return p
}

// testdataDir is where a problem's input/output files live. Problems that never
// received an upload have no TestdataPath yet, so fall back to the per-problem
// directory an upload would create.
func (h *TestcaseHandler) testdataDir(p *model.Problem) string {
	if p.TestdataPath != "" {
		return p.TestdataPath
	}
	return filepath.Join(h.dataDir, p.ID)
}

// testcasePath joins name onto dir, refusing anything that could escape it.
func testcasePath(dir, name string) (string, error) {
	if name == "" || len(name) > 128 || name != filepath.Base(name) ||
		strings.Contains(name, "..") || strings.ContainsRune(name, '\\') ||
		strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("invalid file name")
	}
	return filepath.Join(dir, name), nil
}

// persistDir records the directory on the problem the first time test data is
// written without an upload having set it already.
func (h *TestcaseHandler) persistDir(r *http.Request, p *model.Problem, dir string) error {
	if p.TestdataPath != "" {
		return nil
	}
	p.TestdataPath = dir
	return h.store.Update(r.Context(), p.ID, p)
}

// List returns every file in the testdata directory with its size, role and
// paired counterpart, so the workspace can offer view/download/delete.
func (h *TestcaseHandler) List(w http.ResponseWriter, r *http.Request) {
	p := h.authorProblem(w, r)
	if p == nil {
		return
	}

	dir := h.testdataDir(p)
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		http.Error(w, "failed to read testcase directory", http.StatusInternalServerError)
		return
	}

	inputs := make(map[string]string)
	outputs := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		stem := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if testcaseInputExts[ext] {
			inputs[stem] = entry.Name()
		} else if testcaseOutputExts[ext] {
			outputs[stem] = entry.Name()
		}
	}

	files := make([]testcaseFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Hidden entries are ours, not the setter's: a crashed inline save can
		// leave a ".testcase-*" temp file behind.
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		file := testcaseFile{Name: name, Size: info.Size(), Role: "other"}
		if testcaseInputExts[ext] {
			file.Role = "input"
			file.Pair = outputs[stem]
		} else if testcaseOutputExts[ext] {
			file.Role = "output"
			file.Pair = inputs[stem]
		}
		files = append(files, file)
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	respondJSON(w, http.StatusOK, map[string]any{"files": files})
}

// Get returns one testcase file. `?download=1` streams the raw bytes as an
// attachment; otherwise the file comes back as JSON for the inline editor.
func (h *TestcaseHandler) Get(w http.ResponseWriter, r *http.Request) {
	p := h.authorProblem(w, r)
	if p == nil {
		return
	}

	path, err := testcasePath(h.testdataDir(p), chi.URLParam(r, "name"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if info.Size() > maxTestcaseContent {
		http.Error(w, fmt.Sprintf("file is %d bytes; only files up to %d bytes can be opened here — download it instead", info.Size(), maxTestcaseContent), http.StatusRequestEntityTooLarge)
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}

	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", info.Name()))
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}

	respondJSON(w, http.StatusOK, testcaseContent{
		Name:    info.Name(),
		Size:    info.Size(),
		Content: string(data),
		Binary:  !utf8.Valid(data),
	})
}

// Save creates or overwrites one testcase file with inline content.
func (h *TestcaseHandler) Save(w http.ResponseWriter, r *http.Request) {
	p := h.authorProblem(w, r)
	if p == nil {
		return
	}

	dir := h.testdataDir(p)
	path, err := testcasePath(dir, chi.URLParam(r, "name"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxTestcaseContent+4096)
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if len(req.Content) > maxTestcaseContent {
		http.Error(w, fmt.Sprintf("content exceeds the %d byte limit", maxTestcaseContent), http.StatusRequestEntityTooLarge)
		return
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, "failed to create directory", http.StatusInternalServerError)
		return
	}

	// Write to a temp file and rename so a failed write never leaves a
	// half-written test case behind for the judge to read.
	tmp, err := os.CreateTemp(dir, ".testcase-*")
	if err != nil {
		http.Error(w, "failed to write file", http.StatusInternalServerError)
		return
	}
	if _, err := tmp.WriteString(req.Content); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		http.Error(w, "failed to write file", http.StatusInternalServerError)
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		http.Error(w, "failed to write file", http.StatusInternalServerError)
		return
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		http.Error(w, "failed to write file", http.StatusInternalServerError)
		return
	}

	if err := h.persistDir(r, p, dir); err != nil {
		http.Error(w, "failed to update problem", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "saved", "name": filepath.Base(path)})
}

// Delete removes one testcase file from the problem's data directory.
func (h *TestcaseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	p := h.authorProblem(w, r)
	if p == nil {
		return
	}

	path, err := testcasePath(h.testdataDir(p), chi.URLParam(r, "name"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to delete file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *TestcaseHandler) Upload(w http.ResponseWriter, r *http.Request) {
	p := h.authorProblem(w, r)
	if p == nil {
		return
	}

	if err := r.ParseMultipartForm(maxTestcaseUpload); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		http.Error(w, "no files provided", http.StatusBadRequest)
		return
	}

	probDir := h.testdataDir(p)
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
// Matching is by filename stem (prefix before extension).
func autoDiscoverTestCases(targetDir string) []model.TestCaseScore {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return nil
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

		if testcaseInputExts[ext] {
			inputFiles[stem] = name
		} else if testcaseOutputExts[ext] {
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
