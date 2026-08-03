package evacuation_test

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/clock/fakeclock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/rep/evacuation"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
)

var _ = Describe("EvacuationCleanup", func() {
	const (
		exitTimeoutOffset = 5 * time.Second
	)

	var (
		logger                   *lagertest.TestLogger
		cellID                   string
		gracefulShutdownInterval time.Duration
		proxyReloadDuration      time.Duration
		exitTimeoutInterval      time.Duration

		fakeClock          *fakeclock.FakeClock
		fakeBBSClient      *fake_bbs.FakeInternalClient
		fakeExecutorClient *fakes.FakeClient
		fakeMetronClient   *mfakes.FakeIngressClient

		cleanup        *evacuation.EvacuationCleanup
		cleanupProcess ifrit.Process

		errCh  chan error
		doneCh chan struct{}
	)

	BeforeEach(func() {
		cellID = "the-cell-id"
		logger = lagertest.NewTestLogger("cleanup")
		gracefulShutdownInterval = 20 * time.Second
		proxyReloadDuration = 10 * time.Second
		exitTimeoutInterval = gracefulShutdownInterval + proxyReloadDuration + exitTimeoutOffset

		fakeClock = fakeclock.NewFakeClock(time.Now())
		fakeBBSClient = &fake_bbs.FakeInternalClient{}
		fakeExecutorClient = &fakes.FakeClient{}
		fakeMetronClient = new(mfakes.FakeIngressClient)

		errCh = make(chan error, 1)
		doneCh = make(chan struct{})
		cleanup = evacuation.NewEvacuationCleanup(
			logger,
			cellID,
			gracefulShutdownInterval,
			proxyReloadDuration,
			fakeBBSClient,
			fakeExecutorClient,
			fakeClock,
			fakeMetronClient,
		)
	})

	JustBeforeEach(func() {
		cleanupProcess = ginkgomon.Invoke(cleanup)
		go func() {
			err := <-cleanupProcess.Wait()
			errCh <- err
			close(doneCh)
		}()
	})

	AfterEach(func() {
		cleanupProcess.Signal(os.Interrupt)
		fakeClock.Increment(exitTimeoutInterval)
		Eventually(doneCh).Should(BeClosed())
	})

	It("does not exit", func() {
		Consistently(errCh).ShouldNot(Receive())
	})

	Context("when the process is signalled", func() {
		var evacuatingActualLRP, evacuatingActualLRPWithReplacement *models.ActualLRP

		BeforeEach(func() {
			evacuatingActualLRP = model_helpers.NewValidEvacuatingActualLRP("evacuating-process-guid", 0)

			evacuatingActualLRPWithReplacementProcessGuid := "process-guid"
			evacuatingActualLRPWithReplacement = model_helpers.NewValidEvacuatingActualLRP(evacuatingActualLRPWithReplacementProcessGuid, 0)

			actualLRPs := []*models.ActualLRP{
				model_helpers.NewValidActualLRP("running-process-guid", 0),
				evacuatingActualLRP,
				evacuatingActualLRPWithReplacement,
				model_helpers.NewValidActualLRP(evacuatingActualLRPWithReplacementProcessGuid, 0),
			}

			fakeBBSClient.ActualLRPsReturns(actualLRPs, nil)

			fakeExecutorClient.ListContainersReturnsOnCall(0,
				[]executor.Container{
					{
						Guid:    "container1",
						State:   executor.StateRunning,
						RunInfo: executor.RunInfo{LogConfig: models.LogConfig{Guid: "log-guid-1", SourceName: "source-name-1", Index: 0}},
					},
					{
						Guid:    "container2",
						State:   executor.StateRunning,
						RunInfo: executor.RunInfo{LogConfig: models.LogConfig{Guid: "log-guid-2", SourceName: "source-name-2", Index: 1}},
					},
				},
				nil,
			)
			fakeExecutorClient.ListContainersReturnsOnCall(1, []executor.Container{}, nil)
		})

		JustBeforeEach(func() {
			cleanupProcess.Signal(os.Kill)
		})

		It("logs the number of stranded evacuating actual lrps", func() {
			Eventually(logger).Should(gbytes.Say("finished-evacuating.*\"stranded-evacuating-actual-lrps\":2"))
		})

		It("emits a metric for the number of stranded evacuating actual lrps", func() {
			var nilObject interface{}
			Eventually(errCh).Should(Receive(&nilObject))
			metric, value, _ := fakeMetronClient.SendMetricArgsForCall(0)
			Expect(metric).To(Equal("StrandedEvacuatingActualLRPs"))
			Expect(value).To(BeEquivalentTo(2))
		})

		Describe("deleting running containers", func() {
			Context("When containers fail to exit initially", func() {
				BeforeEach(func() {
					fakeExecutorClient.ListContainersReturnsOnCall(1,
						[]executor.Container{
							{
								Guid:    "container1",
								State:   executor.StateRunning,
								RunInfo: executor.RunInfo{LogConfig: models.LogConfig{Guid: "log-guid-1", SourceName: "source-name-1", Index: 0}},
							},
							{
								Guid:    "container2",
								State:   executor.StateCompleted,
								RunInfo: executor.RunInfo{LogConfig: models.LogConfig{Guid: "log-guid-2", SourceName: "source-name-2", Index: 1}},
							},
						},
						nil,
					)
				})
				It("should delete all of the containers that are still running", func() {
					Consistently(errCh).ShouldNot(Receive())
					fakeClock.Increment(time.Second * 1)

					var nilObject interface{}
					Eventually(errCh).Should(Receive(&nilObject))
					Expect(fakeExecutorClient.ListContainersCallCount()).To(Equal(3))
					Expect(fakeExecutorClient.DeleteContainerCallCount()).To(Equal(2))

					_, traceID, c1 := fakeExecutorClient.DeleteContainerArgsForCall(0)
					Expect(traceID).To(BeEmpty())
					_, traceID, c2 := fakeExecutorClient.DeleteContainerArgsForCall(1)
					Expect(traceID).To(BeEmpty())
					containers := []string{c1, c2}
					Expect(containers).To(ConsistOf("container1", "container2"))
				})
			})
			It("emits app logs indicating evacuation timeout", func() {
				Eventually(fakeMetronClient.SendAppLogCallCount).Should(Equal(2))
				msg, containerSource, tags := fakeMetronClient.SendAppLogArgsForCall(0)
				Expect(tags["source_id"]).To(Equal("log-guid-1"))
				Expect(containerSource).To(Equal("source-name-1"))
				Expect(tags["instance_id"]).To(Equal(strconv.Itoa(0)))
				Expect(msg).To(Equal(fmt.Sprintf("Cell %s reached evacuation timeout for instance %s", cellID, "container1")))

				msg, containerSource, tags = fakeMetronClient.SendAppLogArgsForCall(1)
				Expect(tags["source_id"]).To(Equal("log-guid-2"))
				Expect(containerSource).To(Equal("source-name-2"))
				Expect(tags["instance_id"]).To(Equal(strconv.Itoa(1)))
				Expect(msg).To(Equal(fmt.Sprintf("Cell %s reached evacuation timeout for instance %s", cellID, "container2")))
			})

			Describe("when DeleteContainer hangs", func() {
				BeforeEach(func() {
					fakeExecutorClient.DeleteContainerStub = func(lager.Logger, string, string) error {
						time.Sleep(time.Minute)
						return nil
					}
				})

				It("gives up after the graceful shutdown interval expires", func() {
					fakeClock.WaitForNWatchersAndIncrement(exitTimeoutInterval, 2)
					Eventually(doneCh).Should(BeClosed())
				})
			})

			Describe("when DeleteContainer fails", func() {
				BeforeEach(func() {
					fakeExecutorClient.DeleteContainerReturnsOnCall(0, errors.New("some-error"))
				})

				It("gives up after the graceful shutdown interval expires", func() {
					Eventually(logger).Should(gbytes.Say(".*failed-to-delete-container\".*some-error"))
					Eventually(doneCh).Should(BeClosed())
				})
			})

			Describe("when ListContainers fails the first time", func() {
				BeforeEach(func() {
					fakeExecutorClient.ListContainersStub = func(lager.Logger) ([]executor.Container, error) {
						return nil, errors.New("cannot talk to garden")
					}
				})

				It("should exit immediately", func() {
					Eventually(doneCh).Should(BeClosed())
				})

				It("should logs the error", func() {
					Eventually(logger.Buffer()).Should(gbytes.Say("cannot talk to garden"))
				})
			})

			Describe("when ListContainers fails while listing containers", func() {
				BeforeEach(func() {
					fakeExecutorClient.ListContainersStub = func(lager.Logger) ([]executor.Container, error) {
						if fakeExecutorClient.ListContainersCallCount() == 1 {
							return []executor.Container{
								{Guid: "container1", State: executor.StateRunning},
								{Guid: "container2", State: executor.StateRunning},
							}, nil
						}

						return nil, errors.New("cannot talk to garden")
					}
				})

				It("should exit immediately", func() {
					Eventually(doneCh).Should(BeClosed())
				})

				It("should logs the error", func() {
					Eventually(logger.Buffer()).Should(gbytes.Say("cannot talk to garden"))
				})
			})

			Context("when the containers do not delete in time", func() {
				BeforeEach(func() {
					fakeExecutorClient.ListContainersStub = func(lager.Logger) ([]executor.Container, error) {
						return []executor.Container{
							{Guid: "container1", State: executor.StateRunning},
							{Guid: "container2", State: executor.StateRunning},
						}, nil
					}
				})

				It("gives up after the graceful shutdown interval expires", func() {
					Eventually(fakeExecutorClient.ListContainersCallCount).Should(Equal(2))
					Expect(fakeExecutorClient.DeleteContainerCallCount()).To(Equal(2))
					Consistently(errCh).ShouldNot(Receive())

					exitTimeoutTicker := int(exitTimeoutInterval / time.Second)

					for i := 0; i < exitTimeoutTicker-1; i++ {
						fakeClock.WaitForNWatchersAndIncrement(1*time.Second, 2)
						Eventually(fakeExecutorClient.ListContainersCallCount).Should(Equal(i + 3))
					}

					Consistently(errCh).ShouldNot(Receive())
					fakeClock.WaitForNWatchersAndIncrement(1*time.Second, 2)
					Eventually(errCh).Should(Receive(HaveOccurred()))
				})
			})
		})

		Describe("when fetching the actual lrp groups fails", func() {
			BeforeEach(func() {
				fakeBBSClient.ActualLRPsReturns(nil, errors.New("failed"))
			})

			It("exits with an error", func() {
				var err error
				Eventually(errCh).Should(Receive(&err))
				Expect(err).To(Equal(errors.New("failed")))
			})
		})
	})
})
