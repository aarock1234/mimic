# Go Style Guide

Project-agnostic Go style guide. Favor idiomatic Go over clever abstractions.

## Philosophy

Write boring code. Prefer explicit over implicit. Optimize for reading, not writing. Design zero values to work without initialization. Keep functions small and interfaces smaller.

## Quick Reference

### Tools

```bash
go vet ./...                 # static analysis
gofmt -l .                   # check formatting
go mod tidy                  # sync dependencies
go test ./...                # run all tests
go test -race ./...          # with race detection
go build -o bin/app ./cmd/app
```

### Import Order

Three groups, blank line between each: stdlib, external, internal.

```go
import (
    "context"
    "errors"
    "fmt"

    "github.com/google/uuid"

    "project/internal/service"
    "project/pkg/client"

    _ "project/pkg/logger"  // side-effect imports last with comment
)
```

### Naming Conventions

| Category       | Convention               | Examples                            |
| -------------- | ------------------------ | ----------------------------------- |
| Exported types | PascalCase               | `Storage`, `UserEvent`, `UserID`    |
| Unexported     | camelCase                | `processItem`, `defaultTimeout`     |
| Interfaces     | PascalCase, `-er` suffix | `Reader`, `Writer`, `Processor`     |
| Constants      | PascalCase               | `DefaultTimeout`, `MaxRetries`      |
| Acronyms       | All caps                 | `userID`, `httpClient`, `parseJSON` |
| Files          | snake_case               | `item_service.go`, `http_client.go` |

## Error Handling

### Basic Pattern

Return errors as last value. Check immediately. Wrap with context using `fmt.Errorf` and `%w`.

```go
func (s *Service) Get(ctx context.Context, id string) (*Item, error) {
    item, err := s.repo.Get(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("get item %s: %w", id, err)
    }
    return item, nil
}
```

Error messages: lowercase, no punctuation, add context.

### Sentinel Errors

For expected error conditions. Check with `errors.Is`.

```go
var (
    ErrNotFound     = errors.New("not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrInvalidInput = errors.New("invalid input")
)

if errors.Is(err, ErrNotFound) {
    // handle not found
}
```

### Custom Error Types

For errors that carry additional data. Check with `errors.As`.

```go
type ValidationError struct {
    Field string
    Reason string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed: %s: %s", e.Field, e.Reason)
}

// Usage
var valErr *ValidationError
if errors.As(err, &valErr) {
    log.Error("validation failed", "field", valErr.Field)
}
```

### When to Panic

Only in `init()` for unrecoverable setup failures. Never in library code.

```go
func init() {
    transport, err := getTransport()
    if err != nil {
        panic("transport initialization failed")
    }
}
```

## Struct Patterns

### Constructor Pattern

```go
type Service struct {
    repo   Repository
    logger *slog.Logger
    timeout time.Duration
}

func NewService(repo Repository, logger *slog.Logger) *Service {
    return &Service{
        repo:    repo,
        logger:  logger,
        timeout: 10 * time.Second,
    }
}
```

### Functional Options Pattern

For complex configuration.

```go
type Option func(*Service)

func WithTimeout(d time.Duration) Option {
    return func(s *Service) { s.timeout = d }
}

func NewService(repo Repository, opts ...Option) *Service {
    s := &Service{repo: repo, timeout: 10 * time.Second}
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage
svc := NewService(repo, WithTimeout(5*time.Second))
```

### Struct Embedding

For shared behavior across implementations.

```go
type BaseProcessor struct {
    logger *slog.Logger
    config Config
}

func (b *BaseProcessor) Validate(input Input) error {
    if input.ID == "" {
        return errors.New("input id required")
    }
    return nil
}

type PDFProcessor struct {
    BaseProcessor  // inherits Validate() method
    pdfConfig PDFConfig
}

func (p *PDFProcessor) Process(input Input) error {
    if err := p.Validate(input); err != nil {
        return err
    }
    // PDF-specific logic
}
```

### Zero Values

Design structs to be usable without initialization when possible.

```go
// GOOD: zero value works
var cache Cache
cache.Set("key", "value")  // works even if not initialized

// Implementation
type Cache struct {
    mu    sync.RWMutex
    items map[string]string
}

func (c *Cache) Set(key, value string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.items == nil {
        c.items = make(map[string]string)
    }
    c.items[key] = value
}
```

## Type Safety & Interfaces

### Never Use `any` Unless Absolutely Necessary

Order of preference: Fully typed → Generics → Semi-typed → `any` (last resort)

```go
// BAD: any everywhere
func ProcessData(data any) any {
    // No type safety, requires type assertions everywhere
    return data
}

func FindItem(items []any, id string) any {
    for _, item := range items {
        if m, ok := item.(map[string]any); ok {
            if m["id"] == id {
                return item
            }
        }
    }
    return nil
}

// GOOD: generics with type parameters
func ProcessData[T any](data T) T {
    return data
}

// GOOD: interface constraints
type HasID interface {
    GetID() string
}

func FindItem[T HasID](items []T, id string) *T {
    for i := range items {
        if items[i].GetID() == id {
            return &items[i]
        }
    }
    return nil
}

// GOOD: multiple constraints
type Identifiable interface {
    GetID() string
}

type Timestamped interface {
    GetCreatedAt() time.Time
}

func SortByCreation[T interface{ Identifiable; Timestamped }](items []T) []T {
    sorted := make([]T, len(items))
    copy(sorted, items)
    sort.Slice(sorted, func(i, j int) bool {
        return sorted[i].GetCreatedAt().Before(sorted[j].GetCreatedAt())
    })
    return sorted
}

// GOOD: generics for container types
type Result[T any] struct {
    value T
    err   error
}

func (r Result[T]) Unwrap() (T, error) {
    return r.value, r.err
}

func NewResult[T any](value T, err error) Result[T] {
    return Result[T]{value: value, err: err}
}

// GOOD: generic utility functions
func Map[T, U any](items []T, fn func(T) U) []U {
    result := make([]U, len(items))
    for i, item := range items {
        result[i] = fn(item)
    }
    return result
}

func Filter[T any](items []T, predicate func(T) bool) []T {
    result := make([]T, 0)
    for _, item := range items {
        if predicate(item) {
            result = append(result, item)
        }
    }
    return result
}

// Usage: type inference works automatically
ids := Map(items, func(item Item) string { return item.ID })
activeItems := Filter(items, func(item Item) bool { return item.Active })
```

### Order of Preference Examples

```go
// 1. BEST: Fully typed
type ProcessorConfig struct {
    Timeout  time.Duration
    Retries  int
    Endpoint string
}

// 2. ACCEPTABLE: Semi-typed with known key types
type ProcessorRegistry map[string]ProcessorConfig
type HandlerMap map[string]func(context.Context, Item) error

// 3. ACCEPTABLE: Semi-typed with union-like values
type Status string

const (
    StatusPending    Status = "pending"
    StatusProcessing Status = "processing"
    StatusCompleted  Status = "completed"
)

type StatusMap map[string]Status

// 4. LAST RESORT: Semi-typed with any
type DynamicHandlers map[string]any // Only when handlers have varying signatures

// 5. AVOID: Fully untyped
// type BadMap map[any]any // DON'T DO THIS
```

### JSON: Always Use Structs, Not Maps

```go
// BAD: map[string]any
func Handle(data map[string]any) error {
    name, ok := data["name"].(string)  // type assertions everywhere
    if !ok {
        return errors.New("invalid name")
    }
}

// GOOD: struct types
type Request struct {
    Name  string `json:"name"`
    Age   int    `json:"age"`
    Email string `json:"email,omitempty"`
}

func Handle(req Request) error {
    // type-safe, validated at unmarshal time
}
```

Only use `map[string]any` when structure is truly dynamic (plugin configs, user-defined metadata).

### Interface Design

Define interfaces where used (consumer side). Keep them small. Accept interfaces, return concrete types.

```go
// In service package
type Repository interface {
    Get(ctx context.Context, id string) (*Item, error)
    Save(ctx context.Context, item *Item) error
}

type Service struct {
    repo Repository  // accepts interface
}

func NewService(repo Repository) *Service {  // returns concrete type
    return &Service{repo: repo}
}
```

### Interface Composition

```go
type Reader interface {
    Read(ctx context.Context, id string) ([]byte, error)
}

type Writer interface {
    Write(ctx context.Context, id string, data []byte) error
}

// Compose interfaces
type ReadWriter interface {
    Reader
    Writer
}

type Storage struct {
    rw ReadWriter  // accepts composed interface
}
```

### Verify Interface Implementation

```go
var _ Repository = (*PostgresRepo)(nil)  // compile-time check
```

## Concurrency Patterns

### Worker Pool

```go
func (s *Service) ProcessBatch(ctx context.Context, items []Item) error {
    const concurrency = 10
    taskCh := make(chan Item, concurrency*2)
    var wg sync.WaitGroup

    // Start workers
    for i := 0; i < concurrency; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for item := range taskCh {
                if err := s.process(ctx, item); err != nil {
                    s.logger.Error("process failed", "error", err)
                }
            }
        }()
    }

    // Send work
    go func() {
        defer close(taskCh)
        for _, item := range items {
            select {
            case <-ctx.Done():
                return
            case taskCh <- item:
            }
        }
    }()

    wg.Wait()
    return ctx.Err()
}
```

### Parallel Fetch with Result Channels

```go
func (s *Service) FetchBoth(id string) (*Data, error) {
    type result struct {
        data *Response
        err  error
    }

    ch1 := make(chan result, 1)
    ch2 := make(chan result, 1)

    go func() {
        data, err := s.fetchOne(id)
        ch1 <- result{data, err}
    }()

    go func() {
        data, err := s.fetchTwo(id)
        ch2 <- result{data, err}
    }()

    var r1, r2 *Response
    for range 2 {
        select {
        case res := <-ch1:
            if res.err != nil {
                return nil, res.err
            }
            r1 = res.data
        case res := <-ch2:
            if res.err != nil {
                return nil, res.err
            }
            r2 = res.data
        }
    }

    return &Data{One: *r1, Two: *r2}, nil
}
```

### Errgroup for Concurrent Operations

```go
import "golang.org/x/sync/errgroup"

func (s *Service) ProcessAll(ctx context.Context, items []Item) error {
    g, ctx := errgroup.WithContext(ctx)

    for _, item := range items {
        item := item  // capture loop variable
        g.Go(func() error {
            return s.process(ctx, item)
        })
    }

    return g.Wait()
}
```

### Thread-Safe State

```go
type Cycle[T any] struct {
    items []T
    index int
    mu    sync.Mutex
}

func (c *Cycle[T]) Next() T {
    c.mu.Lock()
    defer c.mu.Unlock()

    item := c.items[c.index]
    c.index = (c.index + 1) % len(c.items)
    return item
}
```

## HTTP Client Pattern

Structure: `packages/utils/httpclient/{client.go, proxies.go, cookies.go}`

Wrap `*http.Client` for cleaner API. Include cookie jar. Always wrap errors with context.

### client.go

```go
package httpclient

import (
    "fmt"
    "github.com/enetx/http"
    "net/http/cookiejar"
    "net/url"
)

type Client struct {
    http  *http.Client
    proxy *url.URL
}

func NewClient(proxy *url.URL) (*Client, error) {
    httpClient, err := newClient(proxy)
    if err != nil {
        return nil, fmt.Errorf("creating http client: %w", err)
    }

    return &Client{
        http:  httpClient,
        proxy: proxy,
    }, nil
}

func newClient(proxy *url.URL) (*http.Client, error) {
    jar, err := cookiejar.New(nil)
    if err != nil {
        return nil, fmt.Errorf("creating cookie jar: %w", err)
    }

    return &http.Client{
        Transport: &http.Transport{
            Proxy: http.ProxyURL(proxy),
        },
        Jar: jar,
    }, nil
}
```

### proxies.go

```go
package httpclient

import (
    "bufio"
    "fmt"
    "net/url"
    "os"
    "strings"
)

func ParseProxy(line string) (*url.URL, error) {
    parts := strings.Split(line, ":")
    if len(parts) != 2 && len(parts) != 4 {
        return nil, fmt.Errorf("invalid proxy: %s", line)
    }

    proxy := &url.URL{
        Scheme: "http",
        Host:   parts[0] + ":" + parts[1],
    }

    if len(parts) == 4 {
        proxy.User = url.UserPassword(parts[2], parts[3])
    }

    return proxy, nil
}

func ImportProxies(filename string) ([]*url.URL, error) {
    f, err := os.Open(filename)
    if err != nil {
        return nil, fmt.Errorf("opening proxy file: %w", err)
    }
    defer f.Close()

    var proxies []*url.URL
    scanner := bufio.NewScanner(f)
    for scanner.Scan() {
        proxy, err := ParseProxy(scanner.Text())
        if err != nil {
            return nil, fmt.Errorf("parsing proxy line %q: %w", scanner.Text(), err)
        }
        proxies = append(proxies, proxy)
    }

    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("scanning proxy file: %w", err)
    }

    return proxies, nil
}
```

### cookies.go

```go
package httpclient

import (
    "github.com/enetx/http"
    "net/url"
)

func (c *Client) SetCookies(url *url.URL, cookies []*http.Cookie) {
    for _, cookie := range cookies {
        c.jar.SetCookies(url, []*http.Cookie{cookie})
    }
}
```

### Usage Example

```go
type APIClient struct {
    http    *httpclient.Client
    baseURL string
}

func NewAPIClient(baseURL string, proxy *url.URL) (*APIClient, error) {
    client, err := httpclient.NewClient(proxy)
    if err != nil {
        return nil, fmt.Errorf("creating http client: %w", err)
    }

    return &APIClient{
        http:    client,
        baseURL: baseURL,
    }, nil
}

func (c *APIClient) Get(ctx context.Context, path string, response any) error {
    req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
    if err != nil {
        return fmt.Errorf("creating request: %w", err)
    }

    req.Header.Set("Accept", "application/json")

    res, err := c.http.http.Do(req)
    if err != nil {
        return fmt.Errorf("executing request: %w", err)
    }
    defer res.Body.Close()

    if res.StatusCode != http.StatusOK {
        return fmt.Errorf("unexpected status: %d", res.StatusCode)
    }

    if err := json.NewDecoder(res.Body).Decode(response); err != nil {
        return fmt.Errorf("decoding response: %w", err)
    }

    return nil
}
```

## Code Organization

### Package Structure

```
project/
├── cmd/
│   └── server/
│       └── main.go           # entrypoint
├── internal/
│   ├── handlers/             # HTTP handlers
│   ├── services/             # business logic
│   └── repository/           # data access
├── pkg/
│   ├── client/               # HTTP client utilities
│   └── models/               # shared types
└── packages/
    └── logger/               # default logger setup
```

### Layered Architecture

**Repository Layer** - Data access

```go
type Repository interface {
    Get(ctx context.Context, id string) (*Item, error)
    Save(ctx context.Context, item *Item) error
}

type pgRepo struct {
    db     *sql.DB
    logger *slog.Logger
}

func NewRepository(db *sql.DB, logger *slog.Logger) Repository {
    return &pgRepo{db: db, logger: logger}
}
```

**Service Layer** - Business logic

```go
type Service struct {
    repo   Repository
    logger *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) *Service {
    return &Service{repo: repo, logger: logger}
}

func (s *Service) Process(ctx context.Context, id string) error {
    item, err := s.repo.Get(ctx, id)
    if err != nil {
        return fmt.Errorf("get item: %w", err)
    }

    // business logic here

    return s.repo.Save(ctx, item)
}
```

**Handler Layer** - HTTP/transport

```go
type Handler struct {
    service *Service
    logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
    return &Handler{service: service, logger: logger}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")

    item, err := h.service.Get(r.Context(), id)
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            http.Error(w, "not found", http.StatusNotFound)
            return
        }
        h.logger.Error("get failed", "error", err)
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(item)
}
```

### Dependency Wiring

Prefer manual wiring. Keep it explicit in `main.go`.

```go
func main() {
    logger := slog.Default()

    db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
    if err != nil {
        logger.Error("failed to open database", "error", err)
        os.Exit(1)
    }
    defer db.Close()

    repo := repository.New(db, logger)
    service := service.New(repo, logger)
    handler := handler.New(service, logger)

    mux := http.NewServeMux()
    mux.HandleFunc("GET /items/{id}", handler.Get)
    mux.HandleFunc("POST /items", handler.Create)

    server := &http.Server{
        Addr:         ":8080",
        Handler:      mux,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
    }

    logger.Info("server starting", "port", "8080")
    if err := server.ListenAndServe(); err != nil {
        logger.Error("server error", "error", err)
    }
}
```

## Context Propagation

First parameter for functions doing I/O. Pass through call chain. Use for cancellation and timeouts.

```go
func (s *Service) Process(ctx context.Context, id string) error {
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    return s.doWork(ctx, id)
}
```

## Logging

Use `log/slog` with structured logging. Initialize default logger via side-effect import.

```go
// packages/logger/logger.go
package log

import (
    "log/slog"
    "os"

    "github.com/lmittmann/tint"
)

func init() {
    slog.SetDefault(slog.New(tint.NewHandler(os.Stdout, nil)))
}
```

```go
// main.go
import (
    "log/slog"

    _ "project/packages/logger"
)

func main() {
    slog.Info("starting application")
}
```

Use structured attributes. All log messages lowercase.

```go
s.logger.InfoContext(ctx, "item processed",
    slog.Duration("duration", elapsed),
    slog.String("item_id", id),
)

s.logger.ErrorContext(ctx, "failed to save",
    slog.Any("error", err),
)
```

## Constants and Enums

Use typed constants for enum behavior. Use `iota` for sequential integers.

```go
// String-based enum
type Status string

const (
    StatusPending   Status = "pending"
    StatusActive    Status = "active"
    StatusCompleted Status = "completed"
)

// Integer-based enum
type Priority int

const (
    PriorityLow Priority = iota
    PriorityMedium
    PriorityHigh
    PriorityCritical
)
```

## Testing

See [TESTING.md](TESTING.md) for comprehensive testing patterns.

Quick commands:

```bash
go test ./...                     # all tests
go test -v ./pkg/storage/...      # verbose, specific package
go test -run TestName ./...       # specific test
go test -race ./...               # race detection
go test -cover ./...              # coverage
```

## Development Workflow

### Dependency Management

Prefer `go mod tidy` over `go get` for adding dependencies.

**Workflow:**

1. Add import to your `.go` file: `import "github.com/user/package"`
2. Run `go mod tidy` to fetch and add to `go.mod`

```bash
# PREFERRED: Import in code, then tidy
go mod tidy

# Use go get for specific versions
go get github.com/user/package@v1.2.3

# Use go get for upgrading dependencies
go get -u github.com/user/package

# Update all dependencies to latest minor/patch
go get -u ./...
```

**Why prefer `go mod tidy`:**

- Automatically manages both additions and removals
- Ensures `go.mod` and `go.sum` are in sync
- Removes unused dependencies
- Cleaner workflow: write code first, manage deps second
- Less error-prone than manual `go get` for each package

### Static Analysis

Use `go vet` to check for suspicious code without building. Faster than full build for catching common errors.

```bash
go vet ./...                 # all packages
go vet ./internal/services   # specific package
gofmt -l .                   # check formatting
go vet ./... && gofmt -l .   # combine checks
```

**Common issues `go vet` catches:**

- Printf format string mismatches
- Unreachable code
- Incorrect use of sync primitives
- Shadow variables
- Struct tags validation

**When to use:**

- During development for quick validation
- In pre-commit hooks
- In CI/CD before running tests
