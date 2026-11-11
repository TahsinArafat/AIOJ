package handler

import (
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

type ContestHandler struct {
	store        *postgres.ContestStore
	ratingStore  *postgres.RatingStore
	problemStore store.ProblemStore
	userStore    store.UserStore
	teamStore    store.TeamStore
	cache        *scoreboardCache
	// notify and hooks are optional; nil instances are no-ops.
	notify *notify.Notifier
	hooks  *webhook.Dispatcher
	// feedStore and feedToken back the ICPC-style event feed. Both are optional:
	// with no token the endpoint answers 503 rather than exposing the feed.
	feedStore feedLister
	feedToken string
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
	return &ContestHandler{store: s, ratingStore: rs, problemStore: ps, userStore: us, teamStore: ts, cache: newScoreboardCache(30 * time.Second)}
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

	password := ""
	if req.Password != nil {
		password = *req.Password
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
		Password:             password,
		Description:          req.Description,
		Status:               initialContestStatus(claims.Role),
		Visible:              claims.Role == "admin",
		GroupID:              req.GroupID,
		CreatedBy:            claims.UserID,
		RegistrationRequired: req.RegistrationRequired,
		RegistrationDeadline: req.RegistrationDeadline,
		MaxParticipants:      req.MaxParticipants,
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
	for i, pid := range req.ProblemIDs {
		idx := string(rune('A' + i))
		// Skip problems this creator may not use rather than failing the whole
		// contest: a bad id must not cost the round, and silently attaching
		// somebody else's draft would leak it through the contest endpoint.
		if p, err := h.problemStore.GetByID(r.Context(), pid); err == nil && p != nil &&
			canReadProblem(r.Context(), h.problemStore, p, claims) {
			h.store.AddProblem(r.Context(), c.ID, pid, idx, 100, i)
		}
	}
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

	claims := middleware.GetUserClaims(r)
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}

	allowed, err := h.store.CheckGroupRestriction(r.Context(), c.ID, userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	isManager := claims != nil && (claims.Role == "admin" || h.store.HasAccess(r.Context(), c.ID, claims.UserID, "manager", "tester", "judge"))
	if !allowed && !isManager {
		http.Error(w, "forbidden: restricted to group members", http.StatusForbidden)
		return
	}

	now := time.Now()
	if !c.Visible || now.Before(c.StartTime) {
		if claims == nil || !isManager {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}
	if !isManager {
		c.Password = ""
	}
	problems, _ := h.store.GetProblems(r.Context(), c.ID)
	ratings, _ := h.ratingStore.GetByContest(r.Context(), c.ID)
	c.RatingCalculated = len(ratings) > 0
	respondJSON(w, http.StatusOK, map[string]interface{}{"contest": c, "problems": problems})
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

	items, total, _ := h.store.ListWithDivision(r.Context(), offset, limit, division)
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
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
	if req.Password != nil {
		c.Password = *req.Password
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
	c.GroupID = req.GroupID
	if req.Slug != "" {
		c.Slug = req.Slug
	}
	if err := h.store.Update(r.Context(), c); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
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
	claims := middleware.GetUserClaims(r)

	if !canReadContest(r.Context(), h.store, contest, claims) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	isJudge := claims != nil && (claims.Role == "admin" || h.store.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "judge"))
	canSeeJudge := isJudge // preserve original capability before view override

	// Allow admins/judges to explicitly request the public (frozen) view via ?view=public
	viewParam := r.URL.Query().Get("view")
	if viewParam == "public" && isJudge {
		isJudge = false
	}

	// Check cache (key: contestID:view:page:limit:judge)
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	// isJudge belongs in the key: a judge sees past the freeze and gets
	// is_judge:true, so without it the first judge to load the page seeded the
	// unfrozen board for every anonymous visitor until the entry expired.
	cacheKey := contest.ID + ":" + viewParam + ":" + pageStr + ":" + limitStr + ":" + strconv.FormatBool(isJudge)
	if cached, ok := h.cache.get(cacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(cached)
		return
	}

	problems, _ := h.store.GetProblems(r.Context(), contest.ID)

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

	type userProblemKey struct {
		UserID    string
		ProblemID string
	}
	submissionsByUserProblem := make(map[userProblemKey][]format.Submission)
	frozenSubmissionsByUserProblem := make(map[userProblemKey]int)

	for _, row := range rows {
		key := userProblemKey{UserID: row.UserID, ProblemID: row.ProblemID}
		if frozen && !isJudge && beforeTime != nil && row.CreatedAt.After(*beforeTime) {
			frozenSubmissionsByUserProblem[key]++
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

	participantsScores := make([]format.ParticipantScore, 0, len(participants))
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
	}

	entries := make([]*Entry, len(ranks))
	for i, rank := range ranks {
		probMap := make(map[string]model.ProblemResult)
		for _, pResult := range rank.Score.Problems {
			key := userProblemKey{UserID: rank.Score.UserID, ProblemID: pResult.ProblemIndex}
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
		"entries": pagedEntries, "problems": problems,
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
	if err := h.store.AddPermission(r.Context(), c.ID, req.UserID, req.Level); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
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

	for i, pid := range req.ProblemIDs {
		idx := string(rune('A' + i))
		// Same rule as Create: only attach a problem this creator may read, so an
		// educational round cannot be used to surface someone else's draft.
		if p, err := h.problemStore.GetByID(r.Context(), pid); err == nil && p != nil &&
			canReadProblem(r.Context(), h.problemStore, p, claims) {
			h.store.AddProblem(r.Context(), c.ID, pid, idx, 100, i)
		}
	}

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

	if err := h.store.AddProblem(r.Context(), c.ID, req.ProblemID, req.Index, req.Score, 0); err != nil {
		http.Error(w, "failed to add problem", http.StatusInternalServerError)
		return
	}

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

	respondJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (h *ContestHandler) DownloadPDF(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	id := chi.URLParam(r, "id")

	contest, err := h.store.GetByID(r.Context(), id)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	isJudge := claims != nil && (claims.Role == "admin" || h.store.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "judge"))

	// A PDF is the whole round's statements in one file, so it needs the same
	// guard the contest page has and this endpoint shipped without: an
	// unpublished or not-yet-started contest must not hand its statements to an
	// anonymous visitor just because statement_hidden defaults to false.
	if !canReadContest(r.Context(), h.store, contest, claims) {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

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

	// Per-problem stats carry the problem title, so the contest's own visibility
	// has to be checked here too — "public like the scoreboard" is only true once
	// the round is public.
	claims := middleware.GetUserClaims(r)
	if !canReadContest(r.Context(), h.store, contest, claims) {
		http.Error(w, "not found", http.StatusNotFound)
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

	if err := h.store.UpdateProblem(r.Context(), c.ID, problemID, req.Index, req.Score, req.SortOrder); err != nil {
		http.Error(w, "failed to update problem", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
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
