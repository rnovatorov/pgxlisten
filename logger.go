package pgxlisten

type logger interface {
	Printf(format string, v ...any)
}

type noopLogger struct{}

func (noopLogger) Printf(format string, v ...any) {}
