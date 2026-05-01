package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

// NewLogger initializes a custom slog handler with colors and formatting.
func NewLogger() *slog.Logger {
	return slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		AddSource:  false,
		Level:      slog.LevelDebug,
		TimeFormat: time.Kitchen,
		NoColor:    false,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// 4-letter levels with colors
			if a.Key == slog.LevelKey && len(groups) == 0 {
				level := a.Value.Any().(slog.Level)
				switch level {
				case slog.LevelDebug:
					return slog.String(a.Key, "DEBG")
				case slog.LevelInfo:
					return tint.Attr(2, slog.String(a.Key, "INFO"))
				case slog.LevelWarn:
					return tint.Attr(3, slog.String(a.Key, "WARN"))
				case slog.LevelError:
					return tint.Attr(1, slog.String(a.Key, "ERRO"))
				}
			}
			// color(red) to error Attr
			if a.Value.Kind() == slog.KindAny {
				if _, ok := a.Value.Any().(error); ok {
					return tint.Attr(9, a)
				}
			}

			// color(yellow) to source location
			if a.Key == slog.SourceKey && len(groups) == 0 {
				return tint.Attr(12, a)
			}
			// Key in Cyan, Value in default
			if a.Key != slog.MessageKey && a.Key != slog.TimeKey && a.Key != "" {
				colorKey := "\033[36m" + a.Key + "\033[0m"
				return slog.Attr{
					Key:   colorKey,
					Value: a.Value,
				}
			}
			return a
		},
	}))
}

/* Example Log statements

// DEBUG: High-volume, granular information for troubleshooting
logger.Debug("cache lookup result",
	"key", "user_profile_123",
	"hit", false,
	"latency_ms", 12)

// INFO: General operational milestones
logger.Info("http request handled",
	"method", "POST",
	"path", "/v1/orders",
	"status", 201,
	"ip", "192.168.1.50")

// WARN: Potentially harmful situations or unusual behavior
logger.Warn("slow database query",
	"duration", "1.2s",
	"query", "SELECT * FROM analytics",
	"rows_affected", 45000)

// ERROR: Serious issues that failed a specific operation
logger.Error("failed to process payment",
	"error", errors.New("insufficient funds"),
	"transaction_id", "tx_998877",
	"gateway", "stripe")

*/
