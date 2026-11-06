package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Token type tags. Access tokens and 2FA challenge tokens are both signed with
// the same key, so the type claim is what keeps one from being replayed as the
// other: a challenge token is issued after the password check alone, and
// without this tag it was accepted by AuthMiddleware as a valid session.
const (
	TokenTypeAccess = "access"
	TokenType2FA    = "2fa"
)

type Claims struct {
	UserID   string `json:"uid"`
	Username string `json:"uname"`
	Role     string `json:"role"`
	// Type must equal TokenTypeAccess for the token to authorize a request.
	Type string `json:"typ"`
	// MustChangePassword is set while the account still has a seeded or
	// admin-reset password. AuthMiddleware restricts such a session to the
	// endpoints needed to set a new one.
	MustChangePassword bool `json:"pwd_change,omitempty"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewJWTManager(secret string, accessTTL, refreshTTL time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (m *JWTManager) GenerateAccessToken(userID, username, role string) (string, error) {
	return m.GenerateAccessTokenWithState(userID, username, role, false)
}

// GenerateAccessTokenWithState issues a session token that also carries the
// account's password-change requirement, so middleware can enforce it without a
// database round trip on every request.
func (m *JWTManager) GenerateAccessTokenWithState(userID, username, role string, mustChangePassword bool) (string, error) {
	claims := Claims{
		UserID:             userID,
		Username:           username,
		Role:               role,
		Type:               TokenTypeAccess,
		MustChangePassword: mustChangePassword,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	// Reject anything that is not a session token. A 2FA challenge token is
	// signed with this same key but proves only "password was correct", and it
	// carries no uid — accepting it here authenticated the caller as user "".
	if claims.Type != TokenTypeAccess {
		return nil, fmt.Errorf("not an access token")
	}
	if claims.UserID == "" {
		return nil, fmt.Errorf("token has no subject")
	}
	return claims, nil
}

func (m *JWTManager) GenerateRefreshToken() (raw, hashed string) {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	raw = hex.EncodeToString(bytes)
	h := sha256.Sum256([]byte(raw))
	hashed = hex.EncodeToString(h[:])
	return
}

func (m *JWTManager) RefreshTTL() time.Duration { return m.refreshTTL }

// ChallengeClaims are the parsed contents of a short-lived 2FA challenge JWT.
type ChallengeClaims struct {
	UserID      string
	ChallengeID string
}

// GenerateChallengeToken issues a 5-minute-class JWT used only for the
// second login step (type claim must be "2fa").
func (m *JWTManager) GenerateChallengeToken(userID, challengeID string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub":          userID,
		"challenge_id": challengeID,
		"type":         "2fa",
		"exp":          time.Now().Add(ttl).Unix(),
		"iat":          time.Now().Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(m.secret)
}

// ParseChallengeToken validates a 2FA challenge JWT and returns its claims.
func (m *JWTManager) ParseChallengeToken(tokenString string) (*ChallengeClaims, error) {
	parsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("bad claims type")
	}
	if typ, _ := claims["type"].(string); typ != "2fa" {
		return nil, errors.New("not a 2fa challenge token")
	}
	sub, _ := claims["sub"].(string)
	cid, _ := claims["challenge_id"].(string)
	return &ChallengeClaims{UserID: sub, ChallengeID: cid}, nil
}
