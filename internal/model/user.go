package model

import "time"

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email,omitempty"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	IsBot        bool      `json:"is_bot"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type UserProfile struct {
	UserID         string `json:"user_id"`
	Rating         int    `json:"rating"`
	MaxRating      int    `json:"max_rating"`
	ContestCount   int    `json:"contest_count"`
	ProblemsSolved int    `json:"problems_solved"`
	Submissions    int    `json:"submissions"`
	Bio            string `json:"bio,omitempty"`
	AvatarURL      string `json:"avatar_url,omitempty"`
	FirstName      string `json:"first_name,omitempty"`
	LastName       string `json:"last_name,omitempty"`
	Country        string `json:"country,omitempty"`
	City           string `json:"city,omitempty"`
	Organization   string `json:"organization,omitempty"`
	GithubURL      string `json:"github_url,omitempty"`
	ShowEmail      bool   `json:"show_email"`
	ShowTags       bool   `json:"show_tags"`
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
