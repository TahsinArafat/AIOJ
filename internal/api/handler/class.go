package handler

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type ClassHandler struct {
	classStore *postgres.ClassStore
	orgStore   *postgres.OrganizationStore
}

func NewClassHandler(cs *postgres.ClassStore, os *postgres.OrganizationStore) *ClassHandler {
	return &ClassHandler{classStore: cs, orgStore: os}
}

func generateInviteCode() (string, error) {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, 8)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		code[i] = chars[n.Int64()]
	}
	return string(code), nil
}

func (h *ClassHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	orgID := chi.URLParam(r, "orgId")
	role, _ := h.orgStore.GetMemberRole(r.Context(), orgID, claims.UserID)
	if role != "owner" && role != "admin" && claims.Role != "admin" {
		http.Error(w, "forbidden: only org owner/admin can create classes", http.StatusForbidden)
		return
	}

	var req model.CreateClassRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}

	code, err := generateInviteCode()
	if err != nil {
		http.Error(w, "failed to generate invite code", http.StatusInternalServerError)
		return
	}

	c := &model.Class{
		OrganizationID: orgID,
		Name:           req.Name,
		Description:    req.Description,
		InviteCode:     code,
		CreatedBy:      claims.UserID,
	}

	if err := h.classStore.Create(r.Context(), c); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, c)
}

func (h *ClassHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	c, err := h.classStore.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if c == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, c)
}

func (h *ClassHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	items, total, _ := h.classStore.List(r.Context(), orgID, offset, limit)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *ClassHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	classID := chi.URLParam(r, "id")
	class, err := h.classStore.GetByID(r.Context(), classID)
	if err != nil || class == nil {
		http.Error(w, "class not found", http.StatusNotFound)
		return
	}

	role, _ := h.orgStore.GetMemberRole(r.Context(), class.OrganizationID, claims.UserID)
	if role != "owner" && role != "admin" && claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req model.CreateClassRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	updated := &model.Class{Name: req.Name, Description: req.Description}
	if err := h.classStore.Update(r.Context(), classID, updated); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *ClassHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	classID := chi.URLParam(r, "id")
	class, err := h.classStore.GetByID(r.Context(), classID)
	if err != nil || class == nil {
		http.Error(w, "class not found", http.StatusNotFound)
		return
	}

	role, _ := h.orgStore.GetMemberRole(r.Context(), class.OrganizationID, claims.UserID)
	if role != "owner" && claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := h.classStore.Delete(r.Context(), classID); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *ClassHandler) JoinByCode(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		InviteCode string `json:"invite_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.InviteCode == "" {
		http.Error(w, "invite_code required", http.StatusBadRequest)
		return
	}

	class, err := h.classStore.GetByInviteCode(r.Context(), req.InviteCode)
	if err != nil || class == nil {
		http.Error(w, "invalid invite code", http.StatusNotFound)
		return
	}

	if err := h.classStore.AddMember(r.Context(), class.ID, claims.UserID, "student"); err != nil {
		http.Error(w, "join failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "joined", "class_id": class.ID})
}

func (h *ClassHandler) Leave(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	classID := chi.URLParam(r, "id")
	if err := h.classStore.RemoveMember(r.Context(), classID, claims.UserID); err != nil {
		http.Error(w, "leave failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "left"})
}

func (h *ClassHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "id")
	members, err := h.classStore.GetMembers(r.Context(), classID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": members})
}
