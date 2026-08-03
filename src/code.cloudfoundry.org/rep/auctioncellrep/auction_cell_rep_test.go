package auctioncellrep_test

import (
	"errors"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/containermetrics"
	fake_client "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/auctioncellrep"
	fakes "code.cloudfoundry.org/rep/auctioncellrep/auctioncellrepfakes"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context/fake_evacuation_context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	repURL     = "https://foo.cell.service.cf.internal:8888"
	cellID     = "some-cell-id"
	cellIndex  = 15
	linuxStack = "linux"
	linuxPath  = "/data/rootfs/linux"
)

var _ = Describe("AuctionCellRep", func() {
	var (
		cellRep                      *auctioncellrep.AuctionCellRep
		client                       *fake_client.FakeClient
		logger                       *lagertest.TestLogger
		evacuationReporter           *fake_evacuation_context.FakeEvacuationReporter
		fakeContainerMetricsProvider *fakes.FakeContainerMetricsProvider

		linuxRootFSURL string
		commonErr      error

		placementTags, optionalPlacementTags []string
		enableContainerProxy                 bool
		proxyMemoryAllocation                int

		fakeContainerAllocator *fakes.FakeBatchContainerAllocator
	)

	BeforeEach(func() {
		client = new(fake_client.FakeClient)
		logger = lagertest.NewTestLogger("test")
		evacuationReporter = &fake_evacuation_context.FakeEvacuationReporter{}
		fakeContainerMetricsProvider = new(fakes.FakeContainerMetricsProvider)
		fakeContainerAllocator = new(fakes.FakeBatchContainerAllocator)

		linuxRootFSURL = models.PreloadedRootFS(linuxStack)

		commonErr = errors.New("Failed to fetch")
		enableContainerProxy = false
		proxyMemoryAllocation = 12
		client.HealthyReturns(true)
	})

	JustBeforeEach(func() {
		cellRep = auctioncellrep.New(
			cellID,
			cellIndex,
			repURL,
			rep.StackPathMap{linuxStack: linuxPath},
			fakeContainerMetricsProvider,
			[]string{"docker"},
			"the-zone",
			client,
			evacuationReporter,
			placementTags,
			optionalPlacementTags,
			proxyMemoryAllocation,
			enableContainerProxy,
			fakeContainerAllocator,
		)
	})

	Describe("Metrics", func() {
		var (
			// 	containers []executor.Container
			metrics *rep.ContainerMetricsCollection
		)

		JustBeforeEach(func() {
			// client.ListContainersReturns(containers, nil)
			var err error
			metrics, err = cellRep.Metrics(logger)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the rep has no containers", func() {
			It("should return the cell-id and empty lrp and task metrics", func() {
				Expect(metrics.CellID).To(Equal(cellID))
				Expect(metrics.LRPs).To(BeEmpty())
				Expect(metrics.Tasks).To(BeEmpty())
			})
		})

		Context("when the rep has an lrp container", func() {
			var metricValues containermetrics.CachedContainerMetrics
			BeforeEach(func() {
				metricValues = containermetrics.CachedContainerMetrics{
					MetricGUID:       "some-metric-guid",
					CPUUsageFraction: 0.8,
					DiskUsageBytes:   10,
					DiskQuotaBytes:   20,
					MemoryUsageBytes: 5,
					MemoryQuotaBytes: 10,
				}
				container := createContainer(executor.StateRunning, rep.LRPLifecycle)
				client.ListContainersReturns([]executor.Container{container}, nil)
				fakeContainerMetricsProvider.MetricsReturns(map[string]*containermetrics.CachedContainerMetrics{
					"some-container-guid": &metricValues,
				})
			})

			It("should return metrics for the running container", func() {
				Expect(metrics.LRPs).To(HaveLen(1))
				Expect(metrics.Tasks).To(BeEmpty())

				lrpMetrics := metrics.LRPs[0]
				Expect(lrpMetrics.ProcessGUID).To(Equal("some-process-guid"))
				Expect(lrpMetrics.InstanceGUID).To(Equal("some-instance-guid"))
				Expect(lrpMetrics.Index).To(Equal(int32(1)))
				Expect(lrpMetrics.CachedContainerMetrics).To(Equal(metricValues))
			})
		})

		Context("when the rep has a task container", func() {
			var metricValues containermetrics.CachedContainerMetrics
			BeforeEach(func() {
				metricValues = containermetrics.CachedContainerMetrics{
					MetricGUID:       "some-metric-guid",
					CPUUsageFraction: 0.8,
					DiskUsageBytes:   10,
					DiskQuotaBytes:   20,
					MemoryUsageBytes: 5,
					MemoryQuotaBytes: 10,
				}
				container := createContainer(executor.StateRunning, rep.TaskLifecycle)
				client.ListContainersReturns([]executor.Container{container}, nil)
				fakeContainerMetricsProvider.MetricsReturns(map[string]*containermetrics.CachedContainerMetrics{
					"some-container-guid": &metricValues,
				})
			})

			It("should return metrics for the running container", func() {
				Expect(metrics.LRPs).To(BeEmpty())
				Expect(metrics.Tasks).To(HaveLen(1))

				taskMetrics := metrics.Tasks[0]
				Expect(taskMetrics.TaskGUID).To(Equal("some-container-guid"))
				Expect(taskMetrics.CachedContainerMetrics).To(Equal(metricValues))
			})
		})
	})

	Describe("State", func() {
		var (
			containers []executor.Container
		)

		Context("when the rep has a container", func() {
			var (
				state models.CellState
			)

			JustBeforeEach(func() {
				client.ListContainersReturns(containers, nil)
				var healthy bool
				var err error
				state, healthy, err = cellRep.State(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(healthy).To(BeTrue())
			})

			Context("with TaskLifecycle", func() {
				createTaskContainer := func(state executor.State) executor.Container {
					return createContainer(state, rep.TaskLifecycle)
				}

				Context("in Reserved state", func() {
					BeforeEach(func() {
						container := createTaskContainer(executor.StateReserved)
						containers = []executor.Container{container}
					})

					It("returns a running Task", func() {
						Expect(state.Tasks).To(HaveLen(1))
						Expect(state.Tasks[0].State).To(Equal(models.Task_Running))
					})
				})

				Context("in Running state", func() {
					BeforeEach(func() {
						container := createTaskContainer(executor.StateRunning)
						containers = []executor.Container{container}
					})

					It("returns a running Task", func() {
						Expect(state.Tasks).To(HaveLen(1))
						Expect(state.Tasks[0].State).To(Equal(models.Task_Running))
					})
				})

				Context("in Completed state", func() {
					BeforeEach(func() {
						container := createTaskContainer(executor.StateCompleted)
						containers = []executor.Container{container}
					})

					It("returns a completed Task", func() {
						Expect(state.Tasks).To(HaveLen(1))
						Expect(state.Tasks[0].State).To(Equal(models.Task_Completed))
					})

					Context("and the Failed flag is set", func() {
						BeforeEach(func() {
							containers[0].RunResult.Failed = true
						})

						It("returns a Failed Task", func() {
							Expect(state.Tasks).To(HaveLen(1))
							Expect(state.Tasks[0].State).To(Equal(models.Task_Completed))
							Expect(state.Tasks[0].Failed).To(BeTrue())
						})
					})
				})
			})

			Context("with LRPLifecycle", func() {
				createLRPContainer := func(state executor.State) executor.Container {
					return createContainer(state, rep.LRPLifecycle)
				}

				Context("in Reserved state", func() {
					BeforeEach(func() {
						containers = []executor.Container{createLRPContainer(executor.StateReserved)}
					})

					It("returns a claimed LRP", func() {
						Expect(state.LRPs).To(HaveLen(1))
						Expect(state.LRPs[0].State).To(Equal(models.ActualLRPStateClaimed))

						Expect(state.StartingContainerCount).To(Equal(1))
					})
				})

				Context("in Initializing state", func() {
					BeforeEach(func() {
						containers = []executor.Container{createLRPContainer(executor.StateInitializing)}
					})

					It("returns a claimed LRP", func() {
						Expect(state.LRPs).To(HaveLen(1))
						Expect(state.LRPs[0].State).To(Equal(models.ActualLRPStateClaimed))

						Expect(state.StartingContainerCount).To(Equal(1))
					})
				})

				Context("in Created state", func() {
					BeforeEach(func() {
						containers = []executor.Container{createLRPContainer(executor.StateCreated)}
					})

					It("returns a Claimed LRP", func() {
						Expect(state.LRPs).To(HaveLen(1))
						Expect(state.LRPs[0].State).To(Equal(models.ActualLRPStateClaimed))

						Expect(state.StartingContainerCount).To(Equal(1))
					})
				})

				Context("in Completed state", func() {
					BeforeEach(func() {
						containers = []executor.Container{createLRPContainer(executor.StateCompleted)}
					})

					It("returns a Running LRP", func() {
						Expect(state.LRPs).To(HaveLen(1))
						Expect(state.LRPs[0].State).To(Equal("SHUTDOWN"))

						Expect(state.StartingContainerCount).To(BeZero())
					})

					Context("and the LRP has crashed", func() {
						BeforeEach(func() {
							containers[0].RunResult.Failed = true
						})

						It("returns a Running LRP", func() {
							Expect(state.LRPs).To(HaveLen(1))
							Expect(state.LRPs[0].State).To(Equal("CRASHED"))

							Expect(state.StartingContainerCount).To(BeZero())
						})
					})
				})

				Context("in Running state", func() {
					BeforeEach(func() {
						containers = []executor.Container{createLRPContainer(executor.StateRunning)}
					})

					It("returns a Running LRP", func() {
						Expect(state.LRPs).To(HaveLen(1))
						Expect(state.LRPs[0].State).To(Equal(models.ActualLRPStateRunning))

						Expect(state.StartingContainerCount).To(BeZero())
					})

					Context("returns the right index", func() {
						BeforeEach(func() {
							containers[0].Tags[rep.ProcessIndexTag] = "100"
						})

						It("returns the right index", func() {
							Expect(state.LRPs).To(HaveLen(1))
							Expect(state.LRPs[0].Index).To(BeNumerically("==", 100))
						})
					})

					Context("with a different domain", func() {
						BeforeEach(func() {
							containers[0].Tags[rep.DomainTag] = "random-domain"
						})

						It("returns the right index", func() {
							Expect(state.LRPs).To(HaveLen(1))
							Expect(state.LRPs[0].Domain).To(Equal("random-domain"))
						})
					})

					Context("with a process guid", func() {
						BeforeEach(func() {
							containers[0].Tags[rep.ProcessGuidTag] = "random-guid"
						})

						It("returns the right index", func() {
							Expect(state.LRPs).To(HaveLen(1))
							Expect(state.LRPs[0].ProcessGuid).To(Equal("random-guid"))
						})
					})

					Context("with different rootfs", func() {
						BeforeEach(func() {
							containers[0].RootFSPath = "docker://cloudfoundry/grace"
						})

						It("returns the right index", func() {
							Expect(state.LRPs).To(HaveLen(1))
							Expect(state.LRPs[0].RootFs).To(Equal("docker://cloudfoundry/grace"))
						})
					})

					Context("with placement tags", func() {
						BeforeEach(func() {
							containers[0].Tags[rep.PlacementTagsTag] = `["random-placement-tag"]`
						})

						It("returns the right index", func() {
							Expect(state.LRPs).To(HaveLen(1))
							Expect(state.LRPs[0].PlacementTags).To(ConsistOf([]string{"random-placement-tag"}))
						})
					})

					Context("with volume drivers", func() {
						BeforeEach(func() {
							containers[0].Tags[rep.VolumeDriversTag] = `["random-volume-driver"]`
						})

						It("returns the right index", func() {
							Expect(state.LRPs).To(HaveLen(1))
							Expect(state.LRPs[0].VolumeDrivers).To(ConsistOf([]string{"random-volume-driver"}))
						})
					})

					Context("with different resource usage", func() {
						BeforeEach(func() {
							containers[0].Resource = executor.Resource{
								MemoryMB: 2048,
								DiskMB:   4096,
								MaxPids:  10,
							}
						})

						It("returns the right index", func() {
							Expect(state.LRPs).To(HaveLen(1))
							Expect(state.LRPs[0].Resource).To(Equal(models.Resource{
								MemoryMB: 2048,
								DiskMB:   4096,
								MaxPids:  10,
							}))
						})
					})
				})
			})
		})

		It("queries the client and returns state", func() {
			evacuationReporter.EvacuatingReturns(true)
			totalResources := executor.ExecutorResources{
				MemoryMB:   1024,
				DiskMB:     2048,
				Containers: 4,
			}

			availableResources := executor.ExecutorResources{
				MemoryMB:   512,
				DiskMB:     256,
				Containers: 2,
			}

			volumeDrivers := []string{"lewis", "nico", "sebastian", "felipe"}

			client.TotalResourcesReturns(totalResources, nil)
			client.RemainingResourcesReturns(availableResources, nil)
			client.ListContainersReturns(containers, nil)
			client.VolumeDriversReturns(volumeDrivers, nil)

			state, healthy, err := cellRep.State(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(healthy).To(BeTrue())

			Expect(state.CellID).To(Equal(cellID))
			Expect(state.CellIndex).To(Equal(cellIndex))
			Expect(state.RepURL).To(Equal(repURL))

			Expect(state.Evacuating).To(BeTrue())
			Expect(state.RootFSProviders).To(Equal(models.RootFSProviders{
				models.PreloadedRootFSScheme:    models.NewFixedSetRootFSProvider("linux"),
				models.PreloadedOCIRootFSScheme: models.NewFixedSetRootFSProvider("linux"),
				"docker":                        models.ArbitraryRootFSProvider{},
			}))

			Expect(state.AvailableResources).To(Equal(models.Resources{
				MemoryMB:   int32(availableResources.MemoryMB),
				DiskMB:     int32(availableResources.DiskMB),
				Containers: availableResources.Containers,
			}))

			Expect(state.TotalResources).To(Equal(models.Resources{
				MemoryMB:   int32(totalResources.MemoryMB),
				DiskMB:     int32(totalResources.DiskMB),
				Containers: totalResources.Containers,
			}))

			Expect(state.VolumeDrivers).To(ConsistOf(volumeDrivers))
			Expect(state.ProxyMemoryAllocationMB).To(Equal(0))
		})

		Context("when enableContainerProxy is true", func() {
			BeforeEach(func() {
				enableContainerProxy = true
			})

			It("returns a state with a proxyMemoryAllocation greater than 0", func() {
				state, _, err := cellRep.State(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(state.ProxyMemoryAllocationMB).To(Equal(proxyMemoryAllocation))
			})
		})

		Context("when the cell is not healthy", func() {
			BeforeEach(func() {
				client.HealthyReturns(false)
			})

			It("errors when reporting state", func() {
				_, healthy, err := cellRep.State(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(healthy).To(BeFalse())
			})
		})

		Context("when the client fails to fetch total resources", func() {
			BeforeEach(func() {
				client.TotalResourcesReturns(executor.ExecutorResources{}, commonErr)
			})

			It("should return an error and no state", func() {
				_, _, err := cellRep.State(logger)
				Expect(err).To(MatchError(commonErr))
			})
		})

		Context("when the client fails to fetch available resources", func() {
			BeforeEach(func() {
				client.RemainingResourcesReturns(executor.ExecutorResources{}, commonErr)
			})

			It("should return an error and no state", func() {
				_, _, err := cellRep.State(logger)
				Expect(err).To(MatchError(commonErr))
			})
		})

		Context("when the client fails to list containers", func() {
			BeforeEach(func() {
				client.ListContainersReturns(nil, commonErr)
			})

			It("should return an error and no state", func() {
				_, _, err := cellRep.State(logger)
				Expect(err).To(MatchError(commonErr))
			})
		})

		Context("when placement tags have been set", func() {
			BeforeEach(func() {
				placementTags = []string{"quack", "oink"}
			})

			It("returns the tags as part of the state", func() {
				state, healthy, err := cellRep.State(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(healthy).To(BeTrue())
				Expect(state.PlacementTags).To(ConsistOf(placementTags))
			})
		})

		Context("when optional placement tags have been set", func() {
			BeforeEach(func() {
				optionalPlacementTags = []string{"baa", "cluck"}
			})

			It("returns the tags as part of the state", func() {
				state, healthy, err := cellRep.State(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(healthy).To(BeTrue())
				Expect(state.OptionalPlacementTags).To(ConsistOf(optionalPlacementTags))
			})
		})
	})

	Describe("Perform", func() {
		var (
			remainingCellMemory int

			lrpAuctionOne, lrpAuctionTwo, lrpAuctionThree models.SchedulingLRP
			lrpAuctions                                   []models.SchedulingLRP
			work                                          models.Work

			successfulLRP, unsuccessfulLRP   models.SchedulingLRP
			successfulTask, unsuccessfulTask models.SchedulingTask
		)

		BeforeEach(func() {
			remainingCellMemory = 8192

			successfulLRP = models.NewSchedulingLRP(
				"ig-1",
				models.ActualLRPKey{
					ProcessGuid: "process-guid",
					Index:       0,
					Domain:      "domain",
				},
				models.Resource{},
				models.PlacementConstraint{},
			)

			unsuccessfulLRP = models.NewSchedulingLRP(
				"ig-2",
				models.ActualLRPKey{
					ProcessGuid: "process-guid",
					Index:       1,
					Domain:      "domain",
				},
				models.Resource{},
				models.PlacementConstraint{},
			)

			successfulTask = models.NewSchedulingTask(
				"ig-1",
				"domain",
				models.Resource{},
				models.PlacementConstraint{},
			)

			unsuccessfulTask = models.NewSchedulingTask(
				"ig-2",
				"domain",
				models.Resource{},
				models.PlacementConstraint{},
			)
		})

		JustBeforeEach(func() {
			client.RemainingResourcesReturns(executor.ExecutorResources{MemoryMB: remainingCellMemory}, nil)
			lrpAuctions = []models.SchedulingLRP{lrpAuctionOne, lrpAuctionTwo, lrpAuctionThree}
		})

		It("requests container allocation for all provided LRPs and Tasks", func() {
			fakeContainerAllocator.BatchLRPAllocationRequestReturns([]models.SchedulingLRP{unsuccessfulLRP})
			fakeContainerAllocator.BatchTaskAllocationRequestReturns([]models.SchedulingTask{unsuccessfulTask})

			cellRep.Perform(logger, "some-trace-id", models.Work{
				LRPs:  []models.SchedulingLRP{successfulLRP, unsuccessfulLRP},
				Tasks: []models.SchedulingTask{successfulTask, unsuccessfulTask},
			})

			Expect(fakeContainerAllocator.BatchLRPAllocationRequestCallCount()).To(Equal(1))
			_, traceID, _, _, lrpRequests := fakeContainerAllocator.BatchLRPAllocationRequestArgsForCall(0)
			Expect(traceID).To(Equal("some-trace-id"))
			Expect(lrpRequests).To(ConsistOf(successfulLRP, unsuccessfulLRP))

			Expect(fakeContainerAllocator.BatchTaskAllocationRequestCallCount()).To(Equal(1))
			_, traceID, taskRequests := fakeContainerAllocator.BatchTaskAllocationRequestArgsForCall(0)
			Expect(traceID).To(Equal("some-trace-id"))
			Expect(taskRequests).To(ConsistOf(successfulTask, unsuccessfulTask))
		})

		It("returns LRPs and Tasks that could not be allocated", func() {
			fakeContainerAllocator.BatchLRPAllocationRequestReturns([]models.SchedulingLRP{unsuccessfulLRP})
			fakeContainerAllocator.BatchTaskAllocationRequestReturns([]models.SchedulingTask{unsuccessfulTask})

			failedWork, err := cellRep.Perform(logger, "some-trace-id", models.Work{
				LRPs:  []models.SchedulingLRP{successfulLRP, unsuccessfulLRP},
				Tasks: []models.SchedulingTask{successfulTask, unsuccessfulTask},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(failedWork.LRPs).To(ConsistOf(unsuccessfulLRP))
			Expect(failedWork.Tasks).To(ConsistOf(unsuccessfulTask))
		})

		Context("when evacuating", func() {
			BeforeEach(func() {
				evacuationReporter.EvacuatingReturns(true)

				lrp := models.NewSchedulingLRP(
					"ig-1",
					models.NewActualLRPKey("process-guid", 1, "tests"),
					models.NewResource(2048, 1024, 100),
					models.NewPlacementConstraint(linuxRootFSURL, nil, []string{}),
				)

				task := models.NewSchedulingTask(
					"the-task-guid",
					"tests",
					models.NewResource(2048, 1024, 100),
					models.NewPlacementConstraint(linuxRootFSURL, nil, []string{}),
				)

				work = models.Work{
					LRPs:  []models.SchedulingLRP{lrp},
					Tasks: []models.SchedulingTask{task},
				}
			})

			It("returns all work it was given", func() {
				Expect(cellRep.Perform(logger, "some-trace-id", work)).To(Equal(work))
			})
		})

		Context("when the cell only has enough resources to run a subset of the workloads", func() {
			var smallestLRP, middleLRP, largestLRP models.SchedulingLRP

			BeforeEach(func() {
				remainingCellMemory = 8192
				largestLRP = models.SchedulingLRP{Resource: models.Resource{MemoryMB: 6144}}
				middleLRP = models.SchedulingLRP{Resource: models.Resource{MemoryMB: int32(remainingCellMemory) - largestLRP.MemoryMB}}
				smallestLRP = models.SchedulingLRP{Resource: models.Resource{MemoryMB: 1}}
			})

			It("allocates containers for the largest workloads it can run", func() {
				failedWork, err := cellRep.Perform(logger, "some-trace-id", models.Work{
					LRPs:  []models.SchedulingLRP{smallestLRP, middleLRP, largestLRP},
					Tasks: []models.SchedulingTask{},
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(failedWork.LRPs).To(ConsistOf(smallestLRP))

				Expect(fakeContainerAllocator.BatchLRPAllocationRequestCallCount()).To(Equal(1))

				_, traceID, proxyEnabledArg, proxyMemFootprintArg, lrpRequests := fakeContainerAllocator.BatchLRPAllocationRequestArgsForCall(0)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(proxyEnabledArg).To(BeFalse())
				Expect(proxyMemFootprintArg).To(Equal(12))
				Expect(lrpRequests).To(ConsistOf(largestLRP, middleLRP))
			})

			Context("when envoy needs to be placed in the container", func() {
				BeforeEach(func() {
					enableContainerProxy = true
					proxyMemoryAllocation = remainingCellMemory - int(largestLRP.MemoryMB)
				})

				It("accounts for the proxy overhead when determining which workloads to run and which to reject", func() {
					failedWork, err := cellRep.Perform(logger, "some-trace-id", models.Work{
						LRPs:  []models.SchedulingLRP{smallestLRP, middleLRP, largestLRP},
						Tasks: []models.SchedulingTask{},
					})

					Expect(err).NotTo(HaveOccurred())
					Expect(failedWork.LRPs).To(ConsistOf(smallestLRP, middleLRP))

					Expect(fakeContainerAllocator.BatchLRPAllocationRequestCallCount()).To(Equal(1))

					_, traceID, proxyEnabledArg, proxyMemFootprintArg, lrpRequests := fakeContainerAllocator.BatchLRPAllocationRequestArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(proxyEnabledArg).To(BeTrue())
					Expect(proxyMemFootprintArg).To(Equal(proxyMemoryAllocation))
					Expect(lrpRequests).To(ConsistOf(largestLRP))
				})
			})
		})

		Context("when the workload's cell ID does not match the cell's ID", func() {
			It("rejects the workload", func() {
				_, err := cellRep.Perform(logger, "some-trace-id", models.Work{
					LRPs:   lrpAuctions,
					CellID: "do-not-want-your-work",
				})
				Expect(err).To(MatchError(auctioncellrep.ErrCellIdMismatch))
			})
		})
	})
})

func createContainer(state executor.State, lifecycle string) executor.Container {
	return executor.Container{
		Guid:     "some-container-guid",
		Resource: executor.NewResource(20, 10, 100),
		Tags: executor.Tags{
			rep.LifecycleTag:     lifecycle,
			rep.ProcessGuidTag:   "some-process-guid",
			rep.ProcessIndexTag:  "1",
			rep.DomainTag:        "domain",
			rep.InstanceGuidTag:  "some-instance-guid",
			rep.PlacementTagsTag: `["pt"]`,
			rep.VolumeDriversTag: `["vd"]`,
		},
		State: state,
	}
}
