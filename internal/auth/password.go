package auth

import (
	"errors"
	"strings"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

var ErrPasswordTooWeak = errors.New("password does not meet complexity requirements")
var ErrPasswordCommon = errors.New("password is too common")

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	return string(bytes), err
}

func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// ValidatePasswordStrength enforces:
//   - length >= 12
//   - at least one upper, one lower, one digit, one symbol
func ValidatePasswordStrength(p string) error {
	if len(p) < 12 {
		return ErrPasswordTooWeak
	}
	var hasUpper, hasLower, hasDigit, hasSym bool
	for _, r := range p {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSym = true
		}
	}
	if !(hasUpper && hasLower && hasDigit && hasSym) {
		return ErrPasswordTooWeak
	}
	return nil
}

// IsCommonPassword checks a small static list of top breached passwords.
// Production should swap in a HIBP k-anonymity feed.
var commonPasswords = map[string]bool{
	"password": true, "password123": true, "qwerty": true, "letmein": true,
	"123456789": true, "iloveyou": true, "admin123": true, "welcome": true,
	"monkey": true, "dragon": true,
}

func IsCommonPassword(p string) bool {
	return commonPasswords[strings.ToLower(p)]
}
