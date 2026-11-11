package oauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

var stateSecret []byte

func SetStateSecret(b []byte) { stateSecret = b }

// IssueStateToken returns raw="nonce.exp" and an HMAC signature.
//
// The optional next is a post-login destination carried inside the signed
// token, so it survives the provider round-trip without a cookie. It is
// base64url-encoded because a path may contain dots and the token is split on
// ".". The variadic keeps every existing call site unchanged.
func IssueStateToken(ttl time.Duration, next ...string) (string, []byte, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", nil, err
	}
	exp := time.Now().Add(ttl).Unix()
	raw := hex.EncodeToString(nonce) + "." + strconv.FormatInt(exp, 10)
	if len(next) > 0 && next[0] != "" {
		raw += "." + base64.RawURLEncoding.EncodeToString([]byte(next[0]))
	}
	return raw, sign(raw), nil
}

// ValidateStateToken checks HMAC equality and expiry embedded in raw.
func ValidateStateToken(raw string, sig []byte, _ time.Duration) (bool, error) {
	expected := sign(raw)
	if !hmac.Equal(sig, expected) {
		return false, errors.New("bad signature")
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 2 && len(parts) != 3 {
		return false, errors.New("malformed")
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return false, err
	}
	if time.Now().Unix() > exp {
		return false, errors.New("expired")
	}
	return true, nil
}

// StateTokenNext returns the post-login destination carried in a raw state
// token, or "" when the token has none. Only meaningful after
// ValidateStateToken has accepted the token: the value is signed, but it is
// not trusted for a redirect until NormalizeNext has vetted it.
func StateTokenNext(raw string) string {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return ""
	}
	next, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return ""
	}
	return string(next)
}

// NormalizeNext validates a post-login destination carried through the OAuth
// state token. Only same-origin paths are accepted: no scheme, no
// protocol-relative "//", and no backslash a browser would normalise to a
// slash. Anything else is dropped rather than rejected, because a sign-in
// must still complete.
func NormalizeNext(raw string) string {
	if raw == "" || len(raw) > 1024 {
		return ""
	}
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.Contains(raw, "\\") {
		return ""
	}
	return raw
}

// IssueStateTokenWith issues a token using an explicit secret (tests / handlers).
func IssueStateTokenWith(secret []byte, ttl time.Duration, next ...string) (string, []byte, error) {
	prev := stateSecret
	stateSecret = secret
	defer func() { stateSecret = prev }()
	return IssueStateToken(ttl, next...)
}

// ValidateStateTokenWith validates using an explicit secret.
func ValidateStateTokenWith(secret []byte, raw string, sig []byte, ttl time.Duration) (bool, error) {
	prev := stateSecret
	stateSecret = secret
	defer func() { stateSecret = prev }()
	return ValidateStateToken(raw, sig, ttl)
}

func sign(raw string) []byte {
	h := hmac.New(sha256.New, stateSecret)
	h.Write([]byte(raw))
	return h.Sum(nil)
}

// EncodeSig / DecodeSig convert HMAC bytes for URL transport.
func EncodeSig(sig []byte) string { return base64.RawURLEncoding.EncodeToString(sig) }

func DecodeSig(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
