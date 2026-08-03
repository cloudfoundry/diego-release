package internal

import (
	"fmt"
	"sync"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"
)

type evacuationLRPProcessor struct {
	bbsClient           bbs.InternalClient
	containerDelegate   ContainerDelegate
	metronClient        loggingclient.IngressClient
	cellID              string
	availabilityZone    string
	evacuatedContainers sync.Map
}

func newEvacuationLRPProcessor(bbsClient bbs.InternalClient, containerDelegate ContainerDelegate, metronClient loggingclient.IngressClient, cellID string, availabilityZone string) LRPProcessor {
	return &evacuationLRPProcessor{
		bbsClient:         bbsClient,
		containerDelegate: containerDelegate,
		metronClient:      metronClient,
		cellID:            cellID,
		availabilityZone:  availabilityZone,
	}
}

func (p *evacuationLRPProcessor) Process(logger lager.Logger, traceID string, container executor.Container) {
	logger = logger.Session("evacuation-lrp-processor", lager.Data{
		"container-guid":  container.Guid,
		"container-state": container.State,
	})
	logger.Debug("start")

	lrpKey, err := rep.ActualLRPKeyFromTags(container.Tags)
	if err != nil {
		logger.Error("failed-to-generate-lrp-key", err)
		return
	}

	instanceKey, err := rep.ActualLRPInstanceKeyFromContainer(container, p.cellID)
	if err != nil {
		logger.Error("failed-to-generate-instance-key", err)
		return
	}

	lrpContainer := newLRPContainer(lrpKey, instanceKey, container)

	switch lrpContainer.Container.State {
	case executor.StateReserved:
		p.processReservedContainer(logger, traceID, lrpContainer)
	case executor.StateInitializing:
		p.processInitializingContainer(logger, traceID, lrpContainer)
	case executor.StateCreated:
		p.processCreatedContainer(logger, traceID, lrpContainer)
	case executor.StateRunning:
		p.processRunningContainer(logger, traceID, lrpContainer, container.RunInfo.LogConfig)
	case executor.StateCompleted:
		p.processCompletedContainer(logger, traceID, lrpContainer)
	default:
		p.processInvalidContainer(logger, traceID, lrpContainer)
	}
}

func (p *evacuationLRPProcessor) processReservedContainer(logger lager.Logger, traceID string, lrpContainer *lrpContainer) {
	logger = logger.Session("process-reserved-container")
	p.evacuateClaimedLRPContainer(logger, traceID, lrpContainer)
}

func (p *evacuationLRPProcessor) processInitializingContainer(logger lager.Logger, traceID string, lrpContainer *lrpContainer) {
	logger = logger.Session("process-initializing-container")
	p.evacuateClaimedLRPContainer(logger, traceID, lrpContainer)
}

func (p *evacuationLRPProcessor) processCreatedContainer(logger lager.Logger, traceID string, lrpContainer *lrpContainer) {
	logger = logger.Session("process-created-container")
	p.evacuateClaimedLRPContainer(logger, traceID, lrpContainer)
}

func (p *evacuationLRPProcessor) processRunningContainer(logger lager.Logger, traceID string, lrpContainer *lrpContainer, logConfig models.LogConfig) {
	logger = logger.Session("process-running-container")

	logger.Debug("extracting-net-info-from-container")
	netInfo, err := rep.ActualLRPNetInfoFromContainer(lrpContainer.Container)
	if err != nil {
		logger.Error("failed-extracting-net-info-from-container", err)
		return
	}
	logger.Debug("succeeded-extracting-net-info-from-container")

	if _, ok := p.evacuatedContainers.LoadOrStore(lrpContainer.Guid, struct{}{}); !ok {
		sourceName, tags := logConfig.GetSourceNameAndTagsForLogging()
		metricError := p.metronClient.SendAppLog(fmt.Sprintf("Cell %s requesting replacement for instance %s", p.cellID, lrpContainer.ActualLRPInstanceKey.InstanceGuid), sourceName, tags)
		if metricError != nil {
			logger.Debug("failed-sending-app-log-about-replacement-for-instance-request", lager.Data{"error": metricError})
		}
	}

	logger.Info("bbs-evacuate-running-actual-lrp", lager.Data{"net_info": netInfo})
	internalRoutes := []*models.ActualLRPInternalRoute{}
	for _, internalRoute := range lrpContainer.InternalRoutes {
		internalRoutes = append(internalRoutes, &models.ActualLRPInternalRoute{Hostname: internalRoute.Hostname})
	}
	keepContainer, err := p.bbsClient.EvacuateRunningActualLRP(logger, traceID, lrpContainer.ActualLRPKey, lrpContainer.ActualLRPInstanceKey, netInfo, internalRoutes, lrpContainer.MetricsConfig.Tags, lrpContainer.Routable, p.availabilityZone)
	if !keepContainer {
		p.containerDelegate.DeleteContainer(logger, traceID, lrpContainer.Container.Guid)
	} else if err != nil {
		logger.Error("failed-to-evacuate-running-actual-lrp", err, lager.Data{"lrp-key": lrpContainer.ActualLRPKey})
	}
}

func (p *evacuationLRPProcessor) processCompletedContainer(logger lager.Logger, traceID string, lrpContainer *lrpContainer) {
	logger = logger.Session("process-completed-container")

	if lrpContainer.RunResult.Stopped {
		_, err := p.bbsClient.EvacuateStoppedActualLRP(logger, traceID, lrpContainer.ActualLRPKey, lrpContainer.ActualLRPInstanceKey)
		if err != nil {
			logger.Error("failed-to-evacuate-stopped-actual-lrp", err, lager.Data{"lrp-key": lrpContainer.ActualLRPKey})
		}
	} else {
		_, err := p.bbsClient.EvacuateCrashedActualLRP(logger, traceID, lrpContainer.ActualLRPKey, lrpContainer.ActualLRPInstanceKey, lrpContainer.RunResult.FailureReason)
		if err != nil {
			logger.Error("failed-to-evacuate-crashed-actual-lrp", err, lager.Data{"lrp-key": lrpContainer.ActualLRPKey})
		}
	}

	p.containerDelegate.DeleteContainer(logger, traceID, lrpContainer.Guid)
}

func (p *evacuationLRPProcessor) processInvalidContainer(logger lager.Logger, traceID string, lrpContainer *lrpContainer) {
	logger = logger.Session("process-invalid-container")
	logger.Error("not-processing-container-in-invalid-state", nil)
}

func (p *evacuationLRPProcessor) evacuateClaimedLRPContainer(logger lager.Logger, traceID string, lrpContainer *lrpContainer) {
	_, err := p.bbsClient.EvacuateClaimedActualLRP(logger, traceID, lrpContainer.ActualLRPKey, lrpContainer.ActualLRPInstanceKey)
	if err != nil {
		logger.Error("failed-to-unclaim-actual-lrp", err, lager.Data{"lrp-key": lrpContainer.ActualLRPKey})
	}

	p.containerDelegate.DeleteContainer(logger, traceID, lrpContainer.Container.Guid)
}
