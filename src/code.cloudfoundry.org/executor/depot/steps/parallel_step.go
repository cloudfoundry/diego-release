package steps

import (
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/tedsuo/ifrit"
)

type parallelStep struct {
	substeps []ifrit.Runner
}

func NewParallel(substeps []ifrit.Runner) *parallelStep {
	return &parallelStep{
		substeps: substeps,
	}
}

func (step *parallelStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	var subProcesses []ifrit.Process
	for _, subStep := range step.substeps {
		subProcesses = append(subProcesses, ifrit.Background(subStep))
	}

	done := make(chan struct{})
	defer close(done)

	go waitForChildrenToBeReady(done, subProcesses, ready)
	go waitForSignal(done, signals, subProcesses)

	aggregate := &multierror.Error{}
	aggregate.ErrorFormat = multiErrorFormat

	for _, subProcess := range subProcesses {
		err := <-subProcess.Wait()
		if err != nil {
			aggregate = multierror.Append(aggregate, err)
		}
	}

	return aggregate.ErrorOrNil()
}

func waitForSignal(done <-chan struct{}, signals <-chan os.Signal, ps []ifrit.Process) {
	select {
	case <-done:
		return
	case s := <-signals:
		cancel(ps, s)
	}
}

func cancel(processes []ifrit.Process, signal os.Signal) {
	for _, p := range processes {
		p.Signal(signal)
	}
}

func waitForChildrenToBeReady(done <-chan struct{}, ps []ifrit.Process, ready chan<- struct{}) {
	for _, p := range ps {
		select {
		case <-done:
			// do not leak goroutine if a subProcess exits before it is ready
			return
		case <-p.Ready():
		}
	}
	close(ready)
}
