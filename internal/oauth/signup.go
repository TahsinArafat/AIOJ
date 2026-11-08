package oauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PendingSignup is the provider identity of a first-time sign-in, before the
// local account exists.
//
// Nothing is written to the database by the OAuth callback itself: the browser
// is handed a signed copy of this struct and posts it back together with the
// username the person picked. Two things follow from that. An abandoned
// sign-in leaves no orphan row, and a provider-derived handle never becomes a
// real account name behind somebody's back.
type PendingSignup struct {
	Provider       string `json:"p"`
	ProviderUserID string `json:"u"`
	Email          string `json:"e,omitempty"`
	SuggestedName  string `json:"s,omitempty"`
	ExpiresAt      int64  `json:"x"`
}

var (
	ErrSignupTokenInvalid = errors.New("sign-up ticket is not valid")
	ErrSignupTokenExpired = errors.New("sign-up ticket has expired, start again")
)

// IssueSignupToken serializes and signs a pending sign-up. The secret is the
// same one that signs OAuth state, so no extra configuration is needed.
func IssueSignupToken(secret []byte, p *PendingSignup, ttl time.Duration) (string, error) {
	if p.ExpiresAt == 0 {
		p.ExpiresAt = time.Now().Add(ttl).Unix()
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + base64.RawURLEncoding.EncodeToString(signSignup(secret, body)), nil
}

// ParseSignupToken verifies the signature and expiry and returns the identity.
// It fails closed: anything that is not an unexpired, correctly signed entry
// is an error, so a tampered ticket can never mint an account.
func ParseSignupToken(secret []byte, token string) (*PendingSignup, error) {
	body, sig, ok := strings.Cut(token, ".")
	if !ok || body == "" || sig == "" {
		return nil, ErrSignupTokenInvalid
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, signSignup(secret, body)) {
		return nil, ErrSignupTokenInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil, ErrSignupTokenInvalid
	}
	var p PendingSignup
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, ErrSignupTokenInvalid
	}
	if p.Provider == "" || p.ProviderUserID == "" {
		return nil, ErrSignupTokenInvalid
	}
	if time.Now().Unix() > p.ExpiresAt {
		return nil, ErrSignupTokenExpired
	}
	return &p, nil
}

func signSignup(secret []byte, body string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(body))
	return mac.Sum(nil)
}

// Handle rules for a username somebody chooses themselves.
const (
	UsernameMinLen = 3
	UsernameMaxLen = 32
)

var (
	ErrUsernameTooShort = fmt.Errorf("username must be at least %d characters", UsernameMinLen)
	ErrUsernameTooLong  = fmt.Errorf("username must be at most %d characters", UsernameMaxLen)
	ErrUsernameCharset  = errors.New("username may only use letters, numbers, dots, dashes and underscores, and must start with a letter or number")
)

// NormalizeUsername trims and lowercases a handle a person typed, and reports
// why it cannot be used.
//
// Unlike sanitizeUsername it never rewrites silently. A chosen handle is what
// other people will see in URLs and rankings, so an unusable one has to be
// rejected and re-typed rather than quietly turned into some other name.
func NormalizeUsername(raw string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(raw))
	if len(name) < UsernameMinLen {
		return "", ErrUsernameTooShort
	}
	if len(name) > UsernameMaxLen {
		return "", ErrUsernameTooLong
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		alnum := c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
		switch {
		case alnum:
		case i > 0 && (c == '_' || c == '-' || c == '.'):
		default:
			return "", ErrUsernameCharset
		}
	}
	if last := name[len(name)-1]; last == '_' || last == '-' || last == '.' {
		return "", ErrUsernameCharset
	}
	return name, nil
}

// SuggestUsername turns a provider-derived name into a handle that
// NormalizeUsername accepts, so the sign-up form can prefill a value that will
// go through unless somebody already holds it.
func SuggestUsername(raw string) string {
	if name, err := NormalizeUsername(raw); err == nil {
		return name
	}
	if name, err := NormalizeUsername(strings.Trim(sanitizeUsername(raw), "_.-")); err == nil {
		return name
	}
	return "user"
}
