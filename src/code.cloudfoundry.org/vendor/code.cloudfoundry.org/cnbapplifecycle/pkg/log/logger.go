package log

import (
	"io"

	apexLog "github.com/apex/log"
	"github.com/buildpacks/lifecycle/cmd"
	"github.com/buildpacks/lifecycle/log"
	packLog "github.com/buildpacks/pack/pkg/logging"
)

var _ packLog.Logger = (*Logger)(nil)
var _ log.LoggerHandlerWithLevel = (*Logger)(nil)

type Logger struct {
	*log.DefaultLogger
}

func (l *Logger) IsVerbose() bool {
	return l.LogLevel() == apexLog.DebugLevel
}

func (l *Logger) Writer() io.Writer {
	return cmd.Stdout
}

func NewLogger() *Logger {
	return &Logger{
		DefaultLogger: cmd.DefaultLogger,
	}
}
