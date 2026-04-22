// Package slogadapter wires log/slog into the repo-owned logging contract.
//
// Importing logging alone does not pull in this package, so consumers
// using only the Logger interface or a different backend keep their
// dependency graph free of slog wiring.
package slogadapter

import (
	"context"
	"log/slog"

	"github.com/amikos-tech/ami-gin/logging"
)

type adapter struct {
	logger *slog.Logger
}

// New returns a logging.Logger that forwards to the supplied *slog.Logger.
// A nil logger collapses to the noop logger so callers do not need to guard
// optional configuration.
func New(l *slog.Logger) logging.Logger {
	if l == nil {
		return logging.NewNoop()
	}
	return adapter{logger: l}
}

func (a adapter) Enabled(level logging.Level) bool {
	return a.logger.Enabled(context.Background(), toSlogLevel(level))
}

func (a adapter) Log(level logging.Level, msg string, attrs ...logging.Attr) {
	if !a.Enabled(level) {
		return
	}
	slogAttrs := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		slogAttrs = append(slogAttrs, slog.String(a.Key, a.Value))
	}
	a.logger.LogAttrs(context.Background(), toSlogLevel(level), msg, slogAttrs...)
}

// toSlogLevel maps the repo levels to slog levels. Unknown levels fall back to
// slog.LevelError so a future logging.Level constant cannot crash adapters
// that have not been updated yet.
func toSlogLevel(level logging.Level) slog.Level {
	switch level {
	case logging.LevelDebug:
		return slog.LevelDebug
	case logging.LevelInfo:
		return slog.LevelInfo
	case logging.LevelWarn:
		return slog.LevelWarn
	case logging.LevelError:
		return slog.LevelError
	default:
		return slog.LevelError
	}
}
