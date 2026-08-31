// Package netutil centralises all builder networking: a DNS resolver that
// consults the system resolver first and falls back to a configurable list of
// public DNS servers, plus the shared HTTP client built on top of it.
//
// The fallback exists for environments where the system resolver is broken or
// unavailable — notably Android/Termux, where the pure Go resolver may
// mistakenly query [::1]:53. Every network operation the builder performs
// (GitHub API calls, release asset, checksum and dgst downloads, git clone)
// resolves hostnames through this package, so a single dialer covers all of
// them and no download fails solely because of a broken system resolver.
package netutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"
	"time"
)

// DefaultFallbackServers lists the public DNS servers tried sequentially when
// the system resolver fails. Entries are host:port.
var DefaultFallbackServers = []string{
	"8.8.8.8:53",
	"8.8.4.4:53",
	"1.1.1.1:53",
	"1.0.0.1:53",
}

const (
	systemLookupTimeout   = 5 * time.Second
	fallbackLookupTimeout = 5 * time.Second
	dnsDialTimeout        = 5 * time.Second
)

// Resolver resolves hostnames through the normal system resolver first and,
// if that fails, through fallback DNS servers tried sequentially. Its
// DialContext makes it directly usable as an http.Transport dialer.
type Resolver struct {
	mu      sync.RWMutex
	servers []string

	// lookupSystem resolves host with the platform resolver. It is a field so
	// tests can simulate a broken system resolver without touching the host
	// configuration.
	lookupSystem func(ctx context.Context, host string) ([]string, error)
}

// NewResolver returns a Resolver configured with DefaultFallbackServers.
func NewResolver() *Resolver {
	return &Resolver{
		servers: slices.Clone(DefaultFallbackServers),
		lookupSystem: func(ctx context.Context, host string) ([]string, error) {
			return net.DefaultResolver.LookupHost(ctx, host)
		},
	}
}

// SetServers replaces the fallback DNS servers. Every entry must be a
// host:port pair. The system resolver is always consulted first regardless
// of this list.
func (r *Resolver) SetServers(servers []string) error {
	for _, server := range servers {
		if _, _, err := net.SplitHostPort(server); err != nil {
			return fmt.Errorf("fallback DNS server %q is not host:port: %w", server, err)
		}
	}

	r.mu.Lock()
	r.servers = slices.Clone(servers)
	r.mu.Unlock()
	return nil
}

// Servers returns a copy of the current fallback DNS servers.
func (r *Resolver) Servers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Clone(r.servers)
}

// Lookup resolves host to a list of IP addresses. The system resolver is
// consulted first; on failure, each fallback DNS server is tried in order
// until one resolves the host.
func (r *Resolver) Lookup(ctx context.Context, host string) ([]string, error) {
	if ip := parseIP(host); ip != nil {
		return []string{host}, nil
	}

	sysCtx, cancel := context.WithTimeout(ctx, systemLookupTimeout)
	ips, sysErr := r.systemLookup()(sysCtx, host)
	cancel()
	if sysErr == nil && len(ips) > 0 {
		return ips, nil
	}
	if sysErr == nil {
		sysErr = errors.New("no addresses returned")
	}

	servers := r.Servers()
	var fbErr error
	for _, server := range servers {
		fbCtx, cancel := context.WithTimeout(ctx, fallbackLookupTimeout)
		ips, err := r.lookupVia(fbCtx, server, host)
		cancel()
		if err == nil && len(ips) > 0 {
			return ips, nil
		}
		if err != nil {
			fbErr = err
		}
	}
	if fbErr == nil {
		fbErr = errors.New("no addresses returned")
	}

	return nil, fmt.Errorf("resolve %q: system resolver: %v; fallback DNS (%s): %v",
		host, sysErr, strings.Join(servers, ", "), fbErr)
}

// DialContext dials network/address, resolving hostnames through Lookup. IP
// literal addresses are dialed directly without any resolution.
func (r *Resolver) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		// Not in host:port form; there is nothing to resolve.
		var d net.Dialer
		return d.DialContext(ctx, network, address)
	}

	ips := []string{host}
	if parseIP(host) == nil {
		ips, err = r.Lookup(ctx, host)
		if err != nil {
			return nil, err
		}
	}

	var dialErr error
	for _, ip := range ips {
		parsed := parseIP(ip)
		if parsed != nil && !supportsNetwork(network, parsed) {
			continue
		}

		conn, err := (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ip, port))
		if err == nil {
			return conn, nil
		}
		dialErr = err
	}

	if dialErr != nil {
		return nil, dialErr
	}
	return nil, fmt.Errorf("no usable addresses for %q over %s", address, network)
}

// lookupVia resolves host through the given DNS server. The resolver's Dial
// function ignores the server address selected by the Go DNS client — which
// may be an unusable default such as [::1]:53 on Android/Termux — and
// connects to server instead.
func (r *Resolver) lookupVia(ctx context.Context, server, host string) ([]string, error) {
	res := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := &net.Dialer{Timeout: dnsDialTimeout}
			return d.DialContext(ctx, network, server)
		},
	}
	return res.LookupHost(ctx, host)
}

func (r *Resolver) systemLookup() func(ctx context.Context, host string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lookupSystem
}

// parseIP parses s as an IP literal, tolerating an optional %zone suffix.
func parseIP(s string) net.IP {
	if host, _, ok := strings.Cut(s, "%"); ok {
		return net.ParseIP(host)
	}
	return net.ParseIP(s)
}

// supportsNetwork reports whether ip can be used for the given network
// family ("tcp", "tcp4", "tcp6", "udp", ...).
func supportsNetwork(network string, ip net.IP) bool {
	switch network {
	case "tcp4", "udp4":
		return ip.To4() != nil
	case "tcp6", "udp6":
		return ip.To4() == nil
	default:
		return true
	}
}
