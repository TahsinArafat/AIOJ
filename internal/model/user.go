package model

import "time"

type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Email        string `json:"email,omitempty"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	IsBot        bool   `json:"is_bot"`
	// MustChangePassword marks an account still holding a seeded or
	// admin-issued password. Its sessions may only reach the endpoints needed
	// to set a new one.
	MustChangePassword bool      `json:"must_change_password,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type UserProfile struct {
	UserID string `json:"user_id"`
	// Email and CreatedAt live on `users`, not `user_profiles`, and are read
	// for the owner's own settings page (never for another user: the public
	// shape is PublicProfile, which honours show_email).
	Email          string    `json:"email"`
	CreatedAt      time.Time `json:"created_at"`
	Rating         int       `json:"rating"`
	MaxRating      int       `json:"max_rating"`
	ContestCount   int       `json:"contest_count"`
	ProblemsSolved int       `json:"problems_solved"`
	Submissions    int       `json:"submissions"`
	Bio            string    `json:"bio,omitempty"`
	AvatarURL      string    `json:"avatar_url,omitempty"`
	FirstName      string    `json:"first_name,omitempty"`
	LastName       string    `json:"last_name,omitempty"`
	Country        string    `json:"country,omitempty"`
	City           string    `json:"city,omitempty"`
	Organization   string    `json:"organization,omitempty"`
	GithubURL      string    `json:"github_url,omitempty"`
	ShowEmail      bool      `json:"show_email"`
	ShowTags       bool      `json:"show_tags"`
}

// EmailVerificationToken is a one-shot link token for verifying an email.
type EmailVerificationToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	User         *User  `json:"user,omitempty"`
	Requires2FA  bool   `json:"requires_2fa,omitempty"`
	ChallengeID  string `json:"challenge_id,omitempty"`
}

// OAuthSignupChallenge is the answer to a first-time OAuth callback: no
// session yet, because the account is only created once the person has chosen
// a username. SignupToken is a signed, short-lived copy of the provider
// identity that CompleteSignup exchanges for a real account.
type OAuthSignupChallenge struct {
	RequiresUsername  bool   `json:"requires_username"`
	SignupToken       string `json:"signup_token"`
	SuggestedUsername string `json:"suggested_username,omitempty"`
	Provider          string `json:"provider,omitempty"`
	Email             string `json:"email,omitempty"`
}

// OAuthSignupRequest finishes a pending OAuth sign-up.
type OAuthSignupRequest struct {
	SignupToken string `json:"signup_token"`
	Username    string `json:"username"`
}

// OAuthUsernameCheck answers the sign-up form's availability lookup.
type OAuthUsernameCheck struct {
	Available bool   `json:"available"`
	Username  string `json:"username,omitempty"`
	Error     string `json:"error,omitempty"`
}

// TOTPSecret is a per-user TOTP enrollment (enabled after first valid code).
type TOTPSecret struct {
	UserID    string     `json:"user_id"`
	Secret    string     `json:"-"`
	Enabled   bool       `json:"enabled"`
	EnabledAt *time.Time `json:"enabled_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// OAuthLink binds an external provider identity to a local user account.
type OAuthLink struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	Provider       string    `json:"provider"`
	ProviderUserID string    `json:"provider_user_id"`
	LinkedAt       time.Time `json:"linked_at"`
}

type PublicProfile struct {
	ID             string    `json:"id"`
	Username       string    `json:"username"`
	Email          string    `json:"email,omitempty"`
	Rating         int       `json:"rating"`
	RatingChange   int       `json:"rating_change"`
	ContestsPlayed int       `json:"contests_played"`
	ProblemsSolved int       `json:"problems_solved"`
	Bio            string    `json:"bio,omitempty"`
	AvatarURL      string    `json:"avatar_url,omitempty"`
	FirstName      string    `json:"first_name,omitempty"`
	LastName       string    `json:"last_name,omitempty"`
	Country        string    `json:"country,omitempty"`
	City           string    `json:"city,omitempty"`
	Organization   string    `json:"organization,omitempty"`
	GithubURL      string    `json:"github_url,omitempty"`
	ShowEmail      bool      `json:"show_email"`
	ShowTags       bool      `json:"show_tags"`
	CreatedAt      time.Time `json:"created_at"`
}

type SetterApplication struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// UserFilter is the query the admin users panel issues. Zero values mean "no
// constraint": an empty Search matches everyone, an empty Role/Status disables
// that filter, and Sort/Desc fall back to newest-first.
//
// It is a struct rather than another positional argument list because the
// listing has five optional dimensions, and ListUsers(offset, limit) had
// already reached the point where every new filter meant touching every
// caller and every test stub.
type UserFilter struct {
	// Search matches username, email, or the exact user id (case-insensitive
	// substring for the first two).
	Search string
	// Role restricts to one role; Status restricts to one account state.
	Role string
	// Status is one of: "", "active", "banned", "unverified", "must_change_password".
	Status string
	// Sort is one of: "", "created", "username", "rating", "submissions", "last_login".
	Sort string
	// Desc reverses the sort. Only meaningful together with Sort.
	Desc   bool
	Offset int
	Limit  int
}

// AdminUserRow is one row of the admin users table: enough to identify the
// account, see its privileges and state, and judge whether it is in use —
// without a second request per row.
type AdminUserRow struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	IsBot    bool   `json:"is_bot"`

	EmailVerified      bool `json:"email_verified"`
	MustChangePassword bool `json:"must_change_password"`

	// BannedAt is nil for an active account. The timestamp, not a bool, is what
	// an appeal is judged against.
	BannedAt  *time.Time `json:"banned_at,omitempty"`
	BanReason string     `json:"ban_reason,omitempty"`

	Rating         int `json:"rating"`
	ProblemsSolved int `json:"problems_solved"`
	Submissions    int `json:"submissions"`

	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// AdminUserDetail is the single-user view behind the panel's detail drawer.
// It carries the profile fields an administrator needs to identify a person
// (country, organisation, linked GitHub) and the 2FA state that decides whether
// a password reset is enough to lock an attacker out.
type AdminUserDetail struct {
	AdminUserRow
	MaxRating    int    `json:"max_rating"`
	ContestCount int    `json:"contest_count"`
	Country      string `json:"country,omitempty"`
	Organization string `json:"organization,omitempty"`
	GithubURL    string `json:"github_url,omitempty"`
	Bio          string `json:"bio,omitempty"`
	TOTPEnabled  bool   `json:"totp_enabled"`
	// ActiveSessions is the number of unspent refresh tokens — the observable
	// answer to "is someone signed in as this user right now".
	ActiveSessions int `json:"active_sessions"`
}

type RankingEntry struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	Rating         int    `json:"rating"`
	RatingChange   int    `json:"rating_change"`
	ContestsPlayed int    `json:"contests_played"`
	Country        string `json:"country,omitempty"`
}

type PasswordResetToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

type UpdateProfileRequest struct {
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Country      string `json:"country"`
	City         string `json:"city"`
	Organization string `json:"organization"`
	GithubURL    string `json:"github_url"`
	Bio          string `json:"bio"`
	AvatarURL    string `json:"avatar_url"`
	ShowEmail    *bool  `json:"show_email"`
	ShowTags     *bool  `json:"show_tags"`
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}
