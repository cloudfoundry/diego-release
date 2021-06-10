// +build windows

package consulrunner

import (
	"time"

	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

func stopSignal(process ifrit.Process, interval time.Duration) {
	ginkgomon.Kill(process, interval)
}
