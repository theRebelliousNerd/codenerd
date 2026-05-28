package perception

import (
	"net/http"
	"time"
)

// sharedTransport is a process-wide HTTP transport tuned for sustained,
// parallel LLM API traffic. Go's default Transport caps idle connections
// per host at 2, which serializes campaign-mode parallel calls behind the
// same provider host. The limits below (64 idle per host, 128 max per
// host, HTTP/2 preferred) let parallel LLM calls actually run in parallel.
var sharedTransport = &http.Transport{
	Proxy:                 http.ProxyFromEnvironment,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          256,
	MaxIdleConnsPerHost:   64,
	MaxConnsPerHost:       128,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
	ReadBufferSize:        64 * 1024,
	WriteBufferSize:       64 * 1024,
}

// NewSharedHTTPClient returns an http.Client that uses a process-wide
// pooled Transport tuned for sustained, parallel LLM API traffic.
// On a Ryzen 5950X with 128 GB RAM the pooled connection limits (64 idle
// per host, 128 max per host) keep campaign-mode parallel calls from
// being serialized behind Go's default MaxIdleConnsPerHost=2.
func NewSharedHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Transport: sharedTransport, Timeout: timeout}
}
