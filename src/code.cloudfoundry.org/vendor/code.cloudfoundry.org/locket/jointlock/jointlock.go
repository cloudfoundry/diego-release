package jointlock

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"

	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
)

func NewJointLock(clock clock.Clock, timeout time.Duration, runners ...grouper.Member) ifrit.Runner {
	if len(runners) > 1 {
		for i := 1; i < len(runners); i++ {
			runners[i].Runner = NewTimeoutRunner(clock, timeout, runners[i])
		}
	}

	return grouper.NewOrdered(os.Interrupt, runners)
}
