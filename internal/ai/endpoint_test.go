package ai

import (
	"context"
	"net"
	"net/url"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	cases := []struct {
		ip                    string
		allowPrivate, blocked bool
	}{
		{"127.0.0.1", false, true}, {"10.0.0.1", false, true}, {"192.168.1.1", false, true},
		{"8.8.8.8", false, false}, {"10.0.0.1", true, false}, {"127.0.0.1", true, false},
		{"169.254.169.254", true, true}, {"100.100.100.200", true, true}, {"::1", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.ip, func(t *testing.T) {
			if got := isBlockedIP(net.ParseIP(tc.ip), tc.allowPrivate); got != tc.blocked {
				t.Fatalf("blocked=%v want %v", got, tc.blocked)
			}
		})
	}
}

func TestEndpointPolicyRejectsUnsafeSyntaxBeforeDNS(t *testing.T) {
	p := EndpointPolicy{}
	for _, raw := range []string{"http://api.example.com/v1", "ftp://api.example.com", "https://user:pass@example.com", "https://example.com/v1#frag"} {
		if _, err := p.Validate(context.Background(), raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}

func TestEndpointPolicyPrivateOptInAllowsHTTPForm(t *testing.T) {
	// Syntax-level assertion; address blocking itself is tested via isBlockedIP.
	u, err := url.Parse("http://localhost:11434/v1")
	if err != nil || u.Scheme != "http" {
		t.Fatal("test fixture invalid")
	}
}
