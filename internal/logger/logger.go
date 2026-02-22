package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
)

const (
	LevelTrace slog.Level = slog.LevelDebug - ((iota + 1) * 4)
)

type ContextHandler struct {
	slog.Handler
}

// Handle overrides the default Handle method to add context values.
func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if key, ok := ctx.Value("idempotency_key").(string); ok {
		r.AddAttrs(slog.String("idempotency_key", key))
	}

	return h.Handler.Handle(ctx, r)
}

func init() {
	w := os.Stdout
	slog.SetDefault(slog.New(
		&ContextHandler{
			Handler: tint.NewHandler(colorable.NewColorable(w), &tint.Options{
				TimeFormat: "01/02 03:04:05 pm MST", // "Mon, Jan 2 2006, 3:04:05 pm MST",
				NoColor:    disableColor(w),
				Level:      logLevelFromEnv(),
				ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
					if attr.Key != slog.LevelKey || len(groups) != 0 {
						return attr
					}

					level, ok := attr.Value.Any().(slog.Level)
					if !ok {
						return attr
					}

					switch level {
					case LevelTrace:
						return tint.Attr(12, slog.String(attr.Key, "TRC"))
					default:
						return attr
					}
				},
			}),
		}),
	)
}

func logLevelFromEnv() slog.Level {
	switch strings.ToLower(
		strings.TrimSpace(
			os.Getenv("LOG_LEVEL"),
		),
	) {
	case "trace":
		return LevelTrace
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return LevelTrace
	}
}

func disableColor(out *os.File) bool {
	forceColor := strings.EqualFold(os.Getenv("LOG_FORCE_COLOR"), "1") ||
		strings.EqualFold(os.Getenv("LOG_FORCE_COLOR"), "true") ||
		strings.EqualFold(os.Getenv("LOG_FORCE_COLOR"), "yes")

	if forceColor {
		return false
	}

	return !isatty.IsTerminal(out.Fd())
}
