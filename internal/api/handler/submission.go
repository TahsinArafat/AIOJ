package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/queue"
	"github.com/tahsinarafat/aioj/internal/store"
)

type SubmissionHandler struct {
	subStore  store.SubmissionStore
	probStore store.ProblemStore
	contestStore store.ContestStore
	queue     queue.JudgeQueue
	wsManager *WSManager
	exec      *executor.Client
	langDir   string
}

func NewSubmissionHandler(sub store.SubmissionStore, prob store.ProblemStore, contest store.ContestStore,
	q queue.JudgeQueue, ws *WSManager, exec *executor.Client, langDir string) *SubmissionHandler {
	return &SubmissionHandler{subStore: sub, probStore: prob, contestStore: contest, queue: q, wsManager: ws, exec: exec, langDir: langDir}
}

type CustomRunRequest struct {
	SourceCode string `json:"source_code"`
	Language   string `json:"language"`
	Input      string `json:"input"`
}

type CustomRunResponse struct {
	Status        string `json:"status"`
	Stdout        string `json:"stdout,omitempty"`
	Stderr        string `json:"stderr,omitempty"`
	TimeUsed      int    `json:"time_used"`
	MemoryUsed    int    `json:"memory_used"`
	CompileOutput string `json:"compile_output,omitempty"`
}

func (h *SubmissionHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ProblemID  string `json:"problem_id"`
		Language   string `json:"language"`
		SourceCode string `json:"source_code"`
		ContestID  string `json:"contest_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.ProblemID == "" || req.Language == "" || req.SourceCode == "" {
		http.Error(w, "problem_id, language, and source_code are required", http.StatusBadRequest)
		return
	}

	sub := &model.Submission{
		ID:         uuid.New().String(),
		ProblemID:  req.ProblemID,
		UserID:     claims.UserID,
		Language:   req.Language,
		SourceCode: req.SourceCode,
		CodeSize:   len(req.SourceCode),
		Status:     model.StatusPending,
	}
	if req.ContestID != "" {
		sub.ContestID = req.ContestID
	}

	if err := h.subStore.Create(r.Context(), sub); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	h.queue.Enqueue(r.Context(), sub.ID)
	respondJSON(w, http.StatusCreated, sub)
}

func (h *SubmissionHandler) CreateUpsolving(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ProblemID  string `json:"problem_id"`
		Language   string `json:"language"`
		SourceCode string `json:"source_code"`
		ContestID  string `json:"contest_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.ProblemID == "" || req.Language == "" || req.SourceCode == "" || req.ContestID == "" {
		http.Error(w, "all fields required", http.StatusBadRequest)
		return
	}

	sub := &model.Submission{
		ID:         uuid.New().String(),
		ProblemID:  req.ProblemID,
		UserID:     claims.UserID,
		Language:   req.Language,
		SourceCode: req.SourceCode,
		CodeSize:   len(req.SourceCode),
		Status:     model.StatusPending,
		ContestID:  req.ContestID,
	}

	if err := h.subStore.Create(r.Context(), sub); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	h.queue.Enqueue(r.Context(), sub.ID)
	respondJSON(w, http.StatusCreated, sub)
}

func (h *SubmissionHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sub, err := h.subStore.GetByID(r.Context(), id)
	if err != nil || sub == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, sub)
}

func (h *SubmissionHandler) ListByUser(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	subs, total, err := h.subStore.ListByUser(r.Context(), claims.UserID, offset, limit)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": subs, "total": total})
}

func (h *SubmissionHandler) ListByProblem(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	problem, err := h.probStore.GetBySlug(r.Context(), slug)
	if err != nil || problem == nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	items, total, err := h.subStore.ListByProblem(r.Context(), problem.ID, offset, limit)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *SubmissionHandler) CustomRun(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req CustomRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.SourceCode == "" || req.Language == "" {
		http.Error(w, "source_code and language are required", http.StatusBadRequest)
		return
	}

	langs, err := compiler.LoadLanguages(h.langDir)
	if err != nil {
		http.Error(w, "failed to load language configs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	cfg := langs[req.Language]
	if cfg == nil {
		http.Error(w, "unsupported language: "+req.Language, http.StatusBadRequest)
		return
	}

	compileOutput := ""
	copyIn := map[string]executor.CmdFile{}
	var args []string

	if cfg.CompileCmd != "" {
		srcName := "Main" + cfg.Extensions[0]
		cmdStr := cfg.CompileCmd
		cmdStr = strings.ReplaceAll(cmdStr, "{{exe}}", "Main")
		cmdStr = strings.ReplaceAll(cmdStr, "{{src}}", srcName)
		cmdStr = strings.ReplaceAll(cmdStr, "{{dir}}", ".")
		compileAndRunCmd := cmdStr + " && ./Main"
		copyIn[srcName] = executor.CmdFile{Content: req.SourceCode}
		args = []string{"/bin/sh", "-c", compileAndRunCmd}
	} else {
		srcName := "Main" + cfg.Extensions[0]
		rtParts := strings.Fields(cfg.Runtime)
		args = append(rtParts, "/box/"+srcName)
		copyIn[srcName] = executor.CmdFile{Content: req.SourceCode}
	}

	cpuNs := uint64(5.0 * 1e9 * cfg.TimeLimitMultiplier)
	memBytes := uint64(256.0 * 1024 * 1024 * cfg.MemoryLimitMultiplier)

	start := time.Now()
	resp, err := h.exec.Run(&executor.ExecRequest{
		Cmd: []executor.Cmd{{
			Args:        args,
			Env:         []string{"PATH=/usr/bin:/bin"},
			CPULimit:    cpuNs,
			MemoryLimit: memBytes,
			ProcLimit:   16,
			CopyIn:      copyIn,
			Files: []executor.CmdFile{
				{Content: req.Input},
				{Name: "stdout", Max: 10 * 1024 * 1024},
				{Name: "stderr", Max: 10 * 1024 * 1024},
			},
		}},
	})
	elapsed := int(time.Since(start).Milliseconds())

	if err != nil {
		respondJSON(w, http.StatusOK, CustomRunResponse{
			Status:        "SE",
			CompileOutput: "execution client error: " + err.Error(),
		})
		return
	}

	if len(resp) == 0 {
		respondJSON(w, http.StatusOK, CustomRunResponse{
			Status:        "SE",
			CompileOutput: "no execution response received",
		})
		return
	}

	cr := resp[0]
	slog.Info("custom run result", "status", cr.Status, "exit", cr.ExitStatus, "error", cr.Error)

	stdout := ""
	if out, ok := cr.Files["stdout"]; ok {
		stdout = out
	}
	stderr := ""
	if errOut, ok := cr.Files["stderr"]; ok {
		stderr = errOut
	}

	if cfg.CompileCmd != "" && cr.Status == "Nonzero Exit Status" {
		compileOutput = stderr
		if compileOutput == "" {
			compileOutput = cr.Error
		}
		respondJSON(w, http.StatusOK, CustomRunResponse{
			Status:        "CE",
			CompileOutput: compileOutput,
		})
		return
	}

	status := "success"
	switch cr.Status {
	case "Accepted":
		status = "success"
	case "TimeLimitExceeded":
		status = "TLE"
	case "MemoryLimitExceeded":
		status = "MLE"
	case "RuntimeError":
		status = "RE"
	default:
		status = "RE"
	}

	respondJSON(w, http.StatusOK, CustomRunResponse{
		Status:        status,
		Stdout:        stdout,
		Stderr:        stderr,
		TimeUsed:      elapsed,
		MemoryUsed:    int(cr.Memory / 1024),
		CompileOutput: compileOutput,
	})
}
