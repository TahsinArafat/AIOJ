package oauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"time"
)

var stateSecret []byte

func SetStateSecret(b []byte) { stateSecret = b }

// IssueStateToken returns raw="nonce.exp" and an HMAC signature.
func IssueStateToken(ttl time.Duration) (string, []byte, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", nil, err
	}
	exp := time.Now().Add(ttl).Unix()
	raw := hex.EncodeToString(nonce) + "." + strconv.FormatInt(exp, 10)
	return raw, sign(raw), nil
}

// ValidateStateToken checks HMAC equality and expiry embedded in raw.
func ValidateStateToken(raw string, sig []byte, _ time.Duration) (bool, error) {
	expected := sign(raw)
	if !hmac.Equal(sig, expected) {
		return false, errors.New("bad signature")
	}
	parts := splitRaw(raw)
	if len(parts) != 2 {
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

// IssueStateTokenWith issues a token using an explicit secret (tests / handlers).
func IssueStateTokenWith(secret []byte, ttl time.Duration) (string, []byte, error) {
	prev := stateSecret
	stateSecret = secret
	defer func() { stateSecret = prev }()
	return IssueStateToken(ttl)
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

func splitRaw(raw string) []string {
	for i := len(raw) - 1; i >= 0; i-- {
		if raw[i] == '.' {
			return []string{raw[:i], raw[i+1:]}
		}
	}
	return nil
}

// EncodeSig / DecodeSig convert HMAC bytes for URL transport.
func EncodeSig(sig []byte) string { return base64.RawURLEncoding.EncodeToString(sig) }

func DecodeSig(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
