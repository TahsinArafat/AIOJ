package oauth

import (
	"strings"
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

func TestStateToken_CarriesNext(t *testing.T) {
	SetStateSecret([]byte("test-secret"))
	// A path may contain dots and a query string; the base64url encoding keeps
	// the "." split unambiguous.
	raw, sig, err := IssueStateToken(5*time.Minute, "/problems?difficulty=hard")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := ValidateStateToken(raw, sig, 5*time.Minute); err != nil || !ok {
		t.Fatalf("expected valid, got ok=%v err=%v", ok, err)
	}
	if got := StateTokenNext(raw); got != "/problems?difficulty=hard" {
		t.Errorf("StateTokenNext = %q", got)
	}
}

func TestStateToken_WithoutNext(t *testing.T) {
	SetStateSecret([]byte("test-secret"))
	raw, sig, _ := IssueStateToken(5 * time.Minute)
	if ok, _ := ValidateStateToken(raw, sig, 5*time.Minute); !ok {
		t.Fatal("expected valid")
	}
	if got := StateTokenNext(raw); got != "" {
		t.Errorf("StateTokenNext = %q, want empty", got)
	}
}

func TestNormalizeNext(t *testing.T) {
	cases := map[string]string{
		"/problems":                     "/problems",
		"/contests/abc/problem/A?x=1":   "/contests/abc/problem/A?x=1",
		"":                              "",
		"//evil.com":                    "",
		"https://evil.com":              "",
		`/\evil.com`:                    "",
		"/" + strings.Repeat("a", 1025): "",
	}
	for in, want := range cases {
		if got := NormalizeNext(in); got != want {
			t.Errorf("NormalizeNext(%q) = %q, want %q", in, got, want)
		}
	}
}
