// Package logger builds the application's structured logger using standard log/slog.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a structured logger writing to stderr at the given level (debug|info|warn|error).
func New(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	return slog.New(h)
}
