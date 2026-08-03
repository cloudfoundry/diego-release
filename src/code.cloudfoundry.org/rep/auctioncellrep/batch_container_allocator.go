package auctioncellrep

import (
	"encoding/json"
	"strconv"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"
)

//go:generate counterfeiter . BatchContainerAllocator
type BatchContainerAllocator interface {
	BatchLRPAllocationRequest(lager.Logger, string, bool, int, []models.SchedulingLRP) []models.SchedulingLRP
	BatchTaskAllocationRequest(lager.Logger, string, []models.SchedulingTask) []models.SchedulingTask
}

type containerAllocator struct {
	generateInstanceGuid func() (string, error)
	stackPathMap         rep.StackPathMap
	executorClient       executor.Client
}

func NewContainerAllocator(instanceGuidGenerator func() (string, error), stackPathMap rep.StackPathMap, executorClient executor.Client) BatchContainerAllocator {
	return containerAllocator{
		generateInstanceGuid: instanceGuidGenerator,
		stackPathMap:         stackPathMap,
		executorClient:       executorClient,
	}
}

func buildLRPTags(lrp models.SchedulingLRP, instanceGuid string) executor.Tags {
	tags := executor.Tags{}
	tags[rep.DomainTag] = lrp.Domain
	tags[rep.ProcessGuidTag] = lrp.ProcessGuid
	tags[rep.ProcessIndexTag] = strconv.Itoa(int(lrp.Index))
	tags[rep.LifecycleTag] = rep.LRPLifecycle
	tags[rep.InstanceGuidTag] = instanceGuid

	placementTags, _ := json.Marshal(lrp.PlacementConstraint.PlacementTags)
	volumeDrivers, _ := json.Marshal(lrp.PlacementConstraint.VolumeDrivers)
	tags[rep.PlacementTagsTag] = string(placementTags)
	tags[rep.VolumeDriversTag] = string(volumeDrivers)

	return tags
}

func buildTaskTags(task models.SchedulingTask) executor.Tags {
	tags := executor.Tags{}
	tags[rep.LifecycleTag] = rep.TaskLifecycle
	tags[rep.DomainTag] = task.Domain

	placementTags, _ := json.Marshal(task.PlacementConstraint.PlacementTags)
	volumeDrivers, _ := json.Marshal(task.PlacementConstraint.VolumeDrivers)
	tags[rep.PlacementTagsTag] = string(placementTags)
	tags[rep.VolumeDriversTag] = string(volumeDrivers)
	return tags
}

func (ca containerAllocator) BatchLRPAllocationRequest(logger lager.Logger, traceID string, enableContainerProxy bool, proxyMemoryAllocation int, lrps []models.SchedulingLRP) (unallocatedLRPs []models.SchedulingLRP) {
	logger = logger.Session("lrp-allocate-instances")
	requests := make([]executor.AllocationRequest, 0, len(lrps))
	lrpGuidMap := make(map[string]models.SchedulingLRP, len(lrps))

	for _, lrp := range lrps {
		instanceGuid, err := ca.generateInstanceGuid()
		if err != nil {
			unallocatedLRPs = append(unallocatedLRPs, lrp)
			continue
		}

		_, err = ca.stackPathMap.PathForRootFS(lrp.RootFs)
		if err != nil {
			unallocatedLRPs = append(unallocatedLRPs, lrp)
			continue
		}

		memoryMB := int(lrp.MemoryMB)
		if memoryMB > 0 && enableContainerProxy {
			memoryMB += proxyMemoryAllocation
		}

		resource := executor.NewResource(memoryMB, int(lrp.DiskMB), int(lrp.MaxPids))
		containerGuid := rep.LRPContainerGuid(lrp.ProcessGuid, instanceGuid)

		lrpGuidMap[containerGuid] = lrp
		requests = append(requests, executor.NewAllocationRequest(containerGuid, &resource, true, buildLRPTags(lrp, instanceGuid)))
	}

	if len(unallocatedLRPs) > 0 {
		logger.Info("failed-to-translate-lrps-to-containers", lager.Data{"num-failed-to-translate": len(unallocatedLRPs)})
	}

	logger.Info("requesting-container-allocation", lager.Data{"num-requesting-allocation": len(requests)})
	var failures []executor.AllocationFailure
	if len(requests) > 0 {
		failures = ca.executorClient.AllocateContainers(logger, traceID, requests)
	}

	logger.Info("succeeded-requesting-container-allocation", lager.Data{"num-failed-to-allocate": len(failures)})

	for _, failure := range failures {
		logger.Error("container-allocation-failure", &failure, lager.Data{"failed-request": failure.AllocationRequest})
		if lrp, found := lrpGuidMap[failure.Guid]; found {
			unallocatedLRPs = append(unallocatedLRPs, lrp)
		}
	}

	return unallocatedLRPs
}

func (ca containerAllocator) BatchTaskAllocationRequest(logger lager.Logger, traceID string, tasks []models.SchedulingTask) (unallocatedTasks []models.SchedulingTask) {
	logger = logger.Session("task-allocate-instances")

	failedTasks := make([]models.SchedulingTask, 0)
	taskMap := make(map[string]models.SchedulingTask, len(tasks))
	requests := make([]executor.AllocationRequest, 0, len(tasks))

	for _, task := range tasks {
		taskMap[task.TaskGuid] = task
		_, err := ca.stackPathMap.PathForRootFS(task.RootFs)
		if err != nil {
			failedTasks = append(failedTasks, task)
			continue
		}

		tags := buildTaskTags(task)
		resource := executor.NewResource(int(task.MemoryMB), int(task.DiskMB), int(task.MaxPids))
		requests = append(requests, executor.NewAllocationRequest(task.TaskGuid, &resource, false, tags))
	}

	if len(failedTasks) > 0 {
		logger.Info("failed-to-translate-tasks-to-containers", lager.Data{"num-failed-to-translate": len(failedTasks)})
		unallocatedTasks = append(unallocatedTasks, failedTasks...)
	}

	logger.Info("requesting-container-allocation", lager.Data{"num-requesting-allocation": len(requests)})
	var failures []executor.AllocationFailure
	if len(requests) > 0 {
		failures = ca.executorClient.AllocateContainers(logger, traceID, requests)
	}

	for _, failure := range failures {
		logger.Error("container-allocation-failure", &failure, lager.Data{"failed-request": failure.AllocationRequest})
		if task, found := taskMap[failure.Guid]; found {
			unallocatedTasks = append(unallocatedTasks, task)
		}
	}

	return unallocatedTasks
}
