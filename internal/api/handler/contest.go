package handler

import (
	"encoding/json"
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
	pdfgen "github.com/tahsinarafat/aioj/internal/pdf"
	"github.com/tahsinarafat/aioj/internal/rating"
	"github.com/tahsinarafat/aioj/internal/store"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
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
	cache        *scoreboardCache
}

func NewContestHandler(s *postgres.ContestStore, rs *postgres.RatingStore, ps store.ProblemStore) *ContestHandler {
	return &ContestHandler{store: s, ratingStore: rs, problemStore: ps, cache: newScoreboardCache(30 * time.Second)}
}

func (h *ContestHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if claims.Role != "admin" && claims.Role != "teacher" {
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
		cf, err := format.Create(fmtName, req.FormatConfig)
		if err != nil {
			http.Error(w, "invalid format config: "+err.Error(), http.StatusBadRequest)
			return
		}
		formatConfigJSON = req.FormatConfig
		_ = cf
	} else {
		factory, ok := format.Get(fmtName)
		if !ok {
			http.Error(w, "unknown format: "+fmtName, http.StatusBadRequest)
			return
		}
		f, _ := factory(nil)
		formatConfigJSON = f.DefaultConfig()
	}

	c := &model.Contest{
		ID:           uuid.New().String(),
		Slug:         req.Slug,
		Title:        req.Title,
		Type:         req.Type,
		Format:       fmtName,
		FormatConfig: formatConfigJSON,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
		FreezeTime:   req.FreezeTime,
		Password:     req.Password,
		Description:  req.Description,
		Visible:      true,
		CreatedBy:    claims.UserID,
	}
	if err := h.store.Create(r.Context(), c); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	for i, pid := range req.ProblemIDs {
		idx := string(rune('A' + i))
		h.store.AddProblem(r.Context(), c.ID, pid, idx, 100, i)
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
	now := time.Now()
	if !c.Visible || now.Before(c.StartTime) {
		claims := middleware.GetUserClaims(r)
		if claims == nil || (claims.Role != "admin" && !h.store.HasAccess(r.Context(), c.ID, claims.UserID, "manager", "tester")) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}
	problems, _ := h.store.GetProblems(r.Context(), c.ID)
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
	if req.Password != "" {
		c.Password = req.Password
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
	id := chi.URLParam(r, "id")
	if err := h.store.Delete(r.Context(), id); err != nil {
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

	if !contest.Visible || now.Before(contest.StartTime) {
		if claims == nil || (claims.Role != "admin" && !h.store.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "tester")) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	isJudge := claims != nil && (claims.Role == "admin" || h.store.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "judge"))
	canSeeJudge := isJudge // preserve original capability before view override

	// Allow admins/judges to explicitly request the public (frozen) view via ?view=public
	viewParam := r.URL.Query().Get("view")
	if viewParam == "public" && isJudge {
		isJudge = false
	}

	// Check cache (key: contestID:view:page:limit)
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	cacheKey := contest.ID + ":" + viewParam + ":" + pageStr + ":" + limitStr
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
		Rank         int                           `json:"rank"`
		UserID       string                        `json:"user_id"`
		Username     string                        `json:"username"`
		TotalSolved  int                           `json:"total_solved"`
		TotalPenalty int                           `json:"total_penalty"`
		TotalScore   int                           `json:"total_score"`
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

	participants, _ := h.store.GetParticipants(r.Context(), contest.ID)

	standings := make([]rating.ContestStanding, 0, len(participants))
	for i, uid := range participants {
		latest, _ := h.ratingStore.GetLatestByUser(r.Context(), uid)
		oldRating := rating.DefaultRating
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

	ratingService := rating.NewService(h.ratingStore, nil)
	changes := ratingService.CalculateContestRatings(standings)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"changes": changes,
	})
}

func (h *ContestHandler) CreateEducational(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if claims.Role != "admin" && claims.Role != "teacher" {
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
		Visible:           true,
		CreatedBy:         claims.UserID,
	}

	if err := h.store.Create(r.Context(), c); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}

	for i, pid := range req.ProblemIDs {
		idx := string(rune('A' + i))
		h.store.AddProblem(r.Context(), c.ID, pid, idx, 100, i)
	}

	respondJSON(w, http.StatusCreated, c)
}

func (h *ContestHandler) RegisterTeam(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contestID := chi.URLParam(r, "id")
	var req struct {
		TeamID string `json:"team_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	_, err := h.store.RegisterTeam(r.Context(), contestID, req.TeamID)
	if err != nil {
		http.Error(w, "registration failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"status": "registered"})
}

func (h *ContestHandler) ListTeamRegistrations(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "id")
	teams, err := h.store.ListTeamRegistrations(r.Context(), contestID)
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

	if err := h.store.AddProblem(r.Context(), id, req.ProblemID, req.Index, req.Score, 0); err != nil {
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
	if err := h.store.RemoveProblem(r.Context(), id, problemID); err != nil {
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

	isJudge := claims != nil && (claims.Role == "admin" || h.store.HasAccess(r.Context(), id, claims.UserID, "manager", "judge"))

	if !contest.PDFEnabled && !isJudge {
		http.Error(w, "PDF generation is disabled for this contest", http.StatusForbidden)
		return
	}

	contestProblems, _ := h.store.GetProblems(r.Context(), id)

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
	claims := middleware.GetUserClaims(r)
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

	// Require auth: admin or contest manager/judge.
	// Stats are public like scoreboard — no auth check needed.
	_ = claims

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

	if err := h.store.UpdateProblem(r.Context(), id, problemID, req.Index, req.Score, req.SortOrder); err != nil {
		http.Error(w, "failed to update problem", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
