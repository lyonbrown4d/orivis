package logging

import (
	"log/slog"
	"strings"

	"github.com/arcgolabs/logx"
)

func New(level string) (*slog.Logger, error) {
	return logx.New(
		logx.WithConsole(true),
		logx.WithLevel(parseLevel(level)),
		logx.WithCaller(true),
	)
}

func Close(logger *slog.Logger) error {
	return logx.Close(logger)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
