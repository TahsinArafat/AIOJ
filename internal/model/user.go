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
	ProblemsSolved int    `json:"problems_solved"`
	Submissions    int    `json:"submissions"`
	Bio            string `json:"bio,omitempty"`
	AvatarURL      string `json:"avatar_url,omitempty"`
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
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         *User  `json:"user"`
}

type PublicProfile struct {
	ID             string    `json:"id"`
	Username       string    `json:"username"`
	Rating         int       `json:"rating"`
	RatingChange   int       `json:"rating_change"`
	ContestsPlayed int       `json:"contests_played"`
	ProblemsSolved int       `json:"problems_solved"`
	CreatedAt      time.Time `json:"created_at"`
}

type SetterApplication struct {
	UserID    string    `json:"user_id"`
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
}

type PasswordResetToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}
