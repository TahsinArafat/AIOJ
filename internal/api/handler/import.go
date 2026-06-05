package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/fps"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
	"github.com/tahsinarafat/aioj/internal/vjudge"
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

func (h *ImportHandler) ImportCodeforces(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if claims.Role != "admin" && claims.Role != "setter" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		ContestID     string `json:"contest_id"`
		ProblemIndex  string `json:"problem_index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.ContestID == "" || req.ProblemIndex == "" {
		http.Error(w, "contest_id and problem_index are required", http.StatusBadRequest)
		return
	}

	parser := vjudge.NewProblemParser(func(ctx context.Context, url string) (string, error) {
		httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return "", err
		}
		httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	})

	prob, err := parser.ParseCodeforcesProblem(r.Context(), req.ContestID, req.ProblemIndex)
	if err != nil {
		http.Error(w, "failed to parse problem: "+err.Error(), http.StatusBadRequest)
		return
	}

	prob.ID = uuid.New().String()
	prob.Slug = strings.ToLower(strings.ReplaceAll(fmt.Sprintf("cf-%s-%s", req.ContestID, req.ProblemIndex), " ", "-"))
	if existing, _ := h.probStore.GetBySlug(r.Context(), prob.Slug); existing != nil {
		prob.Slug = prob.Slug + "-" + uuid.New().String()[:8]
	}
	prob.CreatedBy = claims.UserID
	prob.Visible = true
	if prob.Tags == nil {
		prob.Tags = []string{}
	}
	if prob.SampleCases == nil {
		prob.SampleCases = []model.SampleCase{}
	}

	if err := h.probStore.Create(r.Context(), prob); err != nil {
		http.Error(w, "failed to save problem: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "success",
		"problem_id": prob.ID,
		"slug":       prob.Slug,
	})
}

func (h *ImportHandler) ImportCSES(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if claims.Role != "admin" && claims.Role != "setter" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		ProblemID string `json:"problem_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.ProblemID == "" {
		http.Error(w, "problem_id is required", http.StatusBadRequest)
		return
	}

	parser := vjudge.NewProblemParser(func(ctx context.Context, url string) (string, error) {
		httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return "", err
		}
		httpReq.Header.Set("User-Agent", "Mozilla/5.0")
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	})

	prob, err := parser.ParseCSESProblem(r.Context(), req.ProblemID)
	if err != nil {
		http.Error(w, "failed to parse CSES problem: "+err.Error(), http.StatusBadRequest)
		return
	}

	prob.ID = uuid.New().String()
	prob.Slug = fmt.Sprintf("cses-%s", strings.ToLower(req.ProblemID))
	if existing, _ := h.probStore.GetBySlug(r.Context(), prob.Slug); existing != nil {
		prob.Slug = prob.Slug + "-" + uuid.New().String()[:8]
	}
	prob.CreatedBy = claims.UserID
	prob.Visible = true
	if prob.Tags == nil { prob.Tags = []string{} }
	if prob.SampleCases == nil { prob.SampleCases = []model.SampleCase{} }

	if err := h.probStore.Create(r.Context(), prob); err != nil {
		http.Error(w, "failed to save problem: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"status":     "success",
		"problem_id": prob.ID,
		"slug":       prob.Slug,
	})
}

func (h *ImportHandler) ImportAtCoder(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if claims.Role != "admin" && claims.Role != "setter" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		ContestID string `json:"contest_id"`
		ProblemID string `json:"problem_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.ContestID == "" || req.ProblemID == "" {
		http.Error(w, "contest_id and problem_id are required", http.StatusBadRequest)
		return
	}

	parser := vjudge.NewProblemParser(func(ctx context.Context, url string) (string, error) {
		httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return "", err
		}
		httpReq.Header.Set("User-Agent", "Mozilla/5.0")
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	})

	prob, err := parser.ParseAtCoderProblem(r.Context(), req.ContestID, req.ProblemID)
	if err != nil {
		http.Error(w, "failed to parse AtCoder problem: "+err.Error(), http.StatusBadRequest)
		return
	}

	prob.ID = uuid.New().String()
	prob.Slug = fmt.Sprintf("atcoder-%s", strings.ToLower(req.ProblemID))
	if existing, _ := h.probStore.GetBySlug(r.Context(), prob.Slug); existing != nil {
		prob.Slug = prob.Slug + "-" + uuid.New().String()[:8]
	}
	prob.CreatedBy = claims.UserID
	prob.Visible = true
	if prob.Tags == nil {
		prob.Tags = []string{}
	}
	if prob.SampleCases == nil {
		prob.SampleCases = []model.SampleCase{}
	}

	if err := h.probStore.Create(r.Context(), prob); err != nil {
		http.Error(w, "failed to save problem: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"status":     "success",
		"problem_id": prob.ID,
		"slug":       prob.Slug,
	})
}

func (h *ImportHandler) ImportToph(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if claims.Role != "admin" && claims.Role != "setter" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		ProblemID string `json:"problem_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.ProblemID == "" {
		http.Error(w, "problem_id is required", http.StatusBadRequest)
		return
	}

	parser := vjudge.NewProblemParser(func(ctx context.Context, url string) (string, error) {
		httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return "", err
		}
		httpReq.Header.Set("User-Agent", "Mozilla/5.0")
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	})

	prob, err := parser.ParseTophProblem(r.Context(), req.ProblemID)
	if err != nil {
		http.Error(w, "failed to parse Toph problem: "+err.Error(), http.StatusBadRequest)
		return
	}

	prob.ID = uuid.New().String()
	prob.Slug = fmt.Sprintf("toph-%s", strings.ToLower(req.ProblemID))
	if existing, _ := h.probStore.GetBySlug(r.Context(), prob.Slug); existing != nil {
		prob.Slug = prob.Slug + "-" + uuid.New().String()[:8]
	}
	prob.CreatedBy = claims.UserID
	prob.Visible = true
	if prob.Tags == nil {
		prob.Tags = []string{}
	}
	if prob.SampleCases == nil {
		prob.SampleCases = []model.SampleCase{}
	}

	if err := h.probStore.Create(r.Context(), prob); err != nil {
		http.Error(w, "failed to save problem: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"status":     "success",
		"problem_id": prob.ID,
		"slug":       prob.Slug,
	})
}

func (h *ImportHandler) ImportQOJ(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if claims.Role != "admin" && claims.Role != "setter" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		ProblemID string `json:"problem_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.ProblemID == "" {
		http.Error(w, "problem_id is required", http.StatusBadRequest)
		return
	}

	parser := vjudge.NewProblemParser(func(ctx context.Context, url string) (string, error) {
		httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return "", err
		}
		httpReq.Header.Set("User-Agent", "Mozilla/5.0")
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(body), nil
	})

	prob, err := parser.ParseQOJProblem(r.Context(), req.ProblemID)
	if err != nil {
		http.Error(w, "failed to parse QOJ problem: "+err.Error(), http.StatusBadRequest)
		return
	}

	prob.ID = uuid.New().String()
	prob.Slug = fmt.Sprintf("qoj-%s", strings.ToLower(req.ProblemID))
	if existing, _ := h.probStore.GetBySlug(r.Context(), prob.Slug); existing != nil {
		prob.Slug = prob.Slug + "-" + uuid.New().String()[:8]
	}
	prob.CreatedBy = claims.UserID
	prob.Visible = true
	if prob.Tags == nil {
		prob.Tags = []string{}
	}
	if prob.SampleCases == nil {
		prob.SampleCases = []model.SampleCase{}
	}

	if err := h.probStore.Create(r.Context(), prob); err != nil {
		http.Error(w, "failed to save problem: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"status":     "success",
		"problem_id": prob.ID,
		"slug":       prob.Slug,
	})
}

func (h *ImportHandler) Import(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if claims.Role != "admin" && claims.Role != "setter" {
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
