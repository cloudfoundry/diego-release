package steps

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tedsuo/ifrit"
)

type outputWrapperStep struct {
	substep ifrit.Runner
	prefix  string
	reader  io.Reader
}

// This step ignores the error from the substep and returns the content of
// Reader as an emittable error. This is used to wrap the output of the
// healthcheck as the error instead of using the exit status or the process
func NewOutputWrapper(substep ifrit.Runner, reader io.Reader) ifrit.Runner {
	return NewOutputWrapperWithPrefix(substep, reader, "")
}

func NewOutputWrapperWithPrefix(substep ifrit.Runner, reader io.Reader, prefix string) ifrit.Runner {
	return &outputWrapperStep{
		substep: substep,
		reader:  reader,
		prefix:  prefix,
	}
}

func (step *outputWrapperStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	subStepErr := step.substep.Run(signals, ready)

	if subStepErr == nil {
		return nil
	}

	bytes, err := io.ReadAll(step.reader)
	if err != nil {
		return fmt.Errorf("Error reading from process output buffer: %s", err)
	}

	msg := fmt.Sprintf("Failed to invoke process: %s", subStepErr)

	readerErr := string(bytes)
	if readerErr != "" {
		msg = strings.TrimSpace(readerErr)
	}

	if step.prefix != "" {
		msg = step.prefix + ": " + msg
	}
	return NewEmittableError(subStepErr, "%s", msg)
}
