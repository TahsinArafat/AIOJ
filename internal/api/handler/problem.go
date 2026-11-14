package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/fps"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/notify"
	"github.com/tahsinarafat/aioj/internal/store"
	"github.com/tahsinarafat/aioj/internal/webhook"
)

type ProblemHandler struct {
	store store.ProblemStore
	// userStore resolves a collaborator reference to an account. Optional so the
	// seeder and tests can build a handler without one; a nil store means only a
	// literal UUID can be added, never a username.
	userStore userRefLookup
	// notify and hooks are optional: nil instances are no-ops, so constructing a
	// handler without them (tests, the seeder) still works.
	notify *notify.Notifier
	hooks  *webhook.Dispatcher
}

func NewProblemHandler(s store.ProblemStore) *ProblemHandler {
	return &ProblemHandler{store: s}
}

// WithUsers attaches the user store the collaborator form needs to turn a typed
// username into an account id. Returns the handler for call-site chaining.
func (h *ProblemHandler) WithUsers(us userRefLookup) *ProblemHandler {
	h.userStore = us
	return h
}

// WithNotifier attaches a notifier and returns the handler, for call-site chaining.
func (h *ProblemHandler) WithNotifier(n *notify.Notifier) *ProblemHandler {
	h.notify = n
	return h
}

// WithWebhooks attaches a webhook dispatcher and returns the handler.
func (h *ProblemHandler) WithWebhooks(d *webhook.Dispatcher) *ProblemHandler {
	h.hooks = d
	return h
}

// initialStatus is the review state a newly created problem starts in. Only an
// admin may publish directly; a setter's problem starts as a draft and reaches
// the archive through the review queue. visible is derived from status by the
// store, so this one decision is the whole publish policy for creation.
func initialStatus(requested bool, role string) string {
	if requested && role == "admin" {
		return model.ProblemStatusPublished
	}
	return model.ProblemStatusDraft
}

// slugFromTitle derives a URL segment from a problem title.
//
// The setter's create screen asks for a name and nothing else, so the slug can
// no longer come from the client. Anything that is not a letter or digit folds
// to a hyphen, and a title with nothing usable left (CJK, emoji) falls back to a
// random suffix rather than an empty segment.
func slugFromTitle(title string) string {
	var b strings.Builder
	lastHyphen := true // leading separators are dropped
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen {
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 60 {
		slug = strings.Trim(slug[:60], "-")
	}
	if slug == "" {
		// A non-Latin title ("素数筛法") yields no ASCII at all; the slug still has
		// to be unique and non-empty, and the title stays intact on the problem.
		return "problem-" + strings.Split(uuid.New().String(), "-")[0]
	}
	return slug
}

func (h *ProblemHandler) List(w http.ResponseWriter, r *http.Request) {
	// A contestant account exists for one round and is not an archive citizen.
	// Answered as an empty page rather than 403 so the SPA renders its normal
	// "no problems" state instead of an error banner it has no copy for.
	if isArchiveBlocked(middleware.GetUserClaims(r)) {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"data": []model.ProblemListItem{}, "total": 0, "offset": 0, "limit": 0,
		})
		return
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	difficulty := r.URL.Query().Get("difficulty")
	search := r.URL.Query().Get("search")
	source := r.URL.Query().Get("source")
	rating := r.URL.Query().Get("rating")
	sortBy := r.URL.Query().Get("sort")
	var filterTags []string
	if tagsStr := r.URL.Query().Get("tags"); tagsStr != "" {
		filterTags = strings.Split(tagsStr, ",")
	}

	// The archive is published-only for everyone except an admin, who sees every
	// problem regardless of review state (drafts included). This is the one
	// listing surface, so deciding it here covers the plain archive, the
	// filtered archive, and the setter workspace's "All Problems" tab.
	admin := isAdmin(middleware.GetUserClaims(r))

	var items []model.ProblemListItem
	var total int
	var err error

	if difficulty != "" || len(filterTags) > 0 || search != "" || source != "" || rating != "" || sortBy != "" {
		items, total, err = h.store.ListWithFilter(r.Context(), offset, limit, admin, difficulty, filterTags, search, source, rating, sortBy)
	} else {
		items, total, err = h.store.List(r.Context(), offset, limit, admin)
	}

	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":   items,
		"total":  total,
		"offset": offset,
		"limit":  limit,
	})
}

func (h *ProblemHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.store.GetAllTags(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": tags})
}

func (h *ProblemHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	p, err := h.store.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	claims := middleware.GetUserClaims(r)
	// Privacy guard: a draft is its authors' business. Kept as 403 rather than
	// 404 because this endpoint has always answered that way for a private slug.
	if !canReadProblem(r.Context(), h.store, p, claims) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	// A problem that is still in play in a contest is not part of the archive: it
	// belongs to its contest until the round is over, the way Codeforces keeps a
	// round's problems out of the problem set. Answering 404 rather than a blanked
	// copy also stops this endpoint being a second way to reach it at all.
	//
	// The people who author or test it keep access — they are not the audience the
	// round is measuring — and so does an admin.
	if !canSeeSpoilers(r.Context(), h.store, p, claims) && problemIsLive(r.Context(), h.store, p) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, p)
}

// defaultLimit returns v when it is a usable limit, otherwise fallback.
//
// The limits are the sandbox's CPU and memory ceilings, so a zero or negative
// value is not a valid setting: it makes every submission on the problem exceed
// its limit instantly. The forms render these as plain number inputs, and an
// input cleared mid-edit serializes as 0.
func defaultLimit(v, fallback int) int {
	if v > 0 {
		return v
	}
	return fallback
}

// maxProblemMemoryKB is the largest memory limit a problem may carry: 2 GiB.
//
// defaultLimit only guards the low end, so before this an admin (or an API
// client) could store any value at all. That value is handed straight to the
// sandbox as a cgroup memory limit, so an oversized one asks the host for RAM it
// does not have — the judge then either fails the submission or takes the whole
// machine down with it. The judge additionally rejects anything above its own
// JUDGE_MAX_JUDGMENT_MEM_MB budget, but a problem should not be storable in a
// shape that can never be judged.
const maxProblemMemoryKB = 2097152 // 2 GiB

// memoryLimitError reports why a memory limit is unusable, or "" when it is fine.
// Returns "" for non-positive values: those are the "input cleared mid-edit" case
// defaultLimit already handles, not a deliberate oversized request.
func memoryLimitError(kb int) string {
	if kb > maxProblemMemoryKB {
		return "memory_limit must be at most 2097152 KB (2 GiB)"
	}
	return ""
}

func (h *ProblemHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	var req model.CreateProblemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	// Reject the enum fields before touching the row: a CHECK violation would
	// otherwise surface as an opaque 500 from the store.
	if req.ProblemType != nil && !model.ValidProblemType(*req.ProblemType) {
		http.Error(w, "invalid problem_type", http.StatusBadRequest)
		return
	}
	if req.ScoringMode != "" && !model.ValidScoringMode(req.ScoringMode) {
		http.Error(w, "invalid scoring_mode", http.StatusBadRequest)
		return
	}
	if req.SubtaskAggregation != "" && !model.ValidSubtaskAggregation(req.SubtaskAggregation) {
		http.Error(w, "invalid subtask_aggregation", http.StatusBadRequest)
		return
	}

	p.Title = req.Title
	p.Description = req.Description
	// Limits are clamped, not taken verbatim. The form renders them as empty
	// inputs while a setter retypes a value, and Number('') is 0; storing that
	// handed the sandbox a zero CPU limit and every submission on the problem
	// TLE'd with no visible cause. Create has always defaulted these, so Update
	// defaulting them too keeps the two paths from disagreeing.
	p.TimeLimit = defaultLimit(req.TimeLimit, 1000)
	p.MemoryLimit = defaultLimit(req.MemoryLimit, 262144)
	if msg := memoryLimitError(p.MemoryLimit); msg != "" {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	p.Difficulty = req.Difficulty
	p.Rating = max(req.Rating, 0)
	p.InputFormat = req.InputFormat
	p.OutputFormat = req.OutputFormat
	p.Constraints = req.Constraints
	p.Hint = req.Hint
	p.SampleCases = req.SampleCases
	p.TestCaseScore = req.TestCaseScore
	p.Tags = req.Tags
	p.SPJ = req.SPJ
	p.SPJLanguage = req.SPJLanguage
	p.SPJSourceCode = req.SPJSourceCode
	p.CheckerType = req.CheckerType
	p.FloatEpsilon = req.FloatEpsilon
	// Editing must not publish. Before this, an owner could send visible:true
	// here and put a draft in the archive without any review — the queue would
	// have been decorative. Only an admin's edit may change the state, and
	// publishing is still an explicit admin action (SetReviewState), so a
	// non-admin's visible flag is ignored outright. p.Status carries the
	// existing value for everybody else, and the store derives visible from it.
	//
	// `visible` is a *bool because most saves never mention it: the statement
	// form does not carry a visibility control, so omitting it read as false and
	// every admin's typo fix silently unpublished a published problem.
	if claims.Role == "admin" && req.Visible != nil {
		if *req.Visible {
			p.Status = model.ProblemStatusPublished
		} else if p.Status == model.ProblemStatusPublished {
			p.Status = model.ProblemStatusDraft
		}
	}
	// interactive and problem_type have to agree or the same problem is
	// interactive for the judge and standard for everyone reading the enum. An
	// empty resolution means the request said nothing about the type, so the
	// stored value stands.
	if req.Interactive != nil {
		p.Interactive = *req.Interactive
	}
	if resolved := model.ResolveProblemType(req.ProblemType, req.Interactive); resolved != "" {
		p.ProblemType = resolved
	}
	if req.ScoringMode != "" {
		p.ScoringMode = req.ScoringMode
	}
	if req.SubtaskAggregation != "" {
		p.SubtaskAggregation = req.SubtaskAggregation
	}
	p.InteractorLanguage = req.InteractorLanguage
	p.InteractorSourceCode = req.InteractorSourceCode
	if err := h.store.Update(r.Context(), p.ID, p); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ProblemHandler) Delete(w http.ResponseWriter, r *http.Request) {
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
	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), p.ID, claims.UserID, "owner") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if p.TestdataPath != "" {
		if err := os.RemoveAll(p.TestdataPath); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	if err := h.store.Delete(r.Context(), p.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// canAuthorProblem reports whether the caller may author this problem's data.
// Distinct from canReadProblem in problem_view.go: that one answers "may I see
// the statement", this one "may I take the test data away with me".
func (h *ProblemHandler) canAuthorProblem(r *http.Request, p *model.Problem) bool {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		return false
	}
	if claims.Role == "admin" || p.CreatedBy == claims.UserID {
		return true
	}
	return h.store.HasAccess(r.Context(), p.ID, claims.UserID, "owner", "co_author")
}

func (h *ProblemHandler) Export(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	prob, err := h.store.GetBySlug(r.Context(), slug)
	if err != nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}
	if prob == nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}
	// An export is the whole problem — statement plus every test file — so it is
	// the most valuable thing a private draft can leak. Authoring rights, not
	// mere authentication: the route's feature gate admits any signed-in user.
	if !canReadProblem(r.Context(), h.store, prob, middleware.GetUserClaims(r)) ||
		!h.canAuthorProblem(r, prob) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	xmlBytes, err := fps.GenerateXML([]*model.Problem{prob})
	if err != nil {
		http.Error(w, "failed to generate FPS XML: "+err.Error(), http.StatusInternalServerError)
		return
	}

	zipBuf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(zipBuf)

	xmlFile, err := zipWriter.Create("problem.xml")
	if err != nil {
		http.Error(w, "failed to create XML in ZIP: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := xmlFile.Write(xmlBytes); err != nil {
		http.Error(w, "failed to write XML to ZIP: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if prob.TestdataPath != "" {
		files, err := os.ReadDir(prob.TestdataPath)
		if err == nil {
			for _, f := range files {
				if f.IsDir() {
					continue
				}

				fPath := filepath.Join(prob.TestdataPath, f.Name())
				data, err := os.ReadFile(fPath)
				if err != nil {
					continue
				}

				zf, err := zipWriter.Create(f.Name())
				if err != nil {
					continue
				}
				if _, err := zf.Write(data); err != nil {
					continue
				}
			}
		}
	}

	if err := zipWriter.Close(); err != nil {
		http.Error(w, "failed to close ZIP writer: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+prob.Slug+".zip\"")
	w.Write(zipBuf.Bytes())
}

func (h *ProblemHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req model.CreateProblemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.ProblemType != nil && !model.ValidProblemType(*req.ProblemType) {
		http.Error(w, "invalid problem_type", http.StatusBadRequest)
		return
	}
	if req.ScoringMode != "" && !model.ValidScoringMode(req.ScoringMode) {
		http.Error(w, "invalid scoring_mode", http.StatusBadRequest)
		return
	}
	if req.SubtaskAggregation != "" && !model.ValidSubtaskAggregation(req.SubtaskAggregation) {
		http.Error(w, "invalid subtask_aggregation", http.StatusBadRequest)
		return
	}
	var slugDerived bool
	if req.Title == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}
	// The create screen is a name prompt: it hands over a title and nothing
	// else, and the statement is written in the workspace it redirects to. A
	// title-only request therefore has to produce a usable problem, which means
	// deriving the slug here and accepting an empty description (the column is
	// NOT NULL, so this has to be an explicit "").
	if req.Slug == "" {
		req.Slug = slugFromTitle(req.Title)
		slugDerived = true
	}
	req.TimeLimit = defaultLimit(req.TimeLimit, 1000)
	req.MemoryLimit = defaultLimit(req.MemoryLimit, 262144)
	if msg := memoryLimitError(req.MemoryLimit); msg != "" {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	if req.Difficulty == "" {
		req.Difficulty = "easy"
	}
	if req.CheckerType == "" {
		req.CheckerType = "exact"
	}
	if req.FloatEpsilon == 0 {
		req.FloatEpsilon = 1e-6
	}
	// A creation request may legitimately omit these; nil means "not published",
	// which is what initialStatus already falls back to.
	wantVisible := req.Visible != nil && *req.Visible
	wantInteractive := req.Interactive != nil && *req.Interactive
	// A new problem has no stored type to preserve, so an empty resolution means
	// the caller said nothing: fall back to the column default.
	problemType := model.ResolveProblemType(req.ProblemType, req.Interactive)
	if problemType == "" {
		problemType = model.ProblemTypeStandard
	}
	prob := &model.Problem{
		ID:                   uuid.New().String(),
		Slug:                 req.Slug,
		Title:                req.Title,
		Description:          req.Description,
		InputFormat:          req.InputFormat,
		OutputFormat:         req.OutputFormat,
		Constraints:          req.Constraints,
		Hint:                 req.Hint,
		TimeLimit:            req.TimeLimit,
		MemoryLimit:          req.MemoryLimit,
		Difficulty:           req.Difficulty,
		Rating:               max(req.Rating, 0),
		Tags:                 req.Tags,
		SampleCases:          req.SampleCases,
		TestCaseScore:        req.TestCaseScore,
		SPJ:                  req.SPJ,
		SPJLanguage:          req.SPJLanguage,
		SPJSourceCode:        req.SPJSourceCode,
		CheckerType:          req.CheckerType,
		FloatEpsilon:         req.FloatEpsilon,
		Interactive:          wantInteractive,
		ProblemType:          problemType,
		InteractorLanguage:   req.InteractorLanguage,
		InteractorSourceCode: req.InteractorSourceCode,
		Source:               "local",
		Status:               initialStatus(wantVisible, claims.Role),
		Visible:              wantVisible && claims.Role == "admin",
		CreatedBy:            claims.UserID,
	}
	// Empty means "let the store apply its default" rather than being written
	// over it, so an omitted field keeps the column's documented value.
	prob.ScoringMode = req.ScoringMode
	prob.SubtaskAggregation = req.SubtaskAggregation
	// A derived slug can collide: nothing stops two setters naming a problem
	// "Two Sum", and the reporter above turns that into a 409 the user cannot
	// act on from a name-only form. Suffix a counter instead, but only for
	// derived slugs — an explicitly requested slug is still the caller's to
	// resolve, so the seeder keeps seeing 409 and skipping.
	var err error
	for attempt := 0; attempt < 50; attempt++ {
		err = h.store.Create(r.Context(), prob)
		if err == nil {
			break
		}
		var pqErr *pq.Error
		if !slugDerived || !errors.As(err, &pqErr) || pqErr.Code != "23505" {
			break
		}
		prob.ID = uuid.New().String()
		prob.Slug = slugFromTitle(req.Title) + "-" + strconv.Itoa(attempt+2)
	}
	if err != nil {
		// A duplicate slug is a client-level conflict, not a server fault. The
		// seeder (and any retrying caller) treats 409 as "already exists, carry
		// on", but this used to leak the raw pq 23505 as a 500 — which made
		// `make sim-seed` fail on every re-run against an existing database.
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			http.Error(w, "problem already exists", http.StatusConflict)
			return
		}
		// Surface the real cause instead of a bare "create failed": this handler
		// is the single entry point for problem creation (incl. the seeder), and
		// a silent 500 here makes the failure impossible to diagnose from logs.
		slog.Error("create problem failed", "slug", prob.Slug, "slug_id", prob.ID, "error", err)
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusCreated, prob)
}

func (h *ProblemHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	p, err := h.store.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	// Who collaborates on a draft is itself private, and answering 403 for
	// hidden slugs versus 200 for visible ones would make this an existence
	// oracle for unpublished problems.
	if !canReadProblem(r.Context(), h.store, p, middleware.GetUserClaims(r)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	perms, err := h.store.GetPermissions(r.Context(), p.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if perms == nil {
		perms = []model.ProblemPermission{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": perms})
}

func (h *ProblemHandler) AddPermission(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	p, err := h.store.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), p.ID, claims.UserID, "owner") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		UserID string `json:"user_id"`
		// Username is what the collaborator form actually sends. The handler only
		// read user_id, so the field arrived empty and the insert put "" into a
		// uuid column: "invalid input syntax for type uuid" and a bare 500, every
		// time. Both keys are accepted so a raw id stays valid input.
		Username string `json:"username"`
		Level    string `json:"access_level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	// The level reaches a CHECK constraint, so an unknown one is a client mistake
	// and must answer 400 rather than a 500 that says nothing.
	switch req.Level {
	case model.AccessLevelOwner, model.AccessLevelCoAuthor, model.AccessLevelTester:
	default:
		http.Error(w, "access_level must be owner, co_author or tester", http.StatusBadRequest)
		return
	}

	// Resolve the reference to a real account before it reaches a uuid column.
	// A typed username is the common case here, so "no such user" is a 404 and only
	// a genuine lookup failure stays a 500.
	userID, err := resolveUserRef(r.Context(), h.userStore, req.Username, req.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if userID == "" {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if err := h.store.AddPermission(r.Context(), p.ID, userID, req.Level); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ProblemHandler) RemovePermission(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	p, err := h.store.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), p.ID, claims.UserID, "owner") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	targetUserID := chi.URLParam(r, "userId")
	if err := h.store.RemovePermission(r.Context(), p.ID, targetUserID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ProblemHandler) ListMyProblems(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	items, total, err := h.store.ListByCreatedBy(r.Context(), claims.UserID, offset, limit)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

// ListSelectable backs the contest problem picker. It answers the public archive
// plus the caller's own and shared problems, so a setter can attach a problem
// they have not published yet — which the plain archive listing can never show
// them, because an unpublished problem is by definition not in the archive.
func (h *ProblemHandler) ListSelectable(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	items, total, err := h.store.ListSelectable(r.Context(), claims.UserID, offset, limit, r.URL.Query().Get("search"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

// SubmitForReview moves an authored problem into the admin queue. Authorship is
// required; a rejected problem may be resubmitted once the author has fixed it,
// which is the point of keeping 'rejected' distinct from 'draft'.
func (h *ProblemHandler) SubmitForReview(w http.ResponseWriter, r *http.Request) {
	p, err := h.store.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil || p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !h.canAuthorProblem(r, p) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if p.Status == model.ProblemStatusPending {
		http.Error(w, "already awaiting review", http.StatusConflict)
		return
	}
	if p.Status == model.ProblemStatusPublished {
		http.Error(w, "already published", http.StatusConflict)
		return
	}
	if err := h.store.SetReviewState(r.Context(), p.ID, model.ProblemStatusPending, "", ""); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ReviewQueue lists problems awaiting an admin. Admin-only: it is the moderation
// surface, and every other state is visible to its author through /problems/my.
func (h *ProblemHandler) ReviewQueue(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = model.ProblemStatusPending
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	items, total, err := h.store.ListByStatus(r.Context(), status, offset, limit)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

// ReviewProblem records an admin verdict: publish, reject, or send back to draft.
// This is the only path that can publish a setter's problem.
func (h *ProblemHandler) ReviewProblem(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	switch req.Status {
	case model.ProblemStatusPublished, model.ProblemStatusRejected, model.ProblemStatusDraft:
	default:
		http.Error(w, "status must be published, rejected or draft", http.StatusBadRequest)
		return
	}

	p, err := h.store.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil || p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := h.store.SetReviewState(r.Context(), p.ID, req.Status, req.Note, claims.UserID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Tell the author. This is the notification that most needed to exist: a
	// setter submits a problem and otherwise has to poll the archive to learn
	// whether an admin ever looked at it.
	title := "Your problem was " + req.Status
	content := p.Title
	if req.Note != "" {
		content += " — " + req.Note
	}
	h.notify.Notify(r.Context(), p.CreatedBy, "problem_review", title, content,
		"/problems/"+p.Slug, notify.CategorySystem)
	h.hooks.Dispatch(r.Context(), webhook.Event{
		Type: "problem.reviewed",
		Payload: map[string]any{
			"slug": p.Slug, "title": p.Title, "status": req.Status, "note": req.Note,
		},
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": req.Status})
}
