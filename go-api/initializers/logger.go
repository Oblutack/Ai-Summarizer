package initializers

import (
	"log/slog"
	"os"
	"strings"
)

// SetupLogger configures structured logging and makes it the process default.
//
//	LOG_LEVEL:  debug | info (default) | warn | error
//	LOG_FORMAT: json (default, for log aggregators) | text (easier to read locally)
func SetupLogger() *slog.Logger {
	var level slog.Level
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler = slog.NewJSONHandler(os.Stdout, opts)
	if strings.ToLower(os.Getenv("LOG_FORMAT")) == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler).With("service", "go-api")
	slog.SetDefault(logger)
	return logger
}
