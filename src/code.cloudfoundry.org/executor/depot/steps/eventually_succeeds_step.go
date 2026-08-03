package steps

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/tedsuo/ifrit"
)

type eventuallySucceedsStep struct {
	create             func() ifrit.Runner
	frequency, timeout time.Duration
	clock              clock.Clock
}

// TODO: use a workpool when running the substep
func NewEventuallySucceedsStep(create func() ifrit.Runner, frequency, timeout time.Duration, clock clock.Clock) ifrit.Runner {
	return &eventuallySucceedsStep{
		create:    create,
		frequency: frequency,
		timeout:   timeout,
		clock:     clock,
	}
}

func (step *eventuallySucceedsStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	var err error

	close(ready)

	startTime := step.clock.Now()
	t := step.clock.NewTimer(step.frequency)

	for {
		select {
		case <-t.C():
		case <-signals:
			return new(CancelledError)
		}

		subProcess := ifrit.Background(step.create())

		select {
		case s := <-signals:
			subProcess.Signal(s)
			return <-subProcess.Wait()
		case err = <-subProcess.Wait():
			if err == nil {
				return nil
			}
		}

		if step.timeout > 0 && step.clock.Now().After(startTime.Add(step.timeout)) {
			return err
		}

		t.Reset(step.frequency)
	}
}
