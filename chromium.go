package mimic

import (
	"fmt"

	utls "github.com/refraction-networking/utls"
	http "github.com/saucesteals/fhttp"
	"github.com/saucesteals/fhttp/http2"
)

// Chromium creates a ClientSpec that mimics a Chromium-based browser's TLS and HTTP/2
// fingerprint. Supported brands are BrandChrome, BrandBrave, and BrandEdge.
// Version should be the full Chromium version string (e.g., "137.0.0.0").
// Minimum supported version is 100.
func Chromium(brand Brand, version string) (*ClientSpec, error) {
	majorStr, majorNum, err := parseMajorVersion(version)
	if err != nil {
		return nil, err
	}

	if majorNum < 100 {
		return nil, fmt.Errorf("chromium %s: %w", version, ErrUnsupportedVersion)
	}

	helloID := chromiumTLSHelloID(majorNum)
	if err := validateTLSHelloID(helloID); err != nil {
		return nil, fmt.Errorf("chromium %s: %w", version, err)
	}

	return &ClientSpec{
		version:      version,
		http2Options: chromiumHTTP2Options(majorNum),
		tlsSpecFn: func(_ Platform) (func() *utls.ClientHelloSpec, error) {
			return newTLSSpecFunc(helloID), nil
		},
		buildHeaders: chromiumBuildHeaders(brand, version, majorStr, majorNum),
	}, nil
}

func chromiumTLSHelloID(majorNum int) utls.ClientHelloID {
	switch {
	case majorNum < 102:
		return utls.HelloChrome_100
	case majorNum < 106:
		return utls.HelloChrome_102
	case majorNum < 112:
		return utls.HelloChrome_106_Shuffle
	case majorNum < 114:
		return utls.HelloChrome_112_PSK_Shuf
	case majorNum < 115:
		return utls.HelloChrome_114_Padding_PSK_Shuf
	case majorNum < 120:
		return utls.HelloChrome_115_PQ
	case majorNum < 131:
		return utls.HelloChrome_120
	case majorNum < 133:
		return utls.HelloChrome_131
	default: // >=133
		return utls.HelloChrome_133
	}
}

func chromiumHTTP2Options(majorNum int) *HTTP2Options {
	opts := &HTTP2Options{
		PseudoHeaderOrder: []string{":method", ":authority", ":scheme", ":path"},
		MaxHeaderListSize: 262144,
		InitialWindowSize: 6291456,
		HeaderTableSize:   65536,
	}

	switch {
	case majorNum < 107:
		opts.Settings = []http2.Setting{
			{ID: http2.SettingHeaderTableSize, Val: 65536},
			{ID: http2.SettingMaxConcurrentStreams, Val: 1000},
			{ID: http2.SettingInitialWindowSize, Val: 6291456},
			{ID: http2.SettingMaxHeaderListSize, Val: 100000},
		}
		opts.MaxHeaderListSize = 100000
	case majorNum < 120:
		opts.Settings = []http2.Setting{
			{ID: http2.SettingHeaderTableSize, Val: 65536},
			{ID: http2.SettingEnablePush, Val: 0},
			{ID: http2.SettingMaxConcurrentStreams, Val: 1000},
			{ID: http2.SettingInitialWindowSize, Val: 6291456},
			{ID: http2.SettingMaxHeaderListSize, Val: 262144},
		}
	default: // >=120
		opts.Settings = []http2.Setting{
			{ID: http2.SettingHeaderTableSize, Val: 65536},
			{ID: http2.SettingEnablePush, Val: 0},
			{ID: http2.SettingInitialWindowSize, Val: 6291456},
			{ID: http2.SettingMaxHeaderListSize, Val: 262144},
		}
	}

	return opts
}

// chromiumBuildHeaders returns a function that generates Chromium-appropriate default headers
// for a given platform. This includes User-Agent, sec-ch-ua, sec-ch-ua-mobile,
// and sec-ch-ua-platform.
func chromiumBuildHeaders(brand Brand, version string, majorStr string, majorNum int) func(Platform) (http.Header, error) {
	return func(p Platform) (http.Header, error) {
		var uaPlatform, hintPlatform string

		switch p {
		case PlatformWindows:
			uaPlatform = "Windows NT 10.0; Win64; x64"
			hintPlatform = "Windows"
		case PlatformMac:
			uaPlatform = "Macintosh; Intel Mac OS X 10_15_7"
			hintPlatform = "macOS"
		case PlatformLinux:
			uaPlatform = "X11; Linux x86_64"
			hintPlatform = "Linux"
		default:
			return nil, fmt.Errorf("chromium on %s: %w", p, ErrUnsupportedPlatform)
		}

		ua := fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", uaPlatform, version)

		// Real Edge appends "Edg/{version}" to the UA string.
		// Brave uses the same UA as Chrome (no additional suffix).
		if brand == BrandEdge {
			ua += fmt.Sprintf(" Edg/%s", version)
		}

		h := http.Header{}
		h.Set("user-agent", ua)
		h.Set("sec-ch-ua", clientHintUA(brand, majorStr, majorNum))
		h.Set("sec-ch-ua-mobile", "?0")
		h.Set("sec-ch-ua-platform", fmt.Sprintf(`"%s"`, hintPlatform))

		return h, nil
	}
}
