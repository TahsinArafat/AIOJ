package oauth

import (
	"testing"
	"time"
)

func TestStateToken_CreateAndValidate(t *testing.T) {
	SetStateSecret([]byte("test-secret"))
	raw, sig, err := IssueStateToken(5 * time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := ValidateStateToken(raw, sig, 5*time.Minute)
	if err != nil || !ok {
		t.Errorf("expected valid, got ok=%v err=%v", ok, err)
	}
}

func TestStateToken_RejectsTampered(t *testing.T) {
	SetStateSecret([]byte("test-secret"))
	raw, sig, _ := IssueStateToken(5 * time.Minute)
	sig[0] ^= 0xff
	ok, _ := ValidateStateToken(raw, sig, 5*time.Minute)
	if ok {
		t.Error("expected tampered signature to fail")
	}
}

func TestStateToken_RejectsExpired(t *testing.T) {
	SetStateSecret([]byte("test-secret"))
	raw, sig, _ := IssueStateToken(-1 * time.Second)
	ok, _ := ValidateStateToken(raw, sig, 5*time.Minute)
	if ok {
		t.Error("expected expired token to fail")
	}
}
