package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp/totp"
)

// GenerateTOTPSecret returns a 20-byte secret encoded as base32 (no padding).
func GenerateTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

func ValidateTOTPCode(secret, code string) bool {
	return totp.Validate(code, secret)
}

// GenerateTOTPCodeForTest returns the current TOTP code for the secret.
// Use only in tests.
func GenerateTOTPCodeForTest(secret string) (string, error) {
	return totp.GenerateCode(secret, time.Now())
}

// generateTOTPCodeAt is a test helper that produces a code for a given
// 30-second time step. Unexported.
func generateTOTPCodeAt(secret string, t int64) string {
	sec, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(t))
	h := hmac.New(sha1.New, sec)
	h.Write(buf[:])
	sum := h.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	code := truncated % 1_000_000
	return fmt.Sprintf("%06d", code)
}
