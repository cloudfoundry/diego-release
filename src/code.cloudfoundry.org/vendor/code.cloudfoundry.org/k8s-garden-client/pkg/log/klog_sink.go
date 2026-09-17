package log

import (
	"code.cloudfoundry.org/lager/v3"
	"github.com/go-logr/logr"
)

type sink struct {
	logger lager.Logger
}

var _ logr.LogSink = &sink{}

func NewSink(logger lager.Logger) *sink {
	return &sink{
		logger: logger,
	}
}

// Enabled implements [logr.LogSink].
func (s *sink) Enabled(level int) bool {
	return true
}

// Error implements [logr.LogSink].
func (s *sink) Error(err error, msg string, keysAndValues ...any) {
	s.logger.Error(msg, err, s.lagerDataFromKeyValues(keysAndValues...))
}

// Info implements [logr.LogSink].
func (s *sink) Info(level int, msg string, keysAndValues ...any) {
	s.logger.Info(msg, s.lagerDataFromKeyValues(keysAndValues...))
}

// Init implements [logr.LogSink].
func (s *sink) Init(info logr.RuntimeInfo) {}

// WithName implements [logr.LogSink].
func (s *sink) WithName(name string) logr.LogSink {
	return NewSink(s.logger.Session(name))
}

// WithValues implements [logr.LogSink].
func (s *sink) WithValues(keysAndValues ...any) logr.LogSink {
	return NewSink(s.logger.Session("", s.lagerDataFromKeyValues(keysAndValues...)))
}

func (s *sink) lagerDataFromKeyValues(keysAndValues ...any) lager.Data {
	lagerData := lager.Data{}
	if len(keysAndValues) > 0 {
		for i := 0; i < len(keysAndValues)-1; i += 2 {
			key, ok := keysAndValues[i].(string)
			if !ok {
				continue
			}
			lagerData[key] = keysAndValues[i+1]
		}
	}
	return lagerData
}
