package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/queue"
	"github.com/tahsinarafat/aioj/internal/store"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
	"github.com/tahsinarafat/aioj/internal/vjudge"
)

// Judge queue priority levels.  Lower = more urgent (min-heap / Redis ZSET).
const (
	// PriorityAuto lets buildAndEnqueue decide from the contest window.
	// Never store this value in the queue — it is always resolved before Enqueue.
	PriorityAuto = 0
	// PriorityContest is for submissions during a live contest window (highest urgency).
	PriorityContest = 1
	// PriorityNormal is for practice, upsolving, and admin single rejudge.
	PriorityNormal = 2
	// PriorityBulk is for background batch rejudge that must not delay live contest judging.
	PriorityBulk = 3
)

type SubmissionHandler struct {
	// contestHandlerAccess carries the shared contest access policy. Submit is the
	// surface the exploit actually hit: the password and group gates were simply
	// absent here, so a contestant could post a solution to a round they could not
	// even read.
	contestHandlerAccess
	subStore     store.SubmissionStore
	probStore    store.ProblemStore
	contestStore store.ContestStore
	queue        queue.JudgeQueue
	wsManager    *WSManager
	exec         *executor.Client
	langDir      string
	vjudgeSvc    *vjudge.Service
	users        store.UserStore
	// submitQuota and runQuota are per-user budgets. Without them a single
	// account could flood the judge queue or use /run as a free compile-and-
	// execute service, starving everybody else.
	submitQuota *userQuota
	runQuota    *userQuota
	// langsOnce / langs cache the YAML language configs loaded from langDir.
	// /run is called on every "run code" click; re-parsing the YAML files on
	// each request was wasted disk I/O.  Languages only change on restart.
	langsOnce sync.Once
	langs     map[string]*compiler.LangConfig
	// audit records setter-initiated rejudges; optional.
	audit *AuditLogHandler
	// scoreboardCache is invalidated when a submission is voided, so a standing
	// that just changed is not served from cache for the rest of its TTL.
	scoreboardCache *scoreboardCache
}

// WithAudit attaches the audit recorder. Call it after NewSubmissionHandler.
func (h *SubmissionHandler) WithAudit(a *AuditLogHandler) *SubmissionHandler {
	h.audit = a
	return h
}

// WithScoreboardCache attaches the standings cache so voiding a submission
// drops the stale entry. Optional: without it the board catches up on TTL.
func (h *SubmissionHandler) WithScoreboardCache(c *scoreboardCache) *SubmissionHandler {
	h.scoreboardCache = c
	return h
}

// WithGrants attaches the password-entitlement store and rebuilds the access
// policy so it can consult it. Returns the handler for call-site chaining.
func (h *SubmissionHandler) WithGrants(gs *postgres.ContestGrantStore) *SubmissionHandler {
	h.rebuildWithGrants(h.contestStore, gs)
	return h
}

func NewSubmissionHandler(sub store.SubmissionStore, prob store.ProblemStore, contest store.ContestStore,
	q queue.JudgeQueue, ws *WSManager, exec *executor.Client, langDir string, vjSvc *vjudge.Service,
	users store.UserStore) *SubmissionHandler {
	h := &SubmissionHandler{subStore: sub, probStore: prob, contestStore: contest, queue: q, wsManager: ws, exec: exec, langDir: langDir, vjudgeSvc: vjSvc, users: users,
		submitQuota: newSubmissionQuota(), runQuota: newRunQuota()}
	h.access = newContestAccessPolicy(contest, nil)
	return h
}

// loadLangs returns the cached language map, initialising it on first call.
// Language configs only change when a new lang/*.yaml is added, which requires
// a process restart — so reading from disk once and caching forever is correct.
func (h *SubmissionHandler) loadLangs() map[string]*compiler.LangConfig {
	h.langsOnce.Do(func() {
		langs, err := compiler.LoadLanguages(h.langDir)
		if err != nil {
			slog.Error("submission: failed to load language configs", "error", err)
			langs = map[string]*compiler.LangConfig{}
		}
		h.langs = langs
	})
	return h.langs
}

// checkQuota enforces a per-user budget and writes a 429 when it is exhausted.
// A nil quota (handler built without the constructor, as some tests do) is
// unlimited.
func checkQuota(w http.ResponseWriter, quota *userQuota, userID string) bool {
	if quota == nil {
		return true
	}
	ok, retryAfter := quota.allow(userID)
	if ok {
		return true
	}
	writeQuotaExceeded(w, retryAfter)
	return false
}

type CustomRunRequest struct {
	SourceCode    string `json:"source_code"`
	Language      string `json:"language"`
	Input         string `json:"input"`
	Expected      string `json:"expected,omitempty"`
	TimeLimitMs   int    `json:"time_limit_ms,omitempty"`
	MemoryLimitKB int    `json:"memory_limit_kb,omitempty"`
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
	// SubmissionType is empty for a normal code submission; "output" when the
	// caller is attaching an output file to an output-only problem.
	SubmissionType string
	// IsTester marks a submission made by a contest judge/tester/manager
	// before the contest window opens.  These get PriorityContest because
	// they validate problems and must not wait behind practice submissions.
	IsTester bool
	// Priority controls queue ordering (lower = more urgent).
	// PriorityAuto (0) means: detect automatically from contest window.
	// PriorityContest (1) = live-contest or tester; PriorityNormal (2) = practice/upsolving;
	// PriorityBulk (3) = background batch rejudge.
	// Admin endpoints pass explicit values; user submissions always use PriorityAuto.
	Priority int
}

func (h *SubmissionHandler) buildAndEnqueue(r *http.Request, w http.ResponseWriter, req submissionBuildRequest) {
	prob, err := h.probStore.GetByID(r.Context(), req.ProblemID)
	if err != nil || prob == nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}

	// The judge has always known how to run an output-only submission
	// (worker.go:242 dispatches before any language lookup), but nothing ever set
	// SubmissionType, so the path was unreachable. Derive it from the problem
	// rather than trusting the client: an output submission on a coded problem
	// would skip compilation entirely and be judged as literal stdout.
	subType := model.SubmissionTypeCode
	if prob.ProblemType == model.ProblemTypeOutputOnly {
		subType = model.SubmissionTypeOutput
	}
	sub := &model.Submission{
		ID:             uuid.New().String(),
		ProblemID:      req.ProblemID,
		UserID:         req.UserID,
		Language:       req.Language,
		SourceCode:     req.SourceCode,
		CodeSize:       len(req.SourceCode),
		Status:         model.StatusPending,
		SubmissionType: subType,
	}
	if req.ContestID != "" {
		sub.ContestID = req.ContestID
		sub.IsUpsolving = req.Upsolving
	}

	if err := h.subStore.Create(r.Context(), sub); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}

	if model.IsRemotePlatform(prob.Source) && h.vjudgeSvc != nil {
		respondJSON(w, http.StatusCreated, sub)
		return
	}

	priority := req.Priority
	if priority == PriorityAuto {
		// Auto-detect: live contest window → PriorityContest, everything else → PriorityNormal.
		priority = PriorityNormal
		if req.ContestID != "" && !req.Upsolving {
			contest, cerr := h.contestStore.GetByID(r.Context(), req.ContestID)
			if cerr == nil && contest != nil {
				now := time.Now()
				if now.After(contest.StartTime) && now.Before(contest.EndTime) {
					// Live contest window.
					priority = PriorityContest
				} else if req.IsTester && now.Before(contest.StartTime) {
					// Pre-contest tester/judge/manager validating the problem set.
					// Give contest priority so their submissions don't queue behind
					// practice work from other users.
					priority = PriorityContest
				}
			}
		}
	}
	h.queue.Enqueue(r.Context(), sub.ID, priority)
	respondJSON(w, http.StatusCreated, sub)
}

// requireVerifiedEmail returns false and writes 403 when the user has not
// verified their email. Store errors surface as 500.
func (h *SubmissionHandler) requireVerifiedEmail(w http.ResponseWriter, r *http.Request, userID string) bool {
	if h.users == nil {
		// Tests or wiring without users store: fail closed would break
		// unit tests that only exercise other paths; fail open only if
		// store was never injected (should not happen in main).
		return true
	}
	verified, err := h.users.IsEmailVerified(r.Context(), userID)
	if err != nil {
		http.Error(w, "verification check failed", http.StatusInternalServerError)
		return false
	}
	if !verified {
		respondJSON(w, http.StatusForbidden, map[string]string{
			"error": "email not verified; check your inbox or resend the verification link",
		})
		return false
	}
	return true
}

func (h *SubmissionHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.requireVerifiedEmail(w, r, claims.UserID) {
		return
	}
	if !checkQuota(w, h.submitQuota, claims.UserID) {
		return
	}

	var req struct {
		ProblemID  string `json:"problem_id"`
		Language   string `json:"language"`
		SourceCode string `json:"source_code"`
		ContestID  string `json:"contest_id,omitempty"`
		// Priority is intentionally NOT accepted from the user request body.
		// Priority is determined server-side by contest-window detection.
		// Admins may set it via the admin rejudge endpoints only.
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.ProblemID == "" || req.SourceCode == "" {
		http.Error(w, "problem_id and source_code are required", http.StatusBadRequest)
		return
	}

	// An output-only problem takes the contestant's ANSWER as source_code, so it
	// needs no language. A coded problem still does.
	prob, err := h.probStore.GetByID(r.Context(), req.ProblemID)
	if err != nil || prob == nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}
	if prob.ProblemType != model.ProblemTypeOutputOnly && req.Language == "" {
		http.Error(w, "language is required", http.StatusBadRequest)
		return
	}

	if req.ContestID != "" {
		contest, err := h.contestStore.GetByID(r.Context(), req.ContestID)
		if err != nil || contest == nil {
			http.Error(w, "contest not found", http.StatusNotFound)
			return
		}
		req.ContestID = contest.ID

		// The gate that was missing entirely. Time-window and registration checks
		// below only decide whether a submitter is *eligible*; they say nothing about
		// whether this caller is allowed in the round at all. Without this, a
		// password-protected or group-restricted contest accepted submissions from
		// anyone who knew its id - the exploit that was reported.
		//
		// Decided before the isJudge test on purpose: staff bypassing the time window
		// is a separate rule, and a denied caller must be refused regardless of it.
		if err := h.requireContestSubmissionAccess(w, r, contest); err != nil {
			return
		}

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
			// Before the contest starts: only judges/managers/testers may submit,
			// to validate the problem set.
			isStaff := claims.Role == "admin" || contest.CreatedBy == claims.UserID ||
				h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "judge", "tester")
			if !isStaff {
				http.Error(w, "contest has not started yet", http.StatusForbidden)
				return
			}
			// Staff are allowed through; IsTester=true gives them PriorityContest.
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
		// IsTester is set to true when staff (admin/creator/judge/manager/tester)
		// submit before the contest starts, so pre-contest validation gets
		// PriorityContest instead of PriorityNormal.
		IsTester: req.ContestID != "" && func() bool {
			c, err := h.contestStore.GetByID(r.Context(), req.ContestID)
			if err != nil || c == nil {
				return false
			}
			return time.Now().Before(c.StartTime) &&
				(claims.Role == "admin" || c.CreatedBy == claims.UserID ||
					h.contestStore.HasAccess(r.Context(), req.ContestID, claims.UserID, "manager", "judge", "tester"))
		}(),
		// Priority is always PriorityAuto for user submissions; server-side
		// contest detection sets the real value inside buildAndEnqueue.
	})
}

func (h *SubmissionHandler) CreateUpsolving(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.requireVerifiedEmail(w, r, claims.UserID) {
		return
	}
	if !checkQuota(w, h.submitQuota, claims.UserID) {
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
	// /run is a free compile-and-execute against the round's problems, so it needs
	// the same gate as submit: unlocking the round is what makes a contest problem
	// actionable.
	if err := h.requireContestSubmissionAccess(w, r, contest); err != nil {
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

	// Source code (and full run details) are private: owner, admins, and
	// contest judges/managers only. Mirrors RetryRemote/RecheckRemote.
	if !h.hasSubmissionAccess(r, sub) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// The judge's own failure text is an internal detail: admins, the problem's
	// setters and contest judges get it, the contestant keeps the generic
	// "the judge failed" message.
	if !h.canSeeJudgeInternals(r, sub) {
		sub.JudgeError = ""
	}

	prob, err := h.probStore.GetByID(r.Context(), sub.ProblemID)
	if err == nil && prob != nil && model.IsRemotePlatform(prob.Source) {
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
	// Submission traffic names its submitters. On a draft that is not public
	// information, and the route carries no auth middleware of its own.
	if !canReadProblem(r.Context(), h.probStore, problem, middleware.GetUserClaims(r)) {
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

	// mine=false means the whole contest's rows. That is a judge's view: it names
	// every submitter and their verdicts, so it is limited to the people running
	// the round. Before this, isJudge was computed and then only echoed back in
	// the response, so any signed-in user could read the entire contest's traffic
	// by adding ?mine=false. The freeze is a scoreboard concern and is applied
	// there; a judge running a round still needs to see its submissions.
	mine := r.URL.Query().Get("mine")
	if mine == "false" {
		if !isJudge {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
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
	if !h.requireVerifiedEmail(w, r, claims.UserID) {
		return
	}
	// /run compiles and executes on demand: the most expensive thing a user can
	// ask for, so it gets the tighter budget.
	if !checkQuota(w, h.runQuota, claims.UserID) {
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

	langs := h.loadLangs()
	cfg := langs[req.Language]
	if cfg == nil {
		http.Error(w, "unsupported language: "+req.Language, http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, h.runOnce(r.Context(), req, cfg))
}

// BatchRun is the "Test Samples" button: one compile-and-execute per sample
// case under a single quota token for the whole sweep.
//
// The client used to fire N separate /run calls. /run is deliberately the
// tightest budget on the platform (burst 3, 0.2 rps) because it compiles and
// executes arbitrary code on demand, so a problem with more than three sample
// cases blew the budget partway through: the later cases answered 429 and the
// button sat in "Testing..." behind a five-second cooldown.
func (h *SubmissionHandler) BatchRun(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.requireVerifiedEmail(w, r, claims.UserID) {
		return
	}

	var req struct {
		SourceCode string `json:"source_code"`
		Language   string `json:"language"`
		Cases      []struct {
			Input    string `json:"input"`
			Expected string `json:"expected"`
		} `json:"cases"`
		TimeLimitMs   int `json:"time_limit_ms,omitempty"`
		MemoryLimitKB int `json:"memory_limit_kb,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.SourceCode == "" || req.Language == "" {
		http.Error(w, "source_code and language are required", http.StatusBadRequest)
		return
	}
	// A sweep is bounded so one request cannot occupy a judge worker for an
	// unbounded stretch. Twenty covers any realistic sample set.
	const maxBatchCases = 20
	if len(req.Cases) == 0 {
		http.Error(w, "at least one case is required", http.StatusBadRequest)
		return
	}
	if len(req.Cases) > maxBatchCases {
		req.Cases = req.Cases[:maxBatchCases]
	}

	cfg := h.loadLangs()[req.Language]
	if cfg == nil {
		http.Error(w, "unsupported language: "+req.Language, http.StatusBadRequest)
		return
	}

	if !checkQuota(w, h.runQuota, claims.UserID) {
		return
	}

	results := make([]CustomRunResponse, 0, len(req.Cases))
	for _, c := range req.Cases {
		results = append(results, h.runOnce(r.Context(), CustomRunRequest{
			SourceCode:    req.SourceCode,
			Language:      req.Language,
			Input:         c.Input,
			Expected:      c.Expected,
			TimeLimitMs:   req.TimeLimitMs,
			MemoryLimitKB: req.MemoryLimitKB,
		}, cfg))
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"results": results})
}

// runShellCommand builds the single shell command that compiles and runs the
// submission inside the sandbox.
//
// The compile step redirects its own stderr into error.txt. Without that the
// compiler's diagnostics went to the shell's stderr, which the sandbox does not
// return — only files named in CopyOut come back. On a failed compile `&&` then
// short-circuits, so error.txt was never even created and the IDE reported
// "Compilation failed (no error output captured)" instead of the real error.
// The braces group the redirect so it also covers compound compile commands.
func runShellCommand(cfg *compiler.LangConfig, srcName string) string {
	fill := func(s string) string {
		s = strings.ReplaceAll(s, "{{exe}}", "Main")
		s = strings.ReplaceAll(s, "{{src}}", srcName)
		return strings.ReplaceAll(s, "{{dir}}", ".")
	}

	const captureRun = " < input.txt > output.txt 2> error.txt"

	if cfg.CompileCmd != "" {
		compile := "{ " + fill(cfg.CompileCmd) + " ; } 2> error.txt && "
		if cfg.Runtime != "" {
			return compile + strings.Join(strings.Fields(fill(cfg.Runtime)), " ") + captureRun
		}
		return compile + "./Main" + captureRun
	}
	if cfg.Runtime != "" {
		return strings.Join(strings.Fields(fill(cfg.Runtime)), " ") + " " + srcName + captureRun
	}
	return "./Main" + captureRun
}

// runOnce performs one compile-and-execute. Split out of CustomRun so BatchRun
// can reuse it without re-running the auth, decode and quota preamble per case.
func (h *SubmissionHandler) runOnce(ctx context.Context, req CustomRunRequest, cfg *compiler.LangConfig) CustomRunResponse {
	compileOutput := ""
	copyIn := map[string]executor.CmdFile{}
	var args []string

	srcName := "Main" + cfg.Extensions[0]
	copyIn[srcName] = executor.CmdFile{Content: req.SourceCode}
	copyIn["input.txt"] = executor.CmdFile{Content: req.Input}

	args = []string{"/bin/sh", "-c", runShellCommand(cfg, srcName)}

	// Client-supplied limits are honored only up to hard ceilings (2x the
	// defaults) so a single user cannot pin shared judge workers for minutes.
	const maxTimeLimitMs = 10 * 1000    // 10s
	const maxMemoryLimitKB = 512 * 1024 // 512MB
	cpuNs := uint64(5.0 * 1e9 * cfg.TimeLimitMultiplier)
	memBytes := uint64(256.0 * 1024 * 1024 * cfg.MemoryLimitMultiplier)
	if req.TimeLimitMs > 0 {
		if req.TimeLimitMs > maxTimeLimitMs {
			req.TimeLimitMs = maxTimeLimitMs
		}
		cpuNs = uint64(req.TimeLimitMs) * 1e6
	}
	if req.MemoryLimitKB > 0 {
		if req.MemoryLimitKB > maxMemoryLimitKB {
			req.MemoryLimitKB = maxMemoryLimitKB
		}
		memBytes = uint64(req.MemoryLimitKB) * 1024
	}

	start := time.Now()
	resp, err := h.exec.Run(ctx, &executor.ExecRequest{
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
		return CustomRunResponse{
			Status:        "SE",
			CompileOutput: "execution client error: " + err.Error(),
		}
	}

	if len(resp) == 0 {
		return CustomRunResponse{
			Status:        "SE",
			CompileOutput: "no execution response received",
		}
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
		return CustomRunResponse{
			Status:        "CE",
			CompileOutput: compileOutput,
		}
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

	return CustomRunResponse{
		Status:        status,
		Stdout:        stdout,
		Stderr:        stderr,
		TimeUsed:      elapsed,
		MemoryUsed:    int(cr.Memory / 1024),
		CompileOutput: compileOutput,
		Passed:        compareOutput(status, stdout, req.Expected),
		Expected:      req.Expected,
	}
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

// canSeeJudgeInternals reports whether the caller may read the judge's failure
// text for a submission. Unlike hasSubmissionAccess, the submitter is excluded:
// a system error is the platform's problem, not theirs to debug.
func (h *SubmissionHandler) canSeeJudgeInternals(r *http.Request, sub *model.Submission) bool {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		return false
	}
	if claims.Role == "admin" {
		return true
	}
	if h.probStore.HasAccess(r.Context(), sub.ProblemID, claims.UserID, "owner", "co_author") {
		return true
	}
	if sub.ContestID != "" {
		return h.contestStore.HasAccess(r.Context(), sub.ContestID, claims.UserID, "manager", "judge")
	}
	return false
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
	// A problem's own setters review the solutions submitted to it.
	return h.probStore.HasAccess(r.Context(), sub.ProblemID, claims.UserID, "owner", "co_author")
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
	if err != nil || prob == nil || !model.IsRemotePlatform(prob.Source) {
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
		go func() {
			// The request context is cancelled the moment this handler returns, which
			// killed the poll before it made a single HTTP call. Detach from it and
			// impose our own deadline instead.
			ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), remotePollTimeout)
			defer cancel()
			h.vjudgeSvc.ForcePoll(ctx, prob.Source, sub.RemoteID, sub.ID)
		}()
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "refreshing"})
}

// requireContestSubmissionAccess decides whether the caller may act on a contest
// problem at all (submit, or /run it). It writes the response and returns an error
// when access is refused, so callers just `if err != nil { return }`.
//
// Kept separate from requireContestAccess because a refused submission has to
// distinguish "unlock it" from "forbidden": a contestant who simply has not typed
// the password yet needs a 403 password_required so the UI can prompt, whereas a
// non-member of a restricted group gets a flat refusal they cannot act on.
func (h *SubmissionHandler) requireContestSubmissionAccess(w http.ResponseWriter, r *http.Request, contest *model.Contest) error {
	decision, err := h.access.Decide(r.Context(), contest, middleware.GetUserClaims(r))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return err
	}
	switch {
	case decision.Allowed():
		return nil
	case decision.NeedsPassword():
		respondJSON(w, http.StatusForbidden, map[string]string{
			"error": "password_required",
			"code":  "password_required",
		})
		return errContestAccessDenied
	default:
		http.Error(w, "forbidden", http.StatusForbidden)
		return errContestAccessDenied
	}
}
