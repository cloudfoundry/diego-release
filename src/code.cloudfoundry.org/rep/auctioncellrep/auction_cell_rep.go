package auctioncellrep

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context"
)

//go:generate counterfeiter . AuctionCellClient

type AuctionCellClient interface {
	State(logger lager.Logger) (models.CellState, bool, error)
	Perform(logger lager.Logger, traceID string, work models.Work) (models.Work, error)
	Reset() error
}

var ErrCellUnhealthy = errors.New("internal cell healthcheck failed")
var ErrCellIdMismatch = errors.New("workload cell ID does not match this cell")
var ErrNotEnoughMemory = errors.New("not enough memory for container and additional memory allocation")

type AuctionCellRep struct {
	cellID                   string
	cellIndex                int
	repURL                   string
	stackPathMap             rep.StackPathMap
	rootFSProviders          models.RootFSProviders
	containerMetricsProvider rep.ContainerMetricsProvider
	zone                     string
	client                   executor.Client
	evacuationReporter       evacuation_context.EvacuationReporter
	placementTags            []string
	optionalPlacementTags    []string
	enableContainerProxy     bool
	proxyMemoryAllocation    int
	allocator                BatchContainerAllocator
}

func New(
	cellID string,
	cellIndex int,
	repURL string,
	preloadedStackPathMap rep.StackPathMap,
	containerMetricsProvider rep.ContainerMetricsProvider,
	arbitraryRootFSes []string,
	zone string,
	client executor.Client,
	evacuationReporter evacuation_context.EvacuationReporter,
	placementTags []string,
	optionalPlacementTags []string,
	proxyMemoryAllocation int,
	enableContainerProxy bool,
	allocator BatchContainerAllocator,
) *AuctionCellRep {
	return &AuctionCellRep{
		cellID:                   cellID,
		cellIndex:                cellIndex,
		repURL:                   repURL,
		stackPathMap:             preloadedStackPathMap,
		rootFSProviders:          rootFSProviders(preloadedStackPathMap, arbitraryRootFSes),
		containerMetricsProvider: containerMetricsProvider,
		zone:                     zone,
		client:                   client,
		evacuationReporter:       evacuationReporter,
		placementTags:            placementTags,
		optionalPlacementTags:    optionalPlacementTags,
		enableContainerProxy:     enableContainerProxy,
		proxyMemoryAllocation:    proxyMemoryAllocation,
		allocator:                allocator,
	}
}

func rootFSProviders(preloaded rep.StackPathMap, arbitrary []string) models.RootFSProviders {
	rootFSProviders := models.RootFSProviders{}
	for _, scheme := range arbitrary {
		rootFSProviders[scheme] = models.ArbitraryRootFSProvider{}
	}

	stacks := make([]string, 0, len(preloaded))
	for stack := range preloaded {
		stacks = append(stacks, stack)
	}
	rootFSProviders[models.PreloadedRootFSScheme] = models.NewFixedSetRootFSProvider(stacks...)
	rootFSProviders[models.PreloadedOCIRootFSScheme] = models.NewFixedSetRootFSProvider(stacks...)

	return rootFSProviders
}

func rootFSURLFromPath(rootfsPath string, stackPathMap rep.StackPathMap) string {
	url, err := url.Parse(rootfsPath)
	if err != nil {
		return rootfsPath
	}

	for k, v := range stackPathMap {
		if rootfsPath == v {
			return fmt.Sprintf("%s:%s", models.PreloadedRootFSScheme, k)
		} else if url.Path == v {
			return fmt.Sprintf("%s:%s?%s", models.PreloadedOCIRootFSScheme, k, url.RawQuery)
		}
	}
	return rootfsPath
}

func (a *AuctionCellRep) State(logger lager.Logger) (models.CellState, bool, error) {
	logger = logger.Session("auction-state")
	logger.Info("providing")

	containers, err := a.client.ListContainers(logger)
	if err != nil {
		logger.Error("failed-to-fetch-containers", err)
		return models.CellState{}, false, err
	}

	totalResources, err := a.client.TotalResources(logger)
	if err != nil {
		logger.Error("failed-to-get-total-resources", err)
		return models.CellState{}, false, err
	}

	availableResources, err := a.client.RemainingResources(logger)
	if err != nil {
		logger.Error("failed-to-get-remaining-resource", err)
		return models.CellState{}, false, err
	}

	volumeDrivers, err := a.client.VolumeDrivers(logger)
	if err != nil {
		logger.Error("failed-to-get-volume-drivers", err)
		return models.CellState{}, false, err
	}

	lrps := []models.SchedulingLRP{}
	tasks := []models.SchedulingTask{}
	startingContainerCount := 0

	for i := range containers {
		container := &containers[i]

		if containerIsStarting(container) {
			startingContainerCount++
		}

		if container.Tags == nil {
			logger.Error("failed-to-extract-container-tags", nil)
			continue
		}

		placementTagsJSON := container.Tags[rep.PlacementTagsTag]
		var placementTags []string
		err := json.Unmarshal([]byte(placementTagsJSON), &placementTags)
		if err != nil {
			logger.Error("cannot-unmarshal-placement-tags", err, lager.Data{"placement-tags": placementTagsJSON})
		}

		volumeDriversJSON := container.Tags[rep.VolumeDriversTag]
		var volumeDrivers []string
		err = json.Unmarshal([]byte(volumeDriversJSON), &volumeDrivers)
		if err != nil {
			logger.Error("cannot-unmarshal-volume-drivers", err, lager.Data{"volume-drivers": volumeDriversJSON})
		}

		resource := models.Resource{MemoryMB: int32(container.MemoryMB), DiskMB: int32(container.DiskMB), MaxPids: int32(container.MaxPids)}
		placementConstraint := models.PlacementConstraint{
			RootFs:        rootFSURLFromPath(container.RootFSPath, a.stackPathMap),
			VolumeDrivers: volumeDrivers,
			PlacementTags: placementTags,
		}

		switch container.Tags[rep.LifecycleTag] {
		case rep.LRPLifecycle:
			key, err := rep.ActualLRPKeyFromTags(container.Tags)
			if err != nil {
				logger.Error("failed-to-extract-key", err)
				continue
			}
			instanceKey, err := rep.ActualLRPInstanceKeyFromContainer(*container, a.cellID)
			if err != nil {
				logger.Error("failed-to-extract-key", err)
				continue
			}
			var state string
			switch container.State {
			case executor.StateRunning:
				state = models.ActualLRPStateRunning
			case executor.StateCompleted:
				state = "SHUTDOWN"
				if container.RunResult.Failed {
					state = "CRASHED"
				}
			default:
				state = models.ActualLRPStateClaimed
			}
			lrp := models.NewSchedulingLRP(instanceKey.InstanceGuid, *key, resource, placementConstraint)
			lrp.State = state
			lrps = append(lrps, lrp)
		case rep.TaskLifecycle:
			domain := container.Tags[rep.DomainTag]
			state := models.Task_Running
			if container.State == executor.StateCompleted {
				state = models.Task_Completed
			}
			task := models.NewSchedulingTask(container.Guid, domain, resource, placementConstraint)
			task.State = state
			task.Failed = container.RunResult.Failed
			tasks = append(tasks, task)
		}
	}

	allocatedProxyMemory := 0
	if a.enableContainerProxy {
		allocatedProxyMemory = a.proxyMemoryAllocation
	}

	state := models.NewCellState(
		a.cellID,
		a.cellIndex,
		a.repURL,
		a.rootFSProviders,
		a.convertResources(availableResources),
		a.convertResources(totalResources),
		lrps,
		tasks,
		a.zone,
		startingContainerCount,
		a.evacuationReporter.Evacuating(),
		volumeDrivers,
		a.placementTags,
		a.optionalPlacementTags,
		allocatedProxyMemory,
	)

	healthy := a.client.Healthy(logger)
	if !healthy {
		logger.Error("failed-garden-health-check", nil)
	}

	logger.Info("provided", lager.Data{
		"available-resources": state.AvailableResources,
		"total-resources":     state.TotalResources,
		"num-lrps":            len(state.LRPs),
		"zone":                state.Zone,
		"evacuating":          state.Evacuating,
	})

	return state, healthy, nil
}

func (a *AuctionCellRep) Metrics(logger lager.Logger) (*rep.ContainerMetricsCollection, error) {
	var lrpMetrics = []rep.LRPMetric{}
	var taskMetrics = []rep.TaskMetric{}

	logger = logger.Session("metrics-collection")
	logger.Info("starting")
	defer logger.Info("complete")

	containers, err := a.client.ListContainers(logger)
	if err != nil {
		logger.Error("failed-to-fetch-containers", err)
		return nil, err
	}

	metrics := a.containerMetricsProvider.Metrics()

	for _, container := range containers {
		if container.Tags == nil {
			logger.Error("failed-to-extract-container-tags", nil)
			continue
		}

		guid := container.Guid
		containerMetrics, ok := metrics[guid]
		if !ok {
			logger.Info("failed-to-get-metrics-for-container", lager.Data{"guid": guid})
			continue
		}

		switch container.Tags[rep.LifecycleTag] {
		case rep.LRPLifecycle:
			key, err := rep.ActualLRPKeyFromTags(container.Tags)
			if err != nil {
				logger.Error("failed-to-extract-key", err)
				continue
			}
			instanceKey, err := rep.ActualLRPInstanceKeyFromContainer(container, a.cellID)
			if err != nil {
				logger.Error("failed-to-extract-key", err)
				continue
			}

			lrpMetric := rep.LRPMetric{
				ProcessGUID:            key.ProcessGuid,
				Index:                  key.Index,
				InstanceGUID:           instanceKey.InstanceGuid,
				CachedContainerMetrics: *containerMetrics,
			}
			lrpMetrics = append(lrpMetrics, lrpMetric)
		case rep.TaskLifecycle:
			taskMetric := rep.TaskMetric{
				TaskGUID:               container.Guid,
				CachedContainerMetrics: *containerMetrics,
			}
			taskMetrics = append(taskMetrics, taskMetric)
		}
	}

	return &rep.ContainerMetricsCollection{
		CellID: a.cellID,
		LRPs:   lrpMetrics,
		Tasks:  taskMetrics,
	}, nil
}

func containerIsStarting(container *executor.Container) bool {
	return container.State == executor.StateReserved ||
		container.State == executor.StateInitializing ||
		container.State == executor.StateCreated
}

func (a *AuctionCellRep) Perform(logger lager.Logger, traceID string, work models.Work) (models.Work, error) {
	var failedWork = models.Work{}

	logger = logger.Session("auction-work", lager.Data{
		"lrp-starts": len(work.LRPs),
		"tasks":      len(work.Tasks),
		"cell-id":    work.CellID,
	})

	if work.CellID != "" && work.CellID != a.cellID {
		logger.Error("cell-id-mismatch", ErrCellIdMismatch)
		return work, ErrCellIdMismatch
	}

	remainingResources, err := a.client.RemainingResources(logger)
	if err != nil {
		logger.Error("failed-gathering-remaining-reosurces", err)
		return work, err
	}

	var lrpRequests []models.SchedulingLRP
	remainingMemory := int32(remainingResources.MemoryMB)

	sort.SliceStable(work.LRPs, func(i, j int) bool {
		return work.LRPs[i].MemoryMB > work.LRPs[j].MemoryMB
	})

	for _, lrp := range work.LRPs {
		requiredMemory := lrp.MemoryMB
		if a.enableContainerProxy {
			requiredMemory += int32(a.proxyMemoryAllocation)
		}
		if requiredMemory <= remainingMemory {
			remainingMemory -= requiredMemory
			lrpRequests = append(lrpRequests, lrp)
		} else {
			failedWork.LRPs = append(failedWork.LRPs, lrp)
		}
	}

	if a.evacuationReporter.Evacuating() {
		return work, nil
	}

	unallocatedLRPs := a.allocator.BatchLRPAllocationRequest(logger, traceID, a.enableContainerProxy, a.proxyMemoryAllocation, lrpRequests)
	failedWork.LRPs = append(failedWork.LRPs, unallocatedLRPs...)
	failedWork.Tasks = a.allocator.BatchTaskAllocationRequest(logger, traceID, work.Tasks)

	return failedWork, nil
}

func (a *AuctionCellRep) convertResources(resources executor.ExecutorResources) models.Resources {
	return models.Resources{
		MemoryMB:   int32(resources.MemoryMB),
		DiskMB:     int32(resources.DiskMB),
		Containers: resources.Containers,
	}
}

func (a *AuctionCellRep) Reset() error {
	return errors.New("not-a-simulation-rep")
}
