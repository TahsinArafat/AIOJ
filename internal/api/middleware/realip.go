package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
)

// defaultTrustedProxies are the networks a reverse proxy is expected to sit on
// when none are configured explicitly: loopback plus the RFC1918 / ULA ranges
// Docker and Kubernetes use for their bridge networks.
func defaultTrustedProxies() []netip.Prefix {
	return []netip.Prefix{
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("fc00::/7"),
		netip.MustParsePrefix("fe80::/10"),
	}
}

// ParseTrustedProxies turns a comma-separated CIDR list into prefixes. Invalid
// entries are reported so a typo cannot silently widen trust.
func ParseTrustedProxies(spec string) []netip.Prefix {
	var out []netip.Prefix
	for _, raw := range strings.Split(spec, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if p, err := netip.ParsePrefix(raw); err == nil {
			out = append(out, p.Masked())
			continue
		}
		if a, err := netip.ParseAddr(raw); err == nil {
			out = append(out, netip.PrefixFrom(a, a.BitLen()))
			continue
		}
		slog.Warn("ignoring invalid TRUSTED_PROXIES entry", "entry", raw)
	}
	return out
}

// TrustedProxiesFromEnv reads TRUSTED_PROXIES (CIDRs, comma separated).
// Unset means the loopback + private-range defaults. An explicit empty value
// disables header trust entirely, which is correct when the API is exposed
// directly to the internet.
func TrustedProxiesFromEnv() []netip.Prefix {
	spec, ok := os.LookupEnv("TRUSTED_PROXIES")
	if !ok {
		return defaultTrustedProxies()
	}
	if strings.TrimSpace(spec) == "" {
		return nil
	}
	return ParseTrustedProxies(spec)
}

// ClientIP returns the request's client address without the ephemeral port.
//
// Rate limiting must key on the host alone: r.RemoteAddr includes a port that
// changes per connection, so keying on it (as this package used to) gave every
// request its own bucket and disabled throttling entirely behind nginx.
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RealIP replaces the deprecated chi RealIP middleware, which trusted
// X-Forwarded-For from any peer. Here the headers are honoured only when the
// direct connection comes from a trusted proxy, so a client hitting the API
// directly cannot spoof its address.
func RealIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if client, ok := resolveClientIP(r, trusted); ok {
				r.RemoteAddr = client
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RealIPFromEnv is RealIP configured from TRUSTED_PROXIES.
func RealIPFromEnv() func(http.Handler) http.Handler {
	return RealIP(TrustedProxiesFromEnv())
}

// resolveClientIP walks X-Forwarded-For from the right, skipping trusted
// proxies, and returns the first address that is not one of them. Falls back to
// X-Real-IP, then to the peer address.
func resolveClientIP(r *http.Request, trusted []netip.Prefix) (string, bool) {
	peer, ok := parseAddr(ClientIP(r))
	if !ok || !isTrusted(peer, trusted) {
		// Direct connection (or untrusted peer): headers are not authoritative.
		return "", false
	}

	var candidates []netip.Addr
	for _, h := range r.Header.Values("X-Forwarded-For") {
		for _, part := range strings.Split(h, ",") {
			if addr, ok := parseAddr(strings.TrimSpace(part)); ok {
				candidates = append(candidates, addr)
			}
		}
	}
	if len(candidates) == 0 {
		if addr, ok := parseAddr(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ok {
			candidates = append(candidates, addr)
		}
	}
	for i := len(candidates) - 1; i >= 0; i-- {
		if !isTrusted(candidates[i], trusted) {
			return candidates[i].String(), true
		}
	}
	if len(candidates) > 0 {
		// Every hop was a trusted proxy; the leftmost entry is the origin.
		return candidates[0].String(), true
	}
	return "", false
}

func parseAddr(s string) (netip.Addr, bool) {
	if addr, err := netip.ParseAddr(s); err == nil {
		return addr.Unmap(), true
	}
	// Tolerate "ip:port" and bracketed IPv6 forms.
	if host, _, err := net.SplitHostPort(s); err == nil {
		if addr, err := netip.ParseAddr(host); err == nil {
			return addr.Unmap(), true
		}
	}
	return netip.Addr{}, false
}

func isTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
