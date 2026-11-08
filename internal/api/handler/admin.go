package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// adminUserRoles is the accepted role set for this handler. It must stay in
// sync with the users_role_check constraint (migrations 000047/000050), or an
// accepted value is rejected by Postgres with a 500 instead of a 400.
var adminUserRoles = map[string]bool{
	"admin": true, "setter": true, "user": true, "bot": true, "contestant": true,
}

// adminUserStatuses are the account-state filters the listing understands.
// An unknown value is a 400 rather than silently "no filter": a typo'd filter
// that quietly returns everything looks like a working filter showing the
// wrong rows.
var adminUserStatuses = map[string]bool{
	"": true, "active": true, "banned": true, "unverified": true, "must_change_password": true,
}

var adminUserSorts = map[string]bool{
	"": true, "created": true, "username": true, "rating": true, "submissions": true, "last_login": true,
}

// maxBulkUsers bounds a single bulk request. The work is one UPDATE per id, so
// an unbounded list turns one request into an arbitrarily long transaction
// holding an admin's HTTP connection open.
const maxBulkUsers = 100

type AdminHandler struct {
	userStore   store.UserStore
	setterStore store.SetterStore
	// refreshToks is optional: when present, a ban or an admin password reset
	// revokes every session. Without it those two actions still work, they just
	// leave the target's existing refresh tokens alive until they expire.
	refreshToks store.RefreshTokenStore
}

func NewAdminHandler(u store.UserStore, s store.SetterStore, rt store.RefreshTokenStore) *AdminHandler {
	return &AdminHandler{userStore: u, setterStore: s, refreshToks: rt}
}

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// ListUsers serves GET /api/admin/users with search, role/status filters,
// sorting and pagination.
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := model.UserFilter{
		Search: strings.TrimSpace(q.Get("search")),
		Role:   q.Get("role"),
		Status: q.Get("status"),
		Sort:   q.Get("sort"),
		// Default is newest-first; Ascending has to be asked for explicitly.
		Desc:   q.Get("order") != "asc",
		Offset: queryInt(r, "offset", 0),
		Limit:  queryInt(r, "limit", 25),
	}
	if !adminUserStatuses[f.Status] {
		http.Error(w, "invalid status filter", http.StatusBadRequest)
		return
	}
	if !adminUserSorts[f.Sort] {
		http.Error(w, "invalid sort field", http.StatusBadRequest)
		return
	}
	if f.Role != "" && !adminUserRoles[f.Role] {
		http.Error(w, "invalid role filter", http.StatusBadRequest)
		return
	}

	users, total, err := h.userStore.ListAdminUsers(r.Context(), f)
	if err != nil {
		slog.Error("list admin users", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":   users,
		"total":  total,
		"offset": f.Offset,
		"limit":  f.Limit,
	})
}

// GetUser serves GET /api/admin/users/{id}.
func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	user, err := h.userStore.GetAdminUser(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		slog.Error("get admin user", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, user)
}

// UpdateUserRole serves PUT /api/admin/users/{id}/role.
func (h *AdminHandler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if !adminUserRoles[req.Role] {
		http.Error(w, "invalid role: must be admin, setter, user, contestant, or bot", http.StatusBadRequest)
		return
	}

	target, ok := h.loadTarget(w, r, userID)
	if !ok {
		return
	}
	// Demoting the last administrator locks everyone out of this panel, and
	// there is no endpoint that can promote anybody back — recovery would need
	// SQL against the database.
	if target.Role == "admin" && req.Role != "admin" {
		if !h.requireAnotherAdmin(w, r) {
			return
		}
	}
	if err := h.userStore.UpdateRole(r.Context(), userID, req.Role); err != nil {
		slog.Error("update role", "error", err)
		http.Error(w, "failed to update role", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// BanUser serves POST /api/admin/users/{id}/ban with {"reason": "..."}.
func (h *AdminHandler) BanUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reason string `json:"reason"`
	}
	// A ban without a reason is allowed (the body may be empty), but a
	// malformed body is not silently ignored.
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
	}
	userID := chi.URLParam(r, "id")
	if claims := middleware.GetUserClaims(r); claims != nil && claims.UserID == userID {
		// Banning yourself would end the session you are banning from, and is
		// never what an administrator means to do.
		http.Error(w, "cannot ban your own account", http.StatusBadRequest)
		return
	}
	target, ok := h.loadTarget(w, r, userID)
	if !ok {
		return
	}
	if target.Role == "admin" && !h.requireAnotherAdmin(w, r) {
		return
	}
	if err := h.userStore.SetUserBan(r.Context(), userID, true, req.Reason); err != nil {
		slog.Error("ban user", "error", err, "user_id", userID)
		http.Error(w, "failed to ban user", http.StatusInternalServerError)
		return
	}
	// Revoke sessions so the ban takes effect now rather than when the current
	// access token expires. A ban that leaves a working session is not a ban.
	h.revokeSessions(r, userID)
	w.WriteHeader(http.StatusOK)
}

// UnbanUser serves DELETE /api/admin/users/{id}/ban.
func (h *AdminHandler) UnbanUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if err := h.userStore.SetUserBan(r.Context(), userID, false, ""); err != nil {
		slog.Error("unban user", "error", err, "user_id", userID)
		http.Error(w, "failed to unban user", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// VerifyUserEmail serves POST /api/admin/users/{id}/verify-email, and
// UnverifyUserEmail the DELETE. Support uses the first path when a verification
// mail never arrived; the second exists so a mistaken verification can be undone.
func (h *AdminHandler) VerifyUserEmail(w http.ResponseWriter, r *http.Request) {
	h.setVerified(w, r, true)
}

func (h *AdminHandler) UnverifyUserEmail(w http.ResponseWriter, r *http.Request) {
	h.setVerified(w, r, false)
}

func (h *AdminHandler) setVerified(w http.ResponseWriter, r *http.Request, verified bool) {
	userID := chi.URLParam(r, "id")
	if err := h.userStore.SetEmailVerified(r.Context(), userID, verified); err != nil {
		slog.Error("set email verified", "error", err, "user_id", userID, "verified", verified)
		http.Error(w, "failed to update email verification", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ResetUserPassword serves PUT /api/admin/users/{id}/password.
//
// This is the compromised-account path: the new password must satisfy the same
// policy as registration (otherwise the admin route becomes a way to install a
// weak credential), and every existing session is revoked.
func (h *AdminHandler) ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	var req struct {
		Password string `json:"password"`
		// ForceChange leaves the account flagged must_change_password, so the
		// admin-chosen password cannot become the account's permanent one.
		ForceChange bool `json:"force_change"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := auth.ValidatePasswordStrength(req.Password); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if auth.IsCommonPassword(req.Password) {
		http.Error(w, auth.ErrPasswordCommon.Error(), http.StatusBadRequest)
		return
	}
	if _, ok := h.loadTarget(w, r, userID); !ok {
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.Error("hash admin password", "error", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	if err := h.userStore.UpdatePassword(r.Context(), userID, hash); err != nil {
		slog.Error("reset user password", "error", err, "user_id", userID)
		http.Error(w, "failed to reset password", http.StatusInternalServerError)
		return
	}
	if req.ForceChange {
		if err := h.userStore.SetMustChangePassword(r.Context(), userID, true); err != nil {
			slog.Error("set must_change_password", "error", err, "user_id", userID)
			http.Error(w, "failed to flag password change", http.StatusInternalServerError)
			return
		}
	}
	h.revokeSessions(r, userID)
	w.WriteHeader(http.StatusOK)
}

// bulkResult is the per-id outcome of a bulk action. One response carries the
// failures instead of aborting the batch: "12 banned, 1 skipped because it is
// the last admin" is more useful than an error after 12 successful writes.
type bulkResult struct {
	ID       string `json:"id"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Username string `json:"username,omitempty"`
}

// BulkUpdate serves POST /api/admin/users/bulk.
func (h *AdminHandler) BulkUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string   `json:"action"`
		IDs    []string `json:"ids"`
		Role   string   `json:"role"`
		Reason string   `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if len(req.IDs) == 0 {
		http.Error(w, "ids is required", http.StatusBadRequest)
		return
	}
	if len(req.IDs) > maxBulkUsers {
		http.Error(w, fmt.Sprintf("at most %d users per request", maxBulkUsers), http.StatusBadRequest)
		return
	}
	switch req.Action {
	case "ban", "unban", "verify_email", "unverify_email", "set_role":
	default:
		http.Error(w, "invalid action: must be ban, unban, verify_email, unverify_email, or set_role",
			http.StatusBadRequest)
		return
	}
	if req.Action == "set_role" && !adminUserRoles[req.Role] {
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}

	claims := middleware.GetUserClaims(r)
	results := make([]bulkResult, 0, len(req.IDs))
	succeeded := 0

	for _, id := range req.IDs {
		res := bulkResult{ID: id}
		if err := h.applyBulkOne(r, claims, req.Action, req.Role, req.Reason, id); err != nil {
			res.Error = err.Error()
		} else {
			res.OK = true
			succeeded++
		}
		results = append(results, res)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"results":   results,
		"succeeded": succeeded,
		"failed":    len(results) - succeeded,
	})
}

// applyBulkOne enforces the same guards as the single-user endpoints. A bulk
// path that skipped them would be a way around "cannot ban the last admin".
func (h *AdminHandler) applyBulkOne(r *http.Request, claims *auth.Claims, action, role, reason, id string) error {
	ctx := r.Context()
	target, err := h.userStore.GetByID(ctx, id)
	if err != nil {
		return errors.New("lookup failed")
	}
	if target == nil {
		return errors.New("user not found")
	}

	switch action {
	case "unban":
		return h.userStore.SetUserBan(ctx, id, false, "")
	case "verify_email":
		return h.userStore.SetEmailVerified(ctx, id, true)
	case "unverify_email":
		return h.userStore.SetEmailVerified(ctx, id, false)
	case "ban":
		if claims != nil && claims.UserID == id {
			return errors.New("cannot ban your own account")
		}
		if target.Role == "admin" {
			if err := h.ensureAnotherAdmin(ctx); err != nil {
				return err
			}
		}
		if err := h.userStore.SetUserBan(ctx, id, true, reason); err != nil {
			return errors.New("ban failed")
		}
		h.revokeSessions(r, id)
		return nil
	case "set_role":
		if target.Role == "admin" && role != "admin" {
			if err := h.ensureAnotherAdmin(ctx); err != nil {
				return err
			}
		}
		if err := h.userStore.UpdateRole(ctx, id, role); err != nil {
			return errors.New("role update failed")
		}
		return nil
	}
	return errors.New("unsupported action")
}

// --- helpers ---------------------------------------------------------------

// loadTarget reads the {id} user, writing the 404 itself. The bool reports
// whether the caller should continue.
func (h *AdminHandler) loadTarget(w http.ResponseWriter, r *http.Request, id string) (*model.User, bool) {
	target, err := h.userStore.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("load admin target", "error", err, "user_id", id)
		http.Error(w, "internal", http.StatusInternalServerError)
		return nil, false
	}
	if target == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return nil, false
	}
	return target, true
}

// requireAnotherAdmin writes a 409 when the operation would leave the instance
// with no usable administrator.
func (h *AdminHandler) requireAnotherAdmin(w http.ResponseWriter, r *http.Request) bool {
	if err := h.ensureAnotherAdmin(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return false
	}
	return true
}

func (h *AdminHandler) ensureAnotherAdmin(ctx context.Context) error {
	n, err := h.userStore.CountAdmins(ctx)
	if err != nil {
		return errors.New("failed to verify remaining administrators")
	}
	if n <= 1 {
		return errors.New("cannot remove the last active administrator")
	}
	return nil
}

// revokeSessions ends every refresh token for the user, so a ban or a password
// reset is not delayed until the current access token expires.
func (h *AdminHandler) revokeSessions(r *http.Request, userID string) {
	if h.refreshToks == nil {
		return
	}
	if err := h.refreshToks.RevokeAll(r.Context(), userID); err != nil {
		slog.Warn("revoke sessions failed", "error", err, "user_id", userID)
	}
}

func (h *AdminHandler) ListSetterApps(w http.ResponseWriter, r *http.Request) {
	apps, _ := h.setterStore.ListApplications(r.Context())
	if apps == nil {
		apps = []model.SetterApplication{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": apps})
}

func (h *AdminHandler) ReviewSetterApp(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Status != "approved" && req.Status != "rejected" {
		http.Error(w, "invalid status: must be approved or rejected", http.StatusBadRequest)
		return
	}
	if err := h.setterStore.UpdateApplicationStatus(r.Context(), userID, req.Status); err != nil {
		http.Error(w, "failed to update application", http.StatusInternalServerError)
		return
	}
	if req.Status == "approved" {
		if err := h.userStore.UpdateRole(r.Context(), userID, "setter"); err != nil {
			http.Error(w, "failed to update user role", http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (h *AdminHandler) ApplySetter(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := h.setterStore.CreateApplication(r.Context(), claims.UserID, req.Reason); err != nil {
		http.Error(w, "failed to submit application", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *AdminHandler) GetSetterStatus(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	app, err := h.setterStore.GetApplication(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "failed to get application status", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, app)
}
