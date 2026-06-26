package depot_test

import (
	"errors"
	"io"
	"math"
	"os"
	"time"

	bbsmodels "code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot"
	"code.cloudfoundry.org/executor/depot/containerstore/containerstorefakes"
	efakes "code.cloudfoundry.org/executor/depot/event/fakes"
	"code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/volman"
	"code.cloudfoundry.org/volman/volmanfakes"
	"code.cloudfoundry.org/workpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Depot", func() {
	var (
		depotClient         executor.Client
		logger              lager.Logger
		eventHub            *efakes.FakeHub
		gardenClient        *fakes.FakeGardenClient
		volmanClient        *volmanfakes.FakeManager
		containerStore      *containerstorefakes.FakeContainerStore
		resources           executor.ExecutorResources
		volumeDrivers       []string
		CreateWorkPoolSize  int
		DeleteWorkPoolSize  int
		ReadWorkPoolSize    int
		MetricsWorkPoolSize int
		diskPath            string
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		eventHub = new(efakes.FakeHub)
		gardenClient = new(fakes.FakeGardenClient)
		volmanClient = new(volmanfakes.FakeManager)
		containerStore = new(containerstorefakes.FakeContainerStore)

		resources = executor.ExecutorResources{
			MemoryMB:   1024,
			DiskMB:     1024,
			Containers: 3,
		}

		CreateWorkPoolSize = 5
		DeleteWorkPoolSize = 5
		ReadWorkPoolSize = 5
		MetricsWorkPoolSize = 5
		diskPath = ""
	})

	JustBeforeEach(func() {
		creationWorkPool, err := workpool.NewWorkPool(CreateWorkPoolSize)
		Expect(err).NotTo(HaveOccurred())
		deletionWorkPool, err := workpool.NewWorkPool(DeleteWorkPoolSize)
		Expect(err).NotTo(HaveOccurred())
		readWorkPool, err := workpool.NewWorkPool(ReadWorkPoolSize)
		Expect(err).NotTo(HaveOccurred())
		metricsWorkPool, err := workpool.NewWorkPool(MetricsWorkPoolSize)
		Expect(err).NotTo(HaveOccurred())

		depotClient = depot.NewClient(
			resources, diskPath, containerStore, gardenClient, volmanClient, eventHub,
			creationWorkPool, deletionWorkPool, readWorkPool, metricsWorkPool,
		)
	})

	Describe("AllocateContainers", func() {
		Context("when allocating a single valid container within executor resource limits", func() {
			var requests []executor.AllocationRequest
			BeforeEach(func() {
				requests = []executor.AllocationRequest{
					newAllocationRequest("guid-1"),
				}
			})

			It("should allocate the container", func() {
				errMessageMap := depotClient.AllocateContainers(logger, "some-trace-id", requests)
				Expect(errMessageMap).To(BeEmpty())

				Expect(containerStore.ReserveCallCount()).To(Equal(1))
				_, traceID, request := containerStore.ReserveArgsForCall(0)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(*request).To(Equal(requests[0]))
			})
		})

		Context("when allocating multiple valid containers", func() {
			var requests []executor.AllocationRequest

			BeforeEach(func() {
				requests = []executor.AllocationRequest{
					newAllocationRequest("guid-1"),
					newAllocationRequest("guid-2"),
					newAllocationRequest("guid-3"),
				}
			})

			It("should allocate all the containers", func() {
				errMessageMap := depotClient.AllocateContainers(logger, "some-trace-id", requests)
				Expect(errMessageMap).To(BeEmpty())

				Expect(containerStore.ReserveCallCount()).To(Equal(3))
				_, traceID, request := containerStore.ReserveArgsForCall(0)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(*request).To(Equal(requests[0]))

				_, traceID, request = containerStore.ReserveArgsForCall(1)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(*request).To(Equal(requests[1]))

				_, traceID, request = containerStore.ReserveArgsForCall(2)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(*request).To(Equal(requests[2]))
			})
		})

		Context("when the container store returns an error while allocating", func() {
			var requests []executor.AllocationRequest

			BeforeEach(func() {
				requests = []executor.AllocationRequest{
					newAllocationRequest("guid-1"),
					newAllocationRequest("guid-2"),
				}

				containerStore.ReserveStub = func(logger lager.Logger, traceID string, req *executor.AllocationRequest) (executor.Container, error) {
					switch req.Guid {
					case "guid-1":
						return executor.Container{}, executor.ErrContainerGuidNotAvailable
					case "guid-2":
						return executor.Container{}, nil
					default:
						return executor.Container{}, errors.New("unexpected input")
					}
				}
			})

			It("should not allocate container with duplicate guid", func() {
				failures := depotClient.AllocateContainers(logger, "some-trace-id", requests)

				Expect(failures).To(HaveLen(1))
				expectedFailure := executor.NewAllocationFailure(&requests[0], executor.ErrContainerGuidNotAvailable.Error())
				Expect(failures[0]).To(BeEquivalentTo(expectedFailure))

				Expect(containerStore.ReserveCallCount()).To(Equal(2))

				_, traceID, request := containerStore.ReserveArgsForCall(0)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(*request).To(Equal(requests[0]))
				_, traceID, request = containerStore.ReserveArgsForCall(1)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(*request).To(Equal(requests[1]))
			})
		})

		Context("when one of the containers has empty guid", func() {
			var requests []executor.AllocationRequest

			BeforeEach(func() {
				requests = []executor.AllocationRequest{
					newAllocationRequest("guid-1"),
					newAllocationRequest(""),
				}
			})

			It("should not allocate container with empty guid", func() {
				failures := depotClient.AllocateContainers(logger, "some-trace-id", requests)
				Expect(failures).To(HaveLen(1))
				expectedFailure := executor.NewAllocationFailure(&requests[1], executor.ErrGuidNotSpecified.Error())
				Expect(failures[0]).To(BeEquivalentTo(expectedFailure))

				Expect(containerStore.ReserveCallCount()).To(Equal(1))

				_, traceID, request := containerStore.ReserveArgsForCall(0)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(*request).To(Equal(requests[0]))
			})
		})
	})

	Describe("RunContainer", func() {
		var (
			containerGuid string
			runRequest    *executor.RunRequest
		)

		BeforeEach(func() {
			containerGuid = "container-guid"
			runRequest = newRunRequest(containerGuid)
		})

		Context("when the container is valid", func() {
			BeforeEach(func() {
				containerStore.InitializeReturns(nil)
			})

			It("should move the container state machine from reserved to initialize, create, and run", func() {
				err := depotClient.RunContainer(logger, "some-trace-id", runRequest)
				Expect(err).NotTo(HaveOccurred())

				Expect(containerStore.InitializeCallCount()).To(Equal(1))
				_, req := containerStore.InitializeArgsForCall(0)
				Expect(req).To(Equal(runRequest))

				Eventually(containerStore.CreateCallCount).Should(Equal(1))
				Eventually(containerStore.RunCallCount).Should(Equal(1))
				_, traceID, guid := containerStore.CreateArgsForCall(0)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(guid).To(Equal(containerGuid))

				_, traceID, guid = containerStore.RunArgsForCall(0)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(guid).To(Equal(containerGuid))
			})
		})

		Context("when the container fails to initialize", func() {
			BeforeEach(func() {
				containerStore.InitializeReturns(executor.ErrContainerNotFound)
			})

			It("should return error", func() {
				err := depotClient.RunContainer(logger, "some-trace-id", newRunRequest("missing-guid"))
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})

		Context("when creating the container fails", func() {
			BeforeEach(func() {
				containerStore.InitializeReturns(nil)
				containerStore.CreateReturns(executor.Container{}, errors.New("some-error"))
			})

			It("returns an error", func() {
				err := depotClient.RunContainer(logger, "some-trace-id", newRunRequest(containerGuid))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when running the container fails", func() {
			BeforeEach(func() {
				containerStore.RunReturns(errors.New("some-error"))
			})

			It("should log the error", func() {
				err := depotClient.RunContainer(logger, "some-trace-id", newRunRequest(containerGuid))
				Expect(err).NotTo(HaveOccurred())

				Eventually(containerStore.RunCallCount).Should(Equal(1))
				Expect(logger).To(gbytes.Say("run-container.failed-running-container-in-garden"))
			})
		})
	})

	Describe("Throttling", func() {
		var (
			numRequests   int
			containerGuid = "garden-store-guid"
		)

		BeforeEach(func() {
			numRequests = 10
			resources = executor.ExecutorResources{
				MemoryMB:   1024,
				DiskMB:     1024,
				Containers: 10,
			}

			CreateWorkPoolSize = 2
			DeleteWorkPoolSize = 6
			ReadWorkPoolSize = 4
			MetricsWorkPoolSize = 5
		})

		Context("Container creation", func() {
			var (
				throttleChan chan struct{}
				doneChan     chan struct{}
			)

			BeforeEach(func() {
				throttleChan = make(chan struct{}, numRequests)
				doneChan = make(chan struct{})

				containerStore.CreateStub = func(logger lager.Logger, traceID string, guid string) (executor.Container, error) {
					throttleChan <- struct{}{}
					<-doneChan
					return executor.Container{}, nil
				}
			})

			It("throttles the requests to Garden", func() {
				for i := 0; i < numRequests; i++ {
					go depotClient.RunContainer(logger, "some-trace-id", newRunRequest(containerGuid))
				}

				Eventually(containerStore.CreateCallCount).Should(Equal(CreateWorkPoolSize))
				Consistently(containerStore.CreateCallCount).Should(Equal(CreateWorkPoolSize))

				Eventually(func() int {
					return len(throttleChan)
				}).Should(Equal(CreateWorkPoolSize))
				Consistently(func() int {
					return len(throttleChan)
				}).Should(Equal(CreateWorkPoolSize))

				doneChan <- struct{}{}

				Eventually(containerStore.CreateCallCount).Should(Equal(CreateWorkPoolSize + 1))
				Consistently(containerStore.CreateCallCount).Should(Equal(CreateWorkPoolSize + 1))

				close(doneChan)
				Eventually(containerStore.CreateCallCount).Should(Equal(numRequests))
			})
		})

		Context("Container Deletion", func() {
			var (
				throttleChan chan struct{}
				doneChan     chan struct{}
			)

			BeforeEach(func() {
				throttleChan = make(chan struct{}, numRequests)
				doneChan = make(chan struct{})
				containerStore.DestroyStub = func(logger lager.Logger, traceID string, guid string) error {
					throttleChan <- struct{}{}
					<-doneChan
					return nil
				}
				containerStore.StopStub = func(logger lager.Logger, traceID string, guid string) error {
					throttleChan <- struct{}{}
					<-doneChan
					return nil
				}
			})

			It("throttles the requests to Garden", func() {
				deleteContainerCount := 0
				for i := 0; i < numRequests; i++ {
					deleteContainerCount++
					go depotClient.DeleteContainer(logger, "some-trace-id", containerGuid)
				}

				Eventually(func() int {
					return len(throttleChan)
				}).Should(Equal(DeleteWorkPoolSize))

				Consistently(func() int {
					return len(throttleChan)
				}).Should(Equal(DeleteWorkPoolSize))

				doneChan <- struct{}{}

				Eventually(func() int {
					return containerStore.StopCallCount() + containerStore.DestroyCallCount()
				}).Should(Equal(DeleteWorkPoolSize + 1))

				close(doneChan)

				Eventually(containerStore.DestroyCallCount).Should(Equal(deleteContainerCount))
			})
		})

		Context("Retrieves containers", func() {
			var (
				throttleChan chan struct{}
				doneChan     chan struct{}
			)

			BeforeEach(func() {
				throttleChan = make(chan struct{}, numRequests)
				doneChan = make(chan struct{})
				containerStore.GetFilesStub = func(logger lager.Logger, guid string, sourcePath string) (io.ReadCloser, error) {
					throttleChan <- struct{}{}
					<-doneChan
					return nil, nil
				}
				containerStore.ListStub = func(logger lager.Logger) []executor.Container {
					throttleChan <- struct{}{}
					<-doneChan
					return []executor.Container{executor.Container{}}
				}
			})

			It("throttles the requests to Garden", func() {
				getFilesCount := 0
				for i := 0; i < numRequests; i++ {
					getFilesCount++
					go depotClient.GetFiles(logger, containerGuid, "/some/path")
				}

				Eventually(throttleChan).Should(HaveLen(ReadWorkPoolSize))
				Consistently(throttleChan).Should(HaveLen(ReadWorkPoolSize))

				doneChan <- struct{}{}

				Eventually(func() int {
					return containerStore.ListCallCount() + containerStore.GetFilesCallCount()
				}).Should(Equal(ReadWorkPoolSize + 1))

				close(doneChan)

				Eventually(containerStore.GetFilesCallCount).Should(Equal(getFilesCount))
			})
		})

		Context("Metrics", func() {
			var (
				throttleChan chan struct{}
				doneChan     chan struct{}
			)

			BeforeEach(func() {
				throttleChan = make(chan struct{}, numRequests)
				doneChan = make(chan struct{})
				containerStore.MetricsStub = func(logger lager.Logger) (map[string]executor.ContainerMetrics, error) {
					throttleChan <- struct{}{}
					<-doneChan
					return map[string]executor.ContainerMetrics{
						"some-guid": executor.ContainerMetrics{},
					}, nil
				}

				containerStore.ListReturns([]executor.Container{executor.Container{Guid: "some-guid"}})
			})

			It("throttles the requests to Garden", func() {
				for i := 0; i < numRequests; i++ {
					go depotClient.GetBulkMetrics(logger)
				}

				Eventually(func() int {
					return len(throttleChan)
				}).Should(Equal(MetricsWorkPoolSize))
				Consistently(func() int {
					return len(throttleChan)
				}).Should(Equal(MetricsWorkPoolSize))

				doneChan <- struct{}{}
				Eventually(containerStore.MetricsCallCount).Should(Equal(MetricsWorkPoolSize + 1))
				close(doneChan)
				Eventually(containerStore.MetricsCallCount).Should(Equal(numRequests))
			})
		})
	})

	Describe("ListContainers", func() {
		var containers []executor.Container
		BeforeEach(func() {
			containers = []executor.Container{
				{Guid: "guid-1"},
				{Guid: "guid-2"},
			}

			containerStore.ListReturns(containers)
		})

		It("lists the containers in the container store", func() {
			returnedContainers, err := depotClient.ListContainers(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedContainers).To(Equal(containers))
			Expect(containerStore.ListCallCount()).To(Equal(1))
		})
	})

	Describe("GetBulkMetrics", func() {
		var metrics map[string]executor.Metrics
		var metricsErr error

		var expectedMetrics map[string]executor.ContainerMetrics

		BeforeEach(func() {
			expectedMetrics = map[string]executor.ContainerMetrics{
				"a-guid": executor.ContainerMetrics{
					MemoryUsageInBytes: 123,
					DiskUsageInBytes:   456,
					TimeSpentInCPU:     100 * time.Second,
				},
				"b-guid": executor.ContainerMetrics{
					MemoryUsageInBytes: 321,
					DiskUsageInBytes:   654,
					TimeSpentInCPU:     100 * time.Second,
				},
			}

			containerStore.MetricsReturns(expectedMetrics, nil)
		})

		JustBeforeEach(func() {
			metrics, metricsErr = depotClient.GetBulkMetrics(logger)
		})

		Context("with no tags", func() {
			BeforeEach(func() {
				containerStore.ListReturns([]executor.Container{
					executor.Container{Guid: "a-guid", RunInfo: executor.RunInfo{MetricsConfig: executor.MetricsConfig{Guid: "a-metrics"}}},
					executor.Container{Guid: "b-guid", RunInfo: executor.RunInfo{MetricsConfig: executor.MetricsConfig{Guid: "b-metrics", Index: 1}}},
				})
			})

			It("gets all the containers", func() {
				Expect(containerStore.ListCallCount()).To(Equal(1))
			})

			It("retrieves all the metrics", func() {
				Expect(containerStore.MetricsCallCount()).To(Equal(1))
			})

			It("does not error", func() {
				Expect(metricsErr).NotTo(HaveOccurred())
			})

			It("returns all the metrics", func() {
				Expect(metrics).To(HaveLen(2))
				Expect(metrics["a-guid"]).To(Equal(executor.Metrics{
					MetricsConfig:    executor.MetricsConfig{Guid: "a-metrics"},
					ContainerMetrics: expectedMetrics["a-guid"],
				}))

				Expect(metrics["b-guid"]).To(Equal(executor.Metrics{
					MetricsConfig:    executor.MetricsConfig{Guid: "b-metrics", Index: 1},
					ContainerMetrics: expectedMetrics["b-guid"],
				}))
			})
		})

		Context("containers with missing metric guids", func() {
			BeforeEach(func() {
				containerStore.ListReturns([]executor.Container{
					executor.Container{Guid: "a-guid"},
					executor.Container{Guid: "b-guid", RunInfo: executor.RunInfo{MetricsConfig: executor.MetricsConfig{Guid: "b-metrics", Index: 1}}},
				})
			})

			It("does not error", func() {
				Expect(metricsErr).NotTo(HaveOccurred())
			})

			It("returns all metrics", func() {
				Expect(metrics).To(HaveLen(2))
				Expect(metrics["a-guid"]).To(Equal(executor.Metrics{
					MetricsConfig:    executor.MetricsConfig{},
					ContainerMetrics: expectedMetrics["a-guid"],
				}))
				Expect(metrics["b-guid"]).To(Equal(executor.Metrics{
					MetricsConfig:    executor.MetricsConfig{Guid: "b-metrics", Index: 1},
					ContainerMetrics: expectedMetrics["b-guid"],
				}))
			})
		})

		Context("when garden fails to get the metrics", func() {
			var expectedError error

			BeforeEach(func() {
				expectedError = errors.New("whoops")
				containerStore.MetricsReturns(nil, expectedError)
			})

			It("propagates the error", func() {
				Expect(metricsErr).To(Equal(expectedError))
			})
		})
	})

	Describe("DeleteContainer", func() {
		It("removes the container from the container store", func() {
			err := depotClient.DeleteContainer(logger, "some-trace-id", "guid-1")
			Expect(err).NotTo(HaveOccurred())

			Expect(containerStore.DestroyCallCount()).To(Equal(1))
			_, traceID, guid := containerStore.DestroyArgsForCall(0)
			Expect(traceID).To(Equal("some-trace-id"))
			Expect(guid).To(Equal("guid-1"))
		})

		Context("when garden store returns an error", func() {
			BeforeEach(func() {
				containerStore.DestroyReturns(errors.New("some-error"))
			})

			It("should return an error", func() {
				Expect(containerStore.DestroyCallCount()).To(Equal(0))
				err := depotClient.DeleteContainer(logger, "some-trace-id", "guid-1")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("UpdateContainer", func() {
		var updateError error
		var updateRequest *executor.UpdateRequest

		BeforeEach(func() {
			irs := bbsmodels.InternalRoutes{
				{Hostname: "a.apps.internal"},
				{Hostname: "b.apps.internal"},
			}
			metricTags := map[string]string{"some-tag": "some-value"}
			updateRequest = newUpdateRequest("a-guid", irs, metricTags)
		})

		JustBeforeEach(func() {
			updateError = depotClient.UpdateContainer(logger, updateRequest)
		})

		It("updates the container in the container store", func() {
			Expect(updateError).NotTo(HaveOccurred())
			Expect(containerStore.UpdateCallCount()).To(Equal(1))
			_, ur := containerStore.UpdateArgsForCall(0)
			Expect(ur).To(Equal(updateRequest))
		})

		Context("when the container store fails to update the container", func() {
			BeforeEach(func() {
				containerStore.UpdateReturns(errors.New("boom!"))
			})

			It("returns the error", func() {
				Expect(updateError).To(Equal(errors.New("boom!")))
			})
		})
	})

	Describe("StopContainer", func() {
		var stopError error
		var stopGuid string

		BeforeEach(func() {
			stopGuid = "some-guid"
		})

		JustBeforeEach(func() {
			stopError = depotClient.StopContainer(logger, "some-trace-id", stopGuid)
		})

		It("stops the container in the container store", func() {
			Expect(stopError).NotTo(HaveOccurred())
			Expect(containerStore.StopCallCount()).To(Equal(1))
			_, traceID, guid := containerStore.StopArgsForCall(0)
			Expect(traceID).To(Equal("some-trace-id"))
			Expect(guid).To(Equal(stopGuid))
		})

		Context("when the container store fails to stop the container", func() {
			BeforeEach(func() {
				containerStore.StopReturns(errors.New("boom!"))
			})

			It("returns the error", func() {
				Expect(stopError).To(Equal(errors.New("boom!")))
			})
		})
	})

	Describe("GetContainer", func() {
		var container executor.Container

		BeforeEach(func() {
			container = executor.Container{Guid: "the-container-guid"}
			containerStore.GetReturns(container, nil)
		})

		It("retrieves the container from the container store", func() {
			fetchedContainer, err := depotClient.GetContainer(logger, "the-container-guid")
			Expect(err).NotTo(HaveOccurred())
			Expect(fetchedContainer).To(Equal(container))

			Expect(containerStore.GetCallCount()).To(Equal(1))
			_, guid := containerStore.GetArgsForCall(0)
			Expect(guid).To(Equal("the-container-guid"))
		})

		Context("when fetching the container from the container store fails", func() {
			BeforeEach(func() {
				containerStore.GetReturns(executor.Container{}, errors.New("failed-to-get-container"))
			})

			It("returns the error", func() {
				_, err := depotClient.GetContainer(logger, "any-guid")
				Expect(err).To(Equal(errors.New("failed-to-get-container")))
			})
		})
	})

	Describe("RemainingResources", func() {
		var remainingResources executor.ExecutorResources

		BeforeEach(func() {
			remainingResources = executor.NewExecutorResources(1024, 1024, 3)
			containerStore.RemainingResourcesReturns(remainingResources)
		})

		It("should reduce resources used by allocated and running containers", func() {
			actualResources, err := depotClient.RemainingResources(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualResources).To(Equal(remainingResources))
		})

		Context("when disk path is configured and partition has less space than the store reports", func() {
			var tmpDir string

			BeforeEach(func() {
				var err error
				tmpDir, err = os.MkdirTemp("", "depot-remaining-disk-test")
				Expect(err).NotTo(HaveOccurred())
				diskPath = tmpDir
				containerStore.RemainingResourcesReturns(executor.NewExecutorResources(1024, math.MaxInt32, 3))
			})

			It("caps remaining disk at the live partition free space from syscall.Statfs", func() {
				result, err := depotClient.RemainingResources(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.DiskMB).NotTo(Equal(math.MaxInt32))
				Expect(result.DiskMB).To(BeNumerically(">", 0))
			})
		})
	})

	Describe("TotalResources", func() {
		Context("when no disk path is configured", func() {
			It("returns the static startup resources", func() {
				result, err := depotClient.TotalResources(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(resources))
			})
		})

		Context("when disk path is configured", func() {
			var tmpDir string

			BeforeEach(func() {
				var err error
				tmpDir, err = os.MkdirTemp("", "depot-total-disk-test")
				Expect(err).NotTo(HaveOccurred())
				diskPath = tmpDir
				resources.DiskMB = math.MaxInt32
			})

			AfterEach(func() {
				os.RemoveAll(tmpDir)
			})

			It("returns the resources it was configured with, with live disk capacity", func() {
				result, err := depotClient.TotalResources(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.MemoryMB).To(Equal(resources.MemoryMB))
				Expect(result.Containers).To(Equal(resources.Containers))
				Expect(result.DiskMB).NotTo(Equal(math.MaxInt32))
				Expect(result.DiskMB).To(BeNumerically(">", 0))
			})
		})
	})

	Describe("VolumeDrivers", func() {
		Context("when getting volume drivers succeeds", func() {
			BeforeEach(func() {
				volmanClient.ListDriversReturns(volman.ListDriversResponse{Drivers: []volman.InfoResponse{
					{Name: "ayrton"},
					{Name: "damon"},
					{Name: "michael"},
				}}, nil)
				volumeDrivers = []string{"ayrton", "damon", "michael"}
			})

			It("should return the list of volume drivers", func() {
				actualDrivers, err := depotClient.VolumeDrivers(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualDrivers).To(ConsistOf(volumeDrivers))
			})
		})

		Context("when getting volume drivers fails", func() {
			BeforeEach(func() {
				volmanClient.ListDriversReturns(volman.ListDriversResponse{}, errors.New("the wheels fell off"))
			})

			It("returns an error", func() {
				_, err := depotClient.VolumeDrivers(logger)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

func newAllocationRequest(guid string, tagses ...executor.Tags) executor.AllocationRequest {
	resource := executor.NewResource(256, 256, -1)
	var tags executor.Tags
	if len(tagses) > 0 {
		tags = tagses[0]
	}
	return executor.NewAllocationRequest(guid, &resource, false, tags)
}

func newRunRequest(guid string) *executor.RunRequest {
	runInfo := executor.RunInfo{}
	r := executor.NewRunRequest(guid, &runInfo, nil)
	return &r
}

func newUpdateRequest(guid string, internalRoutes bbsmodels.InternalRoutes, metricTags map[string]string) *executor.UpdateRequest {
	r := executor.NewUpdateRequest(guid, internalRoutes, metricTags)
	return &r
}
