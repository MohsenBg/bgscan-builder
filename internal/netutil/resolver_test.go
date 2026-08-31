package netutil

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
)

// dnsServer is a minimal in-process UDP DNS server. It answers A and AAAA
// queries from its record map, so the fallback path can be exercised without
// touching the host's DNS configuration.
type dnsServer struct {
	mu      sync.Mutex
	records map[string][]string
	hits    int

	conn *net.UDPConn
	addr string
}

func startDNSServer(t *testing.T, records map[string][]string) *dnsServer {
	t.Helper()

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	s := &dnsServer{records: records, conn: conn, addr: conn.LocalAddr().String()}
	go s.serve()
	return s
}

func (s *dnsServer) serve() {
	buf := make([]byte, 1500)
	for {
		n, from, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			return // closed by test cleanup
		}

		s.mu.Lock()
		s.hits++
		s.mu.Unlock()

		if resp := buildResponse(s.records, buf[:n]); resp != nil {
			_, _ = s.conn.WriteToUDP(resp, from)
		}
	}
}

func (s *dnsServer) hitCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits
}

// buildResponse answers a raw DNS query for the records map. A name without
// records gets a successful empty answer, so callers can distinguish "no
// record" from a server error.
func buildResponse(records map[string][]string, query []byte) []byte {
	if len(query) < 12 {
		return nil
	}
	off := 12
	var labels []string
	for {
		if off >= len(query) {
			return nil
		}
		l := int(query[off])
		off++
		if l == 0 {
			break
		}
		if off+l > len(query) {
			return nil
		}
		labels = append(labels, string(query[off:off+l]))
		off += l
	}
	if off+4 > len(query) {
		return nil
	}
	qtype := binary.BigEndian.Uint16(query[off:])
	off += 4

	name := strings.TrimSuffix(strings.ToLower(strings.Join(labels, ".")), ".")

	var rdata [][]byte
	for _, record := range records[name] {
		ip := net.ParseIP(record)
		switch {
		case qtype == 1 && ip.To4() != nil: // A
			rdata = append(rdata, ip.To4())
		case qtype == 28 && ip.To4() == nil && ip != nil: // AAAA
			rdata = append(rdata, ip.To16())
		}
	}

	resp := make([]byte, 0, off+len(rdata)*16)
	resp = binary.BigEndian.AppendUint16(resp, binary.BigEndian.Uint16(query)) // transaction id
	resp = binary.BigEndian.AppendUint16(resp, 0x8180)                         // response, recursion desired+available, no error
	resp = binary.BigEndian.AppendUint16(resp, 1)                              // question count
	resp = binary.BigEndian.AppendUint16(resp, uint16(len(rdata)))             // answer count
	resp = binary.BigEndian.AppendUint16(resp, 0)                              // authority count
	resp = binary.BigEndian.AppendUint16(resp, 0)                              // additional count
	resp = append(resp, query[12:off]...)                                      // echo the question
	for _, rd := range rdata {
		resp = binary.BigEndian.AppendUint16(resp, 0xC00C) // pointer to question name
		resp = binary.BigEndian.AppendUint16(resp, qtype)
		resp = binary.BigEndian.AppendUint16(resp, 1) // class IN
		resp = binary.BigEndian.AppendUint32(resp, 60)
		resp = binary.BigEndian.AppendUint16(resp, uint16(len(rd)))
		resp = append(resp, rd...)
	}
	return resp
}

// deadUDPPort returns a local UDP address with no listener.
func deadUDPPort(t *testing.T) string {
	t.Helper()

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	addr := conn.LocalAddr().String()
	if err := conn.Close(); err != nil {
		t.Fatalf("close udp: %v", err)
	}
	return addr
}

// brokenSystem returns a system lookup that always fails, simulating a host
// whose resolver is unavailable (e.g. pointing at [::1]:53).
func brokenSystem(err error) func(context.Context, string) ([]string, error) {
	return func(context.Context, string) ([]string, error) { return nil, err }
}

func TestNewResolver_DefaultFallbackServers(t *testing.T) {
	got := NewResolver().Servers()
	if !slices.Equal(got, DefaultFallbackServers) {
		t.Errorf("servers = %v, want %v", got, DefaultFallbackServers)
	}
}

func TestSetServers(t *testing.T) {
	r := NewResolver()

	if err := r.SetServers([]string{"9.9.9.9"}); err == nil {
		t.Error("expected error for server without port")
	}
	if got := r.Servers(); !slices.Equal(got, DefaultFallbackServers) {
		t.Errorf("servers after rejected update = %v, want unchanged %v", got, DefaultFallbackServers)
	}

	if err := r.SetServers([]string{"9.9.9.9:53"}); err != nil {
		t.Fatalf("SetServers: %v", err)
	}
	got := r.Servers()
	if !slices.Equal(got, []string{"9.9.9.9:53"}) {
		t.Errorf("servers = %v, want [9.9.9.9:53]", got)
	}

	got[0] = "tampered" // mutating the returned slice must not affect the resolver
	if r.Servers()[0] != "9.9.9.9:53" {
		t.Errorf("Servers() returned a non-defensive copy: %v", r.Servers())
	}
}

func TestLookup_UsesSystemResolverFirst(t *testing.T) {
	srv := startDNSServer(t, map[string][]string{"host.test": {"198.51.100.7"}})

	r := NewResolver()
	if err := r.SetServers([]string{srv.addr}); err != nil {
		t.Fatalf("SetServers: %v", err)
	}
	r.lookupSystem = func(context.Context, string) ([]string, error) {
		return []string{"192.0.2.1"}, nil
	}

	ips, err := r.Lookup(context.Background(), "host.test.")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !slices.Equal(ips, []string{"192.0.2.1"}) {
		t.Errorf("ips = %v, want [192.0.2.1]", ips)
	}
	if n := srv.hitCount(); n != 0 {
		t.Errorf("fallback server received %d queries, want 0", n)
	}
}

func TestLookup_SystemResolverResolvesLocalhost(t *testing.T) {
	ips, err := NewResolver().Lookup(context.Background(), "localhost")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(ips) == 0 {
		t.Error("Lookup(localhost) returned no addresses")
	}
}

func TestLookup_FallsBackWhenSystemResolverFails(t *testing.T) {
	srv := startDNSServer(t, map[string][]string{"host.test": {"192.0.2.10"}})

	r := NewResolver()
	if err := r.SetServers([]string{srv.addr}); err != nil {
		t.Fatalf("SetServers: %v", err)
	}
	r.lookupSystem = brokenSystem(errors.New("system resolver unavailable"))

	ips, err := r.Lookup(context.Background(), "host.test.")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !slices.Equal(ips, []string{"192.0.2.10"}) {
		t.Errorf("ips = %v, want [192.0.2.10]", ips)
	}
	if n := srv.hitCount(); n < 1 {
		t.Errorf("fallback server received %d queries, want >= 1", n)
	}
}

func TestLookup_FallbackServersTriedSequentially(t *testing.T) {
	empty := startDNSServer(t, nil) // reachable, but has no records
	working := startDNSServer(t, map[string][]string{"host.test": {"192.0.2.20"}})

	r := NewResolver()
	if err := r.SetServers([]string{empty.addr, working.addr}); err != nil {
		t.Fatalf("SetServers: %v", err)
	}
	r.lookupSystem = brokenSystem(errors.New("system resolver unavailable"))

	ips, err := r.Lookup(context.Background(), "host.test.")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !slices.Equal(ips, []string{"192.0.2.20"}) {
		t.Errorf("ips = %v, want [192.0.2.20]", ips)
	}
	if n := empty.hitCount(); n < 1 {
		t.Errorf("first fallback server received %d queries, want >= 1", n)
	}
	if n := working.hitCount(); n < 1 {
		t.Errorf("second fallback server received %d queries, want >= 1", n)
	}
}

func TestLookup_AllResolversFail(t *testing.T) {
	dead := deadUDPPort(t)
	empty := startDNSServer(t, nil) // reachable, but no records

	r := NewResolver()
	if err := r.SetServers([]string{dead, empty.addr}); err != nil {
		t.Fatalf("SetServers: %v", err)
	}
	r.lookupSystem = brokenSystem(errors.New("system resolver unavailable"))

	_, err := r.Lookup(context.Background(), "host.test.")
	if err == nil {
		t.Fatal("expected error when every resolver fails")
	}
	if !strings.Contains(err.Error(), "system resolver") || !strings.Contains(err.Error(), "host.test.") {
		t.Errorf("error should name the host and the system resolver failure, got %v", err)
	}
}

func TestLookup_IPLiteralSkipsResolution(t *testing.T) {
	r := NewResolver()
	r.lookupSystem = func(context.Context, string) ([]string, error) {
		t.Error("system lookup must not run for IP literals")
		return nil, errors.New("must not be called")
	}

	ips, err := r.Lookup(context.Background(), "127.0.0.1")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !slices.Equal(ips, []string{"127.0.0.1"}) {
		t.Errorf("ips = %v, want [127.0.0.1]", ips)
	}
}

func TestDialContext_IPLiteralDialsWithoutDNS(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}

	r := NewResolver()
	r.lookupSystem = brokenSystem(errors.New("system resolver unavailable"))
	if err := r.SetServers([]string{deadUDPPort(t)}); err != nil {
		t.Fatalf("SetServers: %v", err)
	}

	conn, err := r.DialContext(context.Background(), "tcp", u.Host)
	if err != nil {
		t.Fatalf("DialContext: %v", err)
	}
	_ = conn.Close()
}

// TestDefaultHTTPClient_FallbackEndToEnd verifies the exact wiring used in
// production: the shared HTTP client must complete requests through the
// fallback DNS servers when the system resolver is broken.
func TestDefaultHTTPClient_FallbackEndToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "ok")
	}))
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}

	srv := startDNSServer(t, map[string][]string{"host.test": {"127.0.0.1"}})

	prevLookup := DefaultResolver.lookupSystem
	prevServers := DefaultResolver.Servers()
	DefaultResolver.lookupSystem = brokenSystem(errors.New("system resolver unavailable"))
	if err := DefaultResolver.SetServers([]string{srv.addr}); err != nil {
		t.Fatalf("SetServers: %v", err)
	}
	t.Cleanup(func() {
		DefaultResolver.lookupSystem = prevLookup
		_ = DefaultResolver.SetServers(prevServers)
	})

	resp, err := DefaultHTTPClient().Get(fmt.Sprintf("http://host.test.:%s/", u.Port()))
	if err != nil {
		t.Fatalf("GET via fallback DNS: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Errorf("status = %d, body = %q; want 200, \"ok\"", resp.StatusCode, body)
	}
	if n := srv.hitCount(); n < 1 {
		t.Errorf("fallback server received %d queries, want >= 1", n)
	}
}

func TestDefaultHTTPClient_SharedInstance(t *testing.T) {
	if DefaultHTTPClient() != DefaultHTTPClient() {
		t.Error("DefaultHTTPClient must return the same shared instance")
	}
	if DefaultHTTPClient().Transport == nil {
		t.Error("shared client has no transport")
	}
}
