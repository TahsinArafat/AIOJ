package ai

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestCredentialBoxRoundTripAndTamperDetection(t *testing.T) {
	box, err := NewCredentialBox([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt("sk-secret")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ciphertext, credentialPrefix) || strings.Contains(ciphertext, "sk-secret") {
		t.Fatalf("unsafe ciphertext %q", ciphertext)
	}
	plain, err := box.Decrypt(ciphertext)
	if err != nil || plain != "sk-secret" {
		t.Fatalf("Decrypt = %q, %v", plain, err)
	}
	payload, _ := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(ciphertext, credentialPrefix))
	payload[len(payload)/2] ^= 0x01
	tampered := credentialPrefix + base64.RawStdEncoding.EncodeToString(payload)
	if _, err := box.Decrypt(tampered); err == nil {
		t.Fatal("tampered credential decrypted")
	}
}

func TestCredentialBoxRejectsWrongKeyAndLegacyPlaintext(t *testing.T) {
	a, _ := NewCredentialBox([]byte("0123456789abcdef0123456789abcdef"))
	b, _ := NewCredentialBox([]byte("abcdef0123456789abcdef0123456789"))
	ciphertext, _ := a.Encrypt("secret")
	if _, err := b.Decrypt(ciphertext); err == nil {
		t.Fatal("wrong key decrypted credential")
	}
	if _, err := a.Decrypt("plain-secret"); err == nil {
		t.Fatal("legacy plaintext accepted")
	}
}

func TestParseCredentialKey(t *testing.T) {
	if _, err := ParseCredentialKey("too-short"); err == nil {
		t.Fatal("accepted short key")
	}
	key, err := ParseCredentialKey("0123456789abcdef0123456789abcdef")
	if err != nil || len(key) != 32 {
		t.Fatalf("key len=%d err=%v", len(key), err)
	}
}
