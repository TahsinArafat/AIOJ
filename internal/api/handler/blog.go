package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type BlogHandler struct {
	store *postgres.BlogStore
}

func NewBlogHandler(s *postgres.BlogStore) *BlogHandler {
	return &BlogHandler{store: s}
}

func (h *BlogHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.CreateBlogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.Title == "" || req.Content == "" {
		http.Error(w, "title and content required", http.StatusBadRequest)
		return
	}

	p := &model.BlogPost{
		UserID:  claims.UserID,
		Title:   req.Title,
		Content: req.Content,
		Tags:    req.Tags,
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}

	if err := h.store.CreatePost(r.Context(), p); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, p)
}

func (h *BlogHandler) GetPost(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.store.GetPostByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, p)
}

func (h *BlogHandler) ListPosts(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	tag := r.URL.Query().Get("tag")

	items, total, _ := h.store.ListPosts(r.Context(), offset, limit, tag)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *BlogHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	c := &model.Comment{
		UserID:     claims.UserID,
		ParentType: req.ParentType,
		ParentID:   req.ParentID,
		Content:    req.Content,
	}

	if err := h.store.CreateComment(r.Context(), c); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, c)
}

func (h *BlogHandler) GetComments(w http.ResponseWriter, r *http.Request) {
	parentType := chi.URLParam(r, "type")
	parentID := chi.URLParam(r, "id")

	comments, err := h.store.GetComments(r.Context(), parentType, parentID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"data": comments})
}

func (h *BlogHandler) Vote(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
		Value      int    `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.Value != 1 && req.Value != -1 {
		http.Error(w, "invalid vote value", http.StatusBadRequest)
		return
	}

	if err := h.store.Vote(r.Context(), claims.UserID, req.TargetType, req.TargetID, req.Value); err != nil {
		http.Error(w, "vote failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *BlogHandler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	post, err := h.store.GetPostByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if post == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if post.UserID != claims.UserID && claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req model.CreateBlogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.Title == "" || req.Content == "" {
		http.Error(w, "title and content required", http.StatusBadRequest)
		return
	}

	post.Title = req.Title
	post.Content = req.Content
	post.Tags = req.Tags
	if post.Tags == nil {
		post.Tags = []string{}
	}

	if err := h.store.UpdatePost(r.Context(), id, post); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, post)
}

func (h *BlogHandler) DeletePost(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	post, err := h.store.GetPostByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if post == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if post.UserID != claims.UserID && claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := h.store.DeletePost(r.Context(), id); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *BlogHandler) GetUserVote(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	targetType := r.URL.Query().Get("type")
	if targetType == "" {
		targetType = "blog"
	}

	value, err := h.store.GetUserVote(r.Context(), claims.UserID, targetType, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"value": value})
}
