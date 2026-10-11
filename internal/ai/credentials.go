package ai

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const credentialPrefix = "enc:v1:"

// CredentialBox encrypts provider credentials with AES-256-GCM. Ciphertexts
// are self-identifying so future key/envelope versions can coexist.
type CredentialBox struct{ aead cipher.AEAD }

func NewCredentialBox(key []byte) (*CredentialBox, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("AI credential master key must be exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &CredentialBox{aead: aead}, nil
}

func ParseCredentialKey(encoded string) ([]byte, error) {
	if encoded == "" {
		return nil, nil
	}
	if raw, err := base64.StdEncoding.DecodeString(encoded); err == nil && len(raw) == 32 {
		return raw, nil
	}
	if len(encoded) == 32 {
		return []byte(encoded), nil
	}
	return nil, errors.New("AI_CREDENTIAL_MASTER_KEY must be base64-encoded 32 bytes or a 32-byte string")
}

func IsEncryptedCredential(value string) bool { return strings.HasPrefix(value, credentialPrefix) }

func (b *CredentialBox) Encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	if b == nil {
		return "", errors.New("AI credential encryption is not configured")
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := b.aead.Seal(nil, nonce, []byte(plain), []byte(credentialPrefix))
	payload := append(nonce, sealed...)
	return credentialPrefix + base64.RawStdEncoding.EncodeToString(payload), nil
}

func (b *CredentialBox) Decrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !IsEncryptedCredential(value) {
		return "", errors.New("AI credential is stored in legacy plaintext form")
	}
	if b == nil {
		return "", errors.New("AI credential encryption is not configured")
	}
	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, credentialPrefix))
	if err != nil {
		return "", errors.New("invalid encrypted AI credential")
	}
	if len(payload) < b.aead.NonceSize() {
		return "", errors.New("invalid encrypted AI credential")
	}
	nonce, ciphertext := payload[:b.aead.NonceSize()], payload[b.aead.NonceSize():]
	plain, err := b.aead.Open(nil, nonce, ciphertext, []byte(credentialPrefix))
	if err != nil {
		return "", errors.New("cannot decrypt AI credential")
	}
	return string(plain), nil
}
