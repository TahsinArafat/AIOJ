package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

// stubRefreshToks satisfies store.RefreshTokenStore. Counts RevokeAll so the
// "ban/password reset ends sessions" behaviour is assertable.
type stubRefreshToks struct{ revoked []string }

func (s *stubRefreshToks) Create(context.Context, string, string, time.Time) error { return nil }
func (s *stubRefreshToks) Consume(context.Context, string) (string, error)         { return "", nil }
func (s *stubRefreshToks) RevokeAll(_ context.Context, userID string) error {
	s.revoked = append(s.revoked, userID)
	return nil
}

// newAdminReq builds a request with chi's {id} route param and, when actorID is
// set, the claims an authenticated admin would carry.
func newAdminReq(method, target, id, body, actorID string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if id != "" {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}
	if actorID != "" {
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey,
			&auth.Claims{UserID: actorID, Username: "root"}))
	}
	return req
}

func adminUsersFixture() *stubUserForEV {
	return &stubUserForEV{
		users: map[string]*model.User{
			"root":  {ID: "root", Username: "root", Role: "admin"},
			"other": {ID: "other", Username: "someone", Role: "user"},
		},
		adminCount: 2,
	}
}

// TestBanUser_cannotBanSelf: banning yourself ends the session you are banning
// from, and is never what the admin means to click.
func TestBanUser_cannotBanSelf(t *testing.T) {
	us := adminUsersFixture()
	h := &AdminHandler{userStore: us, refreshToks: &stubRefreshToks{}}

	rec := httptest.NewRecorder()
	h.BanUser(rec, newAdminReq(http.MethodPost, "/api/admin/users/root/ban", "root", `{"reason":"oops"}`, "root"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %q)", rec.Code, rec.Body.String())
	}
	if us.banned["root"] {
		t.Fatal("self-ban must not be applied")
	}
}

// TestBanUser_lastAdminIsProtected is the guard that keeps the instance
// administrable: there is no endpoint that can promote a new admin back.
func TestBanUser_lastAdminIsProtected(t *testing.T) {
	us := adminUsersFixture()
	us.adminCount = 1
	h := &AdminHandler{userStore: us, refreshToks: &stubRefreshToks{}}

	rec := httptest.NewRecorder()
	h.BanUser(rec, newAdminReq(http.MethodPost, "/api/admin/users/root/ban", "root", `{}`, "someone-else"))

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body %q)", rec.Code, rec.Body.String())
	}
	if us.banned["root"] {
		t.Fatal("the last administrator must not be banned")
	}
}

// TestBanUser_revokesSessions: a ban that leaves a working refresh token is not
// a ban, it is a suggestion.
func TestBanUser_revokesSessions(t *testing.T) {
	us := adminUsersFixture()
	toks := &stubRefreshToks{}
	h := &AdminHandler{userStore: us, refreshToks: toks}

	rec := httptest.NewRecorder()
	h.BanUser(rec, newAdminReq(http.MethodPost, "/api/admin/users/other/ban", "other", `{"reason":"spam"}`, "root"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if !us.banned["other"] {
		t.Fatal("ban was not written")
	}
	if len(toks.revoked) != 1 || toks.revoked[0] != "other" {
		t.Fatalf("revoked = %v, want [other]", toks.revoked)
	}
}

func TestBanUser_unknownUserIs404(t *testing.T) {
	h := &AdminHandler{userStore: adminUsersFixture()}

	rec := httptest.NewRecorder()
	h.BanUser(rec, newAdminReq(http.MethodPost, "/api/admin/users/ghost/ban", "ghost", `{}`, "root"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// TestUpdateUserRole_cannotDemoteLastAdmin covers the other way to strand an
// instance with no administrator.
func TestUpdateUserRole_cannotDemoteLastAdmin(t *testing.T) {
	us := adminUsersFixture()
	us.adminCount = 1
	h := &AdminHandler{userStore: us}

	rec := httptest.NewRecorder()
	h.UpdateUserRole(rec, newAdminReq(http.MethodPut, "/api/admin/users/root/role", "root", `{"role":"user"}`, "root"))

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body %q)", rec.Code, rec.Body.String())
	}
	if us.users["root"].Role != "admin" {
		t.Fatal("the last administrator was demoted")
	}
}

// TestUpdateUserRole_rejectsUnknownRole: an accepted-but-invalid role reaches
// Postgres and violates users_role_check as a 500 instead of a 400.
func TestUpdateUserRole_rejectsUnknownRole(t *testing.T) {
	h := &AdminHandler{userStore: adminUsersFixture()}

	rec := httptest.NewRecorder()
	h.UpdateUserRole(rec, newAdminReq(http.MethodPut, "/api/admin/users/other/role", "other", `{"role":"superuser"}`, "root"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// TestUpdateUserRole_acceptsContestant: `contestant` is valid per migration
// 000047 but was missing from the handler's allowlist.
func TestUpdateUserRole_acceptsContestant(t *testing.T) {
	us := adminUsersFixture()
	h := &AdminHandler{userStore: us}

	rec := httptest.NewRecorder()
	h.UpdateUserRole(rec, newAdminReq(http.MethodPut, "/api/admin/users/other/role", "other", `{"role":"contestant"}`, "root"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
}

// TestListUsers_validatesFilters: a typo'd status must not silently behave like
// "no filter", which looks like a working filter showing the wrong rows.
func TestListUsers_validatesFilters(t *testing.T) {
	h := &AdminHandler{userStore: adminUsersFixture()}

	for _, target := range []string{
		"/api/admin/users?status=banana",
		"/api/admin/users?sort=password_hash",
		"/api/admin/users?role=superuser",
	} {
		rec := httptest.NewRecorder()
		h.ListUsers(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", target, rec.Code)
		}
	}
}

func TestResetUserPassword_enforcesPolicy(t *testing.T) {
	us := adminUsersFixture()
	h := &AdminHandler{userStore: us}

	// No digit, and too short — the same rules registration applies.
	for _, pw := range []string{"short", "alllowercaseandlong"} {
		body, _ := json.Marshal(map[string]interface{}{"password": pw, "force_change": true})
		rec := httptest.NewRecorder()
		h.ResetUserPassword(rec, newAdminReq(http.MethodPut, "/api/admin/users/other/password", "other", string(body), "root"))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("password %q: status = %d, want 400", pw, rec.Code)
		}
	}
}

// TestResetUserPassword_revokesSessions: the point of an admin reset is to lock
// a compromised session out, so the reset is not complete without revocation.
func TestResetUserPassword_revokesSessions(t *testing.T) {
	toks := &stubRefreshToks{}
	h := &AdminHandler{userStore: adminUsersFixture(), refreshToks: toks}

	body, _ := json.Marshal(map[string]interface{}{"password": "Aioj-Admin-2026!", "force_change": true})
	rec := httptest.NewRecorder()
	h.ResetUserPassword(rec, newAdminReq(http.MethodPut, "/api/admin/users/other/password", "other", string(body), "root"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if len(toks.revoked) != 1 || toks.revoked[0] != "other" {
		t.Fatalf("revoked = %v, want [other]", toks.revoked)
	}
}

// TestBulkUpdate_reportsPartialFailure: one protected account must not abort the
// batch — the admin needs "12 banned, 1 skipped", not an error after 12 writes.
func TestBulkUpdate_reportsPartialFailure(t *testing.T) {
	us := adminUsersFixture()
	us.adminCount = 1
	us.users["second"] = &model.User{ID: "second", Username: "second", Role: "user"}
	h := &AdminHandler{userStore: us, refreshToks: &stubRefreshToks{}}

	body, _ := json.Marshal(map[string]interface{}{
		"action": "ban",
		"ids":    []string{"second", "root"},
		"reason": "bulk test",
	})
	rec := httptest.NewRecorder()
	h.BulkUpdate(rec, newAdminReq(http.MethodPost, "/api/admin/users/bulk", "", string(body), "root"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Results   []bulkResult `json:"results"`
		Succeeded int          `json:"succeeded"`
		Failed    int          `json:"failed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Succeeded != 1 || resp.Failed != 1 {
		t.Fatalf("succeeded=%d failed=%d, want 1/1 (body %s)", resp.Succeeded, resp.Failed, rec.Body.String())
	}
	if !us.banned["second"] {
		t.Error("the permitted ban was not applied")
	}
	if us.banned["root"] {
		t.Error("the last administrator was banned through the bulk path")
	}
}

func TestBulkUpdate_rejectsUnknownAction(t *testing.T) {
	h := &AdminHandler{userStore: adminUsersFixture()}

	body, _ := json.Marshal(map[string]interface{}{"action": "delete", "ids": []string{"other"}})
	rec := httptest.NewRecorder()
	h.BulkUpdate(rec, newAdminReq(http.MethodPost, "/api/admin/users/bulk", "", string(body), "root"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestBulkUpdate_rejectsOversizedBatch(t *testing.T) {
	h := &AdminHandler{userStore: adminUsersFixture()}

	ids := make([]string, maxBulkUsers+1)
	for i := range ids {
		ids[i] = "u"
	}
	body, _ := json.Marshal(map[string]interface{}{"action": "ban", "ids": ids})
	rec := httptest.NewRecorder()
	h.BulkUpdate(rec, newAdminReq(http.MethodPost, "/api/admin/users/bulk", "", string(body), "root"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// TestLogin_bannedUserIsRejected: the password is correct, so the only thing
// standing between a banned account and a fresh token is this check.
func TestLogin_bannedUserIsRejected(t *testing.T) {
	hash, err := auth.HashPassword("Aioj-Admin-2026!")
	if err != nil {
		t.Fatal(err)
	}
	us := &stubUserForEV{
		users: map[string]*model.User{
			"u1": {ID: "u1", Username: "alice", Email: "alice@x.com", PasswordHash: hash, Role: "user"},
		},
		banned: map[string]bool{"u1": true},
	}
	h := &AuthHandler{
		users:       us,
		refreshToks: &stubRefreshToks{},
		jwt:         auth.NewJWTManager("test-secret-at-least-32-bytes-long!!", time.Minute, time.Hour),
	}

	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "Aioj-Admin-2026!"})
	rec := httptest.NewRecorder()
	h.Login(rec, httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body)))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body %q)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "suspended") {
		t.Fatalf("body %q should explain the suspension", rec.Body.String())
	}
	if us.lastLogin != "" {
		t.Error("a rejected login must not record last_login_at")
	}
}

// TestLogin_recordsLastLogin: the admin panel's "last sign-in" column relies on
// this write; without it every account reads "never".
func TestLogin_recordsLastLogin(t *testing.T) {
	hash, err := auth.HashPassword("Aioj-Admin-2026!")
	if err != nil {
		t.Fatal(err)
	}
	us := &stubUserForEV{
		users: map[string]*model.User{
			"u1": {ID: "u1", Username: "alice", Email: "alice@x.com", PasswordHash: hash, Role: "user"},
		},
	}
	h := &AuthHandler{
		users:       us,
		refreshToks: &stubRefreshToks{},
		jwt:         auth.NewJWTManager("test-secret-at-least-32-bytes-long!!", time.Minute, time.Hour),
	}

	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "Aioj-Admin-2026!"})
	rec := httptest.NewRecorder()
	h.Login(rec, httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if us.lastLogin != "u1" {
		t.Fatalf("last_login recorded for %q, want u1", us.lastLogin)
	}
}
