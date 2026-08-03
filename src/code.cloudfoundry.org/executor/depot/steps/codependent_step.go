package steps

import (
	"errors"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/tedsuo/ifrit"
)

var CodependentStepExitedError = errors.New("Codependent step exited")

type codependentStep struct {
	substeps           []ifrit.Runner
	errorOnExit        bool
	cancelOthersOnExit bool
}

func NewCodependent(substeps []ifrit.Runner, errorOnExit bool, cancelOthersOnExit bool) ifrit.Runner {
	return &codependentStep{
		substeps:           substeps,
		errorOnExit:        errorOnExit,
		cancelOthersOnExit: cancelOthersOnExit,
	}
}

func (step *codependentStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	errCh := make(chan error, len(step.substeps))

	var subProcesses []ifrit.Process
	for _, subStep := range step.substeps {
		subProcess := ifrit.Background(subStep)
		subProcesses = append(subProcesses, subProcess)
		go func() {
			err := <-subProcess.Wait()
			errCh <- err
		}()
	}

	done := make(chan struct{})
	defer close(done)

	go waitForSignal(done, signals, subProcesses)
	go waitForChildrenToBeReady(done, subProcesses, ready)

	aggregate := &multierror.Error{}
	aggregate.ErrorFormat = multiErrorFormat

	var cancelled bool

	for range subProcesses {
		err := <-errCh
		if step.errorOnExit && err == nil {
			err = CodependentStepExitedError
		}

		if step.cancelOthersOnExit && err == nil {
			if !cancelled {
				cancelled = true
				cancel(subProcesses, os.Interrupt)
			}
		}

		if err != nil {
			aggregate = multierror.Append(aggregate, err)

			if !cancelled {
				cancelled = true
				cancel(subProcesses, os.Interrupt)
			}
		}
	}

	return aggregate.ErrorOrNil()
}
