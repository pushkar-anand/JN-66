package config

import (
	"log/slog"
	"os"
	"strings"

	bwglogger "github.com/pushkar-anand/build-with-go/logger"
)

// SetupLogger configures the global slog default from cfg.
// If debugOverride is true the level is forced to Debug regardless of cfg.Level.
func SetupLogger(cfg LogConfig, debugOverride bool) {
	format := bwglogger.FormatText
	if strings.EqualFold(cfg.Format, "json") {
		format = bwglogger.FormatJSON
	}
	level := parseLogLevel(cfg.Level)
	if debugOverride {
		level = slog.LevelDebug
	}
	slog.SetDefault(bwglogger.New(
		bwglogger.WithLevel(level),
		bwglogger.WithFormat(format),
		bwglogger.WithWriter(os.Stderr),
	))
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
