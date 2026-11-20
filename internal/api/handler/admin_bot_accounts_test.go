package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/model"
)

// fakeBotStore is the slice of store.BotAccountStore these tests need. The rest
// of the interface is present only so the handler compiles against it.
type fakeBotStore struct {
	account *model.BotAccount
	updated *model.BotAccount
}

func (f *fakeBotStore) List(context.Context, int, int) ([]model.BotAccount, int, error) {
	return nil, 0, nil
}
func (f *fakeBotStore) ListByPlatform(context.Context, string) ([]model.BotAccount, error) {
	return nil, nil
}
func (f *fakeBotStore) GetByID(context.Context, string) (*model.BotAccount, error) {
	return f.account, nil
}
func (f *fakeBotStore) Create(context.Context, *model.BotAccount) error { return nil }
func (f *fakeBotStore) Update(_ context.Context, _ string, ba *model.BotAccount) error {
	f.updated = ba
	return nil
}
func (f *fakeBotStore) Delete(context.Context, string) error            { return nil }
func (f *fakeBotStore) IncrementFailures(context.Context, string) error { return nil }
func (f *fakeBotStore) ResetFailures(context.Context, string) error     { return nil }
func (f *fakeBotStore) MarkUsed(context.Context, string) error          { return nil }
func (f *fakeBotStore) UpdateLastPoll(context.Context, string) error    { return nil }

// TestBotAccountResponseCarriesNoCredentials pins the boundary: the store hands
// the handler decrypted secrets, and the handler must not pass them on.
//
// This is the regression test for the defect that made encrypting at rest
// partly pointless. model.BotAccount masked PlatformPass with json:"-" but
// left APIKey and APISecret serialized, so GetByID handed the admin console
// working credentials it had no use for — and the edit form pre-filled an
// input with them, putting the secret in the page source.
func TestBotAccountResponseCarriesNoCredentials(t *testing.T) {
	store := &fakeBotStore{account: &model.BotAccount{
		ID:           "bot-1",
		UserID:       "user-1",
		Platform:     "codeforces",
		PlatformUser: "alice",
		// Decrypted values, exactly as the store returns them.
		PlatformPass:   "s3cret-password",
		APIKey:         "s3cret-api-key",
		APISecret:      "s3cret-api-secret",
		SessionData:    map[string]string{"cf_clearance": "s3cret-cookie"},
		HasAPIKey:      true,
		HasAPISecret:   true,
		HasSessionData: true,
		Status:         "active",
		RateLimitRPS:   1,
	}}
	// No vjudge service: GetByID must not need one.
	h := NewAdminBotAccountHandler(store, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/bot-accounts/bot-1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "bot-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.GetByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	for _, secret := range []string{"s3cret-password", "s3cret-api-key", "s3cret-api-secret", "s3cret-cookie"} {
		if strings.Contains(body, secret) {
			t.Errorf("response body leaks %q: %s", secret, body)
		}
	}
	for _, key := range []string{`"platform_pass"`, `"api_key"`, `"api_secret"`, `"session_data"`} {
		if strings.Contains(body, key) {
			t.Errorf("response body still contains the %s key: %s", key, body)
		}
	}

	// What the console is allowed to know: that secrets exist.
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got["has_api_key"] != true {
		t.Errorf("has_api_key = %v, want true", got["has_api_key"])
	}
	if got["has_api_secret"] != true {
		t.Errorf("has_api_secret = %v, want true", got["has_api_secret"])
	}
	if got["has_session_data"] != true {
		t.Errorf("has_session_data = %v, want true", got["has_session_data"])
	}
}

// TestBotAccountUpdateAppliesAPIKey covers the inverse silent failure: the
// console has always offered API Key/Secret fields and sent them, but Update's
// request struct never declared them, so the edit did nothing and reported
// success. An empty value must still mean "keep", now that the form cannot
// pre-fill it.
func TestBotAccountUpdateAppliesAPIKey(t *testing.T) {
	store := &fakeBotStore{account: &model.BotAccount{
		ID: "bot-1", Platform: "codeforces", PlatformUser: "alice",
		APIKey: "old-key", APISecret: "old-secret", PlatformPass: "old-pass",
		Status: "active", RateLimitRPS: 1,
	}}
	h := NewAdminBotAccountHandler(store, nil)

	post := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPatch, "/api/admin/bot-accounts/bot-1", strings.NewReader(body))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "bot-1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		h.Update(rec, req)
		return rec
	}

	if rec := post(`{"api_key":"new-key","api_secret":"new-secret"}`); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if store.updated.APIKey != "new-key" || store.updated.APISecret != "new-secret" {
		t.Errorf("API key/secret were not applied: %+v", store.updated)
	}

	// Omitting them, and sending them empty, both mean "unchanged".
	if rec := post(`{"platform_user":"bob"}`); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if store.updated.APIKey != "new-key" || store.updated.APISecret != "new-secret" {
		t.Errorf("an omitted api_key clobbered the stored value: %+v", store.updated)
	}

	if rec := post(`{"api_key":"","api_secret":""}`); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if store.updated.APIKey != "new-key" || store.updated.APISecret != "new-secret" {
		t.Errorf("an empty api_key clobbered the stored value: %+v", store.updated)
	}
}
