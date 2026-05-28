package handler

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/rating"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type ContestHandler struct {
	store       *postgres.ContestStore
	ratingStore *postgres.RatingStore
}

func NewContestHandler(s *postgres.ContestStore, rs *postgres.RatingStore) *ContestHandler {
	return &ContestHandler{store: s, ratingStore: rs}
}

func (h *ContestHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
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
	c := &model.Contest{
		ID:          uuid.New().String(),
		Title:       req.Title,
		Type:        req.Type,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		FreezeTime:  req.FreezeTime,
		Password:    req.Password,
		Description: req.Description,
		Visible:     true,
		CreatedBy:   claims.UserID,
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
	if !contest.Visible || now.Before(contest.StartTime) {
		claims := middleware.GetUserClaims(r)
		if claims == nil || (claims.Role != "admin" && !h.store.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "tester")) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	problems, _ := h.store.GetProblems(r.Context(), id)

	frozen := contest.FreezeTime != nil && now.After(*contest.FreezeTime) && now.Before(contest.EndTime)

	var beforeTime *time.Time
	if frozen {
		beforeTime = contest.FreezeTime
	}

	rows, _ := h.store.GetScoreboardRows(r.Context(), id, beforeTime)
	participants, _ := h.store.GetParticipants(r.Context(), id)

	// Build problem index map
	problemIdx := make(map[string]string) // problemID → index label
	for _, p := range problems {
		problemIdx[p.ProblemID] = p.Index
	}

	type probStat struct {
		solved   bool
		attempts int
		penalty  int // minutes
		time     int
		score    int
	}

	type userStat struct {
		userID   string
		username string
		probs    map[string]*probStat
	}

	userMap := make(map[string]*userStat)
	for _, uid := range participants {
		uname := h.store.GetUsername(r.Context(), uid)
		userMap[uid] = &userStat{
			userID:   uid,
			username: uname,
			probs:    make(map[string]*probStat),
		}
	}

	for _, row := range rows {
		u, ok := userMap[row.UserID]
		if !ok {
			continue
		}
		idx, ok2 := problemIdx[row.ProblemID]
		if !ok2 {
			continue
		}
		ps := u.probs[idx]
		if ps == nil {
			ps = &probStat{}
			u.probs[idx] = ps
		}
		minutes := int(row.CreatedAt.Sub(contest.StartTime).Minutes())
		if contest.Type == "acm" {
			if ps.solved {
				continue
			}
			if row.Status == "ac" {
				ps.solved = true
				ps.penalty = ps.attempts*20 + minutes
				ps.time = minutes
			} else if row.Status == "wa" || row.Status == "tle" || row.Status == "re" {
				ps.attempts++
			}
		} else {
			if row.Score > ps.score {
				ps.score = row.Score
			}
			ps.solved = ps.score >= 100
		}
	}

	type Entry struct {
		Rank         int                           `json:"rank"`
		UserID       string                        `json:"user_id"`
		Username     string                        `json:"username"`
		TotalSolved  int                           `json:"total_solved"`
		TotalPenalty int                           `json:"total_penalty"`
		TotalScore   int                           `json:"total_score"`
		Problems     map[string]model.ProblemResult `json:"problems"`
	}

	entries := make([]*Entry, 0, len(userMap))
	for _, u := range userMap {
		e := &Entry{UserID: u.userID, Username: u.username, Problems: make(map[string]model.ProblemResult)}
		for idx, ps := range u.probs {
			e.Problems[idx] = model.ProblemResult{
				Solved: ps.solved, Attempts: ps.attempts, Time: ps.time,
				Score: ps.score, Pending: 0,
			}
			if ps.solved {
				e.TotalSolved++
				e.TotalPenalty += ps.penalty
			}
			e.TotalScore += ps.score
		}
		entries = append(entries, e)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].TotalSolved != entries[j].TotalSolved {
			return entries[i].TotalSolved > entries[j].TotalSolved
		}
		return entries[i].TotalPenalty < entries[j].TotalPenalty
	})
	for i := range entries {
		entries[i].Rank = i + 1
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries, "problems": problems,
		"frozen": frozen, "contest": contest,
	})
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

	participants, _ := h.store.GetParticipants(r.Context(), contestID)

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
	if claims == nil || claims.Role != "admin" {
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
