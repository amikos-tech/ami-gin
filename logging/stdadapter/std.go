// Package stdadapter wires the stdlib *log.Logger into the repo-owned
// logging contract. Importing logging alone does not pull this adapter
// into the build graph; consumers that want plain log.Print fan-out opt in
// by importing this subpackage explicitly.
package stdadapter

import (
	"fmt"
	"log"
	"strings"

	"github.com/amikos-tech/ami-gin/logging"
)

type adapter struct {
	logger *log.Logger
}

// New returns a logging.Logger that forwards to the supplied *log.Logger.
// A nil logger collapses to the noop logger so callers do not need to guard
// optional configuration. Debug-level events are dropped because the stdlib
// logger has no native severity routing.
func New(l *log.Logger) logging.Logger {
	if l == nil {
		return logging.NewNoop()
	}
	return adapter{logger: l}
}

func (a adapter) Enabled(level logging.Level) bool {
	return level != logging.LevelDebug
}

func (a adapter) Log(level logging.Level, msg string, attrs ...logging.Attr) {
	if !a.Enabled(level) {
		return
	}

	var b strings.Builder
	b.WriteString(levelPrefix(level))
	b.WriteString(msg)
	for _, attr := range attrs {
		fmt.Fprintf(&b, " %s=%s", attr.Key, attr.Value)
	}
	a.logger.Print(b.String())
}

// levelPrefix tags stdlib output with a bracketed severity so operators
// ingesting into journald/Splunk can distinguish Info from Warn from Error.
// Unknown levels fall back to "[ERROR] " to mirror the slog adapter,
// so a future logging.Level constant surfaces as an operator-visible event
// rather than being buried as Info in stdlib output.
// LevelDebug is mapped here defensively; Log's Enabled gate drops Debug
// before the prefix is reached.
func levelPrefix(level logging.Level) string {
	switch level {
	case logging.LevelDebug:
		return "[DEBUG] "
	case logging.LevelInfo:
		return "[INFO] "
	case logging.LevelWarn:
		return "[WARN] "
	case logging.LevelError:
		return "[ERROR] "
	default:
		return "[ERROR] "
	}
}
