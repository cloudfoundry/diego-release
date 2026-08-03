package helpers

import (
	"fmt"
	"runtime"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

func StopProcesses(processes ...ifrit.Process) {
	failures := InterceptGomegaFailures(func() {
		for _, process := range processes {
			if process == nil {
				// sometimes components aren't initialized in individual tests, but a full
				// suite may want AfterEach to clean up everything
				continue
			}

			if runtime.GOOS == "windows" {
				process.Signal(syscall.SIGKILL)
			} else {
				process.Signal(syscall.SIGTERM)
			}

			select {
			case <-process.Wait():
			case <-time.After(time.Minute):
				fmt.Fprintf(GinkgoWriter, "!!!!!!!!!!!!!!!! STOP TIMEOUT !!!!!!!!!!!!!!!!")

				process.Signal(syscall.SIGQUIT)
				Eventually(process.Wait(), 10*time.Second).Should(Receive())

				Expect(true).To(BeFalse(), "process did not shut down cleanly; SIGQUIT sent")
			}
		}
	})

	Expect(failures).To(BeEmpty(), "at least one process failed to shut down cleanly")
}
