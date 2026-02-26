package goutils

import (
	"context"
	"log/slog"
	"os"
)

var logger *slog.Logger

func init() {
	level := slog.LevelInfo
	if StrToBool(os.Getenv("DEBUG")) {
		level = slog.LevelDebug
	}

	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
}

// Log logs a message at Info level.
func Log(msg string, args ...any) {
	logger.Log(context.Background(), slog.LevelInfo, msg, args...)
}

// Err logs a message at Error level.
func Err(msg string, args ...any) {
	logger.Log(context.Background(), slog.LevelError, msg, args...)
}

// Debug logs a message at Debug level. Only outputs when DEBUG env var is truthy.
func Debug(msg string, args ...any) {
	logger.Log(context.Background(), slog.LevelDebug, msg, args...)
}
