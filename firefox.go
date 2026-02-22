package mimic

import (
	"fmt"

	utls "github.com/refraction-networking/utls"
	http "github.com/saucesteals/fhttp"
	"github.com/saucesteals/fhttp/http2"
)

// Firefox creates a ClientSpec that mimics Firefox's TLS and HTTP/2 fingerprint.
// Version should be the Firefox version (e.g., "134.0", "120.0").
// Minimum supported version is 55.
//
// Firefox does not send sec-ch-ua client hint headers.
//
// Note: Firefox sends standalone PRIORITY frames at connection start to build a
// dependency tree. This is not supported by the underlying HTTP/2 transport, so the
// Akamai PRIORITY section of the fingerprint will differ from real Firefox.
// The TLS, SETTINGS, WINDOW_UPDATE, and pseudo-header order are all matched.
func Firefox(version string) (*ClientSpec, error) {
	_, majorNum, err := parseMajorVersion(version)
	if err != nil {
		return nil, err
	}

	if majorNum < 55 {
		return nil, fmt.Errorf("firefox %s: %w", version, ErrUnsupportedVersion)
	}

	helloID := firefoxTLSHelloID(majorNum)
	if err := validateTLSHelloID(helloID); err != nil {
		return nil, fmt.Errorf("firefox %s: %w", version, err)
	}

	return &ClientSpec{
		version:      version,
		http2Options: firefoxHTTP2Options(),
		tlsSpecFn: func(_ Platform) (func() *utls.ClientHelloSpec, error) {
			return newTLSSpecFunc(helloID), nil
		},
		buildHeaders: firefoxBuildHeaders(version),
	}, nil
}

func firefoxTLSHelloID(majorNum int) utls.ClientHelloID {
	switch {
	case majorNum < 56:
		return utls.HelloFirefox_55
	case majorNum < 63:
		return utls.HelloFirefox_56
	case majorNum < 65:
		return utls.HelloFirefox_63
	case majorNum < 99:
		return utls.HelloFirefox_65
	case majorNum < 102:
		return utls.HelloFirefox_99
	case majorNum < 105:
		return utls.HelloFirefox_102
	case majorNum < 120:
		return utls.HelloFirefox_105
	default: // >=120
		return utls.HelloFirefox_120
	}
}

func firefoxHTTP2Options() *HTTP2Options {
	return &HTTP2Options{
		// Firefox's unique pseudo-header order: method, path, authority, scheme
		PseudoHeaderOrder: []string{":method", ":path", ":authority", ":scheme"},
		Settings: []http2.Setting{
			{ID: http2.SettingHeaderTableSize, Val: 65536},
			{ID: http2.SettingInitialWindowSize, Val: 131072},
			{ID: http2.SettingMaxFrameSize, Val: 16384},
		},
		InitialWindowSize: 131072,
		HeaderTableSize:   65536,
		ConnectionFlow:    12517377,
		// Firefox requests use stream 13 as their priority leader with weight 42.
		// Real Firefox also sends standalone PRIORITY frames at connection start,
		// but those are not supported by the underlying HTTP/2 transport.
		HeaderPriority: &http2.PriorityParam{
			StreamDep: 13,
			Exclusive: false,
			Weight:    41, // wire weight is 0-indexed, so 41 = actual weight 42
		},
	}
}

// firefoxBuildHeaders returns a function that generates Firefox-appropriate default headers
// for a given platform. Firefox does not send sec-ch-ua client hint headers.
func firefoxBuildHeaders(version string) func(Platform) (http.Header, error) {
	return func(p Platform) (http.Header, error) {
		var uaPlatform string

		switch p {
		case PlatformWindows:
			uaPlatform = "Windows NT 10.0; Win64; x64"
		case PlatformMac:
			// Firefox uses dots in macOS version, not underscores
			uaPlatform = "Macintosh; Intel Mac OS X 10.15"
		case PlatformLinux:
			uaPlatform = "X11; Linux x86_64"
		default:
			return nil, fmt.Errorf("firefox on %s: %w", p, ErrUnsupportedPlatform)
		}

		ua := fmt.Sprintf(
			"Mozilla/5.0 (%s; rv:%s) Gecko/20100101 Firefox/%s",
			uaPlatform, version, version,
		)

		h := http.Header{}
		h.Set("user-agent", ua)
		return h, nil
	}
}
