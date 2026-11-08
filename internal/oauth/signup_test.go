package oauth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSignupToken_RoundTrip(t *testing.T) {
	in := &PendingSignup{
		Provider: "google", ProviderUserID: "g-1",
		Email: "a@x.com", SuggestedName: "alice",
	}
	tok, err := IssueSignupToken([]byte("s"), in, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ParseSignupToken([]byte("s"), tok)
	if err != nil {
		t.Fatal(err)
	}
	if out.Provider != in.Provider || out.ProviderUserID != in.ProviderUserID ||
		out.Email != in.Email || out.SuggestedName != in.SuggestedName {
		t.Errorf("round trip lost data: %+v", out)
	}
}

func TestSignupToken_RejectsTamperedPayload(t *testing.T) {
	tok, err := IssueSignupToken([]byte("s"), &PendingSignup{Provider: "github", ProviderUserID: "1"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	// Swap the payload for another identity, keeping the original signature.
	other, err := IssueSignupToken([]byte("s"), &PendingSignup{Provider: "github", ProviderUserID: "2"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.SplitN(other, ".", 2)[0] + "." + strings.SplitN(tok, ".", 2)[1]
	if _, err := ParseSignupToken([]byte("s"), tampered); !errors.Is(err, ErrSignupTokenInvalid) {
		t.Errorf("expected an invalid-ticket error, got %v", err)
	}
}

func TestSignupToken_RejectsWrongSecret(t *testing.T) {
	tok, err := IssueSignupToken([]byte("s"), &PendingSignup{Provider: "github", ProviderUserID: "1"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseSignupToken([]byte("other"), tok); !errors.Is(err, ErrSignupTokenInvalid) {
		t.Errorf("expected an invalid-ticket error, got %v", err)
	}
}

func TestSignupToken_RejectsExpired(t *testing.T) {
	tok, err := IssueSignupToken([]byte("s"), &PendingSignup{Provider: "github", ProviderUserID: "1"}, -time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseSignupToken([]byte("s"), tok); !errors.Is(err, ErrSignupTokenExpired) {
		t.Errorf("expected an expired-ticket error, got %v", err)
	}
}

func TestSignupToken_RejectsIncompleteIdentity(t *testing.T) {
	// A ticket naming no provider could not be turned into an account.
	tok, err := IssueSignupToken([]byte("s"), &PendingSignup{SuggestedName: "alice"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseSignupToken([]byte("s"), tok); !errors.Is(err, ErrSignupTokenInvalid) {
		t.Errorf("expected an invalid-ticket error, got %v", err)
	}
}

func TestSignupToken_RejectsGarbage(t *testing.T) {
	for _, tok := range []string{"", ".", "abc", "abc.", ".def", "not.a.token"} {
		if _, err := ParseSignupToken([]byte("s"), tok); err == nil {
			t.Errorf("%q should not parse", tok)
		}
	}
}

func TestNormalizeUsername(t *testing.T) {
	ok := map[string]string{
		"Alice":      "alice",
		"  bob_id  ": "bob_id",
		"a.b-c_1":    "a.b-c_1",
		"abc":        "abc",
	}
	for in, want := range ok {
		got, err := NormalizeUsername(in)
		if err != nil || got != want {
			t.Errorf("NormalizeUsername(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	bad := []string{"", "ab", strings.Repeat("a", 33), "has space", "_leading", "trailing_", "dash-", "dot.", "ünïcode", "a<b"}
	for _, in := range bad {
		if _, err := NormalizeUsername(in); err == nil {
			t.Errorf("NormalizeUsername(%q) should be rejected", in)
		}
	}
}

func TestSuggestUsername_AlwaysUsable(t *testing.T) {
	// Provider handles are often too short, padded with spaces, or already
	// mangled; whatever comes out must still pass validation so the form can
	// prefill it.
	for _, raw := range []string{"", "ab", "Octo Cat", "  ", "..", "averylongdisplayname", "ünïcode"} {
		got := SuggestUsername(raw)
		if _, err := NormalizeUsername(got); err != nil {
			t.Errorf("SuggestUsername(%q) = %q, which is not a valid handle: %v", raw, got, err)
		}
	}
}
