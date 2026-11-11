package webhook

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

/**
 * SSRF guard for outbound webhook URLs.
 *
 * A webhook is a user-supplied URL that the server POSTs to, which makes it the
 * classic server-side request forgery primitive: without a guard, any account
 * could aim one at 127.0.0.1, at the docker network, or at a cloud metadata
 * endpoint and use the API process as a proxy. The deliveries table was designed
 * since migration 000015 and never used, so this guard is arriving with the
 * dispatcher rather than being retrofitted to a live feature.
 */

var (
	ErrNotHTTPS      = errors.New("webhook URL must use https")
	ErrPrivateTarget = errors.New("webhook URL resolves to a private or reserved address")
	ErrBadURL        = errors.New("webhook URL is not a valid absolute URL")
)

// blockedNets are the ranges a webhook must never reach: loopback, private,
// link-local (including 169.254.169.254, the cloud metadata endpoint),
// unique-local, and the unspecified address.
var blockedNets = func() []*net.IPNet {
	cidrs := []string{
		"0.0.0.0/8",      // "this network"
		"10.0.0.0/8",     // RFC1918
		"100.64.0.0/10",  // carrier-grade NAT
		"127.0.0.0/8",    // loopback
		"169.254.0.0/16", // link-local, incl. cloud metadata
		"172.16.0.0/12",  // RFC1918
		"192.0.0.0/24",   // IETF protocol assignments
		"192.168.0.0/16", // RFC1918
		"198.18.0.0/15",  // benchmarking
		"::1/128",        // loopback v6
		"fc00::/7",       // unique-local v6
		"fe80::/10",      // link-local v6
		// Deliberately NOT ::ffff:0:0/96: net.ParseIP returns IPv4 in 16-byte
		// v4-mapped form, so that range contains every IPv4 address and blocked
		// every public target. The v4 CIDRs above reject private IPv4.
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			panic("webhook: bad blocked CIDR " + c)
		}
		nets = append(nets, n)
	}
	return nets
}()

// ValidateURL reports whether a webhook may target rawURL.
//
// It resolves the hostname and checks EVERY returned address, so a name that
// resolves to both a public and a private address is rejected — checking only the
// first would leave a rebinding hole.
func ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ErrBadURL
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return ErrNotHTTPS
	}

	host := u.Hostname()
	if host == "" {
		return ErrBadURL
	}

	// A literal IP needs no DNS, but must still be checked.
	if ip := net.ParseIP(host); ip != nil {
		if isBlocked(ip) {
			return ErrPrivateTarget
		}
		return nil
	}

	// Anything that looks like a bare hostname for internal infrastructure.
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") ||
		strings.HasSuffix(lower, ".internal") || strings.HasSuffix(lower, ".local") {
		return ErrPrivateTarget
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		// An unresolvable host is not delivered to; treat it as invalid rather
		// than storing a webhook that can never fire.
		return fmt.Errorf("%w: cannot resolve %s", ErrBadURL, host)
	}
	if len(ips) == 0 {
		return fmt.Errorf("%w: %s resolved to no addresses", ErrBadURL, host)
	}
	for _, ip := range ips {
		if isBlocked(ip) {
			return ErrPrivateTarget
		}
	}
	return nil
}

func isBlocked(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	for _, n := range blockedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
