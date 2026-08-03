package steps

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/tedsuo/ifrit"
)

type consistentlySucceedsStep struct {
	create    func() ifrit.Runner
	clock     clock.Clock
	frequency time.Duration
}

// TODO: use a workpool when running the substep
func NewConsistentlySucceedsStep(create func() ifrit.Runner, frequency time.Duration, clock clock.Clock) ifrit.Runner {
	return &consistentlySucceedsStep{
		create:    create,
		frequency: frequency,
		clock:     clock,
	}
}

func (step *consistentlySucceedsStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	t := step.clock.NewTimer(step.frequency)

	close(ready)

	for {
		select {
		case <-signals:
			return new(CancelledError)
		case <-t.C():
		}

		consistentStep := step.create()

		process := ifrit.Background(consistentStep)

		select {
		case err := <-process.Wait():
			if err != nil {
				return err
			}
		case s := <-signals:
			process.Signal(s)
			return <-process.Wait()
		}

		t.Reset(step.frequency)
	}
}
