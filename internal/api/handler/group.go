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

type GroupHandler struct {
	store     *postgres.GroupStore
	userStore store.UserStore
}

func NewGroupHandler(s *postgres.GroupStore, us store.UserStore) *GroupHandler {
	return &GroupHandler{store: s, userStore: us}
}

func (h *GroupHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	joinPolicy := req.JoinPolicy
	if joinPolicy != "manual_approve" {
		joinPolicy = "auto_approve"
	}

	g := &model.Group{
		Name:        req.Name,
		Description: req.Description,
		IsPublic:    req.IsPublic,
		MaxMembers:  req.MaxMembers,
		JoinPolicy:  joinPolicy,
		CreatedBy:   claims.UserID,
	}

	if err := h.store.Create(r.Context(), g); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, g)
}

func (h *GroupHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	g, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if g == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	respondJSON(w, http.StatusOK, g)
}

func (h *GroupHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	items, total, _ := h.store.List(r.Context(), offset, limit)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *GroupHandler) Join(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	groupID := chi.URLParam(r, "id")
	if err := h.store.AddMember(r.Context(), groupID, claims.UserID, "member"); err != nil {
		http.Error(w, "join failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "joined"})
}

func (h *GroupHandler) Leave(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	groupID := chi.URLParam(r, "id")
	if err := h.store.RemoveMember(r.Context(), groupID, claims.UserID); err != nil {
		http.Error(w, "leave failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "left"})
}

func (h *GroupHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	members, err := h.store.GetMembers(r.Context(), groupID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"data": members})
}

func (h *GroupHandler) AddContest(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	groupID := chi.URLParam(r, "id")

	role, err := h.store.GetMemberRole(r.Context(), groupID, claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if role != "owner" && role != "manager" && claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		ContestID string `json:"contest_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if err := h.store.AddContest(r.Context(), groupID, req.ContestID); err != nil {
		http.Error(w, "add contest failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

func (h *GroupHandler) ListByUser(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	items, err := h.store.ListByUser(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

func (h *GroupHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	groupID := chi.URLParam(r, "id")

	role, err := h.store.GetMemberRole(r.Context(), groupID, claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if role != "owner" && role != "manager" && claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	g, err := h.store.GetByID(r.Context(), groupID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if g == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var req model.UpdateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.Name != "" {
		g.Name = req.Name
	}
	if req.Description != "" {
		g.Description = req.Description
	}
	if req.IsPublic != nil {
		g.IsPublic = *req.IsPublic
	}
	if req.MaxMembers != nil {
		g.MaxMembers = req.MaxMembers
	}
	if req.JoinPolicy == "auto_approve" || req.JoinPolicy == "manual_approve" {
		g.JoinPolicy = req.JoinPolicy
	}

	if err := h.store.Update(r.Context(), groupID, g); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, g)
}

func (h *GroupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	groupID := chi.URLParam(r, "id")

	role, err := h.store.GetMemberRole(r.Context(), groupID, claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if role != "owner" && claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := h.store.Delete(r.Context(), groupID); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *GroupHandler) GetContests(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	contests, err := h.store.GetContests(r.Context(), groupID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"data": contests})
}

// InviteMember invites a user to a group. Owner/manager only.
func (h *GroupHandler) InviteMember(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	groupID := chi.URLParam(r, "id")

	role, err := h.store.GetMemberRole(r.Context(), groupID, claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if role != "owner" && role != "manager" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req model.GroupInviteRequest
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

	if err := h.store.AddMember(r.Context(), groupID, user.ID, "invited"); err != nil {
		http.Error(w, "invite failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "invited"})
}

// JoinByCode allows a user to join a group using an invite code.
func (h *GroupHandler) JoinByCode(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.JoinByCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.InviteCode == "" {
		http.Error(w, "invite_code required", http.StatusBadRequest)
		return
	}

	group, err := h.store.GetByInviteCode(r.Context(), req.InviteCode)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if group == nil {
		http.Error(w, "group not found", http.StatusNotFound)
		return
	}

	existingRole, err := h.store.GetMemberRole(r.Context(), group.ID, claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if existingRole != "" {
		http.Error(w, "already a member or pending", http.StatusConflict)
		return
	}

	var role string
	if group.JoinPolicy == "manual_approve" {
		role = "requested"
	} else {
		role = "member"
	}

	if err := h.store.AddMember(r.Context(), group.ID, claims.UserID, role); err != nil {
		http.Error(w, "join failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "joined", "role": role})
}

// RespondInviteRequest handles group invite/request responses.
func (h *GroupHandler) RespondInviteRequest(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	groupID := chi.URLParam(r, "id")

	var req model.GroupRespondRequest
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
		if req.UserID != claims.UserID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := h.store.UpdateMemberRole(r.Context(), groupID, claims.UserID, "member"); err != nil {
			http.Error(w, "accept failed", http.StatusInternalServerError)
			return
		}
	case "decline":
		if req.UserID != claims.UserID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := h.store.RemoveMember(r.Context(), groupID, claims.UserID); err != nil {
			http.Error(w, "decline failed", http.StatusInternalServerError)
			return
		}
	case "approve":
		role, err := h.store.GetMemberRole(r.Context(), groupID, claims.UserID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if role != "owner" && role != "manager" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := h.store.UpdateMemberRole(r.Context(), groupID, req.UserID, "member"); err != nil {
			http.Error(w, "approve failed", http.StatusInternalServerError)
			return
		}
	case "reject":
		role, err := h.store.GetMemberRole(r.Context(), groupID, claims.UserID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if role != "owner" && role != "manager" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if err := h.store.RemoveMember(r.Context(), groupID, req.UserID); err != nil {
			http.Error(w, "reject failed", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "invalid action: must be accept, decline, approve, or reject", http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": req.Action + "ed"})
}

// GetPendingMembers lists pending invites/requests (owner/manager only).
func (h *GroupHandler) GetPendingMembers(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	groupID := chi.URLParam(r, "id")

	role, err := h.store.GetMemberRole(r.Context(), groupID, claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if role != "owner" && role != "manager" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	members, err := h.store.GetPendingMembers(r.Context(), groupID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": members})
}

func (h *GroupHandler) RemoveContest(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	groupID := chi.URLParam(r, "id")
	contestID := chi.URLParam(r, "contestId")

	role, err := h.store.GetMemberRole(r.Context(), groupID, claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if role != "owner" && role != "manager" && claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := h.store.RemoveContest(r.Context(), groupID, contestID); err != nil {
		http.Error(w, "remove contest failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}
