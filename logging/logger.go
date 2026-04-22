package logging

// Level identifies the supported logging severities.
type Level uint8

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger is the repo-owned backend-neutral logging contract.
// Implementations must be safe for concurrent use.
// The contract is intentionally context-free: callers manage trace correlation
// externally; the logger never injects trace IDs or span IDs into log output.
type Logger interface {
	Enabled(Level) bool
	Log(Level, string, ...Attr)
}

// Debug emits a debug-level message. It guards on Enabled before calling Log
// so callers do not pay for argument construction when debug is disabled.
func Debug(logger Logger, msg string, attrs ...Attr) {
	logger = Default(logger)
	if !logger.Enabled(LevelDebug) {
		return
	}
	logger.Log(LevelDebug, msg, attrs...)
}

// Info emits an info-level message.
func Info(logger Logger, msg string, attrs ...Attr) {
	Default(logger).Log(LevelInfo, msg, attrs...)
}

// Warn emits a warn-level message.
func Warn(logger Logger, msg string, attrs ...Attr) {
	Default(logger).Log(LevelWarn, msg, attrs...)
}

// Error emits an error-level message.
func Error(logger Logger, msg string, attrs ...Attr) {
	Default(logger).Log(LevelError, msg, attrs...)
}
