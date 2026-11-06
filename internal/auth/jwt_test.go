package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func testManager() *JWTManager {
	return NewJWTManager(strings.Repeat("k", 32), 15*time.Minute, 168*time.Hour)
}

func TestValidateToken_AcceptsAccessToken(t *testing.T) {
	m := testManager()
	token, err := m.GenerateAccessToken("user-1", "alice", "admin")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := m.ValidateToken(token)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if claims.UserID != "user-1" || claims.Username != "alice" || claims.Role != "admin" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.Type != TokenTypeAccess {
		t.Fatalf("Type = %q, want %q", claims.Type, TokenTypeAccess)
	}
}

// TestValidateToken_Rejects2FAChallenge is the regression test for the token
// type confusion: the challenge token is minted after the password check alone,
// so accepting it as a session token skipped the second factor entirely.
func TestValidateToken_Rejects2FAChallenge(t *testing.T) {
	m := testManager()
	challenge, err := m.GenerateChallengeToken("user-1", "challenge-1", 5*time.Minute)
	if err != nil {
		t.Fatalf("generate challenge: %v", err)
	}

	if _, err := m.ValidateToken(challenge); err == nil {
		t.Fatal("a 2FA challenge token must not validate as an access token")
	}
}

func TestValidateToken_RejectsMissingSubject(t *testing.T) {
	m := testManager()
	// Correct type, but no uid — the old code returned this with UserID "".
	claims := Claims{
		Role: "admin",
		Type: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, err := m.ValidateToken(token); err == nil {
		t.Fatal("a token without a subject must be rejected")
	}
}

func TestValidateToken_RejectsUntypedToken(t *testing.T) {
	m := testManager()
	// A token from a build before the type claim existed.
	claims := jwt.MapClaims{
		"uid":  "user-1",
		"role": "admin",
		"exp":  time.Now().Add(time.Minute).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, err := m.ValidateToken(token); err == nil {
		t.Fatal("an untyped token must be rejected")
	}
}

func TestValidateToken_RejectsForeignSecret(t *testing.T) {
	issuer := testManager()
	token, err := issuer.GenerateAccessToken("user-1", "alice", "user")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	other := NewJWTManager(strings.Repeat("z", 32), time.Minute, time.Hour)
	if _, err := other.ValidateToken(token); err == nil {
		t.Fatal("a token signed with another key must be rejected")
	}
}

func TestParseChallengeToken(t *testing.T) {
	m := testManager()
	challenge, err := m.GenerateChallengeToken("user-1", "challenge-1", 5*time.Minute)
	if err != nil {
		t.Fatalf("generate challenge: %v", err)
	}

	claims, err := m.ParseChallengeToken(challenge)
	if err != nil {
		t.Fatalf("parse challenge: %v", err)
	}
	if claims.UserID != "user-1" || claims.ChallengeID != "challenge-1" {
		t.Fatalf("unexpected challenge claims: %+v", claims)
	}

	// An access token must not be usable as a 2FA challenge.
	access, _ := m.GenerateAccessToken("user-1", "alice", "user")
	if _, err := m.ParseChallengeToken(access); err == nil {
		t.Fatal("an access token must not parse as a 2FA challenge")
	}
}

func TestGenerateRefreshToken_UniqueAndHashed(t *testing.T) {
	m := testManager()
	raw1, hash1 := m.GenerateRefreshToken()
	raw2, hash2 := m.GenerateRefreshToken()

	if raw1 == raw2 || hash1 == hash2 {
		t.Fatal("refresh tokens must be unique")
	}
	if len(raw1) != 64 || len(hash1) != 64 {
		t.Fatalf("expected 64 hex chars, got raw=%d hash=%d", len(raw1), len(hash1))
	}
	if strings.Contains(hash1, raw1) || hash1 == raw1 {
		t.Fatal("the stored value must be a hash, not the raw token")
	}
}
