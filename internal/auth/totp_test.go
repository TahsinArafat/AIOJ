package auth

import (
	"testing"
	"time"
)

func TestGenerateSecret(t *testing.T) {
	s, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 32 { // base32 of 20 bytes
		t.Errorf("expected 32-char secret, got %d", len(s))
	}
}

func TestValidateCode(t *testing.T) {
	secret, _ := GenerateTOTPSecret()
	code, err := GenerateTOTPCodeForTest(secret)
	if err != nil {
		t.Fatal(err)
	}
	if !ValidateTOTPCode(secret, code) {
		t.Error("expected code to validate")
	}
}

func TestValidateCode_RejectsExpired(t *testing.T) {
	secret, _ := GenerateTOTPSecret()
	old := time.Now().Add(-10*time.Minute).Unix() / 30
	code := generateTOTPCodeAt(secret, old)
	if ValidateTOTPCode(secret, code) {
		t.Error("expected old code to be rejected")
	}
}
