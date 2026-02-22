package mimic

import (
	"fmt"
	"strings"

	utls "github.com/refraction-networking/utls"
	http "github.com/saucesteals/fhttp"
	"github.com/saucesteals/fhttp/http2"
)

// settingEnableConnectProtocol is the HTTP/2 SETTINGS_ENABLE_CONNECT_PROTOCOL (0x8).
// Safari 17+ sends this in its SETTINGS frame. fhttp does not define this constant,
// so we cast the raw ID.
const settingEnableConnectProtocol = http2.SettingID(0x8)

// Safari creates a ClientSpec that mimics Safari's TLS and HTTP/2 fingerprint.
// Version should be the Safari version (e.g., "18.3", "17.0", "16.0").
// Minimum supported version is 16.
//
// The TLS fingerprint is platform-dependent: macOS and iPadOS use the Safari
// desktop fingerprint, while iOS uses the iOS-specific fingerprint.
//
// Safari does not send sec-ch-ua client hint headers.
func Safari(version string) (*ClientSpec, error) {
	_, majorNum, err := parseMajorVersion(version)
	if err != nil {
		return nil, err
	}

	if majorNum < 16 {
		return nil, fmt.Errorf("safari %s: %w", version, ErrUnsupportedVersion)
	}

	// validate both platform-specific TLS specs at construction time
	for _, id := range []utls.ClientHelloID{utls.HelloSafari_16_0, utls.HelloIOS_14} {
		if err := validateTLSHelloID(id); err != nil {
			return nil, fmt.Errorf("safari: %w", err)
		}
	}

	return &ClientSpec{
		version:      version,
		http2Options: safariHTTP2Options(),
		tlsSpecFn:    safariTLSSpecFn,
		buildHeaders: safariBuildHeaders(version),
	}, nil
}

// safariTLSSpecFn returns the appropriate TLS spec function based on the platform.
// iOS uses a different TLS fingerprint than macOS/iPadOS.
func safariTLSSpecFn(p Platform) (func() *utls.ClientHelloSpec, error) {
	switch p {
	case PlatformIOS:
		return newTLSSpecFunc(utls.HelloIOS_14), nil
	case PlatformMac, PlatformIPadOS:
		return newTLSSpecFunc(utls.HelloSafari_16_0), nil
	default:
		return nil, fmt.Errorf("safari on %s: %w", p, ErrUnsupportedPlatform)
	}
}

func safariHTTP2Options() *HTTP2Options {
	return &HTTP2Options{
		// Safari's unique pseudo-header order: method, scheme, path, authority
		PseudoHeaderOrder: []string{":method", ":scheme", ":path", ":authority"},
		Settings: []http2.Setting{
			{ID: http2.SettingHeaderTableSize, Val: 4096},
			{ID: http2.SettingEnablePush, Val: 0},
			{ID: http2.SettingMaxConcurrentStreams, Val: 100},
			{ID: http2.SettingInitialWindowSize, Val: 2097152},
			{ID: http2.SettingMaxFrameSize, Val: 16384},
			{ID: settingEnableConnectProtocol, Val: 1},
		},
		InitialWindowSize: 2097152,
		HeaderTableSize:   4096,
		ConnectionFlow:    10485760,
	}
}

// safariBuildHeaders returns a function that generates Safari-appropriate default headers
// for a given platform. Safari does not send sec-ch-ua client hint headers.
func safariBuildHeaders(version string) func(Platform) (http.Header, error) {
	return func(p Platform) (http.Header, error) {
		var ua string

		switch p {
		case PlatformMac:
			// macOS Safari freezes the OS version at 10_15_7 for privacy
			ua = fmt.Sprintf(
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/%s Safari/605.1.15",
				version,
			)
		case PlatformIOS:
			// iOS Safari version generally matches the iOS version
			iosVer := strings.ReplaceAll(version, ".", "_")
			ua = fmt.Sprintf(
				"Mozilla/5.0 (iPhone; CPU iPhone OS %s like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/%s Mobile/15E148 Safari/604.1",
				iosVer, version,
			)
		case PlatformIPadOS:
			iosVer := strings.ReplaceAll(version, ".", "_")
			ua = fmt.Sprintf(
				"Mozilla/5.0 (iPad; CPU OS %s like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/%s Mobile/15E148 Safari/604.1",
				iosVer, version,
			)
		default:
			return nil, fmt.Errorf("safari on %s: %w", p, ErrUnsupportedPlatform)
		}

		h := http.Header{}
		h.Set("user-agent", ua)
		return h, nil
	}
}
