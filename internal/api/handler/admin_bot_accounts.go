package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
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
		http.Error(w, "failed to create bot account: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if h.vjudgeSvc != nil {
		if len(ba.SessionData) > 0 {
			h.vjudgeSvc.SetCookies(ba.Platform, ba.SessionData)
			slog.Info("bot cookies loaded from session_data", "platform", ba.Platform)
		} else if ba.PlatformUser != "" && ba.PlatformPass != "" {
			go func() {
				cookies, err := h.vjudgeSvc.Login(r.Context(), ba.Platform)
				if err != nil {
					slog.Error("bot auto-login failed", "platform", ba.Platform, "err", err)
					return
				}
				if len(cookies) > 0 {
					ba.SessionData = cookies
					h.store.Update(r.Context(), ba.ID, ba)
					h.vjudgeSvc.UpdateCookies(ba.Platform, cookies)
					slog.Info("bot auto-login succeeded, cookies saved", "platform", ba.Platform)
				}
			}()
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
		PlatformUser *string           `json:"platform_user"`
		PlatformPass *string           `json:"platform_pass"`
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
	if req.PlatformPass != nil {
		existing.PlatformPass = *req.PlatformPass
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
		http.Error(w, "failed to update bot account", http.StatusInternalServerError)
		return
	}

	if h.vjudgeSvc != nil {
		if req.SessionData != nil && len(req.SessionData) > 0 {
			h.vjudgeSvc.SetCookies(existing.Platform, req.SessionData)
			slog.Info("bot cookies updated from session_data", "platform", existing.Platform)
		} else if req.PlatformUser != nil || req.PlatformPass != nil {
			go func() {
				cookies, err := h.vjudgeSvc.Login(r.Context(), existing.Platform)
				if err != nil {
					slog.Error("bot auto-login failed after update", "platform", existing.Platform, "err", err)
					return
				}
				if len(cookies) > 0 {
					existing.SessionData = cookies
					h.store.Update(r.Context(), id, existing)
					h.vjudgeSvc.UpdateCookies(existing.Platform, cookies)
					slog.Info("bot auto-login succeeded after update, cookies saved", "platform", existing.Platform)
				}
			}()
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
