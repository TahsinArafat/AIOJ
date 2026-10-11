package ai

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type EndpointPolicy struct {
	AllowPrivate bool
	Resolver     *net.Resolver
	Dialer       *net.Dialer
}

func (p EndpointPolicy) Validate(ctx context.Context, raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint URL")
	}
	if u.Scheme != "https" && !(p.AllowPrivate && u.Scheme == "http") {
		return nil, errors.New("endpoint must use HTTPS (private-network opt-in also permits HTTP)")
	}
	if u.User != nil || u.Fragment != "" || u.Hostname() == "" {
		return nil, errors.New("endpoint URL contains forbidden credentials, fragment, or missing host")
	}
	if err := p.validateHost(ctx, u.Hostname()); err != nil {
		return nil, err
	}
	return u, nil
}

func (p EndpointPolicy) validateHost(ctx context.Context, host string) error {
	resolver := p.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	ips, err := resolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("resolve endpoint host: %w", err)
	}
	if len(ips) == 0 {
		return errors.New("endpoint host resolved to no addresses")
	}
	for _, ip := range ips {
		if isBlockedIP(ip, p.AllowPrivate) {
			return fmt.Errorf("endpoint resolves to blocked address %s", ip)
		}
	}
	return nil
}

func isBlockedIP(ip net.IP, allowPrivate bool) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
		return true
	}
	// Cloud metadata ranges stay blocked even with private-network opt-in.
	if ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("100.100.100.200")) {
		return true
	}
	if !allowPrivate && (ip.IsLoopback() || ip.IsPrivate()) {
		return true
	}
	return false
}

func (p EndpointPolicy) HTTPClient(timeout time.Duration) *http.Client {
	dialer := p.Dialer
	if dialer == nil {
		dialer = &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if err := p.validateHost(ctx, host); err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
	}
	client := &http.Client{Timeout: timeout, Transport: transport}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		_, err := p.Validate(req.Context(), req.URL.String())
		return err
	}
	return client
}
