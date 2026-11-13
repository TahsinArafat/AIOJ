package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/contest/format"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/acm"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/atcoder"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/codeforces"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/ioi"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/oi"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/notify"
	pdfgen "github.com/tahsinarafat/aioj/internal/pdf"
	"github.com/tahsinarafat/aioj/internal/rating"
	"github.com/tahsinarafat/aioj/internal/store"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
	"github.com/tahsinarafat/aioj/internal/webhook"
)

type scoreboardCacheEntry struct {
	data      []byte
	expiresAt time.Time
}

type scoreboardCache struct {
	mu      sync.RWMutex
	entries map[string]scoreboardCacheEntry
	ttl     time.Duration
}

func newScoreboardCache(ttl time.Duration) *scoreboardCache {
	return &scoreboardCache{entries: make(map[string]scoreboardCacheEntry), ttl: ttl}
}

func (c *scoreboardCache) get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[key]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.data, true
}

func (c *scoreboardCache) set(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = scoreboardCacheEntry{data: data, expiresAt: time.Now().Add(c.ttl)}
}

// invalidate drops every cached view of one contest.
//
// Keys are `contestID:view:page:limit:judge`, and a judge, a competitor and a
// paginated reader all hold separate entries — so anything that changes the
// standings (an adjustment, a freeze edit) has to clear them all or one visitor
// keeps seeing the pre-correction board until the TTL expires.
func (c *scoreboardCache) invalidate(contestID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prefix := contestID + ":"
	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
}

type ContestHandler struct {
	// contestHandlerAccess embeds the shared access policy. ContestHandler
	// satisfies contestAccessStore directly (it holds the contest store), so the
	// policy is built in NewContestHandler.
	contestHandlerAccess
	store        *postgres.ContestStore
	ratingStore  *postgres.RatingStore
	problemStore store.ProblemStore
	userStore    store.UserStore
	teamStore    store.TeamStore
	cache        *scoreboardCache
	// adjustments holds the manual corrections judges apply to a round. Optional
	// so tests can build the handler without one; a nil store means corrections
	// simply do not exist, which is the pre-feature behaviour.
	adjustments store.ContestAdjustmentStore
	// notify and hooks are optional; nil instances are no-ops.
	notify *notify.Notifier
	hooks  *webhook.Dispatcher
	// feedStore and feedToken back the ICPC-style event feed. Both are optional:
	// with no token the endpoint answers 503 rather than exposing the feed.
	feedStore feedLister
	feedToken string
	// grants is the password-entitlement store. Optional so tests can build the
	// handler without one; a nil store means no round can be unlocked, which
	// fails closed for password-protected contests.
	grants *postgres.ContestGrantStore
}

// WithFeed enables the event feed. An empty token leaves it disabled.
func (h *ContestHandler) WithFeed(store feedLister, token string) *ContestHandler {
	h.feedStore = store
	h.feedToken = token
	return h
}

// WithWebhooks attaches a webhook dispatcher and returns the handler.
func (h *ContestHandler) WithWebhooks(d *webhook.Dispatcher) *ContestHandler {
	h.hooks = d
	return h
}

// WithNotifier attaches a notifier and returns the handler, for call-site chaining.
func (h *ContestHandler) WithNotifier(n *notify.Notifier) *ContestHandler {
	h.notify = n
	return h
}

func NewContestHandler(s *postgres.ContestStore, rs *postgres.RatingStore, ps store.ProblemStore, us store.UserStore, ts store.TeamStore) *ContestHandler {
	h := &ContestHandler{store: s, ratingStore: rs, problemStore: ps, userStore: us, teamStore: ts, cache: newScoreboardCache(30 * time.Second)}
	h.access = newContestAccessPolicy(s, nil)
	return h
}

// WithGrants attaches the password-entitlement store and rebuilds the access
// policy so it can consult it. Returns the handler for call-site chaining, matching
// WithAdjustments/WithFeed/WithNotifier.
func (h *ContestHandler) WithGrants(gs *postgres.ContestGrantStore) *ContestHandler {
	h.grants = gs
	h.access = newContestAccessPolicy(h.store, gs)
	return h
}

// WithAdjustments attaches the manual-correction ledger and returns the handler,
// for call-site chaining.
func (h *ContestHandler) WithAdjustments(as store.ContestAdjustmentStore) *ContestHandler {
	h.adjustments = as
	return h
}

// ScoreboardCache exposes the cache so the adjustment handler can drop a round's
// cached standings the moment it writes a correction.
func (h *ContestHandler) ScoreboardCache() *scoreboardCache { return h.cache }

// adjustmentsFor loads the manual corrections for a round. A handler built
// without the ledger (tests, older wiring) simply has none.
func (h *ContestHandler) adjustmentsFor(r *http.Request, contestID string) (map[string]model.AdjustmentTotals, error) {
	if h.adjustments == nil {
		return nil, nil
	}
	return h.adjustments.TotalsByUser(r.Context(), contestID)
}

// adjustmentFor reports the deltas applied to one participant, or nil when the
// round has not corrected them.
func adjustmentFor(adjusted map[string]model.AdjustmentTotals, userID string) *model.AdjustmentTotals {
	if adj, ok := adjusted[userID]; ok {
		return &adj
	}
	return nil
}

// initialContestStatus mirrors the problem rule: an admin publishes directly,
// everyone else proposes. A setter announcing a rated round to the public
// without review is exactly what the queue exists to prevent.
func initialContestStatus(role string) string {
	if role == "admin" {
		return model.ContestStatusPublished
	}
	return model.ContestStatusDraft
}

// derefBool/derefInt apply a default when an optional request field is absent.
// The create path needs them because the request DTO marks these fields optional
// to keep update's "omitted means unchanged" semantics.
func derefBool(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}

func derefInt(v *int, fallback int) int {
	if v == nil {
		return fallback
	}
	return *v
}

func (h *ContestHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if claims.Role != "admin" && claims.Role != "setter" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req model.CreateContestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		req.Type = "acm"
	}

	fmtName := req.Format
	if fmtName == "" {
		fmtName = "acm"
	}
	var formatConfigJSON []byte
	if len(req.FormatConfig) > 0 {
		if _, err := format.Create(fmtName, req.FormatConfig); err != nil {
			http.Error(w, "invalid format config: "+err.Error(), http.StatusBadRequest)
			return
		}
		formatConfigJSON = req.FormatConfig
	} else {
		factory, ok := format.Get(fmtName)
		if !ok {
			http.Error(w, "unknown format: "+fmtName, http.StatusBadRequest)
			return
		}
		f, _ := factory(nil)
		formatConfigJSON = f.DefaultConfig()
	}

	// The password is hashed at the boundary, exactly once, and only the hash goes
	// any further. Before this the plaintext was stored and never compared to
	// anything, so a "password protected" round was readable by anyone.
	passwordHash, err := hashContestPassword(req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Every surface audience is validated at the boundary, like every other enum on
	// this DTO: a bad key or value becomes a 400 with a reason rather than a
	// silently-dropped entry. An omitted map on create means "every surface inherits",
	// which is the behaviour every round had before this setting existed.
	accessConfig, err := validateAccessConfig(req.AccessConfig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c := &model.Contest{
		ID:                   uuid.New().String(),
		Slug:                 req.Slug,
		Title:                req.Title,
		Type:                 req.Type,
		Format:               fmtName,
		FormatConfig:         formatConfigJSON,
		StartTime:            req.StartTime,
		EndTime:              req.EndTime,
		FreezeTime:           req.FreezeTime,
		PasswordHash:         passwordHash,
		Description:          req.Description,
		Status:               initialContestStatus(claims.Role),
		Visible:              claims.Role == "admin",
		AccessConfig:         accessConfig,
		GroupID:              req.GroupID,
		CreatedBy:            claims.UserID,
		RegistrationRequired: derefBool(req.RegistrationRequired, false),
		RegistrationDeadline: req.RegistrationDeadline,
		MaxParticipants:      req.MaxParticipants,
		Division:             derefInt(req.Division, model.DivisionNone),
		// A new round is rated by default: that is what the schema default
		// (is_rated=true) and every competitive round mean. An omitted field on
		// create therefore must not silently produce an unrated contest.
		IsRated:     derefBool(req.IsRated, true),
		RatedForMin: derefInt(req.RatedForMin, 0),
		RatedForMax: derefInt(req.RatedForMax, 9999),
	}

	if err := h.store.Create(r.Context(), c); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	// Attach the problems that survived the permission check, then derive the
	// letters from the result. Numbering the request as it is walked would leave
	// a gap wherever a problem was skipped — the exact drift that made the
	// problems tab and the standings disagree about which problem is "A".
	attached := 0
	for _, pid := range req.ProblemIDs {
		// Skip problems this creator may not use rather than failing the whole
		// contest: a bad id must not cost the round, and silently attaching
		// somebody else's draft would leak it through the contest endpoint.
		if p, err := h.problemStore.GetByID(r.Context(), pid); err == nil && p != nil &&
			canReadProblem(r.Context(), h.problemStore, p, claims) {
			if err := h.store.AddProblem(r.Context(), c.ID, pid,
				model.ContestProblemIndex(attached), 100, attached); err == nil {
				attached++
			}
		}
	}
	// Derive the letters from the attached set, so a skipped problem cannot
	// leave a hole.
	h.store.RenumberProblems(r.Context(), c.ID)
	respondJSON(w, http.StatusCreated, c)
}

func (h *ContestHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	c, err := h.store.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if c == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// One gate replaces the three hand-written ones that used to live here (group
	// membership, visible/started, and clearing the password field). They were
	// enforced here and nowhere else, which is why the statements, the booklet, the
	// standings and submit stayed open on a round this function refused.
	//
	// This one surface asks for the *shell*: an upcoming round must render its
	// title, its times and a countdown, and refusing it here is what showed
	// "Contest not found" on a contest that existed and had merely not begun.
	viewer, ok := h.requireContestPageAccess(w, r, c, false)
	if !ok {
		return
	}
	claims := viewer.access.claims
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}
	isManager := viewer.access.decision.IsStaff()
	upcoming := viewer.access.decision.IsUpcoming()
	now := time.Now()

	// The problem list is part of the round's content, not its shell, so it waits
	// for the start. Titles and letters are exactly what a round keeps secret
	// before it opens, and it would be incoherent to publish them while every
	// statement behind them refuses to open. An empty slice, not nil, so the wire
	// shape stays the same for a client that expects a list.
	//
	// Whether to serve it at all is the problem-list surface's own answer, already
	// resolved by requireContestPageAccess — including the case this page would
	// otherwise get wrong, where the round publishes its list and nothing else, so
	// the page opens for the list while the round's content stays closed. Deciding
	// it there rather than here is what keeps the page's admission rule and its
	// body in agreement: two computations of one question is how a page ends up
	// admitting a caller and then rendering them nothing.
	problems := []model.ContestProblem{}
	if viewer.showProblems {
		problems, _ = h.store.GetProblems(r.Context(), c.ID)
	}

	// Per-problem solve state for the problem list: a green tick for the problems
	// this viewer has solved, and the round-wide solve count so they can see what
	// the field is finding easy. The list otherwise renders a bare "Solve"
	// button with no indication of progress.
	//
	// During a standings freeze the public count is cut at the freeze instant,
	// for the same reason the scoreboard is: a live "solved by N" beside a frozen
	// board leaks the very solves the freeze is hiding. Staff keep the live view,
	// matching what the scoreboard does for them.
	//
	// Anonymous callers still get the counts; viewerID is simply empty, which the
	// query treats as "nobody", so My* come back false and are omitted.
	var countCutoff *time.Time
	if c.FreezeTime != nil && now.After(*c.FreezeTime) && now.Before(c.EndTime) && !isManager {
		countCutoff = c.FreezeTime
	}
	if progress, err := h.store.GetProblemProgress(r.Context(), c.ID, userID, countCutoff); err == nil {
		for i := range problems {
			p := progress[problems[i].ProblemID]
			problems[i].SolvedCount = p.SolvedCount
			problems[i].MySolved = p.MySolved
			problems[i].MyAttempted = p.MyAttempted
			problems[i].MySubmissions = p.MySubmissions
		}
	}

	ratings, _ := h.ratingStore.GetByContest(r.Context(), c.ID)
	c.RatingCalculated = len(ratings) > 0

	// An upcoming round reports that it has no password, whatever the truth is.
	// The policy deliberately answers "not started" before it looks at the
	// password, so that a round which has not begun does not reveal whether it is
	// invite-only — and this field would give that away in the shell. Staff are
	// unaffected: they are granted at an earlier step, so `upcoming` is false for
	// them and the setter workspace still sees the real value.
	if upcoming && !isManager {
		c.HasPassword = false
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"contest":  c,
		"problems": problems,
		// The page has to know whether to offer the management entry point, and
		// the only fact it had was the JWT role. That left a contest's own judges,
		// managers and testers — who run the round without being admins or setters
		// — with no way to reach the panel, so the "Manage Contest" button appeared
		// for admins only. These are the same two verdicts the access policy
		// already computed, so the page and the server cannot disagree.
		"is_staff": isManager,
		"is_judge": viewer.access.decision.IsJudge(),
	})
}

func (h *ContestHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	var division *int
	if divStr := r.URL.Query().Get("division"); divStr != "" {
		d, err := strconv.Atoi(divStr)
		if err == nil {
			division = &d
		}
	}

	// The listing is viewer-aware: an invite-only or group-restricted round is
	// omitted unless this caller is entitled to it. Anonymous callers have no
	// grants, so they simply do not see such rounds.
	viewerID := ""
	if claims := middleware.GetUserClaims(r); claims != nil {
		viewerID = claims.UserID
	}
	items, total, _ := h.store.ListForViewer(r.Context(), offset, limit, division, viewerID)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *ContestHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	c, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if c == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), c.ID, claims.UserID, "manager") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req model.CreateContestRequest
	// Read the body once, decode it twice: the struct is how every ordinary field
	// is read, and `present` records which keys the caller actually sent. Two
	// fields are nullable in the database — registration_deadline and
	// max_participants — and for them "clear it" and "don't touch it" are different
	// requests that decode to the same nil pointer. Without this a setter could set
	// a participant cap and never remove it again: the form omits the field to mean
	// "no cap" and the server read that as "leave the old cap alone". Presence, not
	// value, is the distinction.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	var present map[string]json.RawMessage
	_ = json.Unmarshal(body, &present)
	if req.Title != "" {
		c.Title = req.Title
	}
	if req.Type != "" {
		c.Type = req.Type
	}
	if !req.StartTime.IsZero() {
		c.StartTime = req.StartTime
	}
	if !req.EndTime.IsZero() {
		c.EndTime = req.EndTime
	}
	if req.FreezeTime != nil {
		c.FreezeTime = req.FreezeTime
	}
	// Password semantics on update, matching every other optional field here:
	// nil means "leave it alone", "" means "remove the password" (which also drops
	// the grants, or the round would stay readable to whoever unlocked the old
	// secret), anything else is a new password and invalidates the old grants.
	passwordChanged := false
	if req.Password != nil {
		if *req.Password == "" {
			passwordChanged = c.PasswordHash != ""
			c.PasswordHash = ""
		} else {
			hash, herr := hashContestPassword(req.Password)
			if herr != nil {
				http.Error(w, herr.Error(), http.StatusBadRequest)
				return
			}
			passwordChanged = true
			c.PasswordHash = hash
		}
	}
	if req.Description != "" {
		c.Description = req.Description
	}
	if req.PDFEnabled != nil {
		c.PDFEnabled = *req.PDFEnabled
	}
	if req.StatementHidden != nil {
		c.StatementHidden = *req.StatementHidden
	}
	// The per-surface audiences. Presence decides, like the nullable fields above:
	// an absent key leaves the stored map alone so an unrelated settings save (a
	// rename) cannot silently republish or privatize a surface, while an explicit
	// `"access_config": {}` means "every surface back to inheriting the base gate".
	//
	// Both are real requests and both decode to a nil map from a map-typed field,
	// which is why this reads the key set rather than the value — the same trap
	// that made max_participants unclearable.
	if _, sent := present["access_config"]; sent {
		next, err := validateAccessConfig(req.AccessConfig)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// No cache invalidation here. Update drops the round's cached board
		// unconditionally, a few lines below, so a changed audience is already
		// covered — and computing whether it changed just to maybe-invalidate would
		// be a second place that has to know what the cache depends on.
		c.AccessConfig = next
	}
	if req.UpsolvingEnabled != nil {
		c.UpsolvingEnabled = *req.UpsolvingEnabled
	}
	if req.VirtualContestEnabled != nil {
		c.VirtualContestEnabled = *req.VirtualContestEnabled
	}
	// Rating controls. Create already read these; Update did not, so is_rated
	// could be chosen once and never changed — the Settings tab had no control
	// for it because the server would have dropped the value anyway. The store
	// writes all three (contests.go Update), so nothing else is needed here.
	if req.IsRated != nil {
		c.IsRated = *req.IsRated
	}
	if req.RatedForMin != nil {
		c.RatedForMin = *req.RatedForMin
	}
	if req.RatedForMax != nil {
		c.RatedForMax = *req.RatedForMax
	}
	if req.Visible != nil {
		c.Visible = *req.Visible
	}
	// Registration controls and division. The Settings tab had no control for any
	// of these, and the store's UPDATE did not write them either, so a round's
	// registration policy was frozen at whatever it was created with. Pointers
	// throughout: an omitted field means "leave it alone" — a plain bool would
	// switch registration off on every unrelated save.
	if req.RegistrationRequired != nil {
		c.RegistrationRequired = *req.RegistrationRequired
	}
	if _, sent := present["registration_deadline"]; sent {
		c.RegistrationDeadline = req.RegistrationDeadline
	}
	if _, sent := present["max_participants"]; sent {
		c.MaxParticipants = req.MaxParticipants
	}
	if req.Division != nil {
		c.Division = *req.Division
	}
	c.GroupID = req.GroupID
	if req.Slug != "" {
		c.Slug = req.Slug
	}
	if err := h.store.Update(r.Context(), c); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	// Grants were proof of the OLD secret. Revoking them is what makes rotating the
	// password meaningful: everyone who knew the previous one is back outside.
	if passwordChanged && h.grants != nil {
		if err := h.grants.RevokeAll(r.Context(), c.ID); err != nil {
			http.Error(w, "password saved but revoking previous access failed", http.StatusInternalServerError)
			return
		}
	}
	h.cache.invalidate(c.ID)
	respondJSON(w, http.StatusOK, c)
}

func (h *ContestHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	contest, err := resolveContest(r.Context(), chi.URLParam(r, "id"), h.store)
	if err != nil {
		respondContestLookupError(w, err)
		return
	}
	if err := h.store.Delete(r.Context(), contest.ID); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// scoreboardProblem is the standings header as it goes over the wire: the letter,
// the point value, and the id the cells key on. Deliberately NOT
// model.ContestProblem, which also carries Title and Slug — both of which are the
// problem's name, and neither of which a standings reader needs.
//
// A separate type is the point. The board used to serialize the store row and
// blank Title for a caller who could not read the round, which left Slug in place
// and leaked every problem's name to a signed-out visitor. Enumerating the safe
// fields here means a field added to the model later cannot appear on the board by
// default; it has to be added to this struct on purpose.
type scoreboardProblem struct {
	ProblemID string `json:"problem_id"`
	Index     string `json:"index"`
	Score     int    `json:"score"`
}

// validateAccessConfig checks a caller-supplied surface map and returns the
// normalized form, or an error naming the offending entry.
//
// Rejecting rather than dropping is the opposite of what model.NormalizeAccessConfig
// does at the store boundary, and the difference is who supplied the value. Here it
// is a user request, so a typo deserves a 400 that names it — silently dropping
// "standings: publc" would leave the setter looking at a board that is still
// private with no indication why. At the store the value may have been written by
// an older build or by hand, there is no response to write to, and falling back is
// the only option that does not make a surface unreachable.
//
// An error names the surface and the audience, not just "invalid access_config":
// the caller is a setter staring at a six-row table, and "unknown audience for
// surface 'standings'" is actionable where a bare 400 is not.
func validateAccessConfig(in model.AccessConfig) (model.AccessConfig, error) {
	out := model.AccessConfig{}
	for surface, audience := range in {
		if !model.ValidAccessSurface(surface) {
			return nil, fmt.Errorf("unknown access surface %q", surface)
		}
		if !model.ValidAccessAudience(audience) {
			return nil, fmt.Errorf("audience for surface %q must be one of %q, %q, %q",
				surface, model.AccessParticipants, model.AccessPublic, model.AccessStaff)
		}
		out[surface] = audience
	}
	return out, nil
}

func (h *ContestHandler) Scoreboard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	contest, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if contest == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	now := time.Now()
	// The board's own gate, not the content one. A round may publish its board and
	// keep its statements private — that is what an onsite final or a hiring round
	// is — and gating the board on content made publishing the result impossible
	// without publishing the problems too.
	viewer, ok := h.requireContestSurfaceAccess(w, r, contest, model.SurfaceStandings, false)
	if !ok {
		return
	}
	// IsJudge, not IsStaff: a tester may open the round but must not see past the
	// freeze or be labelled a judge. That distinction was in the original condition
	// (manager|judge) and is preserved here deliberately.
	isJudge := viewer.decision.IsJudge()
	canSeeJudge := isJudge // preserve original capability before view override
	// Allow admins/judges to explicitly request the public (frozen) view via ?view=public
	viewParam := r.URL.Query().Get("view")
	if viewParam == "public" && isJudge {
		isJudge = false
	}

	// Check cache (key: contestID:view:page:limit:judge:canSeeJudge)
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	// isJudge belongs in the key: a judge sees past the freeze and gets
	// is_judge:true, so without it the first judge to load the page seeded the
	// unfrozen board for every anonymous visitor until the entry expired.
	//
	// canSeeJudge belongs in the key and is the one that was missed. It reports
	// whether the CALLER may switch to the judge view, and it survives the
	// ?view=public override above: a judge asking for the public board gets
	// is_judge=false with can_see_judge=true, while an anonymous spectator on a
	// public round gets the same is_judge=false, the same view=public, and the
	// same canReadRound. Every other component of the key matched, so the judge's
	// entry was served to anonymous visitors, and the standings page rendered its
	// "Judge View" toggle to people who had no judge privilege at all. Verified
	// against a live round: an anonymous request for ?view=public returned
	// can_see_judge=true from a HIT.
	//
	// contentAllowed used to be a component too, back when it decided whether the
	// problem titles were in the body. The board now carries letters only for every
	// caller (see boardProblems below), so it no longer varies the response and the
	// slot is gone: keeping it would have stored the same bytes twice under two
	// keys, for two callers who now get identical boards.
	cacheKey := contest.ID + ":" + viewParam + ":" + pageStr + ":" + limitStr + ":" +
		strconv.FormatBool(isJudge) + ":" + strconv.FormatBool(canSeeJudge)
	if cached, ok := h.cache.get(cacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(cached)
		return
	}

	problems, _ := h.store.GetProblems(r.Context(), contest.ID)

	// The board carries letters, never names.
	//
	// A title is one field away from the statement — and so is its slug, which is
	// derived from the title at create time and is therefore the name in URL
	// clothing. Blanking Title alone was not enough: the response still serialized
	// slug, so a signed-out visitor could read every problem's name straight off
	// the standings JSON. Confirmed on the live round, where an anonymous GET
	// returned {"index":"B","slug":"common-binary-harvest"} and the rest.
	//
	// So model.ContestProblem is no longer serialized here at all. The response
	// carries a narrow projection of the three fields a standings table renders,
	// which means a field added to the model later cannot reach this payload by
	// accident. (Slug is populated by GetProblems for the booklet and the
	// problem-list link; it is this response, not that query, that must not carry
	// it.) boardProblems is the only projection of these rows that leaves the
	// process, so the strip is by construction rather than by remembering.
	boardProblems := make([]scoreboardProblem, len(problems))
	for i, p := range problems {
		boardProblems[i] = scoreboardProblem{
			ProblemID: p.ProblemID,
			Index:     p.Index,
			Score:     p.Score,
		}
	}

	frozen := contest.FreezeTime != nil && now.After(*contest.FreezeTime) && now.Before(contest.EndTime)

	var beforeTime *time.Time
	if frozen && !isJudge {
		beforeTime = contest.FreezeTime
	}

	rows, _ := h.store.GetScoreboardRows(r.Context(), contest.ID, nil)
	participants, _ := h.store.GetParticipants(r.Context(), contest.ID)

	fmtName := contest.Format
	if fmtName == "" {
		fmtName = "acm"
	}
	contestFormat, err := format.Create(fmtName, contest.FormatConfig)
	if err != nil {
		http.Error(w, "invalid contest format: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// A submission carries the problem's UUID; the board reads the frozen count
	// back with the letter it already has in hand. Translate once, here, or the
	// two identifiers never meet and every cell reports zero pending.
	problemIndexByID := make(map[string]string, len(problems))
	for _, p := range problems {
		problemIndexByID[p.ProblemID] = p.Index
	}

	type userProblemKey struct {
		UserID    string
		ProblemID string
	}
	type userProblemLetterKey struct {
		UserID string
		Letter string
	}
	submissionsByUserProblem := make(map[userProblemKey][]format.Submission)
	frozenSubmissionsByUserProblem := make(map[userProblemLetterKey]int)

	for _, row := range rows {
		key := userProblemKey{UserID: row.UserID, ProblemID: row.ProblemID}
		if frozen && !isJudge && beforeTime != nil && row.CreatedAt.After(*beforeTime) {
			frozenKey := userProblemLetterKey{UserID: row.UserID, Letter: problemIndexByID[row.ProblemID]}
			frozenSubmissionsByUserProblem[frozenKey]++
			continue
		}
		sub := format.Submission{
			ID:        "",
			UserID:    row.UserID,
			ProblemID: row.ProblemID,
			Status:    strings.ToUpper(row.Status),
			Score:     float64(row.Score),
			CreatedAt: row.CreatedAt,
		}
		submissionsByUserProblem[key] = append(submissionsByUserProblem[key], sub)
	}

	formatProblems := make([]format.Problem, len(problems))
	for i, p := range problems {
		formatProblems[i] = format.Problem{
			ID:    p.ProblemID,
			Index: p.Index,
		}
	}

	// One lookup for the whole page: a generated onsite account is labelled with
	// the team it was minted for, not its throwaway username. Missing keys fall
	// back to the username below.
	displayNames, _ := h.store.GetDisplayNames(r.Context(), contest.ID, participants)

	// Manual corrections, loaded once for the page. They are applied to each
	// participant's totals BEFORE ranking, so a judge's decision actually moves
	// the order here and on the full scoreboard — the two read the same function.
	adjustments, _ := h.adjustmentsFor(r, contest.ID)

	participantsScores := make([]format.ParticipantScore, 0, len(participants))
	// Participants carrying a correction, so the row can be marked and a reader
	// knows the number they see is not purely the judge's output.
	adjustedUsers := make(map[string]model.AdjustmentTotals)
	for _, uid := range participants {
		uname := h.store.GetUsername(r.Context(), uid)
		ps := format.ParticipantScore{
			UserID:   uid,
			Username: uname,
			Problems: make([]format.ProblemResult, 0, len(problems)),
		}

		for _, problem := range formatProblems {
			key := userProblemKey{UserID: uid, ProblemID: problem.ID}
			subs := submissionsByUserProblem[key]

			ctx := format.ScoringContext{
				ContestID:           contest.ID,
				ContestDuration:     contest.EndTime.Sub(contest.StartTime),
				SubmissionStartTime: contest.StartTime,
				Problem:             problem,
				Submissions:         subs,
				FormatConfig:        contest.FormatConfig,
			}

			result, err := contestFormat.ScoreProblem(ctx)
			if err != nil {
				http.Error(w, "scoring error: "+err.Error(), http.StatusInternalServerError)
				return
			}

			ps.Problems = append(ps.Problems, result)
			if result.Solved {
				ps.TotalSolved++
			}
			ps.TotalScore += result.Score
			ps.TotalPenalty += result.Penalty
		}

		// Layer the human correction over the computed totals. The submissions
		// keep the verdicts they earned; this is a separate, reasoned delta, so
		// the round stays reproducible and the correction stays visible.
		if adj, ok := adjustments[uid]; ok && !adj.IsZero() {
			ps.TotalScore += float64(adj.PointDelta)
			ps.TotalSolved += adj.SolvedDelta
			ps.TotalPenalty += adj.PenaltyDelta
			// TotalSolved is a count of problems; a correction can never push it
			// below zero, and a negative count would rank better than solving
			// nothing.
			if ps.TotalSolved < 0 {
				ps.TotalSolved = 0
			}
			if ps.TotalPenalty < 0 {
				ps.TotalPenalty = 0
			}
			adjustedUsers[uid] = adj
		}

		participantsScores = append(participantsScores, ps)
	}

	ranks := contestFormat.RankParticipants(participantsScores)

	type Entry struct {
		Rank         int                            `json:"rank"`
		UserID       string                         `json:"user_id"`
		Username     string                         `json:"username"`
		TeamName     string                         `json:"team_name,omitempty"`
		Institution  string                         `json:"institution,omitempty"`
		TotalSolved  int                            `json:"total_solved"`
		TotalPenalty int                            `json:"total_penalty"`
		TotalScore   int                            `json:"total_score"`
		Problems     map[string]model.ProblemResult `json:"problems"`
		// Set when a judge corrected this participant's standing: the deltas that
		// were applied, so the scoreboard can mark the row as adjusted.
		Adjustment *model.AdjustmentTotals `json:"adjustment,omitempty"`
	}

	entries := make([]*Entry, len(ranks))
	for i, rank := range ranks {
		probMap := make(map[string]model.ProblemResult)
		for _, pResult := range rank.Score.Problems {
			key := userProblemLetterKey{UserID: rank.Score.UserID, Letter: pResult.ProblemIndex}
			pending := frozenSubmissionsByUserProblem[key]

			probMap[pResult.ProblemIndex] = model.ProblemResult{
				Solved:   pResult.Solved,
				Attempts: pResult.Attempts,
				Time:     pResult.Penalty,
				Score:    int(pResult.Score),
				Pending:  pending,
			}
		}

		entries[i] = &Entry{
			Rank:         rank.Position,
			UserID:       rank.Score.UserID,
			Username:     rank.Score.Username,
			TeamName:     displayNames[rank.Score.UserID].TeamName,
			Institution:  displayNames[rank.Score.UserID].Institution,
			TotalSolved:  rank.Score.TotalSolved,
			TotalPenalty: rank.Score.TotalPenalty,
			TotalScore:   int(rank.Score.TotalScore),
			Problems:     probMap,
			Adjustment:   adjustmentFor(adjustedUsers, rank.Score.UserID),
		}
	}

	// Pagination
	totalEntries := len(entries)
	page := 1
	limit := 100
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 1 {
		page = p
	}
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 500 {
		limit = l
	}
	totalPages := (totalEntries + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * limit
	end := start + limit
	if start > totalEntries {
		start = totalEntries
	}
	if end > totalEntries {
		end = totalEntries
	}
	pagedEntries := entries[start:end]

	ratingsForSB, _ := h.ratingStore.GetByContest(r.Context(), contest.ID)
	contest.RatingCalculated = len(ratingsForSB) > 0

	respondData := map[string]interface{}{
		"entries": pagedEntries, "problems": boardProblems,
		"frozen": frozen, "contest": contest,
		"is_judge": isJudge, "can_see_judge": canSeeJudge,
		"pagination": map[string]interface{}{
			"page":        page,
			"limit":       limit,
			"total":       totalEntries,
			"total_pages": totalPages,
		},
	}
	jsonBytes, _ := json.Marshal(respondData)
	h.cache.set(cacheKey, jsonBytes)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonBytes)
}

func (h *ContestHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	c, _ := h.store.GetByID(r.Context(), chi.URLParam(r, "id"))
	if c == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	perms, _ := h.store.GetPermissions(r.Context(), c.ID)
	if perms == nil {
		perms = []model.ContestPermission{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": perms})
}

func (h *ContestHandler) AddPermission(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	c, err := h.store.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if c == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), c.ID, claims.UserID, "manager") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req struct {
		UserID string `json:"user_id"`
		Level  string `json:"access_level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	// The level reaches a CHECK constraint, so an unknown one is a client mistake
	// and must answer 400. Letting it through produced a 500 that said nothing.
	switch req.Level {
	case model.ContestPermissionManager, model.ContestPermissionJudge, model.ContestPermissionTester:
	default:
		http.Error(w, "access_level must be manager, judge or tester", http.StatusBadRequest)
		return
	}

	// Resolve the identifier to a real user id before it reaches a uuid column.
	// This is what produced the reported 500: the field was labelled "User ID" but
	// the form never enforced it, so a typed username went straight into
	// `WHERE user_id = $1` and Postgres answered "invalid input syntax for type
	// uuid". An id that parses but belongs to nobody failed the foreign key the
	// same way. Both are "no such user", which is a 404 and not a server fault.
	userID, err := h.resolveUserRef(r.Context(), req.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if userID == "" {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if err := h.store.AddPermission(r.Context(), c.ID, userID, req.Level); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// resolveUserRef is the contest handler's view of the free function below. The
// moderators box takes either spelling in one field.
func (h *ContestHandler) resolveUserRef(ctx context.Context, ref string) (string, error) {
	return resolveUserRef(ctx, h.userStore, ref, "")
}

// userRefLookup is the slice of the user store this resolution needs, declared on
// the consumer side so a test can satisfy it with two methods.
type userRefLookup interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
}

// resolveUserRef turns an operator-supplied reference into a user id.
//
// Two callers, two shapes: the moderators box has a single field that may hold
// either spelling, while the collaborator form sends username and user_id
// separately. Both route through here so there is one answer to "who is this".
//
// A UUID is checked for existence; anything else is looked up as a username.
// Returns "" when the user does not exist, and an error only when the lookup
// itself failed — the caller answers 404 and 500 respectively. An empty
// username with a non-empty id is the normal collaborator shape and is not an
// error.
func resolveUserRef(ctx context.Context, users userRefLookup, username, userID string) (string, error) {
	ref := strings.TrimSpace(username)
	if ref == "" {
		ref = strings.TrimSpace(userID)
	}
	if ref == "" {
		return "", nil
	}
	if users == nil {
		// No user store to check against: accept the raw value only if it can be
		// a UUID, so this path cannot reintroduce the invalid-uuid 500.
		if _, err := uuid.Parse(ref); err != nil {
			return "", nil
		}
		return ref, nil
	}
	if _, err := uuid.Parse(ref); err == nil {
		u, err := users.GetByID(ctx, ref)
		if err != nil {
			return "", err
		}
		if u == nil {
			return "", nil
		}
		return u.ID, nil
	}
	u, err := users.GetByUsername(ctx, ref)
	if err != nil {
		return "", err
	}
	if u == nil {
		return "", nil
	}
	return u.ID, nil
}

func (h *ContestHandler) RemovePermission(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	c, err := h.store.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if c == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), c.ID, claims.UserID, "manager") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	targetUserID := chi.URLParam(r, "userId")
	// A delete is idempotent, so a value that cannot name a row is already the
	// desired end state. Passing it through reached a uuid column and answered
	// 500 for what is a no-op.
	if _, err := uuid.Parse(targetUserID); err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	if err := h.store.RemovePermission(r.Context(), c.ID, targetUserID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ContestHandler) CalculateRatings(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "id")

	contest, err := h.store.GetByID(r.Context(), contestID)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Ratings are the one irreversible effect of a round: they rewrite every
	// participant's profile. Refuse an unapproved or explicitly unrated contest
	// so a draft can never move the leaderboard, and so is_rated finally means
	// something instead of being a column nobody read.
	if contest.Status != model.ContestStatusPublished {
		http.Error(w, "contest is not published", http.StatusConflict)
		return
	}
	if !contest.IsRated {
		http.Error(w, "contest is not rated", http.StatusConflict)
		return
	}

	participants, _ := h.store.GetParticipants(r.Context(), contest.ID)

	standings := make([]rating.ContestStanding, 0, len(participants))
	for i, uid := range participants {
		latest, _ := h.ratingStore.GetLatestByUser(r.Context(), uid)
		oldRating := 0
		if latest != nil {
			oldRating = latest.NewRating
		}

		standings = append(standings, rating.ContestStanding{
			UserID:    uid,
			Rank:      i + 1,
			OldRating: oldRating,
			Username:  h.store.GetUsername(r.Context(), uid),
		})
	}

	ratingService := rating.NewService(h.ratingStore, h.userStore).WithNotifier(h.notify)
	changes := ratingService.CalculateContestRatings(standings)

	if err := ratingService.ApplyContestRatings(r.Context(), contest.ID, changes); err != nil {
		http.Error(w, "failed to apply ratings", http.StatusInternalServerError)
		return
	}

	// Team ratings never moved: UpdateRating existed with no caller, so a team's
	// rating stayed 0 forever and team rankings sorted a constant. Recalculated
	// here, after the member ratings are applied, so it reflects this round.
	var teamErr string
	if err := ratingService.RecalculateTeamRatings(r.Context(), contest.ID, h.teamStore, h.store, h.userStore); err != nil {
		// The individual ratings are already committed; report the partial failure
		// rather than failing the whole request and implying nothing was applied.
		teamErr = err.Error()
		slog.Error("team rating recalculation failed", "contest_id", contest.ID, "error", err)
	}

	resp := map[string]interface{}{"changes": changes}
	if teamErr != "" {
		resp["team_rating_error"] = teamErr
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *ContestHandler) CreateEducational(w http.ResponseWriter, r *http.Request) {
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
		Title       string    `json:"title"`
		StartTime   time.Time `json:"start_time"`
		EndTime     time.Time `json:"end_time"`
		ProblemIDs  []string  `json:"problem_ids"`
		Description string    `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	config := model.DefaultEducationalConfig()

	c := &model.Contest{
		ID:                uuid.New().String(),
		Title:             req.Title,
		Type:              model.ContestTypeEducational,
		StartTime:         req.StartTime,
		EndTime:           req.EndTime,
		EducationalConfig: &config,
		Description:       req.Description,
		Status:            initialContestStatus(claims.Role),
		CreatedBy:         claims.UserID,
	}

	if err := h.store.Create(r.Context(), c); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}

	attached := 0
	for _, pid := range req.ProblemIDs {
		// Same rule as Create: only attach a problem this creator may read, so an
		// educational round cannot be used to surface someone else's draft.
		if p, err := h.problemStore.GetByID(r.Context(), pid); err == nil && p != nil &&
			canReadProblem(r.Context(), h.problemStore, p, claims) {
			if err := h.store.AddProblem(r.Context(), c.ID, pid,
				model.ContestProblemIndex(attached), 100, attached); err == nil {
				attached++
			}
		}
	}
	h.store.RenumberProblems(r.Context(), c.ID)

	respondJSON(w, http.StatusCreated, c)
}

func (h *ContestHandler) RegisterTeam(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contest, err := resolveContest(r.Context(), chi.URLParam(r, "id"), h.store)
	if err != nil {
		respondContestLookupError(w, err)
		return
	}
	var req struct {
		TeamID string `json:"team_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.TeamID == "" {
		http.Error(w, "team_id required", http.StatusBadRequest)
		return
	}

	// Only the team captain (creator/owner) may register the team for a contest.
	role, err := h.teamStore.GetMemberRole(r.Context(), req.TeamID, claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if role == "" {
		http.Error(w, "forbidden: not a member of this team", http.StatusForbidden)
		return
	}
	if role != "owner" {
		http.Error(w, "forbidden: only the team captain can register the team", http.StatusForbidden)
		return
	}

	tr, err := h.store.RegisterTeam(r.Context(), contest.ID, req.TeamID)
	if err != nil {
		http.Error(w, "registration failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusCreated, tr)
}

func (h *ContestHandler) ListTeamRegistrations(w http.ResponseWriter, r *http.Request) {
	contest, err := resolveContest(r.Context(), chi.URLParam(r, "id"), h.store)
	if err != nil {
		respondContestLookupError(w, err)
		return
	}
	// The roster names every registered team and when each signed up. That is
	// organizer information, not public information, so it belongs to the people
	// running the round. The route used to carry no middleware at all, which also
	// made it an unauthenticated enumeration of contest ids.
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "judge", "tester") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	teams, err := h.store.ListTeamRegistrations(r.Context(), contest.ID)
	if err != nil {
		http.Error(w, "failed to list registrations", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": teams})
}

func (h *ContestHandler) ListAvailableFormats(w http.ResponseWriter, r *http.Request) {
	formats := format.List()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"formats": formats,
	})
}

func (h *ContestHandler) AddProblem(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	c, err := h.store.GetByID(r.Context(), id)
	if err != nil || c == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), c.ID, claims.UserID, "manager", "judge") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		ProblemID string `json:"problem_id"`
		Index     string `json:"index"`
		Score     int    `json:"score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.ProblemID == "" || req.Index == "" {
		http.Error(w, "problem_id and index required", http.StatusBadRequest)
		return
	}

	if req.Score <= 0 {
		req.Score = 100
	}

	// Managing a contest says nothing about the problems put into it. Without
	// this check a setter could attach another setter's private draft to their
	// own round and then read it through the contest endpoint, which
	// deliberately skips the visibility filter — the read half of the leak.
	p, err := h.problemStore.GetByID(r.Context(), req.ProblemID)
	if err != nil || p == nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}
	if !canReadProblem(r.Context(), h.problemStore, p, claims) {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}

	// Appended at the end and then renumbered: the letter is a function of
	// position, so honouring a caller-supplied one is what let two problems share
	// a letter (or skip one) in the first place. req.Index is accepted for
	// compatibility but the stored value is derived below.
	existing, _ := h.store.GetProblems(r.Context(), c.ID)
	if len(existing) >= model.MaxContestProblems {
		http.Error(w, fmt.Sprintf("a contest can hold at most %d problems", model.MaxContestProblems),
			http.StatusBadRequest)
		return
	}
	if err := h.store.AddProblem(r.Context(), c.ID, req.ProblemID,
		model.ContestProblemIndex(len(existing)), req.Score, len(existing)); err != nil {
		http.Error(w, "failed to add problem", http.StatusInternalServerError)
		return
	}
	h.store.RenumberProblems(r.Context(), c.ID)

	respondJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

func (h *ContestHandler) RemoveProblem(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	c, err := h.store.GetByID(r.Context(), id)
	if err != nil || c == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), c.ID, claims.UserID, "manager") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	problemID := chi.URLParam(r, "problemId")
	if err := h.store.RemoveProblem(r.Context(), c.ID, problemID); err != nil {
		http.Error(w, "failed to remove problem", http.StatusInternalServerError)
		return
	}

	// Close the gap the removal left. Without this the round keeps the missing
	// problem's letter, and every surface that computes its own letter (the
	// standings header, the PDF booklet) disagrees with the one that reads the
	// stored value.
	problems, _ := h.store.RenumberProblems(r.Context(), c.ID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "removed",
		// The new letters, so the setter's UI can re-label without a refetch.
		"problems": problems,
	})
}

func (h *ContestHandler) DownloadPDF(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	contest, err := h.store.GetByID(r.Context(), id)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	// A PDF is the whole round's statements in one file, so it needs the same guard
	// the contest page has and this endpoint shipped without: an unpublished or
	// not-yet-started contest must not hand its statements to an anonymous visitor
	// just because statement_hidden defaults to false. Answering 404 (not 403) keeps
	// a draft's existence unconfirmed.
	//
	// Its own surface, not the statement's. Printing is a different act from
	// browsing: a round may hand out a booklet at the door while keeping the
	// statements closed in the browser, which is the case that made this
	// separable from pdf_enabled below.
	viewer, ok := h.requireContestSurfaceAccess(w, r, contest, model.SurfacePDF, true)
	if !ok {
		return
	}
	isJudge := viewer.decision.IsJudge()

	if (contest.StatementHidden || !contest.PDFEnabled) && !isJudge {
		http.Error(w, "PDF generation is disabled or hidden for this contest", http.StatusForbidden)
		return
	}

	contestProblems, _ := h.store.GetProblems(r.Context(), contest.ID)

	var problems []model.ProblemWithSamples
	for _, cp := range contestProblems {
		problem, _ := h.problemStore.GetByID(r.Context(), cp.ProblemID)
		if problem != nil {
			problems = append(problems, model.ProblemWithSamples{
				Problem: *problem,
				Index:   cp.Index,
			})
		}
	}

	generator := pdfgen.NewGenerator()
	pdfBytes, err := generator.GenerateContestPDF(contest, problems)
	if err != nil {
		http.Error(w, "failed to generate PDF", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(pdfBytes)
}

func (h *ContestHandler) ContestStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	contest, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if contest == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Per-problem stats carry the problem title and the round-wide solve count, so
	// this asks the problem-list surface — not the statement's, and not the
	// standalone board's. "Public like the scoreboard" stopped being true the moment
	// a round could publish its board and keep its problem names private: the board
	// carries letters only for every caller (see boardProblems), and a stats
	// endpoint that handed the titles out would undo that with one request.
	if _, ok := h.requireContestSurfaceAccess(w, r, contest, model.SurfaceProblemList, true); !ok {
		return
	}

	stats, err := h.store.GetContestStats(r.Context(), contest.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

func (h *ContestHandler) UpdateProblem(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	c, err := h.store.GetByID(r.Context(), id)
	if err != nil || c == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), c.ID, claims.UserID, "manager", "judge") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	problemID := chi.URLParam(r, "problemId")

	var req struct {
		Index     string `json:"index"`
		Score     int    `json:"score"`
		SortOrder int    `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	// Only position and score are writable per problem. The letter is derived
	// from position, so setting it directly is what let the two disagree.
	if err := h.store.UpdateProblem(r.Context(), c.ID, problemID,
		model.ContestProblemIndex(req.SortOrder), req.Score, req.SortOrder); err != nil {
		http.Error(w, "failed to update problem", http.StatusInternalServerError)
		return
	}
	problems, _ := h.store.RenumberProblems(r.Context(), c.ID)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "updated",
		"problems": problems,
	})
}

func (h *ContestHandler) ListMyContests(w http.ResponseWriter, r *http.Request) {
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

// SubmitForReview moves a proposed contest into the admin queue. Only a manager
// may propose it, and an already-published round is not resubmittable.
func (h *ContestHandler) SubmitForReview(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	c, err := resolveContest(r.Context(), chi.URLParam(r, "id"), h.store)
	if err != nil {
		respondContestLookupError(w, err)
		return
	}
	if claims.Role != "admin" && !h.store.HasAccess(r.Context(), c.ID, claims.UserID, "manager") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	switch c.Status {
	case model.ContestStatusPending:
		http.Error(w, "already awaiting review", http.StatusConflict)
		return
	case model.ContestStatusPublished:
		http.Error(w, "already published", http.StatusConflict)
		return
	}
	if err := h.store.SetReviewState(r.Context(), c.ID, model.ContestStatusPending, "", ""); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ReviewQueue lists contests awaiting an admin. Admin-only.
func (h *ContestHandler) ReviewQueue(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = model.ContestStatusPending
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

// ReviewContest records an admin verdict on a proposed round. This is the only
// path that can publish a setter's contest.
func (h *ContestHandler) ReviewContest(w http.ResponseWriter, r *http.Request) {
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
	case model.ContestStatusPublished, model.ContestStatusRejected, model.ContestStatusDraft:
	default:
		http.Error(w, "status must be published, rejected or draft", http.StatusBadRequest)
		return
	}
	c, err := resolveContest(r.Context(), chi.URLParam(r, "id"), h.store)
	if err != nil {
		respondContestLookupError(w, err)
		return
	}
	if err := h.store.SetReviewState(r.Context(), c.ID, req.Status, req.Note, claims.UserID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	title := "Your contest was " + req.Status
	content := c.Title
	if req.Note != "" {
		content += " — " + req.Note
	}
	h.notify.Notify(r.Context(), c.CreatedBy, "contest_review", title, content,
		"/contests/"+c.ID, notify.CategoryContest)
	h.hooks.Dispatch(r.Context(), webhook.Event{
		Type: "contest.reviewed",
		Payload: map[string]any{
			"contest_id": c.ID, "title": c.Title, "status": req.Status, "note": req.Note,
		},
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": req.Status})
}
