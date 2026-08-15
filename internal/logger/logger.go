package logger

import (
	"io"
	"log/slog"
	"os"
)

func New(w io.Writer, level slog.Level) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewTextHandler(w, opts)
	return slog.New(handler)
}

func Default() *slog.Logger {
	return New(os.Stdout, slog.LevelInfo)
}
