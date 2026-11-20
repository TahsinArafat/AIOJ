package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/ai"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
	"github.com/tahsinarafat/aioj/internal/vjudge"
)

type AdminBotAccountHandler struct {
	store     store.BotAccountStore
	vjudgeSvc *vjudge.Service
}

func NewAdminBotAccountHandler(s store.BotAccountStore, vjSvc *vjudge.Service) *AdminBotAccountHandler {
	return &AdminBotAccountHandler{store: s, vjudgeSvc: vjSvc}
}

// writeBotStoreError turns a store failure into a response.
//
// The one error worth distinguishing is the missing credential key: it is a
// deployment mistake, not a server fault, and telling the admin to set
// AI_CREDENTIAL_MASTER_KEY is more useful than a bare 500. Every other failure
// stays opaque so a database error cannot leak schema details to the caller.
func writeBotStoreError(w http.ResponseWriter, action string, err error) {
	if errors.Is(err, ai.ErrEncryptionNotConfigured) {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	http.Error(w, "failed to "+action+" bot account", http.StatusInternalServerError)
}

// autoLoginTimeout bounds the background login. It is generous because it
// covers a full round trip to a remote judge through whatever proxy the bot is
// configured with; it exists so the goroutine cannot live forever.
const autoLoginTimeout = 2 * time.Minute

// autoLogin signs a bot in after it is created or edited and persists the
// cookies that result.
//
// It re-reads the row before writing instead of saving the copy it was handed:
// an admin may edit the account again while the login is in flight, and writing
// the older copy would silently revert those edits. A deleted account is not an
// error worth retrying — GetByID returns nil and the login is dropped.
func (h *AdminBotAccountHandler) autoLogin(base context.Context, acct model.BotAccount, id string) {
	ctx, cancel := context.WithTimeout(base, autoLoginTimeout)
	defer cancel()

	cookies, err := h.vjudgeSvc.Login(ctx, acct.Platform)
	if err != nil {
		slog.Error("bot auto-login failed", "platform", acct.Platform, "account", id, "err", err)
		return
	}
	if len(cookies) == 0 {
		slog.Warn("bot auto-login returned no cookies", "platform", acct.Platform, "account", id)
		return
	}

	fresh, err := h.store.GetByID(ctx, id)
	if err != nil {
		slog.Error("bot auto-login could not reload account", "account", id, "err", err)
		return
	}
	if fresh == nil {
		slog.Info("bot deleted while auto-login was in flight", "account", id)
		return
	}
	fresh.SessionData = cookies
	if err := h.store.Update(ctx, id, fresh); err != nil {
		// The cookies are live but not durable; say so rather than claim success.
		slog.Error("bot auto-login obtained cookies but could not save them",
			"platform", acct.Platform, "account", id, "err", err)
		return
	}
	h.vjudgeSvc.UpdateCookies(acct.Platform, cookies)
	slog.Info("bot auto-login succeeded, cookies saved", "platform", acct.Platform, "account", id)
}

func (h *AdminBotAccountHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	accounts, total, err := h.store.List(r.Context(), offset, limit)
	if err != nil {
		http.Error(w, "failed to list bot accounts", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": accounts, "total": total})
}

func (h *AdminBotAccountHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ba, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "failed to get bot account", http.StatusInternalServerError)
		return
	}
	if ba == nil {
		http.Error(w, "bot account not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, ba)
}

func (h *AdminBotAccountHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID       string            `json:"user_id"`
		Platform     string            `json:"platform"`
		PlatformUser string            `json:"platform_user"`
		PlatformPass string            `json:"platform_pass"`
		APIKey       string            `json:"api_key"`
		APISecret    string            `json:"api_secret"`
		SessionData  map[string]string `json:"session_data"`
		RateLimitRPS float32           `json:"rate_limit_rps"`
		ProxyURL     string            `json:"proxy_url"`
		ProxyEnabled bool              `json:"proxy_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Platform == "" || req.UserID == "" {
		http.Error(w, "user_id and platform are required", http.StatusBadRequest)
		return
	}
	if req.RateLimitRPS <= 0 {
		req.RateLimitRPS = 1.0
	}
	ba := &model.BotAccount{
		UserID:       req.UserID,
		Platform:     req.Platform,
		PlatformUser: req.PlatformUser,
		PlatformPass: req.PlatformPass,
		APIKey:       req.APIKey,
		APISecret:    req.APISecret,
		SessionData:  req.SessionData,
		Status:       "active",
		RateLimitRPS: req.RateLimitRPS,
		ProxyURL:     req.ProxyURL,
		ProxyEnabled: req.ProxyEnabled,
	}
	if err := h.store.Create(r.Context(), ba); err != nil {
		writeBotStoreError(w, "create", err)
		return
	}

	if h.vjudgeSvc != nil {
		if len(ba.SessionData) > 0 {
			h.vjudgeSvc.SetCookies(ba.Platform, ba.SessionData)
			slog.Info("bot cookies loaded from session_data", "platform", ba.Platform)
		} else if ba.PlatformUser != "" && ba.PlatformPass != "" {
			// WithoutCancel: the login outlives the request. The context it used
			// to capture was cancelled the moment this handler returned, so the
			// remote call and the write that persists the cookies both raced a
			// dead context — and the write's error was discarded, so the only
			// symptom was cookies silently never being saved.
			go h.autoLogin(context.WithoutCancel(r.Context()), *ba, ba.ID)
		}
	}

	respondJSON(w, http.StatusCreated, ba)
}

func (h *AdminBotAccountHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "failed to get bot account", http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, "bot account not found", http.StatusNotFound)
		return
	}
	var req struct {
		PlatformUser *string `json:"platform_user"`
		PlatformPass *string `json:"platform_pass"`
		// APIKey and APISecret are accepted here because the console has always
		// offered to change them. It sent them and the server ignored them, so the
		// edit silently did nothing. Omitted (or empty) keeps the existing value.
		APIKey       *string           `json:"api_key"`
		APISecret    *string           `json:"api_secret"`
		Status       *string           `json:"status"`
		RateLimitRPS *float32          `json:"rate_limit_rps"`
		SessionData  map[string]string `json:"session_data"`
		ProxyURL     *string           `json:"proxy_url"`
		ProxyEnabled *bool             `json:"proxy_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.PlatformUser != nil {
		existing.PlatformUser = *req.PlatformUser
	}
	if req.PlatformPass != nil && *req.PlatformPass != "" {
		existing.PlatformPass = *req.PlatformPass
	}
	if req.APIKey != nil && *req.APIKey != "" {
		existing.APIKey = *req.APIKey
	}
	if req.APISecret != nil && *req.APISecret != "" {
		existing.APISecret = *req.APISecret
	}
	if req.Status != nil {
		validStatuses := map[string]bool{"active": true, "expired": true, "error": true, "banned": true}
		if !validStatuses[*req.Status] {
			http.Error(w, "invalid status: must be active, expired, error, or banned", http.StatusBadRequest)
			return
		}
		existing.Status = *req.Status
	}
	if req.RateLimitRPS != nil {
		existing.RateLimitRPS = *req.RateLimitRPS
	}
	if req.SessionData != nil {
		existing.SessionData = req.SessionData
	}
	if req.ProxyURL != nil {
		existing.ProxyURL = *req.ProxyURL
	}
	if req.ProxyEnabled != nil {
		existing.ProxyEnabled = *req.ProxyEnabled
	}
	if err := h.store.Update(r.Context(), id, existing); err != nil {
		writeBotStoreError(w, "update", err)
		return
	}

	if h.vjudgeSvc != nil {
		// len() on a nil map is 0, so the nil check was redundant (S1009).
		if len(req.SessionData) > 0 {
			h.vjudgeSvc.SetCookies(existing.Platform, req.SessionData)
			slog.Info("bot cookies updated from session_data", "platform", existing.Platform)
		} else if req.PlatformUser != nil || req.PlatformPass != nil {
			go h.autoLogin(context.WithoutCancel(r.Context()), *existing, id)
		}
	}

	respondJSON(w, http.StatusOK, existing)
}

func (h *AdminBotAccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.Delete(r.Context(), id); err != nil {
		http.Error(w, "failed to delete bot account", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *AdminBotAccountHandler) TestLogin(w http.ResponseWriter, r *http.Request) {
	if h.vjudgeSvc == nil {
		http.Error(w, "vjudge service not available", http.StatusInternalServerError)
		return
	}

	var req struct {
		Platform     string            `json:"platform"`
		PlatformUser string            `json:"platform_user"`
		PlatformPass string            `json:"platform_pass"`
		SessionData  map[string]string `json:"session_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.Platform == "" {
		http.Error(w, "platform is required", http.StatusBadRequest)
		return
	}

	// Validate against the platform's own registered bot rather than a ladder of
	// platform names. The ladder listed Codeforces separately because only it
	// had a cookie probe and a specific cookie set worth checking, and named the
	// other four in an || chain — which meant a platform added to the registry
	// silently fell through to the generic branch with the wrong message.
	//
	// The two questions are now asked of the bot: can you validate cookies, and
	// do these ones work. A platform that cannot validate says so instead of
	// being told its cookies are fine.
	if len(req.SessionData) > 0 {
		err := h.vjudgeSvc.ValidateCookies(r.Context(), req.Platform, req.SessionData)
		switch {
		case err == nil:
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":  "ok",
				"message": "Login validated successfully! Cookies are valid.",
				"cookies": len(req.SessionData),
			})
		case errors.Is(err, vjudge.ErrCookieValidationUnsupported):
			// Storing unvalidated cookies is legitimate for a platform with no
			// probe; it just must not be reported as validated.
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":  "ok",
				"message": "Cookies stored. This platform cannot be validated before use.",
				"cookies": len(req.SessionData),
			})
		default:
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":  "error",
				"message": "Cookies invalid: " + err.Error(),
			})
		}
		return
	}

	if req.PlatformUser == "" || req.PlatformPass == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}

	// Whether the credentials need an interactive login on first use is the
	// platform's own business, and the message says what will actually happen
	// rather than naming a proxy service.
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"message": "Credentials configured. Bot will log in on first submission.",
	})
}
