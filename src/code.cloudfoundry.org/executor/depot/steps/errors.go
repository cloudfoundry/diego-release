package steps

import "fmt"

type IsDisplayableError interface {
	IsDisplayable() bool
}

type CancelledError struct{}

func (e *CancelledError) Error() string {
	return "cancelled"
}

func (e *CancelledError) IsDisplayable() bool {
	return false
}

type ExceededGracefulShutdownIntervalError struct{}

func (e *ExceededGracefulShutdownIntervalError) Error() string {
	return "exceeded graceful shutdown interval"
}

func (e *ExceededGracefulShutdownIntervalError) IsDisplayable() bool {
	return false
}

type ExitTimeoutError struct{}

func (e *ExitTimeoutError) Error() string {
	return "process did not exit"
}

func multiErrorFormat(errs []error) string {
	var errStr string
	for _, e := range errs {
		if displayableErr, ok := e.(IsDisplayableError); ok {
			if !displayableErr.IsDisplayable() {
				continue
			}
		}
		if errStr == "" {
			errStr = e.Error()
		} else if e.Error() != "" {
			errStr = fmt.Sprintf("%s; %s", errStr, e)
		}
	}
	return errStr
}
