package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/authmethod"
	"github.com/tahsinarafat/aioj/internal/mail"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

// authUserStore is the user surface the sign-in flow performs: create, look up
// by handle or address, check the ban that must stop the next token, and manage
// the password and the verified flag.
//
// Nine operations out of the store's 21. The omissions are the point: signing in
// must not be able to change a role, count administrators, or read the admin
// listing, all of which the field granted it.
type authUserStore interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	IsBanned(ctx context.Context, id string) (bool, error)
	TouchLastLogin(ctx context.Context, id string) error
	UpdatePassword(ctx context.Context, id, passwordHash string) error
	MarkEmailVerified(ctx context.Context, userID string) error
	IsEmailVerified(ctx context.Context, userID string) (bool, error)
}

type AuthHandler struct {
	users             authUserStore
	refreshToks       store.RefreshTokenStore
	passwordResetToks store.PasswordResetTokenStore
	onsiteStore       store.OnsiteUserStore
	contestStore      contestRefResolver
	jwt               *auth.JWTManager
	evt               store.EmailVerificationTokenStore
	twoFA             store.TOTPSecretStore
	mail              mail.Sender
	mailTpl           *template.Template
	publicURL         string
	mailFrom          string
	// methods gates email/password registration. Nil means "no gate", so the
	// handler keeps working in tests that construct it as a literal.
	methods *authmethod.Resolver
}

// SetAuthMethods installs the sign-in method resolver. Kept as a setter rather
// than a constructor parameter: NewAuthHandler already takes eleven arguments
// and every existing test builds the struct directly.
func (h *AuthHandler) SetAuthMethods(m *authmethod.Resolver) { h.methods = m }

func NewAuthHandler(
	users authUserStore,
	refreshToks store.RefreshTokenStore,
	passwordResetToks store.PasswordResetTokenStore,
	onsiteStore store.OnsiteUserStore,
	contestStore contestRefResolver,
	jwt *auth.JWTManager,
	evt store.EmailVerificationTokenStore,
	twoFA store.TOTPSecretStore,
	m mail.Sender,
	tpl *template.Template,
	publicURL string,
	mailFrom string,
) *AuthHandler {
	return &AuthHandler{
		users: users, refreshToks: refreshToks, passwordResetToks: passwordResetToks,
		onsiteStore: onsiteStore, contestStore: contestStore, jwt: jwt,
		evt: evt, twoFA: twoFA, mail: m, mailTpl: tpl, publicURL: publicURL, mailFrom: mailFrom,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	// Checked before the body is read: a disabled method should not invite a
	// client to submit a password at all.
	if h.methods != nil && !h.methods.Available(r.Context(), authmethod.KeyPasswordRegistration) {
		http.Error(w, "registration with a password is disabled on this instance", http.StatusForbidden)
		return
	}
	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		http.Error(w, "username, email, password required", http.StatusBadRequest)
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
	existing, _ := h.users.GetByUsername(r.Context(), req.Username)
	if existing != nil {
		http.Error(w, "username taken", http.StatusConflict)
		return
	}
	existing, _ = h.users.GetByEmail(r.Context(), req.Email)
	if existing != nil {
		http.Error(w, "email taken", http.StatusConflict)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	user := &model.User{ID: uuid.New().String(), Username: req.Username, Email: req.Email, PasswordHash: hash, Role: "user"}
	if err := h.users.Create(r.Context(), user); err != nil {
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}
	h.sendVerificationEmail(r.Context(), user)
	respondJSON(w, http.StatusCreated, h.tokenResp(r.Context(), user))
}

// sendVerificationEmail issues a one-shot hashed token and emails a verify link.
// Failures are logged, not returned — registration still succeeds.
func (h *AuthHandler) sendVerificationEmail(ctx context.Context, user *model.User) {
	if h.evt == nil || h.mail == nil || h.mailTpl == nil || user.Email == "" {
		return
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		slog.Error("email verification rand failed", "err", err, "user_id", user.ID)
		return
	}
	rawHex := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(rawHex))
	hash := hex.EncodeToString(sum[:])
	if err := h.evt.Create(ctx, uuid.New().String(), user.ID, hash, time.Now().Add(24*time.Hour)); err != nil {
		slog.Error("email verification token create failed", "err", err, "user_id", user.ID)
		return
	}
	verifyURL := strings.TrimRight(h.publicURL, "/") + "/verify-email?token=" + rawHex
	var body bytes.Buffer
	if err := h.mailTpl.ExecuteTemplate(&body, "email_verification.txt", map[string]string{
		"Username":      user.Username,
		"VerifyURL":     verifyURL,
		"ExpiryMinutes": "24",
	}); err != nil {
		slog.Error("render verification email failed", "err", err)
		return
	}
	from := h.mailFrom
	if from == "" {
		from = "noreply@aioj.com"
	}
	_ = h.mail.Send(ctx, &mail.Message{
		From: from, To: []string{user.Email},
		Subject: "Verify your AIOJ email", Body: body.String(),
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	user, err := h.users.GetByUsername(r.Context(), req.Username)
	if err == nil && user != nil && auth.CheckPassword(req.Password, user.PasswordHash) {
		// A banned account must not receive a token even with the right
		// password. This is checked after the password comparison, not before,
		// so the response does not tell an attacker that a guessed password was
		// correct — the ban message is only shown to someone who could have
		// signed in.
		if banned, banErr := h.users.IsBanned(r.Context(), user.ID); banErr == nil && banned {
			http.Error(w, "this account has been suspended", http.StatusForbidden)
			return
		}
		// An onsite team lands HERE from its second sign-in onward, not in the
		// onsite branch below. The users row is created with the same password
		// hash the credential carries, so from then on this ordinary lookup
		// matches and the function returns before the onsite code is reached.
		//
		// That shadowing is what made the earlier enrollment fix look broken: the
		// grant was only ever written on the first login, so a team that had
		// signed in once was never enrolled, and every later sign-in took this
		// branch and skipped it again — permanently locked out of its own round
		// while the login itself kept succeeding.
		h.enrollOnsiteTeam(r.Context(), user)
		// Best effort: a failed bookkeeping write must not fail the login.
		if err := h.users.TouchLastLogin(r.Context(), user.ID); err != nil {
			slog.Warn("touch last login failed", "error", err, "user_id", user.ID)
		}
		if h.maybe2FAChallenge(w, r, user, "") {
			return
		}
		respondJSON(w, http.StatusOK, h.tokenResp(r.Context(), user))
		return
	}

	onsiteUser, err := h.onsiteStore.GetByUsername(r.Context(), req.Username)
	if err != nil || onsiteUser == nil {
		// bcrypt is deliberately slow, and skipping it when the account does not
		// exist made "no such user" answer in microseconds while a real user took
		// ~100ms — a reliable username oracle. Compare against a throwaway hash so
		// both paths cost the same.
		auth.WastePasswordComparisonTime(req.Password)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if !auth.CheckPassword(req.Password, onsiteUser.PasswordHash) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	var dbUser *model.User
	if onsiteUser.IsUsed && onsiteUser.UsedBy != nil {
		dbUser, err = h.users.GetByID(r.Context(), *onsiteUser.UsedBy)
		if err != nil || dbUser == nil {
			http.Error(w, "failed to retrieve user", http.StatusInternalServerError)
			return
		}
	} else {
		dbUser = &model.User{
			ID:           uuid.New().String(),
			Username:     onsiteUser.Username,
			Email:        onsiteUser.Username + "@onsite.aioj",
			PasswordHash: onsiteUser.PasswordHash,
			Role:         "contestant",
			IsBot:        false,
		}
		if err := h.users.Create(r.Context(), dbUser); err != nil {
			http.Error(w, "failed to create user", http.StatusInternalServerError)
			return
		}
		// Onsite accounts have synthetic emails (…@onsite.aioj) and cannot
		// receive mail; treat them as pre-verified so they can submit.
		_ = h.users.MarkEmailVerified(r.Context(), dbUser.ID)
		if err := h.onsiteStore.MarkUsed(r.Context(), onsiteUser.ID, dbUser.ID); err != nil {
			http.Error(w, "failed to update credential state", http.StatusInternalServerError)
			return
		}
	}

	// Every login, not only the first. This used to sit in the else branch above,
	// so a team that had already signed in once was never enrolled — and once the
	// grant is written here, leaving it in that branch would lock those teams out
	// permanently, because the code that grants access lives in the branch they no
	// longer take. Both writes are idempotent (ON CONFLICT DO NOTHING), so a repeat
	// is two no-ops rather than an error.
	_ = h.onsiteStore.AutoRegister(r.Context(), onsiteUser.ContestID, dbUser.ID)

	if h.maybe2FAChallenge(w, r, dbUser, arenaPath(r.Context(), h.contestStore, onsiteUser.ContestID)) {
		return
	}

	accessToken, err := h.jwt.GenerateAccessTokenWithState(dbUser.ID, dbUser.Username, dbUser.Role, dbUser.MustChangePassword)
	if err != nil {
		http.Error(w, "failed to generate tokens", http.StatusInternalServerError)
		return
	}
	rawRefresh, hashedRefresh := h.jwt.GenerateRefreshToken()
	if err := h.refreshToks.Create(r.Context(), dbUser.ID, hashedRefresh, time.Now().Add(h.jwt.RefreshTTL())); err != nil {
		http.Error(w, "failed to save refresh token", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": rawRefresh,
		"user":          dbUser,
		// A generated onsite account belongs to one round. Tell the client where
		// it goes, so signing in lands in that contest rather than the home page.
		"redirect_to": arenaPath(r.Context(), h.contestStore, onsiteUser.ContestID),
	})
}

// contestRefResolver is the one read arenaPath needs. Narrow on purpose: the
// full store.ContestStore is a large interface, and a test that wants to pin a
// redirect target should not have to stub twenty unrelated methods.
type contestRefResolver interface {
	GetByID(context.Context, string) (*model.Contest, error)
}

// arenaPath is the in-app destination for an onsite team account: the contest it
// was generated for.
//
// It returns the round's public slug when there is one, because that is the URL
// the rest of the site links to, and falls back to the id. An unresolvable round
// returns "" so the client keeps its own default rather than navigating to a
// broken path.
func arenaPath(ctx context.Context, contests contestRefResolver, contestID string) string {
	if contests == nil || contestID == "" {
		return ""
	}
	c, err := contests.GetByID(ctx, contestID)
	if err != nil || c == nil {
		return ""
	}
	if slug := c.Slug; slug != "" {
		return "/contests/" + slug
	}
	return "/contests/" + c.ID
}

// enrollOnsiteTeam links a signed-in account back to the onsite credential it
// belongs to, when it has one, and enrolls it in that round.
//
// The lookup is by username against onsite_batch_users, which is the only table
// that records "this round generated a credential for this handle". A normal
// account simply has no row there and this is a single indexed miss, so the
// ordinary login path pays almost nothing for the check.
//
// Deliberately not inferred from the email domain. Onsite accounts get a
// synthetic …@onsite.aioj address, but the credential row is the authoritative
// link, and keying access off a string an admin could edit would make the email
// field a way into a round.
//
// Failures are swallowed on purpose: a login must not fail because enrollment
// bookkeeping did. The next sign-in retries, and every write is idempotent.
func (h *AuthHandler) enrollOnsiteTeam(ctx context.Context, user *model.User) {
	if h.onsiteStore == nil || user == nil {
		return
	}
	cred, err := h.onsiteStore.GetByUsername(ctx, user.Username)
	if err != nil || cred == nil {
		return
	}
	// A credential that belongs to a DIFFERENT account must not grant this one
	// anything. The username match is normally enough — it is UNIQUE — but the
	// check costs nothing and keeps a reused handle from inheriting an old
	// credential's round.
	if cred.UsedBy != nil && *cred.UsedBy != user.ID {
		return
	}
	_ = h.onsiteStore.AutoRegister(ctx, cred.ContestID, user.ID)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	hsh := sha256.Sum256([]byte(req.RefreshToken))
	// Consume (not Validate): the presented token is spent here, so the response
	// carries a rotated one and a replay of the old value is rejected.
	userID, err := h.refreshToks.Consume(r.Context(), hex.EncodeToString(hsh[:]))
	if err != nil {
		// Only a spent token is a verdict on the session. A store that cannot
		// answer is not: reporting 401 would make a database blip or a deploy
		// look like an expired session, and the client would sign the user out
		// with no way to retry. 503 keeps the token alive for the next attempt.
		if !errors.Is(err, postgres.ErrRefreshTokenSpent) {
			slog.Error("refresh token lookup failed", "error", err)
			http.Error(w, "session store unavailable", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}
	user, _ := h.users.GetByID(r.Context(), userID)
	if user == nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	// Refresh is the long-lived path (168h TTL), so a ban has to be enforced
	// here too: banning revokes the stored tokens, but a request already in
	// flight must not be able to mint a fresh one.
	if banned, err := h.users.IsBanned(r.Context(), user.ID); err == nil && banned {
		http.Error(w, "this account has been suspended", http.StatusForbidden)
		return
	}
	respondJSON(w, http.StatusOK, h.tokenResp(r.Context(), user))
}

// Logout revokes the presented refresh token. It is deliberately
// unauthenticated and idempotent: a client whose access token has already
// expired must still be able to end its session, and an unknown token is
// reported as success so logout never leaks whether a token was valid.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.RefreshToken != "" {
		hsh := sha256.Sum256([]byte(req.RefreshToken))
		_, _ = h.refreshToks.Consume(r.Context(), hex.EncodeToString(hsh[:]))
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req model.ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Email == "" {
		http.Error(w, "email required", http.StatusBadRequest)
		return
	}

	// Always return success to prevent email enumeration
	user, err := h.users.GetByEmail(r.Context(), req.Email)
	if err != nil || user == nil {
		respondJSON(w, http.StatusOK, map[string]string{"message": "If the email exists, a reset link has been sent"})
		return
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		respondJSON(w, http.StatusOK, map[string]string{"message": "If the email exists, a reset link has been sent"})
		return
	}
	rawToken := hex.EncodeToString(tokenBytes)
	tokenHash := sha256.Sum256([]byte(rawToken))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	tokenID := uuid.New().String()
	if err := h.passwordResetToks.Create(r.Context(), tokenID, user.ID, tokenHashStr, time.Now().Add(1*time.Hour)); err != nil {
		respondJSON(w, http.StatusOK, map[string]string{"message": "If the email exists, a reset link has been sent"})
		return
	}

	if h.mail != nil && h.mailTpl != nil && user.Email != "" {
		resetURL := strings.TrimRight(h.publicURL, "/") + "/reset-password?token=" + rawToken
		var body bytes.Buffer
		if err := h.mailTpl.ExecuteTemplate(&body, "password_reset.txt", map[string]string{
			"Username": user.Username,
			"ResetURL": resetURL,
			"Expiry":   "1 hour",
		}); err == nil {
			from := h.mailFrom
			if from == "" {
				from = "noreply@aioj.com"
			}
			_ = h.mail.Send(r.Context(), &mail.Message{
				From: from, To: []string{user.Email},
				Subject: "Reset your AIOJ password", Body: body.String(),
			})
		}
	}

	// Never return the raw token — enumeration-safe success only.
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "If the email exists, a reset link has been sent",
	})
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req model.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Token == "" || req.NewPassword == "" {
		http.Error(w, "token and new_password required", http.StatusBadRequest)
		return
	}
	if err := auth.ValidatePasswordStrength(req.NewPassword); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if auth.IsCommonPassword(req.NewPassword) {
		http.Error(w, auth.ErrPasswordCommon.Error(), http.StatusBadRequest)
		return
	}

	tokenHash := sha256.Sum256([]byte(req.Token))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	tok, err := h.passwordResetToks.GetByHash(r.Context(), tokenHashStr)
	if err != nil || tok == nil {
		http.Error(w, "invalid or expired token", http.StatusBadRequest)
		return
	}
	if tok.Used {
		http.Error(w, "token already used", http.StatusBadRequest)
		return
	}
	if time.Now().After(tok.ExpiresAt) {
		http.Error(w, "token expired", http.StatusBadRequest)
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := h.users.UpdatePassword(r.Context(), tok.UserID, hash); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := h.passwordResetToks.MarkUsed(r.Context(), tok.ID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// A reset is how a locked-out user recovers from a compromise. Every session
	// issued with the old password must die with it.
	if err := h.refreshToks.RevokeAll(r.Context(), tok.UserID); err != nil {
		slog.Error("revoke refresh tokens after reset failed", "user_id", tok.UserID, "error", err)
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "password reset successfully"})
}

// ResendVerification re-sends the verify link for the logged-in user.
// Enumeration-safe: always 200 with the same message.
func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := h.users.GetByID(r.Context(), claims.UserID)
	if err != nil || user == nil {
		respondJSON(w, http.StatusOK, map[string]string{
			"message": "If the account needs verification, a link has been sent",
		})
		return
	}
	if verified, _ := h.users.IsEmailVerified(r.Context(), user.ID); verified {
		respondJSON(w, http.StatusOK, map[string]string{
			"message": "If the account needs verification, a link has been sent",
		})
		return
	}
	h.sendVerificationEmail(r.Context(), user)
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "If the account needs verification, a link has been sent",
	})
}

// maybe2FAChallenge returns true and writes a challenge response when the
// user has TOTP 2FA enabled. Returns false when tokens should be issued.
func (h *AuthHandler) maybe2FAChallenge(w http.ResponseWriter, r *http.Request, user *model.User, redirect string) bool {
	if user == nil {
		return false
	}
	return writeTwoFAChallenge(r.Context(), w, h.jwt, h.twoFA, user.ID, redirect)
}

func (h *AuthHandler) tokenResp(ctx context.Context, user *model.User) *model.AuthResponse {
	access, err := h.jwt.GenerateAccessTokenWithState(user.ID, user.Username, user.Role, user.MustChangePassword)
	if err != nil {
		slog.Error("generate access token failed", "user_id", user.ID, "error", err)
	}
	raw, hashed := h.jwt.GenerateRefreshToken()
	if err := h.refreshToks.Create(ctx, user.ID, hashed, time.Now().Add(h.jwt.RefreshTTL())); err != nil {
		// The client would receive a refresh token that does not exist, so it
		// can only re-login once the access token expires. Make that visible.
		slog.Error("persist refresh token failed", "user_id", user.ID, "error", err)
	}
	return &model.AuthResponse{AccessToken: access, RefreshToken: raw, User: user}
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
