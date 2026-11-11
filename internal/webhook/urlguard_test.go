package webhook

import (
	"errors"
	"testing"
)

/**
 * A webhook URL is attacker-controlled and the server POSTs to it, which is the
 * textbook SSRF primitive. These tests pin the guard: the API process must never
 * be usable as a proxy into loopback, the private network, or a cloud metadata
 * endpoint.
 */

func TestValidateURLRejectsInternalTargets(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want error
	}{
		{"loopback v4", "https://127.0.0.1/hook", ErrPrivateTarget},
		{"loopback v6", "https://[::1]/hook", ErrPrivateTarget},
		{"cloud metadata", "https://169.254.169.254/latest/meta-data/", ErrPrivateTarget},
		{"rfc1918 10/8", "https://10.0.0.5/hook", ErrPrivateTarget},
		{"rfc1918 192.168", "https://192.168.1.1/hook", ErrPrivateTarget},
		{"rfc1918 172.16", "https://172.16.0.1/hook", ErrPrivateTarget},
		{"unique local v6", "https://[fd00::1]/hook", ErrPrivateTarget},
		{"v4-mapped v6 loopback", "https://[::ffff:127.0.0.1]/hook", ErrPrivateTarget},
		{"unspecified", "https://0.0.0.0/hook", ErrPrivateTarget},
		{"localhost name", "https://localhost/hook", ErrPrivateTarget},
		{"localhost subdomain", "https://api.localhost/hook", ErrPrivateTarget},
		{"dot internal", "https://redis.internal/hook", ErrPrivateTarget},
		{"dot local", "https://printer.local/hook", ErrPrivateTarget},

		{"plain http", "http://example.com/hook", ErrNotHTTPS},
		{"no scheme", "example.com/hook", ErrBadURL},
		{"empty", "", ErrBadURL},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateURL(tc.url)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ValidateURL(%q) = %v, want %v", tc.url, err, tc.want)
			}
		})
	}
}

// A public host must pass, or the guard would make the feature unusable.
func TestValidateURLAcceptsAPublicHTTPSHost(t *testing.T) {
	// IP literal, so the test needs no DNS and is not flaky offline.
	if err := ValidateURL("https://93.184.216.34/hook"); err != nil {
		t.Fatalf("a public https URL was rejected: %v", err)
	}
}

func TestSignAndVerify(t *testing.T) {
	body := []byte(`{"event":"problem_review"}`)

	sig := Sign("s3cret", body)
	if !Verify("s3cret", body, sig) {
		t.Fatal("a signature did not verify against its own body")
	}
	if Verify("wrong", body, sig) {
		t.Fatal("a signature verified with the wrong secret")
	}
	if Verify("s3cret", []byte(`{"tampered":true}`), sig) {
		t.Fatal("a signature verified against a tampered body")
	}
}
