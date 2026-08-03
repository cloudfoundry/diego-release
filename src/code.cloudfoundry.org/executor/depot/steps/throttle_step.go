package steps

import (
	"os"

	"code.cloudfoundry.org/workpool"
	"github.com/tedsuo/ifrit"
)

type throttleStep struct {
	substep  ifrit.Runner
	workPool *workpool.WorkPool
}

func NewThrottle(substep ifrit.Runner, workPool *workpool.WorkPool) *throttleStep {
	return &throttleStep{
		substep:  substep,
		workPool: workPool,
	}
}

func (step *throttleStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	stepResult := make(chan error)

	step.workPool.Submit(func() {
		stepResult <- step.substep.Run(signals, ready)
	})

	return <-stepResult
}
