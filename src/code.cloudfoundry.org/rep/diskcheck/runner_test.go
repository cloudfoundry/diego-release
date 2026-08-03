package diskcheck_test

import (
	"errors"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	fakeexecutor "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/rep/diskcheck"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context/fake_evacuation_context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
)

var _ = Describe("Runner", func() {
	var (
		runner           *diskcheck.Runner
		process          ifrit.Process
		logger           *lagertest.TestLogger
		executorClient   *fakeexecutor.FakeClient
		evacuatable      *fake_evacuation_context.FakeEvacuatable
		fakeClock        *fakeclock.FakeClock
		checkInterval    time.Duration
		failureThreshold int
		checkPathFunc    func(string) (bool, error)
		checkPathErr     error
		checkPathRO      bool
		checkPathMu      sync.Mutex
		paths            []string
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("diskcheck-test")
		executorClient = &fakeexecutor.FakeClient{}
		evacuatable = &fake_evacuation_context.FakeEvacuatable{}
		fakeClock = fakeclock.NewFakeClock(time.Now())
		checkInterval = 15 * time.Second
		failureThreshold = 1
		checkPathErr = nil
		checkPathRO = false
		paths = []string{"/some/path"}

		checkPathFunc = func(path string) (bool, error) {
			checkPathMu.Lock()
			defer checkPathMu.Unlock()
			return checkPathRO, checkPathErr
		}
	})

	JustBeforeEach(func() {
		runner = diskcheck.NewRunner(logger, fakeClock, paths, checkInterval, failureThreshold, executorClient, evacuatable, checkPathFunc)
		process = ifrit.Background(runner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	Context("when a path is read-only on startup", func() {
		BeforeEach(func() {
			checkPathRO = true
		})

		It("marks the executor unhealthy", func() {
			Eventually(executorClient.SetHealthyCallCount).Should(Equal(1))
			_, healthy := executorClient.SetHealthyArgsForCall(0)
			Expect(healthy).To(BeFalse())
		})

		It("calls evacuate", func() {
			Eventually(evacuatable.EvacuateCallCount).Should(Equal(1))
		})

		It("does not become ready", func() {
			Eventually(evacuatable.EvacuateCallCount).Should(Equal(1))
			Consistently(process.Ready()).ShouldNot(BeClosed())
		})

		It("exits with an error when signalled", func() {
			Eventually(evacuatable.EvacuateCallCount).Should(Equal(1))
			process.Signal(os.Interrupt)
			Eventually(process.Wait()).Should(Receive(Equal(diskcheck.ErrDiskReadOnly)))
		})
	})

	Context("when a path check returns an error on startup", func() {
		BeforeEach(func() {
			checkPathErr = errors.New("statfs failed")
		})

		It("marks the executor unhealthy", func() {
			Eventually(executorClient.SetHealthyCallCount).Should(Equal(1))
			_, healthy := executorClient.SetHealthyArgsForCall(0)
			Expect(healthy).To(BeFalse())
		})

		It("calls evacuate", func() {
			Eventually(evacuatable.EvacuateCallCount).Should(Equal(1))
		})

		It("exits with an error when signalled", func() {
			Eventually(evacuatable.EvacuateCallCount).Should(Equal(1))
			process.Signal(os.Interrupt)
			Eventually(process.Wait()).Should(Receive(Equal(diskcheck.ErrDiskReadOnly)))
		})
	})

	Context("when the second of two paths is unhealthy on startup", func() {
		BeforeEach(func() {
			paths = []string{"/healthy/path", "/unhealthy/path"}
			checkPathFunc = func(path string) (bool, error) {
				if path == "/unhealthy/path" {
					return true, nil
				}
				return false, nil
			}
		})

		It("marks the executor unhealthy and evacuates", func() {
			Eventually(evacuatable.EvacuateCallCount).Should(Equal(1))
			Eventually(executorClient.SetHealthyCallCount).Should(Equal(1))
			_, healthy := executorClient.SetHealthyArgsForCall(0)
			Expect(healthy).To(BeFalse())
		})

		It("does not become ready", func() {
			Eventually(evacuatable.EvacuateCallCount).Should(Equal(1))
			Consistently(process.Ready()).ShouldNot(BeClosed())
		})
	})

	Context("when all paths are healthy on startup", func() {
		BeforeEach(func() {
			checkPathRO = false
			checkPathErr = nil
		})

		It("becomes ready", func() {
			Eventually(process.Ready()).Should(BeClosed())
		})

		It("does not call SetHealthy or Evacuate", func() {
			Eventually(process.Ready()).Should(BeClosed())
			Expect(executorClient.SetHealthyCallCount()).To(Equal(0))
			Expect(evacuatable.EvacuateCallCount()).To(Equal(0))
		})

		Context("and a path becomes read-only during a periodic check", func() {
			JustBeforeEach(func() {
				Eventually(process.Ready()).Should(BeClosed())
				checkPathMu.Lock()
				checkPathRO = true
				checkPathMu.Unlock()
				fakeClock.WaitForWatcherAndIncrement(checkInterval)
			})

			It("marks the executor unhealthy", func() {
				Eventually(executorClient.SetHealthyCallCount).Should(Equal(1))
				_, healthy := executorClient.SetHealthyArgsForCall(0)
				Expect(healthy).To(BeFalse())
			})

			It("calls evacuate", func() {
				Eventually(evacuatable.EvacuateCallCount).Should(Equal(1))
			})

			It("exits with error when signalled", func() {
				Eventually(evacuatable.EvacuateCallCount).Should(Equal(1))
				process.Signal(os.Interrupt)
				Eventually(process.Wait()).Should(Receive(Equal(diskcheck.ErrDiskReadOnly)))
			})
		})

		Context("and a path check returns an error during a periodic check", func() {
			JustBeforeEach(func() {
				Eventually(process.Ready()).Should(BeClosed())
				checkPathMu.Lock()
				checkPathErr = errors.New("statfs failed")
				checkPathMu.Unlock()
				fakeClock.WaitForWatcherAndIncrement(checkInterval)
			})

			It("marks the executor unhealthy", func() {
				Eventually(executorClient.SetHealthyCallCount).Should(Equal(1))
				_, healthy := executorClient.SetHealthyArgsForCall(0)
				Expect(healthy).To(BeFalse())
			})

			It("calls evacuate", func() {
				Eventually(evacuatable.EvacuateCallCount).Should(Equal(1))
			})

			It("exits with error when signalled", func() {
				Eventually(evacuatable.EvacuateCallCount).Should(Equal(1))
				process.Signal(os.Interrupt)
				Eventually(process.Wait()).Should(Receive(Equal(diskcheck.ErrDiskReadOnly)))
			})
		})

		Context("when signalled before any failure", func() {
			It("exits with nil", func() {
				Eventually(process.Ready()).Should(BeClosed())
				process.Signal(os.Interrupt)
				Eventually(process.Wait()).Should(Receive(BeNil()))
			})
		})
	})

	Context("with a failure threshold of 3", func() {
		BeforeEach(func() {
			failureThreshold = 3
		})

		Context("when a path becomes read-only during periodic checks", func() {
			JustBeforeEach(func() {
				Eventually(process.Ready()).Should(BeClosed())
				checkPathMu.Lock()
				checkPathRO = true
				checkPathMu.Unlock()
			})

			It("does not evacuate on the first failure", func() {
				fakeClock.WaitForWatcherAndIncrement(checkInterval)
				Consistently(evacuatable.EvacuateCallCount, 100*time.Millisecond).Should(Equal(0))
			})

			It("evacuates after threshold consecutive failures", func() {
				// Drive one tick per polling interval so the goroutine processes
				// each before the next fires (avoids channel buffer overflow).
				Eventually(func() int {
					fakeClock.Increment(checkInterval)
					return evacuatable.EvacuateCallCount()
				}, 5*time.Second, 20*time.Millisecond).Should(Equal(1))
			})
		})

		Context("when a path check returns an error during periodic checks", func() {
			JustBeforeEach(func() {
				Eventually(process.Ready()).Should(BeClosed())
				checkPathMu.Lock()
				checkPathErr = errors.New("statfs failed")
				checkPathMu.Unlock()
			})

			It("does not evacuate on the first failure", func() {
				fakeClock.WaitForWatcherAndIncrement(checkInterval)
				Consistently(evacuatable.EvacuateCallCount, 100*time.Millisecond).Should(Equal(0))
			})

			It("evacuates after threshold consecutive failures", func() {
				Eventually(func() int {
					fakeClock.Increment(checkInterval)
					return evacuatable.EvacuateCallCount()
				}, 5*time.Second, 20*time.Millisecond).Should(Equal(1))
			})

			Context("when the error clears before the threshold is reached", func() {
				It("resets the counter and does not evacuate", func() {
					fakeClock.WaitForWatcherAndIncrement(checkInterval) // failure 1 of 3
					checkPathMu.Lock()
					checkPathErr = nil // clear — next tick resets counter
					checkPathMu.Unlock()
					fakeClock.WaitForWatcherAndIncrement(checkInterval) // healthy tick, counter → 0
					checkPathMu.Lock()
					checkPathErr = errors.New("statfs failed again")
					checkPathMu.Unlock()
					fakeClock.WaitForWatcherAndIncrement(checkInterval) // failure 1 of 3 again
					// Only 1 failure since the reset — threshold is 3, no evacuation yet.
					Consistently(evacuatable.EvacuateCallCount, 100*time.Millisecond).Should(Equal(0))
				})
			})
		})

		Context("when a path is already read-only on startup", func() {
			BeforeEach(func() {
				checkPathRO = true
			})

			It("evacuates immediately regardless of threshold", func() {
				Eventually(evacuatable.EvacuateCallCount).Should(Equal(1))
				Consistently(process.Ready()).ShouldNot(BeClosed())
			})
		})
	})
})
