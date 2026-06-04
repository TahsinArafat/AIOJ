package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type TeamHandler struct {
	store     *postgres.TeamStore
	userStore store.UserStore
}

func NewTeamHandler(s *postgres.TeamStore, us store.UserStore) *TeamHandler {
	return &TeamHandler{store: s, userStore: us}
}

func (h *TeamHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	isPublic := true
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}

	t := &model.Team{
		Name:        req.Name,
		Description: req.Description,
		IsPublic:    isPublic,
		CreatedBy:   claims.UserID,
	}

	if err := h.store.Create(r.Context(), t); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, t)
}

func (h *TeamHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	items, total, _ := h.store.List(r.Context(), offset, limit)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *TeamHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if t == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, t)
}

func (h *TeamHandler) Join(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	teamID := chi.URLParam(r, "id")
	if err := h.store.AddMember(r.Context(), teamID, claims.UserID, "member"); err != nil {
		http.Error(w, "join failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "joined"})
}

func (h *TeamHandler) Leave(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	teamID := chi.URLParam(r, "id")
	if err := h.store.RemoveMember(r.Context(), teamID, claims.UserID); err != nil {
		http.Error(w, "leave failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "left"})
}

func (h *TeamHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	members, err := h.store.GetMembers(r.Context(), teamID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": members})
}

// UpdateTeam updates team name, description, avatar, is_public. Owner/captain only.
func (h *TeamHandler) UpdateTeam(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	teamID := chi.URLParam(r, "id")

	role, err := h.store.GetMemberRole(r.Context(), teamID, claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if role != "owner" && role != "captain" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req model.UpdateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	team, err := h.store.GetByID(r.Context(), teamID)
	if err != nil || team == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	team.Name = req.Name
	team.Description = req.Description
	team.AvatarURL = req.AvatarURL
	if req.IsPublic != nil {
		team.IsPublic = *req.IsPublic
	}

	if err := h.store.Update(r.Context(), teamID, team); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, team)
}

// DeleteTeam deletes a team. Owner or admin only.
func (h *TeamHandler) DeleteTeam(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	teamID := chi.URLParam(r, "id")

	role, err := h.store.GetMemberRole(r.Context(), teamID, claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if role != "owner" && claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := h.store.Delete(r.Context(), teamID); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// InviteMember invites a user to a team. Owner/captain only.
func (h *TeamHandler) InviteMember(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	teamID := chi.URLParam(r, "id")

	role, err := h.store.GetMemberRole(r.Context(), teamID, claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if role != "owner" && role != "captain" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req model.TeamInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Username == "" {
		http.Error(w, "username required", http.StatusBadRequest)
		return
	}

	user, err := h.userStore.GetByUsername(r.Context(), req.Username)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	if err := h.store.AddMember(r.Context(), teamID, user.ID, "invited"); err != nil {
		http.Error(w, "invite failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "invited"})
}

// RequestJoin allows a user to request joining a private team.
func (h *TeamHandler) RequestJoin(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	teamID := chi.URLParam(r, "id")

	existingRole, err := h.store.GetMemberRole(r.Context(), teamID, claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if existingRole != "" {
		http.Error(w, "already a member or pending", http.StatusConflict)
		return
	}

	if err := h.store.AddMember(r.Context(), teamID, claims.UserID, "requested"); err != nil {
		http.Error(w, "request failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "requested"})
}

// RespondInviteRequest handles invite/request responses: accept, decline, approve, reject.
func (h *TeamHandler) RespondInviteRequest(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	teamID := chi.URLParam(r, "id")

	var req model.TeamRespondRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.UserID == "" || req.Action == "" {
		http.Error(w, "user_id and action required", http.StatusBadRequest)
		return
	}

	switch req.Action {
	case "accept":
		// User accepts an invitation to join a team
		if req.UserID != claims.UserID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := h.store.UpdateMemberRole(r.Context(), teamID, claims.UserID, "member"); err != nil {
			http.Error(w, "accept failed", http.StatusInternalServerError)
			return
		}
	case "decline":
		// User declines an invitation
		if req.UserID != claims.UserID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := h.store.RemoveMember(r.Context(), teamID, claims.UserID); err != nil {
			http.Error(w, "decline failed", http.StatusInternalServerError)
			return
		}
	case "approve":
		// Owner/captain approves a join request
		role, err := h.store.GetMemberRole(r.Context(), teamID, claims.UserID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if role != "owner" && role != "captain" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := h.store.UpdateMemberRole(r.Context(), teamID, req.UserID, "member"); err != nil {
			http.Error(w, "approve failed", http.StatusInternalServerError)
			return
		}
	case "reject":
		// Owner/captain rejects a join request
		role, err := h.store.GetMemberRole(r.Context(), teamID, claims.UserID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if role != "owner" && role != "captain" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := h.store.RemoveMember(r.Context(), teamID, req.UserID); err != nil {
			http.Error(w, "reject failed", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "invalid action: must be accept, decline, approve, or reject", http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": req.Action + "ed"})
}

// GetPendingMembers lists pending invites/requests for a team (owner/captain only).
func (h *TeamHandler) GetPendingMembers(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	teamID := chi.URLParam(r, "id")

	role, err := h.store.GetMemberRole(r.Context(), teamID, claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if role != "owner" && role != "captain" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	members, err := h.store.GetPendingMembers(r.Context(), teamID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": members})
}
