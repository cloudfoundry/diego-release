package jointlock

import (
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
)

func newTimeoutError(name string) error {
	return fmt.Errorf("%s: failed to start in time", name)
}

type timeoutRunner struct {
	runner  grouper.Member
	timeout time.Duration
	clock   clock.Clock
}

func NewTimeoutRunner(clock clock.Clock, timeout time.Duration, runner grouper.Member) *timeoutRunner {
	return &timeoutRunner{
		clock:   clock,
		timeout: timeout,
		runner:  runner,
	}
}

func (t *timeoutRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	proc := ifrit.Background(t.runner)

	err := t.waitForReadyExitOrTimeout(proc, signals)
	if err != nil {
		return err
	}

	close(ready)

	for {
		select {
		case err := <-proc.Wait():
			return err
		case sig := <-signals:
			proc.Signal(sig)
		}
	}
}

func (t *timeoutRunner) waitForReadyExitOrTimeout(proc ifrit.Process, signals <-chan os.Signal) error {
	timer := t.clock.NewTimer(t.timeout)
	for {
		select {
		case <-proc.Ready():
			return nil
		case sig := <-signals:
			proc.Signal(sig)
		case <-timer.C():
			return newTimeoutError(t.runner.Name)
		case err := <-proc.Wait():
			return err
		}
	}
}
