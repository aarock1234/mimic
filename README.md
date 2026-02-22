# Mimic

[![GoDoc](https://godoc.org/github.com/aarock1234/mimic?status.svg)](https://godoc.org/github.com/aarock1234/mimic)

Mimic browser TLS, HTTP/2, and header fingerprints from Go. Supports Chromium
(Chrome, Edge, Brave), Safari (macOS, iOS, iPadOS), and Firefox.

## Installation

```sh
go get github.com/aarock1234/mimic
```

## Quick Start

```go
spec, err := mimic.Chromium(mimic.BrandChrome, "137.0.0.0")
if err != nil {
    panic(err)
}

transport, err := mimic.NewTransport(spec, mimic.PlatformWindows)
if err != nil {
    panic(err)
}

client := &http.Client{Transport: transport}
```

Create a browser spec, create a transport, use it as a standard `http.Client`
transport. Mimic handles TLS fingerprinting, HTTP/2 settings, pseudo-header
ordering, and default browser headers automatically.

## Supported Browsers

### Chromium

`Chromium(brand Brand, version string) (*ClientSpec, error)`

Supports Chrome, Edge, and Brave from version 100 onward. The TLS and HTTP/2
fingerprint is version-aware, mapping to the correct `utls` ClientHello spec
for each major version range (100-133+).

```go
spec, err := mimic.Chromium(mimic.BrandChrome, "137.0.0.0") // Chrome
spec, err := mimic.Chromium(mimic.BrandEdge, "137.0.0.0")   // Edge (adds "Edg/" to UA)
spec, err := mimic.Chromium(mimic.BrandBrave, "137.0.0.0")  // Brave
if err != nil {
    // ErrUnsupportedVersion if version < 100
    panic(err)
}
```

Chromium specs automatically set these default headers:

| Header               | Description                                             |
| -------------------- | ------------------------------------------------------- |
| `user-agent`         | Platform and brand-aware (Edge appends `Edg/{version}`) |
| `sec-ch-ua`          | Client hints with correct GREASE brand per version      |
| `sec-ch-ua-mobile`   | `?0` (desktop)                                          |
| `sec-ch-ua-platform` | `"Windows"`, `"macOS"`, or `"Linux"`                    |

Platforms: `PlatformWindows`, `PlatformMac`, `PlatformLinux`

### Safari

`Safari(version string) (*ClientSpec, error)`

Supports Safari from version 16 onward. The TLS fingerprint is
platform-dependent: macOS and iPadOS use the Safari desktop fingerprint,
while iOS uses the iOS-specific fingerprint.

```go
spec, err := mimic.Safari("18.3")
if err != nil {
    // ErrUnsupportedVersion if version < 16
    panic(err)
}
```

Safari uses a different HTTP/2 fingerprint than Chromium:

- Pseudo-header order: `:method, :scheme, :path, :authority` (Chromium uses `:method, :authority, :scheme, :path`)
- `HEADER_TABLE_SIZE=4096` (Chromium uses 65536)
- `INITIAL_WINDOW_SIZE=2097152` (Chromium uses 6291456)
- `SETTINGS_ENABLE_CONNECT_PROTOCOL=1`
- `WINDOW_UPDATE` connection flow of 10485760

Safari does **not** send `sec-ch-ua` client hint headers. Sending them while
claiming to be Safari is a fingerprinting red flag. Mimic only sets the
`user-agent` header for Safari specs.

Platforms: `PlatformMac`, `PlatformIOS`, `PlatformIPadOS`

### Firefox

`Firefox(version string) (*ClientSpec, error)`

Supports Firefox from version 55 onward, mapping across 8 `utls` fingerprint
profiles (55, 56, 63, 65, 99, 102, 105, 120).

```go
spec, err := mimic.Firefox("134.0")
if err != nil {
    // ErrUnsupportedVersion if version < 55
    panic(err)
}
```

Firefox uses a different HTTP/2 fingerprint than both Chromium and Safari:

- Pseudo-header order: `:method, :path, :authority, :scheme`
- `INITIAL_WINDOW_SIZE=131072` (128 KB)
- `WINDOW_UPDATE` connection flow of 12517377
- HEADERS frame priority: stream dependency 13, weight 42

Firefox does **not** send `sec-ch-ua` client hint headers. Mimic only sets the
`user-agent` header for Firefox specs.

Platforms: `PlatformWindows`, `PlatformMac`, `PlatformLinux`

> Firefox sends standalone PRIORITY frames at connection start to build a
> dependency tree. This is not supported by the underlying HTTP/2 transport,
> so the Akamai PRIORITY section of the fingerprint will differ from real
> Firefox. TLS, SETTINGS, WINDOW_UPDATE, and pseudo-header order are all
> matched.

## Platform Support

|          | Windows | macOS | Linux | iOS | iPadOS |
| -------- | :-----: | :---: | :---: | :-: | :----: |
| Chromium |    x    |   x   |   x   |     |        |
| Safari   |         |   x   |       |  x  |   x    |
| Firefox  |    x    |   x   |   x   |     |        |

## Error Handling

All constructors and `NewTransport` return errors. Two sentinel errors are
available for checking expected failure conditions with `errors.Is`:

```go
import "errors"

spec, err := mimic.Chromium(mimic.BrandChrome, "90.0.0.0")
if errors.Is(err, mimic.ErrUnsupportedVersion) {
    // version is below the minimum for this browser
    // Chromium: < 100, Safari: < 16, Firefox: < 55
}

transport, err := mimic.NewTransport(spec, mimic.PlatformIOS)
if errors.Is(err, mimic.ErrUnsupportedPlatform) {
    // platform is not valid for this browser
    // e.g., iOS is only valid for Safari, not Chromium or Firefox
}
```

| Error                    | Returned When                                                       |
| ------------------------ | ------------------------------------------------------------------- |
| `ErrUnsupportedVersion`  | Version is below the browser's minimum supported version            |
| `ErrUnsupportedPlatform` | Platform is not valid for the browser (see platform support matrix) |

## Creating a Transport

`NewTransport` takes a `ClientSpec`, a `Platform`, and optional
`TransportOption` values. It returns a `*Transport` that implements
`http.RoundTripper`.

```go
spec, err := mimic.Chromium(mimic.BrandChrome, "137.0.0.0")
if err != nil {
    panic(err)
}

transport, err := mimic.NewTransport(spec, mimic.PlatformWindows)
if err != nil {
    panic(err)
}

client := &http.Client{Transport: transport}
```

### Custom Base Transport

Use `WithBaseTransport` to provide your own `*http.Transport` (for proxy
configuration, custom timeouts, etc.):

```go
transport, err := mimic.NewTransport(spec, mimic.PlatformMac,
    mimic.WithBaseTransport(&http.Transport{
        Proxy: http.ProxyFromEnvironment,
    }),
)
if err != nil {
    panic(err)
}
```

If no base transport is provided, a default transport is created with
standard timeouts and connection pooling.

### Advanced: ConfigureTransport

For more control, use `ConfigureTransport` directly to apply TLS and HTTP/2
settings to an existing transport without the `Transport` wrapper:

```go
spec, err := mimic.Chromium(mimic.BrandChrome, "137.0.0.0")
if err != nil {
    panic(err)
}

t := &http.Transport{}
if err := spec.ConfigureTransport(t, mimic.PlatformWindows); err != nil {
    // ErrUnsupportedPlatform if the platform is not valid for this browser
    panic(err)
}

// t is now configured with the correct TLS and HTTP/2 fingerprint
// but you are responsible for setting headers and pseudo-header order
```

## Header Behavior

The `Transport` returned by `NewTransport` automatically handles headers on
each request:

1. **Default headers** are injected if not already set on the request. Any
   header you explicitly set takes precedence.
2. **Pseudo-header order** is set to match the browser's real ordering.
3. **Header order** is randomized if not explicitly set, matching real
   browser behavior (Chromium shuffles non-pseudo headers since version 106).

```go
req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
if err != nil {
    panic(err)
}

// these are set by your code
req.Header.Add("accept", "text/html,*/*")
req.Header.Add("accept-encoding", "gzip, deflate, br")
req.Header.Add("accept-language", "en-US,en;q=0.9")

// these are set automatically by mimic (for Chromium):
// user-agent, sec-ch-ua, sec-ch-ua-mobile, sec-ch-ua-platform

// optional: set your own header order instead of random
// req.Header[http.HeaderOrderKey] = []string{
//     "user-agent", "accept", "accept-encoding", ...
// }

res, err := client.Do(req)
if err != nil {
    panic(err)
}
defer res.Body.Close()
```

## What It Matches

Mimic produces traffic that matches real browser fingerprints across:

| Signal                      | Description                                                              |
| --------------------------- | ------------------------------------------------------------------------ |
| **TLS ClientHello**         | Cipher suites, extensions, curves, and extension order via `utls`        |
| **JA3 / JA4**               | TLS fingerprint hashes derived from the ClientHello                      |
| **HTTP/2 SETTINGS**         | Frame entries, values, and order sent at connection start                |
| **HTTP/2 WINDOW_UPDATE**    | Connection-level flow control value on stream 0                          |
| **HTTP/2 HEADERS priority** | Priority parameters embedded in HEADERS frames                           |
| **Pseudo-header order**     | Browser-specific ordering of `:method`, `:authority`, `:scheme`, `:path` |
| **Header order**            | Randomized to match real Chromium behavior (v106+)                       |
| **User-Agent**              | Platform and brand-aware, including frozen OS versions                   |
| **Client Hints**            | `sec-ch-ua` with correct GREASE brand algorithm (Chromium only)          |

## Examples

Working examples for each browser are in the
[`examples/`](https://github.com/aarock1234/mimic/tree/main/examples)
directory:

- [`examples/chrome`](https://github.com/aarock1234/mimic/tree/main/examples/chrome) - Chrome on Windows
- [`examples/edge`](https://github.com/aarock1234/mimic/tree/main/examples/edge) - Edge on Windows
- [`examples/safari`](https://github.com/aarock1234/mimic/tree/main/examples/safari) - Safari on macOS
- [`examples/firefox`](https://github.com/aarock1234/mimic/tree/main/examples/firefox) - Firefox on Linux

Each example makes a request to [`tls.peet.ws/api/clean`](https://tls.peet.ws/api/clean)
and prints the resulting JA3, JA4, Akamai, and Peetprint fingerprints.
