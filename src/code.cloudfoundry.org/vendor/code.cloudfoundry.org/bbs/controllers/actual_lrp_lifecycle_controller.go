package controllers

import (
	"context"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/events/calculator"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/serviceclient"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"
)

type ActualLRPLifecycleController struct {
	db                   db.ActualLRPDB
	suspectDB            db.SuspectDB
	evacuationDB         db.EvacuationDB
	desiredLRPDB         db.DesiredLRPDB
	auctioneerClient     auctioneer.Client
	serviceClient        serviceclient.ServiceClient
	repClientFactory     rep.ClientFactory
	actualHub            events.Hub
	actualLRPInstanceHub events.Hub
}

func NewActualLRPLifecycleController(
	db db.ActualLRPDB,
	suspectDB db.SuspectDB,
	evacuationDB db.EvacuationDB,
	desiredLRPDB db.DesiredLRPDB,
	auctioneerClient auctioneer.Client,
	serviceClient serviceclient.ServiceClient,
	repClientFactory rep.ClientFactory,
	actualHub events.Hub,
	actualLRPInstanceHub events.Hub,
) *ActualLRPLifecycleController {
	return &ActualLRPLifecycleController{
		db:                   db,
		suspectDB:            suspectDB,
		evacuationDB:         evacuationDB,
		desiredLRPDB:         desiredLRPDB,
		auctioneerClient:     auctioneerClient,
		serviceClient:        serviceClient,
		repClientFactory:     repClientFactory,
		actualHub:            actualHub,
		actualLRPInstanceHub: actualLRPInstanceHub,
	}
}

func findWithPresence(lrps []*models.ActualLRP, presence models.ActualLRP_Presence) *models.ActualLRP {
	for _, lrp := range lrps {
		if lrp.Presence == presence {
			return lrp
		}
	}
	return nil
}

func lookupLRPInSlice(lrps []*models.ActualLRP, key *models.ActualLRPInstanceKey) *models.ActualLRP {
	for _, lrp := range lrps {
		if lrp.ActualLRPInstanceKey == *key {
			return lrp
		}
	}
	return nil
}

func (h *ActualLRPLifecycleController) ClaimActualLRP(ctx context.Context, logger lager.Logger, processGUID string, index int32, actualLRPInstanceKey *models.ActualLRPInstanceKey) error {
	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	lrps, err := h.db.ActualLRPs(ctx, logger, models.ActualLRPFilter{ProcessGuid: processGUID, Index: &index})
	if err != nil {
		return err
	}

	lrp := lookupLRPInSlice(lrps, actualLRPInstanceKey)
	if lrp != nil && lrp.Presence == models.ActualLRP_Suspect {
		logger.Info("ignored-claim-request-from-suspect", lager.Data{
			"process_guid":  processGUID,
			"index":         index,
			"instance_guid": actualLRPInstanceKey,
			"state":         lrp.State,
		})
		return nil
	}

	before, after, err := h.db.ClaimActualLRP(ctx, logger, processGUID, index, actualLRPInstanceKey)
	if err != nil {
		return err
	}

	newLRPs := eventCalculator.RecordChange(before, after, lrps)
	go eventCalculator.EmitEvents(trace.RequestIdFromContext(ctx), lrps, newLRPs)

	return nil
}

func (h *ActualLRPLifecycleController) StartActualLRP(ctx context.Context,
	logger lager.Logger,
	actualLRPKey *models.ActualLRPKey,
	actualLRPInstanceKey *models.ActualLRPInstanceKey,
	actualLRPNetInfo *models.ActualLRPNetInfo,
	actualLRPInternalRoutes []*models.ActualLRPInternalRoute,
	actualLRPMetricTags map[string]string,
	routable bool,
	availabilityZone string,
) error {
	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	lrps, err := h.db.ActualLRPs(ctx, logger, models.ActualLRPFilter{ProcessGuid: actualLRPKey.ProcessGuid, Index: &actualLRPKey.Index})
	if err != nil && err != models.ErrResourceNotFound {
		return err
	}

	lrp := lookupLRPInSlice(lrps, actualLRPInstanceKey)
	if lrp != nil && lrp.Presence == models.ActualLRP_Suspect {
		logger.Info("ignored-start-request-from-suspect", lager.Data{
			"process_guid":  actualLRPKey.ProcessGuid,
			"index":         actualLRPKey.Index,
			"instance_guid": actualLRPInstanceKey,
			"state":         lrp.State,
		})
		return nil
	}

	isCurrentlyRunning := lrp != nil && lrp.State == models.ActualLRPStateRunning

	// creates ordinary running actual LRP if it doesn't exist, otherwise updates
	// the existing ordinary actual LRP to running state
	before, after, err := h.db.StartActualLRP(ctx, logger, actualLRPKey, actualLRPInstanceKey, actualLRPNetInfo, actualLRPInternalRoutes, actualLRPMetricTags, routable, availabilityZone, isCurrentlyRunning)
	if err != nil {
		return err
	}
	newLRPs := eventCalculator.RecordChange(before, after, lrps)

	defer func() {
		go eventCalculator.EmitEvents(trace.RequestIdFromContext(ctx), lrps, newLRPs)
	}()

	evacuating := findWithPresence(lrps, models.ActualLRP_Evacuating)
	suspect := findWithPresence(lrps, models.ActualLRP_Suspect)

	if evacuating != nil {
		err = h.evacuationDB.RemoveEvacuatingActualLRP(ctx, logger, &evacuating.ActualLRPKey, &evacuating.ActualLRPInstanceKey)
		if err != nil {
			logger.Error("failed-to-remove-evacuating-actual-lrp", err, lager.Data{"instance-guid": evacuating.ActualLRPInstanceKey})
		}
		newLRPs = eventCalculator.RecordChange(evacuating, nil, newLRPs)
	}

	var suspectLRP *models.ActualLRP
	// prior to starting this ActualLRP there was a suspect LRP that we need to remove
	if suspect != nil {
		suspectLRP, err = h.suspectDB.RemoveSuspectActualLRP(ctx, logger, actualLRPKey)
		if err != nil {
			logger.Error("failed-to-remove-suspect-lrp", err)
		} else {
			newLRPs = eventCalculator.RecordChange(suspectLRP, nil, newLRPs)
		}
	}

	return nil
}

func (h *ActualLRPLifecycleController) CrashActualLRP(ctx context.Context, logger lager.Logger, actualLRPKey *models.ActualLRPKey, actualLRPInstanceKey *models.ActualLRPInstanceKey, errorMessage string) error {
	lrps, err := h.db.ActualLRPs(ctx, logger, models.ActualLRPFilter{ProcessGuid: actualLRPKey.ProcessGuid, Index: &actualLRPKey.Index})
	if err != nil {
		return err
	}

	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	lrp := lookupLRPInSlice(lrps, actualLRPInstanceKey)
	traceId := trace.RequestIdFromContext(ctx)
	if lrp != nil && lrp.Presence == models.ActualLRP_Suspect {
		suspectLRP, err := h.suspectDB.RemoveSuspectActualLRP(ctx, logger, actualLRPKey)
		if err != nil {
			return err
		}

		afterLRPs := eventCalculator.RecordChange(suspectLRP, nil, lrps)
		logger.Info("removing-suspect-lrp", lager.Data{"ig": suspectLRP.InstanceGuid})
		go eventCalculator.EmitEvents(traceId, lrps, afterLRPs)

		return nil
	}

	before, after, shouldRestart, err := h.db.CrashActualLRP(ctx, logger, actualLRPKey, actualLRPInstanceKey, errorMessage)
	if err != nil {
		return err
	}

	afterLRPs := eventCalculator.RecordChange(before, after, lrps)
	go eventCalculator.EmitCrashEvents(traceId, lrps, afterLRPs)

	if !shouldRestart {
		return nil
	}

	schedInfo, err := h.desiredLRPDB.DesiredLRPSchedulingInfoByProcessGuid(ctx, logger, actualLRPKey.ProcessGuid)
	if err != nil {
		logger.Error("failed-fetching-desired-lrp", err)
		return err
	}

	startRequest := auctioneer.NewLRPStartRequestFromSchedulingInfo(schedInfo, int(actualLRPKey.Index))
	logger.Info("start-lrp-auction-request", lager.Data{"app_guid": schedInfo.ProcessGuid, "index": int(actualLRPKey.Index)})
	err = h.auctioneerClient.RequestLRPAuctions(logger, trace.RequestIdFromContext(ctx), []*auctioneer.LRPStartRequest{&startRequest})
	logger.Info("finished-lrp-auction-request", lager.Data{"app_guid": schedInfo.ProcessGuid, "index": int(actualLRPKey.Index)})
	if err != nil {
		logger.Error("failed-requesting-auction", err)
	}
	return nil
}

func (h *ActualLRPLifecycleController) FailActualLRP(ctx context.Context, logger lager.Logger, key *models.ActualLRPKey, errorMessage string) error {
	lrps, err := h.db.ActualLRPs(ctx, logger, models.ActualLRPFilter{ProcessGuid: key.ProcessGuid, Index: &key.Index})
	if err != nil {
		return err
	}

	before, after, err := h.db.FailActualLRP(ctx, logger, key, errorMessage)
	if err != nil && err != models.ErrResourceNotFound {
		return err
	}

	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	newLRPs := eventCalculator.RecordChange(before, after, lrps)
	go eventCalculator.EmitEvents(trace.RequestIdFromContext(ctx), lrps, newLRPs)

	return nil
}

func (h *ActualLRPLifecycleController) RemoveActualLRP(ctx context.Context, logger lager.Logger, processGUID string, index int32, instanceKey *models.ActualLRPInstanceKey) error {
	beforeLRPs, err := h.db.ActualLRPs(ctx, logger, models.ActualLRPFilter{ProcessGuid: processGUID, Index: &index})
	if err != nil {
		return err
	}

	lrp := findWithPresence(beforeLRPs, models.ActualLRP_Ordinary)
	if lrp == nil {
		return models.ErrResourceNotFound
	}

	err = h.db.RemoveActualLRP(ctx, logger, processGUID, index, instanceKey)
	if err != nil {
		return err
	}

	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	newLRPs := eventCalculator.RecordChange(lrp, nil, beforeLRPs)
	go eventCalculator.EmitEvents(trace.RequestIdFromContext(ctx), beforeLRPs, newLRPs)

	return nil
}

func (h *ActualLRPLifecycleController) RetireActualLRP(ctx context.Context, logger lager.Logger, key *models.ActualLRPKey) error {
	var err error
	var cell *models.CellPresence

	logger = logger.Session("retire-actual-lrp", lager.Data{"process_guid": key.ProcessGuid, "index": key.Index})

	lrps, err := h.db.ActualLRPs(ctx, logger, models.ActualLRPFilter{ProcessGuid: key.ProcessGuid, Index: &key.Index})
	if err != nil {
		return err
	}

	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	lrp := findWithPresence(lrps, models.ActualLRP_Ordinary)
	if lrp == nil {
		return models.ErrResourceNotFound
	}

	newLRPs := make([]*models.ActualLRP, len(lrps))
	copy(newLRPs, lrps)

	defer func() {
		go eventCalculator.EmitEvents(trace.RequestIdFromContext(ctx), lrps, newLRPs)
	}()

	recordChange := func() {
		newLRPs = eventCalculator.RecordChange(lrp, nil, lrps)
	}

	removeLRP := func() error {
		err = h.db.RemoveActualLRP(ctx, logger, lrp.ProcessGuid, lrp.Index, &lrp.ActualLRPInstanceKey)
		if err == nil {
			recordChange()
		}
		return err
	}

	for retryCount := 0; retryCount < models.RetireActualLRPRetryAttempts; retryCount++ {
		switch lrp.State {
		case models.ActualLRPStateUnclaimed, models.ActualLRPStateCrashed:
			err = removeLRP()
		case models.ActualLRPStateClaimed, models.ActualLRPStateRunning:
			cell, err = h.serviceClient.CellById(logger, lrp.CellId)
			if err != nil {
				bbsErr := models.ConvertError(err)
				if bbsErr.Type == models.Error_ResourceNotFound {
					return removeLRP()
				}
				return err
			}

			var client rep.Client
			recordChange()
			client, err = h.repClientFactory.CreateClient(cell.RepAddress, cell.RepUrl, trace.RequestIdFromContext(ctx))
			if err != nil {
				return err
			}
			err = client.StopLRPInstance(logger, lrp.ActualLRPKey, lrp.ActualLRPInstanceKey)
		}

		if err == nil {
			return nil
		}

		logger.Error("retrying-failed-retire-of-actual-lrp", err, lager.Data{"attempt": retryCount + 1})
	}

	return err
}
