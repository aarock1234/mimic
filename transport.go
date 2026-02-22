package mimic

import (
	"fmt"
	"math/rand/v2"
	"net"
	"time"

	http "github.com/saucesteals/fhttp"
)

// TransportOption configures a Transport.
type TransportOption func(*transportConfig)

type transportConfig struct {
	baseTransport *http.Transport
}

// WithBaseTransport sets the underlying HTTP transport.
// If not set, a default transport is created.
func WithBaseTransport(t *http.Transport) TransportOption {
	return func(c *transportConfig) {
		c.baseTransport = t
	}
}

// NewTransport creates a new Transport that mimics the given browser spec on the given platform.
// It configures TLS fingerprinting, HTTP/2 settings, and default headers to match
// the specified browser's real-world behavior.
//
// Use Chromium(), Safari(), or Firefox() to create a ClientSpec.
func NewTransport(spec *ClientSpec, platform Platform, opts ...TransportOption) (*Transport, error) {
	cfg := &transportConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.baseTransport == nil {
		cfg.baseTransport = defaultTransport()
	}

	if err := spec.ConfigureTransport(cfg.baseTransport, platform); err != nil {
		return nil, fmt.Errorf("configuring transport: %w", err)
	}

	headers, err := spec.buildHeaders(platform)
	if err != nil {
		return nil, err
	}

	return &Transport{
		transport:         cfg.baseTransport,
		pseudoHeaderOrder: spec.http2Options.PseudoHeaderOrder,
		defaultHeaders:    headers,
	}, nil
}

func defaultTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// Transport implements http.RoundTripper and handles:
//   - Setting default headers for the mimicked browser
//   - Setting the HTTP/2 pseudo-header order
//   - Randomizing header order to match real browser behavior
type Transport struct {
	transport         http.RoundTripper
	pseudoHeaderOrder []string
	defaultHeaders    http.Header
}

// RoundTrip executes a single HTTP transaction, injecting browser-appropriate
// headers and pseudo-header ordering.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	header := req.Header

	header[http.PHeaderOrderKey] = t.pseudoHeaderOrder

	for key, values := range t.defaultHeaders {
		if existing := header.Get(key); existing != "" {
			continue
		}
		if len(values) > 0 {
			header.Set(key, values[0])
		}
	}

	if header[http.HeaderOrderKey] == nil {
		keys := make([]string, 0, len(header))
		for key := range header {
			keys = append(keys, key)
		}
		rand.Shuffle(len(keys), func(i, j int) {
			keys[i], keys[j] = keys[j], keys[i]
		})
		header[http.HeaderOrderKey] = keys
	}

	return t.transport.RoundTrip(req)
}
