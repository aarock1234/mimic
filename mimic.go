package mimic

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	utls "github.com/refraction-networking/utls"
	http "github.com/saucesteals/fhttp"
	"github.com/saucesteals/fhttp/http2"
)

// Platform represents the operating system platform.
type Platform string

const (
	PlatformWindows Platform = "win"
	PlatformMac     Platform = "mac"
	PlatformLinux   Platform = "linux"
	PlatformIOS     Platform = "ios"
	PlatformIPadOS  Platform = "ipados"
)

// Brand represents the browser brand for Chromium-based browsers.
type Brand string

const (
	BrandChrome Brand = "Google Chrome"
	BrandBrave  Brand = "Brave"
	BrandEdge   Brand = "Microsoft Edge"
)

var (
	ErrUnsupportedVersion  = errors.New("unsupported version")
	ErrUnsupportedPlatform = errors.New("unsupported platform")
)

// HTTP2Options holds HTTP/2 configuration for a browser fingerprint.
type HTTP2Options struct {
	// Settings are the HTTP/2 SETTINGS frame entries sent at connection start.
	Settings []http2.Setting

	// PseudoHeaderOrder is the order of HTTP/2 pseudo headers.
	// Chrome uses m,a,s,p; Safari uses m,s,p,a; Firefox uses m,p,a,s.
	PseudoHeaderOrder []string

	// MaxHeaderListSize is the local limit on received header list size.
	MaxHeaderListSize uint32

	// InitialWindowSize is the stream-level initial flow control window size.
	InitialWindowSize uint32

	// HeaderTableSize is the HPACK decoder table size.
	HeaderTableSize uint32

	// ConnectionFlow is the WINDOW_UPDATE value sent on stream 0 at connection start.
	// A value of 0 uses fhttp's default (15663105).
	ConnectionFlow uint32

	// HeaderPriority controls the priority parameters sent in HEADERS frames.
	// A nil value uses fhttp's default (Exclusive=true, Weight=255).
	HeaderPriority *http2.PriorityParam
}

// ClientSpec holds all browser-specific configuration needed to mimic a browser's
// TLS, HTTP/2, and header fingerprints.
type ClientSpec struct {
	version      string
	http2Options *HTTP2Options
	tlsSpecFn    func(platform Platform) (func() *utls.ClientHelloSpec, error)
	buildHeaders func(platform Platform) (http.Header, error)
}

// Version returns the version string for the mimicked client.
func (c *ClientSpec) Version() string {
	return c.version
}

// HTTP2Opts returns the HTTP/2 configuration for the mimicked client.
func (c *ClientSpec) HTTP2Opts() *HTTP2Options {
	return c.http2Options
}

// PseudoHeaderOrder returns the HTTP/2 pseudo header order for the mimicked client.
func (c *ClientSpec) PseudoHeaderOrder() []string {
	return c.http2Options.PseudoHeaderOrder
}

// ConfigureTransport configures an http.Transport with the client's TLS and HTTP/2
// settings for the given platform. The transport is modified in-place.
func (c *ClientSpec) ConfigureTransport(t *http.Transport, platform Platform) error {
	specFn, err := c.tlsSpecFn(platform)
	if err != nil {
		return err
	}

	t.GetTlsClientHelloSpec = specFn

	t2, err := http2.ConfigureTransports(t)
	if err != nil {
		return fmt.Errorf("enabling http2 support: %w", err)
	}

	t2.Settings = c.http2Options.Settings
	t2.MaxHeaderListSize = c.http2Options.MaxHeaderListSize
	t2.InitialWindowSize = c.http2Options.InitialWindowSize
	t2.HeaderTableSize = c.http2Options.HeaderTableSize

	if c.http2Options.ConnectionFlow > 0 {
		t2.TransportConnFlow = c.http2Options.ConnectionFlow
	}

	if c.http2Options.HeaderPriority != nil {
		t2.HeaderPriority = c.http2Options.HeaderPriority
	}

	return nil
}

// parseMajorVersion extracts the major version string and number from a version string
// like "137.0.0.0" or "18.3".
func parseMajorVersion(version string) (string, int, error) {
	majorStr := strings.SplitN(version, ".", 2)[0]
	majorNum, err := strconv.Atoi(majorStr)
	if err != nil {
		return "", 0, fmt.Errorf("parsing major version %q: %w", majorStr, err)
	}
	return majorStr, majorNum, nil
}

// validateTLSHelloID checks that a utls ClientHelloID can be resolved to a spec.
func validateTLSHelloID(id utls.ClientHelloID) error {
	if _, err := utls.UTLSIdToSpec(id); err != nil {
		return fmt.Errorf("resolving tls spec for %s %s: %w", id.Client, id.Version, err)
	}
	return nil
}

// newTLSSpecFunc returns a function that creates a fresh TLS ClientHelloSpec
// from the given hello ID on each call. A fresh copy is needed because the spec
// may be mutated during the TLS handshake.
func newTLSSpecFunc(id utls.ClientHelloID) func() *utls.ClientHelloSpec {
	return func() *utls.ClientHelloSpec {
		spec, _ := utls.UTLSIdToSpec(id)
		return &spec
	}
}
