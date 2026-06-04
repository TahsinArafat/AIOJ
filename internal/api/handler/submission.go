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
	"github.com/tahsinarafat/aioj/internal/vjudge"
)

type SubmissionHandler struct {
	subStore     store.SubmissionStore
	probStore    store.ProblemStore
	contestStore store.ContestStore
	queue        queue.JudgeQueue
	wsManager    *WSManager
	exec         *executor.Client
	langDir      string
	vjudgeSvc    *vjudge.Service
}

func NewSubmissionHandler(sub store.SubmissionStore, prob store.ProblemStore, contest store.ContestStore,
	q queue.JudgeQueue, ws *WSManager, exec *executor.Client, langDir string, vjSvc *vjudge.Service) *SubmissionHandler {
	return &SubmissionHandler{subStore: sub, probStore: prob, contestStore: contest, queue: q, wsManager: ws, exec: exec, langDir: langDir, vjudgeSvc: vjSvc}
}

type CustomRunRequest struct {
	SourceCode   string `json:"source_code"`
	Language     string `json:"language"`
	Input        string `json:"input"`
	Expected     string `json:"expected,omitempty"`
	TimeLimitMs  int    `json:"time_limit_ms,omitempty"`
	MemoryLimitKB int   `json:"memory_limit_kb,omitempty"`
}

type CustomRunResponse struct {
	Status        string `json:"status"`
	Stdout        string `json:"stdout,omitempty"`
	Stderr        string `json:"stderr,omitempty"`
	TimeUsed      int    `json:"time_used"`
	MemoryUsed    int    `json:"memory_used"`
	CompileOutput string `json:"compile_output,omitempty"`
	Passed        *bool  `json:"passed,omitempty"`
	Expected      string `json:"expected,omitempty"`
}

func normalizeOutput(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n\r ")
	s = strings.TrimSpace(s)
	return s
}

type submissionBuildRequest struct {
	ProblemID  string
	Language   string
	SourceCode string
	ContestID  string
	UserID     string
	Upsolving  bool
	Priority   int // 0=auto (contest=0, normal=1), >0=explicit override
}

func (h *SubmissionHandler) buildAndEnqueue(r *http.Request, w http.ResponseWriter, req submissionBuildRequest) {
	prob, err := h.probStore.GetByID(r.Context(), req.ProblemID)
	if err != nil || prob == nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}

	sub := &model.Submission{
		ID:         uuid.New().String(),
		ProblemID:  req.ProblemID,
		UserID:     req.UserID,
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

	if prob.Source != "" && prob.Source != "local" && h.vjudgeSvc != nil {
		respondJSON(w, http.StatusCreated, sub)
		return
	}

	priority := req.Priority
	if priority == 0 {
		priority = 1
		if req.ContestID != "" && !req.Upsolving {
			contest, cerr := h.contestStore.GetByID(r.Context(), req.ContestID)
			if cerr == nil && contest != nil {
				now := time.Now()
				if now.After(contest.StartTime) && now.Before(contest.EndTime) {
					priority = 0
				}
			}
		}
	}
	h.queue.Enqueue(r.Context(), sub.ID, priority)
	respondJSON(w, http.StatusCreated, sub)
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
		Priority   int    `json:"priority,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.ProblemID == "" || req.Language == "" || req.SourceCode == "" {
		http.Error(w, "problem_id, language, and source_code are required", http.StatusBadRequest)
		return
	}

	if req.ContestID != "" {
		contest, err := h.contestStore.GetByID(r.Context(), req.ContestID)
		if err != nil || contest == nil {
			http.Error(w, "contest not found", http.StatusNotFound)
			return
		}
		req.ContestID = contest.ID

		now := time.Now()
		if now.After(contest.StartTime) && now.Before(contest.EndTime) {
			isJudge := claims.Role == "admin" || contest.CreatedBy == claims.UserID || h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "judge", "tester")
			if isJudge {
				http.Error(w, "judges and admins cannot submit during the contest", http.StatusForbidden)
				return
			}

			if contest.RegistrationRequired {
				registered, err := h.contestStore.IsParticipant(r.Context(), contest.ID, claims.UserID)
				if err != nil || !registered {
					http.Error(w, "not registered for the contest", http.StatusForbidden)
					return
				}
			}
		} else if now.Before(contest.StartTime) {
			http.Error(w, "contest has not started yet", http.StatusForbidden)
			return
		} else {
			if !contest.UpsolvingEnabled {
				isJudge := claims.Role == "admin" || contest.CreatedBy == claims.UserID || h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "judge", "tester")
				if !isJudge {
					http.Error(w, "upsolving is disabled", http.StatusForbidden)
					return
				}
			}
		}
	}

	h.buildAndEnqueue(r, w, submissionBuildRequest{
		ProblemID:  req.ProblemID,
		Language:   req.Language,
		SourceCode: req.SourceCode,
		ContestID:  req.ContestID,
		UserID:     claims.UserID,
		Priority:   req.Priority,
	})
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

	contest, err := h.contestStore.GetByID(r.Context(), req.ContestID)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}
	req.ContestID = contest.ID

	if !contest.UpsolvingEnabled {
		isJudge := claims.Role == "admin" || contest.CreatedBy == claims.UserID || h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "judge", "tester")
		if !isJudge {
			http.Error(w, "upsolving is disabled", http.StatusForbidden)
			return
		}
	}

	h.buildAndEnqueue(r, w, submissionBuildRequest{
		ProblemID:  req.ProblemID,
		Language:   req.Language,
		SourceCode: req.SourceCode,
		ContestID:  req.ContestID,
		UserID:     claims.UserID,
		Upsolving:  true,
	})
}

func (h *SubmissionHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sub, err := h.subStore.GetByID(r.Context(), id)
	if err != nil || sub == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	prob, err := h.probStore.GetByID(r.Context(), sub.ProblemID)
	if err == nil && prob != nil && prob.Source != "" && prob.Source != "local" {
		sub.IsRemote = true
		sub.RemoteOJ = prob.Source
	}

	if sub.ContestID != "" {
		contest, err := h.contestStore.GetByID(r.Context(), sub.ContestID)
		if err == nil && contest != nil {
			if time.Now().Before(contest.EndTime) {
				sub.RemoteID = ""
				sub.RemoteURL = ""
			}
		}
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
	problemID := r.URL.Query().Get("problem_id")
	contestID := r.URL.Query().Get("contest_id")
	subs, total, err := h.subStore.ListByUser(r.Context(), claims.UserID, offset, limit, problemID, contestID)
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

func (h *SubmissionHandler) ListByContest(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "contestId")
	if contestID == "" {
		contestID = chi.URLParam(r, "id")
	}
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
	// Resolve contest (display_id -> UUID) and check permissions
	contest, _ := h.contestStore.GetByID(r.Context(), contestID)
	if contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}
	isAdmin := claims.Role == "admin"
	isJudge := isAdmin || h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "judge")

	// Build filter
	filter := model.SubmissionFilter{
		ProblemID: r.URL.Query().Get("problem_id"),
		Language:  r.URL.Query().Get("language"),
		Status:    r.URL.Query().Get("status"),
	}

	// mine=true (or default) means only current user's submissions
	// mine=false means all submissions in the contest (for everyone)
	mine := r.URL.Query().Get("mine")
	if mine == "false" {
		// Show all — no user filter
	} else {
		// Default: show only user's own
		filter.UserID = claims.UserID
	}

	subs, total, err := h.subStore.ListByContest(r.Context(), contest.ID, offset, limit, filter)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": subs, "total": total, "is_judge": isJudge})
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

	srcName := "Main" + cfg.Extensions[0]
	copyIn[srcName] = executor.CmdFile{Content: req.SourceCode}
	copyIn["input.txt"] = executor.CmdFile{Content: req.Input}

	if cfg.CompileCmd != "" && cfg.Runtime != "" {
		compileCmd := cfg.CompileCmd
		compileCmd = strings.ReplaceAll(compileCmd, "{{exe}}", "Main")
		compileCmd = strings.ReplaceAll(compileCmd, "{{src}}", srcName)
		compileCmd = strings.ReplaceAll(compileCmd, "{{dir}}", ".")
		rtParts := strings.Fields(cfg.Runtime)
		for i, p := range rtParts {
			rtParts[i] = strings.ReplaceAll(p, "{{dir}}", ".")
			rtParts[i] = strings.ReplaceAll(rtParts[i], "{{exe}}", "Main")
		}
		runCmd := compileCmd + " && " + strings.Join(rtParts, " ") + " < input.txt > output.txt 2> error.txt"
		args = []string{"/bin/sh", "-c", runCmd}
	} else if cfg.CompileCmd != "" {
		compileCmd := cfg.CompileCmd
		compileCmd = strings.ReplaceAll(compileCmd, "{{exe}}", "Main")
		compileCmd = strings.ReplaceAll(compileCmd, "{{src}}", srcName)
		compileCmd = strings.ReplaceAll(compileCmd, "{{dir}}", ".")
		runCmd := compileCmd + " && ./Main < input.txt > output.txt 2> error.txt"
		args = []string{"/bin/sh", "-c", runCmd}
	} else if cfg.Runtime != "" {
		rtParts := strings.Fields(cfg.Runtime)
		runCmd := strings.Join(rtParts, " ") + " " + srcName + " < input.txt > output.txt 2> error.txt"
		args = []string{"/bin/sh", "-c", runCmd}
	} else {
		runCmd := "./Main < input.txt > output.txt 2> error.txt"
		args = []string{"/bin/sh", "-c", runCmd}
	}

	cpuNs := uint64(5.0 * 1e9 * cfg.TimeLimitMultiplier)
	memBytes := uint64(256.0 * 1024 * 1024 * cfg.MemoryLimitMultiplier)
	if req.TimeLimitMs > 0 {
		cpuNs = uint64(req.TimeLimitMs) * 1e6
	}
	if req.MemoryLimitKB > 0 {
		memBytes = uint64(req.MemoryLimitKB) * 1024
	}

	start := time.Now()
	resp, err := h.exec.Run(&executor.ExecRequest{
		Cmd: []executor.Cmd{{
			Args:        args,
			Env:         []string{"PATH=/usr/bin:/bin", "HOME=/tmp"},
			CPULimit:    cpuNs,
			MemoryLimit: memBytes,
			ProcLimit:   64,
			CopyIn:      copyIn,
			CopyOut:     []string{"output.txt", "error.txt"},
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
	if out, ok := cr.Files["output.txt"]; ok {
		stdout = out
	}
	stderr := ""
	if errOut, ok := cr.Files["error.txt"]; ok {
		stderr = errOut
	}

	if cfg.CompileCmd != "" && cr.Status == "Nonzero Exit Status" {
		compileOutput = stderr
		if compileOutput == "" {
			compileOutput = cr.Error
		}
		if compileOutput == "" {
			// Some compilers write errors to stdout
			if out, ok := cr.Files["output.txt"]; ok {
				compileOutput = out
			}
		}
		if compileOutput == "" {
			compileOutput = "Compilation failed (no error output captured)"
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
		Passed:        compareOutput(status, stdout, req.Expected),
		Expected:      req.Expected,
	})
}

func compareOutput(status, stdout, expected string) *bool {
	if expected == "" {
		return nil
	}
	if status != "success" {
		passed := false
		return &passed
	}
	actual := normalizeOutput(stdout)
	exp := normalizeOutput(expected)
	result := actual == exp
	return &result
}

func (h *SubmissionHandler) hasSubmissionAccess(r *http.Request, sub *model.Submission) bool {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		return false
	}
	if claims.Role == "admin" {
		return true
	}
	if sub.UserID == claims.UserID {
		return true
	}
	if sub.ContestID != "" {
		isJudge := h.contestStore.HasAccess(r.Context(), sub.ContestID, claims.UserID, "manager", "judge")
		if isJudge {
			return true
		}
	}
	return false
}

func (h *SubmissionHandler) RetryRemote(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sub, err := h.subStore.GetByID(r.Context(), id)
	if err != nil || sub == nil {
		http.Error(w, "submission not found", http.StatusNotFound)
		return
	}

	if !h.hasSubmissionAccess(r, sub) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	prob, err := h.probStore.GetByID(r.Context(), sub.ProblemID)
	if err != nil || prob == nil || prob.Source == "" || prob.Source == "local" {
		http.Error(w, "not a remote OJ problem", http.StatusBadRequest)
		return
	}

	h.subStore.UpdateStatus(r.Context(), id, model.StatusPending)
	h.subStore.UpdateRemoteID(r.Context(), id, "", "")
	h.subStore.UpdateBotID(r.Context(), id, "", "")

	respondJSON(w, http.StatusOK, map[string]string{"status": "retrying"})
}

func (h *SubmissionHandler) RecheckRemote(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sub, err := h.subStore.GetByID(r.Context(), id)
	if err != nil || sub == nil {
		http.Error(w, "submission not found", http.StatusNotFound)
		return
	}

	if !h.hasSubmissionAccess(r, sub) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if sub.RemoteID == "" {
		http.Error(w, "no remote submission ID", http.StatusBadRequest)
		return
	}

	prob, err := h.probStore.GetByID(r.Context(), sub.ProblemID)
	if err != nil || prob == nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}

	if h.vjudgeSvc != nil {
		go h.vjudgeSvc.ForcePoll(r.Context(), prob.Source, sub.RemoteID, sub.ID)
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "refreshing"})
}
