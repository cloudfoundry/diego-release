package auctionrunner_test

import (
	"time"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/rep"
	. "github.com/onsi/gomega"
)

func BuildLRPStartRequest(
	processGuid, domain string,
	indices []int,
	rootFS string,
	memoryMB, diskMB, maxPids int32,
	volumeDriver, placementTags []string,
) auctioneer.LRPStartRequest {
	return auctioneer.NewLRPStartRequest(
		processGuid,
		domain,
		indices,
		rep.NewResource(memoryMB, diskMB, maxPids),
		rep.NewPlacementConstraint(rootFS, placementTags, volumeDriver),
	)
}

func BuildTaskStartRequest(taskGuid, domain, rootFS string, memoryMB, diskMB, maxPids int32) auctioneer.TaskStartRequest {
	return auctioneer.NewTaskStartRequest(*BuildTask(taskGuid, domain, rootFS, memoryMB, diskMB, maxPids, []string{}, []string{}))
}

func BuildLRP(
	guid, domain string,
	index int,
	rootFS string,
	memoryMB, diskMB, maxPids int32,
	placementTags []string,
) *rep.LRP {
	lrpKey := models.NewActualLRPKey(guid, int32(index), domain)
	lrp := rep.NewLRP(
		"",
		lrpKey,
		rep.NewResource(memoryMB, diskMB, maxPids),
		rep.NewPlacementConstraint(rootFS, placementTags, []string{}),
	)
	return &lrp
}

func BuildTask(taskGuid, domain, rootFS string, memoryMB, diskMB, maxPids int32, volumeDrivers, placementTags []string) *rep.Task {
	task := rep.NewTask(
		taskGuid,
		domain,
		rep.NewResource(memoryMB, diskMB, maxPids),
		rep.NewPlacementConstraint(rootFS, placementTags, volumeDrivers),
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
		rep.NewLRP(
			"",
			lrpKey,
			rep.NewResource(memoryMB, diskMB, maxPids),
			rep.NewPlacementConstraint(rootFS, placementTags, volumeDrivers),
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
		rep.NewLRP(
			"",
			lrpKey,
			rep.NewResource(memoryMB, diskMB, maxPids),
			rep.NewPlacementConstraint(rootFS, placementTags, volumeDrivers),
		),
		queueTime,
	)

	a.PlacementError = placementError
	return a
}

func BuildLRPAuctions(start auctioneer.LRPStartRequest, queueTime time.Time) []auctiontypes.LRPAuction {
	auctions := make([]auctiontypes.LRPAuction, 0, len(start.Indices))
	for _, index := range start.Indices {
		lrpKey := models.NewActualLRPKey(start.ProcessGuid, int32(index), start.Domain)
		auctions = append(auctions, auctiontypes.NewLRPAuction(
			rep.NewLRP("", lrpKey, start.Resource, start.PlacementConstraint),
			queueTime,
		))
	}

	return auctions
}

func BuildTaskAuction(task *rep.Task, queueTime time.Time) auctiontypes.TaskAuction {
	return auctiontypes.NewTaskAuction(*task, queueTime)
}

const linuxStack = "linux"

var linuxRootFSURL = models.PreloadedRootFS(linuxStack)

var linuxOnlyRootFSProviders = rep.RootFSProviders{models.PreloadedRootFSScheme: rep.NewFixedSetRootFSProvider(linuxStack)}

const windowsStack = "windows"

var windowsRootFSURL = models.PreloadedRootFS(windowsStack)

var windowsOnlyRootFSProviders = rep.RootFSProviders{models.PreloadedRootFSScheme: rep.NewFixedSetRootFSProvider(windowsStack)}

func BuildCellState(
	cellID string,
	cellIndex int,
	zone string,
	memoryMB int32,
	diskMB int32,
	containers int,
	evacuating bool,
	startingContainerCount int,
	rootFSProviders rep.RootFSProviders,
	lrps []rep.LRP,
	volumeDrivers []string,
	placementTags []string,
	optionalPlacementTags []string,
	proxyMemoryAllocationMB int,
) rep.CellState {
	totalResources := rep.NewResources(memoryMB, diskMB, containers)

	availableResources := totalResources.Copy()
	for i := range lrps {
		availableResources.Subtract(&lrps[i].Resource)
	}

	Expect(availableResources.MemoryMB).To(BeNumerically(">=", 0), "Check your math!")
	Expect(availableResources.DiskMB).To(BeNumerically(">=", 0), "Check your math!")
	Expect(availableResources.Containers).To(BeNumerically(">=", 0), "Check your math!")

	return rep.NewCellState(
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
