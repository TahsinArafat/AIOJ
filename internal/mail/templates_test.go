package mail

import (
	"bytes"
	"strings"
	"testing"
)

func TestTemplate_PasswordReset(t *testing.T) {
	tpl, err := LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	var buf bytes.Buffer
	data := map[string]string{
		"Username": "alice",
		"ResetURL": "https://aioj.com/reset?token=abc",
		"Expiry":   "1 hour",
	}
	if err := tpl.ExecuteTemplate(&buf, "password_reset.txt", data); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := buf.String()
	for _, want := range []string{"alice", "https://aioj.com/reset?token=abc", "1 hour"} {
		if !strings.Contains(got, want) {
			t.Errorf("template missing %q. got:\n%s", want, got)
		}
	}
}

func TestTemplate_EmailVerification(t *testing.T) {
	tpl, err := LoadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "email_verification.txt", map[string]string{
		"Username": "bob", "VerifyURL": "https://aioj.com/verify?token=xyz", "ExpiryMinutes": "24",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "bob") {
		t.Errorf("missing username. got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "https://aioj.com/verify?token=xyz") {
		t.Errorf("missing verify URL. got:\n%s", buf.String())
	}
}
