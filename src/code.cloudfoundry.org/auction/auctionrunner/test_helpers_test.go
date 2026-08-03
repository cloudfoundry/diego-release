package auctionrunner_test

import (
	"time"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/gomega"
)

func BuildLRPStartRequest(
	processGuid, domain string,
	indices []int,
	rootFS string,
	memoryMB, diskMB, maxPids int32,
	volumeDriver, placementTags []string,
) models.LRPStartRequest {
	return models.NewLRPStartRequest(
		processGuid,
		domain,
		indices,
		models.NewResource(memoryMB, diskMB, maxPids),
		models.NewPlacementConstraint(rootFS, placementTags, volumeDriver),
	)
}

func BuildTaskStartRequest(taskGuid, domain, rootFS string, memoryMB, diskMB, maxPids int32) models.TaskStartRequest {
	return models.NewTaskStartRequest(*BuildTask(taskGuid, domain, rootFS, memoryMB, diskMB, maxPids, []string{}, []string{}))
}

func BuildLRP(
	guid, domain string,
	index int,
	rootFS string,
	memoryMB, diskMB, maxPids int32,
	placementTags []string,
) *models.SchedulingLRP {
	lrpKey := models.NewActualLRPKey(guid, int32(index), domain)
	lrp := models.NewSchedulingLRP(
		"",
		lrpKey,
		models.NewResource(memoryMB, diskMB, maxPids),
		models.NewPlacementConstraint(rootFS, placementTags, []string{}),
	)
	return &lrp
}

func BuildTask(taskGuid, domain, rootFS string, memoryMB, diskMB, maxPids int32, volumeDrivers, placementTags []string) *models.SchedulingTask {
	task := models.NewSchedulingTask(
		taskGuid,
		domain,
		models.NewResource(memoryMB, diskMB, maxPids),
		models.NewPlacementConstraint(rootFS, placementTags, volumeDrivers),
	)
	return &task
}

func BuildLRPAuction(
	processGuid, domain string,
	index int,
	rootFS string,
	memoryMB, diskMB, maxPids int32,
	queueTime time.Time,
	volumeDrivers, placementTags []string,
) auctiontypes.LRPAuction {
	lrpKey := models.NewActualLRPKey(processGuid, int32(index), domain)

	return auctiontypes.NewLRPAuction(
		models.NewSchedulingLRP(
			"",
			lrpKey,
			models.NewResource(memoryMB, diskMB, maxPids),
			models.NewPlacementConstraint(rootFS, placementTags, volumeDrivers),
		),
		queueTime,
	)
}

func BuildLRPAuctionWithPlacementError(
	processGuid, domain string,
	index int,
	rootFS string,
	memoryMB, diskMB, maxPids int32,
	queueTime time.Time,
	placementError string,
	volumeDrivers, placementTags []string,
) auctiontypes.LRPAuction {
	lrpKey := models.NewActualLRPKey(processGuid, int32(index), domain)

	a := auctiontypes.NewLRPAuction(
		models.NewSchedulingLRP(
			"",
			lrpKey,
			models.NewResource(memoryMB, diskMB, maxPids),
			models.NewPlacementConstraint(rootFS, placementTags, volumeDrivers),
		),
		queueTime,
	)

	a.PlacementError = placementError
	return a
}

func BuildLRPAuctions(start models.LRPStartRequest, queueTime time.Time) []auctiontypes.LRPAuction {
	auctions := make([]auctiontypes.LRPAuction, 0, len(start.Indices))
	for _, index := range start.Indices {
		lrpKey := models.NewActualLRPKey(start.ProcessGuid, int32(index), start.Domain)
		auctions = append(auctions, auctiontypes.NewLRPAuction(
			models.NewSchedulingLRP("", lrpKey, start.Resource, start.PlacementConstraint),
			queueTime,
		))
	}

	return auctions
}

func BuildTaskAuction(task *models.SchedulingTask, queueTime time.Time) auctiontypes.TaskAuction {
	return auctiontypes.NewTaskAuction(*task, queueTime)
}

const linuxStack = "linux"

var linuxRootFSURL = models.PreloadedRootFS(linuxStack)

var linuxOnlyRootFSProviders = models.RootFSProviders{models.PreloadedRootFSScheme: models.NewFixedSetRootFSProvider(linuxStack)}

const windowsStack = "windows"

var windowsRootFSURL = models.PreloadedRootFS(windowsStack)

var windowsOnlyRootFSProviders = models.RootFSProviders{models.PreloadedRootFSScheme: models.NewFixedSetRootFSProvider(windowsStack)}

func BuildCellState(
	cellID string,
	cellIndex int,
	zone string,
	memoryMB int32,
	diskMB int32,
	containers int,
	evacuating bool,
	startingContainerCount int,
	rootFSProviders models.RootFSProviders,
	lrps []models.SchedulingLRP,
	volumeDrivers []string,
	placementTags []string,
	optionalPlacementTags []string,
	proxyMemoryAllocationMB int,
) models.CellState {
	totalResources := models.NewResources(memoryMB, diskMB, containers)

	availableResources := totalResources.Copy()
	for i := range lrps {
		availableResources.Subtract(&lrps[i].Resource)
	}

	Expect(availableResources.MemoryMB).To(BeNumerically(">=", 0), "Check your math!")
	Expect(availableResources.DiskMB).To(BeNumerically(">=", 0), "Check your math!")
	Expect(availableResources.Containers).To(BeNumerically(">=", 0), "Check your math!")

	return models.NewCellState(
		cellID,
		cellIndex,
		"https://foo.cell.service.cf.internal",
		rootFSProviders,
		availableResources,
		totalResources,
		lrps,
		nil,
		zone,
		startingContainerCount,
		evacuating,
		volumeDrivers,
		placementTags,
		optionalPlacementTags,
		proxyMemoryAllocationMB,
	)
}
