package runner

type logger interface {
	Debug(...interface{})
	Debugf(string, ...interface{})
	Info(...interface{})
	Infof(string, ...interface{})
	Warn(...interface{})
	Warnf(string, ...interface{})
	Error(...interface{})
	Errorf(string, ...interface{})
	Fatal(...interface{})
	Fatalf(string, ...interface{})
}

// emptyLogger is a no-op logger that implements the logger interface.
// This is the default logger used when no other logger is provided.
type emptyLogger struct{}

func (e emptyLogger) Debug(...interface{})          {}
func (e emptyLogger) Debugf(string, ...interface{}) {}
func (e emptyLogger) Info(...interface{})           {}
func (e emptyLogger) Infof(string, ...interface{})  {}
func (e emptyLogger) Warn(...interface{})           {}
func (e emptyLogger) Warnf(string, ...interface{})  {}
func (e emptyLogger) Error(...interface{})          {}
func (e emptyLogger) Errorf(string, ...interface{}) {}
func (e emptyLogger) Fatal(...interface{})          {}
func (e emptyLogger) Fatalf(string, ...interface{}) {}

var log logger = emptyLogger{}

func SetLogger(l logger) {
	log = l
}
