package logging

type noopLogger struct{}

var sharedNoop Logger = noopLogger{}

// NewNoop returns the package-wide noop logger. The returned value is the
// same shared instance on every call; the noop logger has no state.
func NewNoop() Logger {
	return sharedNoop
}

// Default returns the supplied logger when non-nil, otherwise the shared
// noop logger. Boundary packages use this to normalize zero values without
// guarding every call site.
func Default(logger Logger) Logger {
	if logger != nil {
		return logger
	}
	return sharedNoop
}

func (noopLogger) Enabled(Level) bool    { return false }
func (noopLogger) Log(Level, string, ...Attr) {}
