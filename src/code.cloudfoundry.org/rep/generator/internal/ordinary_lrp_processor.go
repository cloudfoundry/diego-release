package internal

import (
	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/ecrhelper"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"
)

type ordinaryLRPProcessor struct {
	bbsClient                  bbs.InternalClient
	containerDelegate          ContainerDelegate
	cellID                     string
	availabilityZone           string
	stackPathMap               rep.StackPathMap
	layeringMode               string
	runRequestConversionHelper rep.RunRequestConversionHelper
}

func newOrdinaryLRPProcessor(
	bbsClient bbs.InternalClient,
	containerDelegate ContainerDelegate,
	cellID string,
	availabilityZone string,
	stackPathMap rep.StackPathMap,
	layeringMode string,
) LRPProcessor {
	runRequestConversionHelper := rep.RunRequestConversionHelper{ECRHelper: ecrhelper.NewECRHelper()}

	return &ordinaryLRPProcessor{
		bbsClient:                  bbsClient,
		containerDelegate:          containerDelegate,
		cellID:                     cellID,
		availabilityZone:           availabilityZone,
		stackPathMap:               stackPathMap,
		layeringMode:               layeringMode,
		runRequestConversionHelper: runRequestConversionHelper,
	}
}

func (p *ordinaryLRPProcessor) Process(logger lager.Logger, traceID string, container executor.Container) {
	logger = logger.Session("ordinary-lrp-processor", lager.Data{
		"container-guid":  container.Guid,
		"container-state": container.State,
	})
	logger.Debug("starting")
	defer logger.Debug("finished")

	lrpKey, err := rep.ActualLRPKeyFromTags(container.Tags)
	if err != nil {
		logger.Error("failed-to-generate-lrp-key", err)
		return
	}
	logger = logger.WithData(lager.Data{"lrp-key": lrpKey})

	instanceKey, err := rep.ActualLRPInstanceKeyFromContainer(container, p.cellID)
	if err != nil {
		logger.Error("failed-to-generate-instance-key", err)
		return
	}
	logger = logger.WithData(lager.Data{"lrp-instance-key": instanceKey})

	lrpContainer := newLRPContainer(lrpKey, instanceKey, container)
	switch lrpContainer.Container.State {
	case executor.StateReserved:
		p.processReservedContainer(logger, traceID, lrpContainer)
	case executor.StateInitializing:
		p.processInitializingContainer(logger, traceID, lrpContainer)
	case executor.StateCreated:
		p.processCreatedContainer(logger, traceID, lrpContainer)
	case executor.StateRunning:
		p.processRunningContainer(logger, traceID, lrpContainer)
	case executor.StateCompleted:
		p.processCompletedContainer(logger, traceID, lrpContainer)
	default:
		p.processInvalidContainer(logger, traceID, lrpContainer)
	}
}

func (p *ordinaryLRPProcessor) processReservedContainer(logger lager.Logger, traceID string, lrpContainer *lrpContainer) {
	logger = logger.Session("process-reserved-container")
	ok := p.claimLRPContainer(logger, traceID, lrpContainer)
	if !ok {
		return
	}

	desired, err := p.bbsClient.DesiredLRPByProcessGuid(logger, traceID, lrpContainer.ProcessGuid)
	if err != nil {
		logger.Error("failed-to-fetch-desired", err)
		return
	}

	runReq, err := p.runRequestConversionHelper.NewRunRequestFromDesiredLRP(lrpContainer.Guid, desired, lrpContainer.ActualLRPKey, lrpContainer.ActualLRPInstanceKey, p.stackPathMap, p.layeringMode)
	if err != nil {
		logger.Error("failed-to-construct-run-request", err)
		return
	}
	ok = p.containerDelegate.RunContainer(logger, traceID, &runReq)
	if !ok {
		// #nosec G104 - ignore errors cleaning up the failed container
		p.bbsClient.RemoveActualLRP(logger, traceID, lrpContainer.ActualLRPKey, lrpContainer.ActualLRPInstanceKey)
		return
	}
}

func (p *ordinaryLRPProcessor) processInitializingContainer(logger lager.Logger, traceID string, lrpContainer *lrpContainer) {
	logger = logger.Session("process-initializing-container")
	p.claimLRPContainer(logger, traceID, lrpContainer)
}

func (p *ordinaryLRPProcessor) processCreatedContainer(logger lager.Logger, traceID string, lrpContainer *lrpContainer) {
	logger = logger.Session("process-created-container")
	p.claimLRPContainer(logger, traceID, lrpContainer)
}

func (p *ordinaryLRPProcessor) processRunningContainer(logger lager.Logger, traceID string, lrpContainer *lrpContainer) {
	logger = logger.Session("process-running-container")

	logger.Debug("extracting-net-info-from-container")
	netInfo, err := rep.ActualLRPNetInfoFromContainer(lrpContainer.Container)
	if err != nil {
		logger.Error("failed-extracting-net-info-from-container", err)
		return
	}
	logger.Debug("succeeded-extracting-net-info-from-container")

	logger.Info("bbs-start-actual-lrp", lager.Data{"net_info": netInfo, "routable": lrpContainer.Routable})
	internalRoutes := []*models.ActualLRPInternalRoute{}
	for _, internalRoute := range lrpContainer.InternalRoutes {
		internalRoutes = append(internalRoutes, &models.ActualLRPInternalRoute{Hostname: internalRoute.Hostname})
	}
	err = p.bbsClient.StartActualLRP(logger, traceID, lrpContainer.ActualLRPKey, lrpContainer.ActualLRPInstanceKey, netInfo, internalRoutes, lrpContainer.MetricsConfig.Tags, lrpContainer.Routable, p.availabilityZone)
	bbsErr := models.ConvertError(err)
	if bbsErr != nil && bbsErr.Type == models.Error_ActualLRPCannotBeStarted {
		p.containerDelegate.StopContainer(logger, traceID, lrpContainer.Guid)
	}
}

func (p *ordinaryLRPProcessor) processCompletedContainer(logger lager.Logger, traceID string, lrpContainer *lrpContainer) {
	logger = logger.Session("process-completed-container")

	if lrpContainer.RunResult.Stopped {
		err := p.bbsClient.RemoveActualLRP(logger, traceID, lrpContainer.ActualLRPKey, lrpContainer.ActualLRPInstanceKey)
		if err != nil {
			logger.Info("failed-to-remove-actual-lrp", lager.Data{"error": err})
		}
	} else {
		err := p.bbsClient.CrashActualLRP(logger, traceID, lrpContainer.ActualLRPKey, lrpContainer.ActualLRPInstanceKey, lrpContainer.RunResult.FailureReason)
		if err != nil {
			logger.Info("failed-to-crash-actual-lrp", lager.Data{"error": err})
		}
	}

	p.containerDelegate.DeleteContainer(logger, traceID, lrpContainer.Guid)
}

func (p *ordinaryLRPProcessor) processInvalidContainer(logger lager.Logger, traceID string, lrpContainer *lrpContainer) {
	logger = logger.Session("process-invalid-container")
	logger.Error("not-processing-container-in-invalid-state", nil)
}

func (p *ordinaryLRPProcessor) claimLRPContainer(logger lager.Logger, traceID string, lrpContainer *lrpContainer) bool {
	err := p.bbsClient.ClaimActualLRP(logger, traceID, lrpContainer.ActualLRPKey, lrpContainer.ActualLRPInstanceKey)
	bbsErr := models.ConvertError(err)
	if err != nil {
		if bbsErr.Type == models.Error_ActualLRPCannotBeClaimed {
			p.containerDelegate.DeleteContainer(logger, traceID, lrpContainer.Guid)
		}
		return false
	}
	return true
}
