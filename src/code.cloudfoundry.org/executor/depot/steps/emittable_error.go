package steps

import "fmt"

type EmittableError struct {
	msg          string
	wrappedError error
}

func NewEmittableError(wrappedError error, message string, args ...interface{}) *EmittableError {
	msg := message
	if len(args) > 0 {
		msg = fmt.Sprintf(message, args...)
	}

	return &EmittableError{
		wrappedError: wrappedError,
		msg:          msg,
	}
}

func (e *EmittableError) Error() string {
	return e.msg
}

func (e *EmittableError) WrappedError() error {
	return e.wrappedError
}
