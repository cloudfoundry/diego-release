package auctioncellrep_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	fake_client "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/auctioncellrep"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ContainerAllocator", func() {
	var (
		enableContainerProxy      bool
		proxyMemoryAllocation     int
		executorClient            *fake_client.FakeClient
		linuxRootFSURL            string
		fakeGenerateContainerGuid func() (string, error)
		logger                    *lagertest.TestLogger
		commonErr                 error

		allocator auctioncellrep.BatchContainerAllocator
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		linuxRootFSURL = models.PreloadedRootFS(linuxStack)
		enableContainerProxy = false
		proxyMemoryAllocation = 12
		executorClient = new(fake_client.FakeClient)
		commonErr = errors.New("Failed to fetch")

		fakeGenerateContainerGuidCallCount := 0
		fakeGenerateContainerGuid = func() (string, error) {
			fakeGenerateContainerGuidCallCount += 1
			return fmt.Sprintf("ig-%d", fakeGenerateContainerGuidCallCount), nil
		}
	})

	JustBeforeEach(func() {
		allocator = auctioncellrep.NewContainerAllocator(
			fakeGenerateContainerGuid,
			rep.StackPathMap{linuxStack: linuxPath},
			executorClient,
		)
	})

	Describe("BatchLRPAllocationRequest", func() {
		var (
			lrp1, lrp2           models.SchedulingLRP
			lrpIndex1, lrpIndex2 int32
		)

		BeforeEach(func() {
			lrpIndex1 = 0
			lrpIndex2 = 1

			lrp1 = models.NewSchedulingLRP(
				"ig-1",
				models.NewActualLRPKey("process-guid", lrpIndex1, "tests"),
				models.NewResource(2048, 1024, 100),
				models.NewPlacementConstraint(linuxRootFSURL, []string{"pt-1"}, []string{"vd-1"}),
			)

			lrp2 = models.NewSchedulingLRP(
				"ig-2",
				models.NewActualLRPKey("process-guid", lrpIndex2, "tests"),
				models.NewResource(2048, 1024, 100),
				models.NewPlacementConstraint("rootfs", []string{"pt-2"}, []string{}),
			)
		})

		It("makes the correct allocation requests for all LRPs", func() {
			allocator.BatchLRPAllocationRequest(logger, "some-trace-id", enableContainerProxy, proxyMemoryAllocation, []models.SchedulingLRP{lrp1, lrp2})

			Expect(executorClient.AllocateContainersCallCount()).To(Equal(1))
			_, traceID, arg := executorClient.AllocateContainersArgsForCall(0)
			Expect(traceID).To(Equal("some-trace-id"))
			Expect(arg).To(ConsistOf(
				allocationRequestFromLRP(lrp1),
				allocationRequestFromLRP(lrp2),
			))
		})

		It("does not mark any LRP Auctions as failed", func() {
			failedWork := allocator.BatchLRPAllocationRequest(logger, "some-trace-id", enableContainerProxy, proxyMemoryAllocation, []models.SchedulingLRP{lrp1, lrp2})
			Expect(failedWork).To(BeEmpty())
		})

		Context("when a container fails to be allocated", func() {
			BeforeEach(func() {
				allocationRequest := allocationRequestFromLRP(lrp2)
				allocationFailure := executor.NewAllocationFailure(&allocationRequest, commonErr.Error())
				executorClient.AllocateContainersReturns([]executor.AllocationFailure{allocationFailure})
			})

			It("marks the corresponding LRP Auctions as failed", func() {
				failedWork := allocator.BatchLRPAllocationRequest(logger, "some-trace-id", enableContainerProxy, proxyMemoryAllocation, []models.SchedulingLRP{lrp1, lrp2})
				Expect(failedWork).To(ConsistOf(lrp2))
			})
		})

		Context("when envoy needs to be placed in the container", func() {
			BeforeEach(func() {
				enableContainerProxy = true
				proxyMemoryAllocation = 32
			})

			It("makes the correct allocation requests for all LRP Auctions with the additional memory allocation", func() {
				allocator.BatchLRPAllocationRequest(logger, "some-trace-id", enableContainerProxy, proxyMemoryAllocation, []models.SchedulingLRP{lrp1, lrp2})

				Expect(executorClient.AllocateContainersCallCount()).To(Equal(1))
				_, traceID, arg := executorClient.AllocateContainersArgsForCall(0)
				Expect(traceID).To(Equal("some-trace-id"))

				allocationRequest1 := allocationRequestFromLRP(lrp1)
				allocationRequest2 := allocationRequestFromLRP(lrp2)
				allocationRequest1.MemoryMB += proxyMemoryAllocation
				allocationRequest2.MemoryMB += proxyMemoryAllocation

				Expect(arg).To(ConsistOf(
					allocationRequest1,
					allocationRequest2,
				))
			})

			Context("when the LRP has unlimited memory and additional memory is allocated for the proxy", func() {
				BeforeEach(func() {
					lrp1.MemoryMB = 0
				})

				It("requests an LRP with unlimited memory", func() {
					allocator.BatchLRPAllocationRequest(logger, "some-trace-id", enableContainerProxy, proxyMemoryAllocation, []models.SchedulingLRP{lrp1, lrp2})

					expectedResource := executor.NewResource(0, int(lrp1.DiskMB), int(lrp1.MaxPids))

					_, traceID, arg := executorClient.AllocateContainersArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(len(arg)).To(BeNumerically(">=", 1))

					Expect(arg[0].Resource).To(Equal(expectedResource))
				})
			})
		})

		Context("when no requests need to be made", func() {
			It("doesn't make any requests to the executorClient", func() {
				allocator.BatchLRPAllocationRequest(logger, "some-trace-id", enableContainerProxy, proxyMemoryAllocation, []models.SchedulingLRP{})
				Expect(executorClient.AllocateContainersCallCount()).To(Equal(0))
			})
		})

		Describe("handling RootFS paths", func() {
			var validLRP, invalidLRP models.SchedulingLRP

			BeforeEach(func() {
				validLRP = models.NewSchedulingLRP(
					"ig-1",
					models.NewActualLRPKey("process-guid", lrpIndex1, "tests"),
					models.NewResource(2048, 1024, 100),
					models.NewPlacementConstraint(
						linuxRootFSURL,
						[]string{"pt-1"},
						[]string{"vd-1"},
					),
				)

				invalidLRP = models.NewSchedulingLRP(
					"ig-2",
					models.NewActualLRPKey("process-guid", lrpIndex2, "tests"),
					models.NewResource(2048, 1024, 100),
					models.NewPlacementConstraint("rootfs", []string{"pt-2"}, []string{}),
				)
			})

			Context("when an LRP specifies an invalid RootFS URL", func() {
				BeforeEach(func() {
					invalidLRP.RootFs = "%x"
				})

				It("only makes container allocation requests for the remaining LRPs", func() {
					allocator.BatchLRPAllocationRequest(logger, "some-trace-id", enableContainerProxy, proxyMemoryAllocation, []models.SchedulingLRP{validLRP, invalidLRP})

					Expect(executorClient.AllocateContainersCallCount()).To(Equal(1))
					_, traceID, arg := executorClient.AllocateContainersArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(arg).To(ConsistOf(
						allocationRequestFromLRP(validLRP),
					))
				})

				It("marks the other LRP as failed", func() {
					failedLRPs := allocator.BatchLRPAllocationRequest(logger, "some-trace-id", enableContainerProxy, proxyMemoryAllocation, []models.SchedulingLRP{validLRP, invalidLRP})
					Expect(failedLRPs).To(ConsistOf(invalidLRP))
				})
			})

			Context("when a LRP specifies a preloaded RootFSes for which it cannot determine a RootFS path", func() {
				BeforeEach(func() {
					invalidLRP.RootFs = "preloaded:not-on-cell"
				})

				It("only makes container allocation requests for the LRPs with valid RootFS paths", func() {
					allocator.BatchLRPAllocationRequest(logger, "some-trace-id", enableContainerProxy, proxyMemoryAllocation, []models.SchedulingLRP{validLRP, invalidLRP})

					Expect(executorClient.AllocateContainersCallCount()).To(Equal(1))
					_, traceID, arg := executorClient.AllocateContainersArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(arg).To(ConsistOf(
						allocationRequestFromLRP(validLRP),
					))
				})

				It("marks the LRPs with invalid RootFS paths as failed", func() {
					failedLRPs := allocator.BatchLRPAllocationRequest(logger, "some-trace-id", enableContainerProxy, proxyMemoryAllocation, []models.SchedulingLRP{validLRP, invalidLRP})
					Expect(failedLRPs).To(HaveLen(1))
					Expect(failedLRPs).To(ContainElement(invalidLRP))
				})
			})

			Context("when a LRP specifies a blank RootFS URL", func() {
				BeforeEach(func() {
					validLRP.RootFs = ""
				})

				It("makes the correct allocation request for it, passing along the blank path to the executor client", func() {
					allocator.BatchLRPAllocationRequest(logger, "some-trace-id", enableContainerProxy, proxyMemoryAllocation, []models.SchedulingLRP{validLRP})

					Expect(executorClient.AllocateContainersCallCount()).To(Equal(1))
					_, traceID, arg := executorClient.AllocateContainersArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(arg).To(ConsistOf(
						allocationRequestFromLRP(validLRP),
					))
				})
			})

			Context("when the lrp uses docker rootfs scheme", func() {
				BeforeEach(func() {
					validLRP.RootFs = "docker://cloudfoundry/grace"
				})

				It("makes the container allocation request with an unchanged rootfs url", func() {
					allocator.BatchLRPAllocationRequest(logger, "some-trace-id", enableContainerProxy, proxyMemoryAllocation, []models.SchedulingLRP{validLRP})

					Expect(executorClient.AllocateContainersCallCount()).To(Equal(1))
					_, traceID, arg := executorClient.AllocateContainersArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(arg).To(ConsistOf(
						allocationRequestFromLRP(validLRP),
					))
				})
			})
		})
	})

	Describe("BatchTaskAllocationRequest", func() {
		var (
			task1, task2 models.SchedulingTask
		)

		BeforeEach(func() {
			resource1 := models.NewResource(256, 512, 256)
			placement1 := models.NewPlacementConstraint("tests", []string{"pt-1"}, []string{"vd-1"})
			task1 = models.NewSchedulingTask("the-task-guid-1", "tests", resource1, placement1)
			task1.RootFs = linuxRootFSURL

			resource2 := models.NewResource(512, 1024, 256)
			placement2 := models.NewPlacementConstraint("linux", []string{"pt-2"}, []string{})
			task2 = models.NewSchedulingTask("the-task-guid-2", "tests", resource2, placement2)
			task2.RootFs = "unsupported-arbitrary://still-goes-through"
		})

		It("makes the correct allocation requests for all Tasks", func() {
			allocator.BatchTaskAllocationRequest(logger, "some-trace-id", []models.SchedulingTask{task1, task2})

			Expect(executorClient.AllocateContainersCallCount()).To(Equal(1))
			_, traceID, arg := executorClient.AllocateContainersArgsForCall(0)
			Expect(traceID).To(Equal("some-trace-id"))
			Expect(arg).To(ConsistOf(
				allocationRequestFromTask(task1, `["pt-1"]`, `["vd-1"]`),
				allocationRequestFromTask(task2, `["pt-2"]`, `[]`),
			))
		})

		Context("when all containers can be successfully allocated", func() {
			BeforeEach(func() {
				executorClient.AllocateContainersReturns([]executor.AllocationFailure{})
			})

			It("does not mark any Tasks as failed", func() {
				failedTasks := allocator.BatchTaskAllocationRequest(logger, "some-trace-id", []models.SchedulingTask{task1, task2})
				Expect(failedTasks).To(BeEmpty())
			})
		})

		Context("when a container fails to be allocated", func() {
			BeforeEach(func() {
				resource := executor.NewResource(int(task1.MemoryMB), int(task1.DiskMB), int(task1.MaxPids))
				tags := executor.Tags{}
				allocationRequest := executor.NewAllocationRequest(
					task1.TaskGuid,
					&resource,
					false,
					tags,
				)
				allocationFailure := executor.NewAllocationFailure(&allocationRequest, commonErr.Error())
				executorClient.AllocateContainersReturns([]executor.AllocationFailure{allocationFailure})
			})

			It("marks the corresponding Tasks as failed", func() {
				failedTasks := allocator.BatchTaskAllocationRequest(logger, "some-trace-id", []models.SchedulingTask{task1, task2})
				Expect(failedTasks).To(ConsistOf(task1))
			})

			It("logs the container allocation failure", func() {
				allocator.BatchTaskAllocationRequest(logger, "some-trace-id", []models.SchedulingTask{task1, task2})
				Eventually(logger).Should(gbytes.Say("container-allocation-failure.*failed-request.*the-task-guid-1"))
			})
		})

		Context("when no requests need to be made", func() {
			It("doesn't make any requests to the executorClient", func() {
				allocator.BatchTaskAllocationRequest(logger, "some-trace-id", []models.SchedulingTask{})
				Expect(executorClient.AllocateContainersCallCount()).To(Equal(0))
			})
		})

		Describe("handling RootFS paths", func() {
			var validTask, invalidTask models.SchedulingTask

			BeforeEach(func() {
				resource1 := models.NewResource(256, 512, 256)
				placement1 := models.NewPlacementConstraint("tests", []string{"pt-1"}, []string{"vd-1"})
				validTask = models.NewSchedulingTask("the-task-guid-1", "tests", resource1, placement1)
				validTask.RootFs = linuxRootFSURL

				resource2 := models.NewResource(512, 1024, 256)
				placement2 := models.NewPlacementConstraint("linux", []string{"pt-2"}, []string{})
				invalidTask = models.NewSchedulingTask("the-task-guid-2", "tests", resource2, placement2)
			})

			Context("when a Task specifies an invalid RootFS URL", func() {
				BeforeEach(func() {
					invalidTask.RootFs = "%x"
				})

				It("only makes container allocation requests for the remaining Tasks", func() {
					allocator.BatchTaskAllocationRequest(logger, "some-trace-id", []models.SchedulingTask{validTask, invalidTask})

					Expect(executorClient.AllocateContainersCallCount()).To(Equal(1))
					_, traceID, arg := executorClient.AllocateContainersArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(arg).To(ConsistOf(
						allocationRequestFromTask(validTask, `["pt-1"]`, `["vd-1"]`),
					))
				})

				It("marks the Task as failed", func() {
					failedTasks := allocator.BatchTaskAllocationRequest(logger, "some-trace-id", []models.SchedulingTask{validTask, invalidTask})
					Expect(failedTasks).To(ConsistOf(invalidTask))
				})
			})

			Context("when a Task specifies a preloaded RootFSes for which it cannot determine a RootFS path", func() {
				BeforeEach(func() {
					invalidTask.RootFs = "preloaded:not-on-cell"
				})

				It("only makes container allocation requests for the tasks with valid RootFS paths", func() {
					allocator.BatchTaskAllocationRequest(logger, "some-trace-id", []models.SchedulingTask{validTask, invalidTask})

					Expect(executorClient.AllocateContainersCallCount()).To(Equal(1))
					_, traceID, arg := executorClient.AllocateContainersArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(arg).To(ConsistOf(
						allocationRequestFromTask(validTask, `["pt-1"]`, `["vd-1"]`),
					))
				})

				It("marks the tasks with invalid RootFS paths as failed", func() {
					failedTasks := allocator.BatchTaskAllocationRequest(logger, "some-trace-id", []models.SchedulingTask{validTask, invalidTask})
					Expect(failedTasks).To(HaveLen(1))
					Expect(failedTasks).To(ContainElement(invalidTask))
				})
			})

			Context("when a Task specifies a blank RootFS URL", func() {
				BeforeEach(func() {
					validTask.RootFs = ""
				})

				It("makes the correct allocation request for it, passing along the blank path to the executor client", func() {
					allocator.BatchTaskAllocationRequest(logger, "some-trace-id", []models.SchedulingTask{validTask})

					Expect(executorClient.AllocateContainersCallCount()).To(Equal(1))
					_, traceID, arg := executorClient.AllocateContainersArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(arg).To(ConsistOf(
						allocationRequestFromTask(validTask, `["pt-1"]`, `["vd-1"]`),
					))
				})
			})
		})
	})
})

func allocationRequestFromLRP(lrp models.SchedulingLRP) executor.AllocationRequest {
	resource := executor.NewResource(
		int(lrp.MemoryMB),
		int(lrp.DiskMB),
		int(lrp.MaxPids),
	)

	placementTagsBytes, err := json.Marshal(lrp.PlacementTags)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	volumeDriversBytes, err := json.Marshal(lrp.VolumeDrivers)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	return executor.NewAllocationRequest(
		lrp.InstanceGUID,
		&resource,
		true,
		executor.Tags{
			rep.LifecycleTag:     rep.LRPLifecycle,
			rep.DomainTag:        lrp.Domain,
			rep.PlacementTagsTag: string(placementTagsBytes),
			rep.VolumeDriversTag: string(volumeDriversBytes),
			rep.ProcessGuidTag:   lrp.ProcessGuid,
			rep.ProcessIndexTag:  strconv.Itoa(int(lrp.Index)),
			rep.InstanceGuidTag:  lrp.InstanceGUID,
		},
	)
}

func allocationRequestFromTask(task models.SchedulingTask, placementTags, volumeDrivers string) executor.AllocationRequest {
	resource := executor.NewResource(int(task.MemoryMB), int(task.DiskMB), int(task.MaxPids))
	return executor.NewAllocationRequest(
		task.TaskGuid,
		&resource,
		false,
		executor.Tags{
			rep.LifecycleTag:     rep.TaskLifecycle,
			rep.DomainTag:        task.Domain,
			rep.PlacementTagsTag: placementTags,
			rep.VolumeDriversTag: volumeDrivers,
		},
	)
}
