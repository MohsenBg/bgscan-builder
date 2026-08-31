package netutil

import (
	"net/http"
	"sync"
)

// DefaultResolver is the process-wide resolver shared by all network
// operations. Its fallback servers can be reconfigured at any time with
// SetServers.
var DefaultResolver = NewResolver()

var (
	httpClientOnce sync.Once
	httpClient     *http.Client
)

// DefaultHTTPClient returns the process-wide HTTP client. Every network
// operation in the builder shares this single client, whose transport
// resolves hostnames through DefaultResolver: the system resolver first,
// then sequential fallback to the configured DNS servers. A broken system
// resolver (e.g. an unavailable [::1]:53 on Android/Termux) therefore cannot
// break any download.
func DefaultHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		httpClient = &http.Client{Transport: HTTPTransport(DefaultResolver)}
	})
	return httpClient
}

// HTTPTransport returns an http.Transport whose connections resolve
// hostnames through r. It is a fully initialized transport cloned from
// http.DefaultTransport and is safe for concurrent use.
func HTTPTransport(r *Resolver) *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = r.DialContext
	return tr
}
