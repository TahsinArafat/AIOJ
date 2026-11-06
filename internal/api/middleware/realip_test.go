package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"testing"
)

// unsetenv removes a variable for the duration of the test, restoring the
// previous state afterwards. t.Setenv cannot express "unset", which is the case
// TrustedProxiesFromEnv distinguishes from "set but empty".
func unsetenv(t *testing.T, key string) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unsetenv %s: %v", key, err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, prev)
			return
		}
		_ = os.Unsetenv(key)
	})
}

func withRemoteAddr(r *http.Request, addr string) *http.Request {
	r.RemoteAddr = addr
	return r
}

func TestClientIP_StripsPort(t *testing.T) {
	req := withRemoteAddr(httptest.NewRequest("GET", "/", nil), "203.0.113.9:51820")
	if got := ClientIP(req); got != "203.0.113.9" {
		t.Fatalf("ClientIP() = %q, want 203.0.113.9", got)
	}

	req = withRemoteAddr(httptest.NewRequest("GET", "/", nil), "[2001:db8::1]:443")
	if got := ClientIP(req); got != "2001:db8::1" {
		t.Fatalf("ClientIP() = %q, want 2001:db8::1", got)
	}
}

func TestRealIP_UntrustedPeerCannotSpoof(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	h := RealIP(trusted)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := ClientIP(r); got != "203.0.113.9" {
			t.Errorf("ClientIP() = %q, want the real peer 203.0.113.9", got)
		}
	}))

	req := withRemoteAddr(httptest.NewRequest("GET", "/", nil), "203.0.113.9:1234")
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-IP", "1.2.3.4")
	h.ServeHTTP(httptest.NewRecorder(), req)
}

func TestRealIP_TrustedProxyIsHonoured(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	tests := []struct {
		name string
		xff  string
		xrip string
		want string
	}{
		{"single hop", "198.51.100.7", "", "198.51.100.7"},
		{"chain with trusted hop", "198.51.100.7, 10.0.0.2", "", "198.51.100.7"},
		{"spoofed left entry is ignored", "1.2.3.4, 198.51.100.7", "", "198.51.100.7"},
		{"x-real-ip fallback", "", "198.51.100.8", "198.51.100.8"},
		{"all hops trusted", "10.0.0.9, 10.0.0.2", "", "10.0.0.9"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := RealIP(trusted)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := ClientIP(r); got != tt.want {
					t.Errorf("ClientIP() = %q, want %q", got, tt.want)
				}
			}))

			req := withRemoteAddr(httptest.NewRequest("GET", "/", nil), "10.0.0.2:443")
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xrip != "" {
				req.Header.Set("X-Real-IP", tt.xrip)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
}

func TestTrustedProxiesFromEnv(t *testing.T) {
	t.Run("default trusts loopback and private ranges", func(t *testing.T) {
		unsetenv(t, "TRUSTED_PROXIES")
		got := TrustedProxiesFromEnv()
		if len(got) == 0 {
			t.Fatal("expected default trusted proxy list")
		}
		if !isTrusted(netip.MustParseAddr("172.17.0.2"), got) {
			t.Error("docker bridge address should be trusted by default")
		}
	})

	t.Run("explicit empty disables trust", func(t *testing.T) {
		t.Setenv("TRUSTED_PROXIES", " ")
		if got := TrustedProxiesFromEnv(); len(got) != 0 {
			t.Fatalf("expected no trusted proxies, got %v", got)
		}
	})

	t.Run("parses CIDRs and bare addresses", func(t *testing.T) {
		t.Setenv("TRUSTED_PROXIES", "10.1.0.0/16, 192.0.2.5 ,bogus")
		got := TrustedProxiesFromEnv()
		if len(got) != 2 {
			t.Fatalf("expected 2 prefixes (invalid entry dropped), got %v", got)
		}
		if !isTrusted(netip.MustParseAddr("10.1.9.9"), got) {
			t.Error("10.1.9.9 should be trusted")
		}
		if !isTrusted(netip.MustParseAddr("192.0.2.5"), got) {
			t.Error("192.0.2.5 should be trusted")
		}
	})
}
