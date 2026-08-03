package controllers

import (
	"context"
	"sync"

	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/metrics"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/serviceclient"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/workpool"
)

//go:generate counterfeiter -generate

//counterfeiter:generate -o fakes/fake_retirer.go . Retirer
type Retirer interface {
	RetireActualLRP(ctx context.Context, logger lager.Logger, key *models.ActualLRPKey) error
}

type LRPConvergenceController struct {
	logger                 lager.Logger
	clock                  clock.Clock
	lrpDB                  db.LRPDB
	suspectDB              db.SuspectDB
	domainDB               db.DomainDB
	actualHub              events.Hub
	actualLRPInstanceHub   events.Hub
	auctioneerClient       models.AuctioneerClient
	serviceClient          serviceclient.ServiceClient
	repClientFactory       models.RepClientFactory
	retirer                Retirer
	convergenceWorkersSize int
	lrpStatMetronNotifier  metrics.LRPStatMetronNotifier
}

func NewLRPConvergenceController(
	logger lager.Logger,
	clock clock.Clock,
	db db.LRPDB,
	suspectDB db.SuspectDB,
	domainDB db.DomainDB,
	actualHub events.Hub,
	actualLRPInstanceHub events.Hub,
	auctioneerClient models.AuctioneerClient,
	serviceClient serviceclient.ServiceClient,
	repClientFactory models.RepClientFactory,
	retirer Retirer,
	convergenceWorkersSize int,
	lrpStatMetronNotifier metrics.LRPStatMetronNotifier,
) *LRPConvergenceController {
	return &LRPConvergenceController{
		logger:                 logger,
		clock:                  clock,
		lrpDB:                  db,
		suspectDB:              suspectDB,
		domainDB:               domainDB,
		actualHub:              actualHub,
		actualLRPInstanceHub:   actualLRPInstanceHub,
		auctioneerClient:       auctioneerClient,
		serviceClient:          serviceClient,
		repClientFactory:       repClientFactory,
		retirer:                retirer,
		convergenceWorkersSize: convergenceWorkersSize,
		lrpStatMetronNotifier:  lrpStatMetronNotifier,
	}
}

func (h *LRPConvergenceController) ConvergeLRPs(ctx context.Context) {
	logger := h.logger.Session("converge-lrps")
	traceId := trace.RequestIdFromContext(ctx)

	start := h.clock.Now()

	var err error
	var cellSet models.CellSet
	logger.Debug("listing-cells")

	cellSet, err = h.serviceClient.Cells(logger)
	if err == models.ErrResourceNotFound {
		logger.Info("no-cells-found")
		cellSet = models.CellSet{}
	} else if err != nil {
		logger.Error("failed-listing-cells", err)
		// convergence should run again later
		return
	}
	logger.Debug("succeeded-listing-cells")

	convergenceResult := h.lrpDB.ConvergeLRPs(ctx, logger, cellSet)

	events := convergenceResult.Events
	for _, e := range events {
		go h.actualHub.Emit(e)
	}

	instanceEvents := convergenceResult.InstanceEvents
	for _, e := range instanceEvents {
		go h.actualLRPInstanceHub.Emit(e)
	}

	keysToRetire := convergenceResult.KeysToRetire
	retireLogger := logger.WithData(lager.Data{"retiring_lrp_count": len(keysToRetire)})
	works := []func(){}
	for _, key := range keysToRetire {
		dereferencedKey := *key
		works = append(works, func() {
			err := h.retirer.RetireActualLRP(ctx, retireLogger, &dereferencedKey)
			if err != nil {
				logger.Error("retiring-lrp-failed", err)
			}
		})
	}

	startRequests := []*models.LRPStartRequest{}
	startRequestLock := &sync.Mutex{}

	defer func() {
		startLogger := logger.WithData(lager.Data{"start_requests_count": len(startRequests)})
		if len(startRequests) > 0 {
			startLogger.Debug("requesting-start-auctions")
			err = h.auctioneerClient.RequestLRPAuctions(logger, traceId, startRequests)
			if err != nil {
				startLogger.Error("failed-to-request-starts", err, lager.Data{"lrp_start_auctions": startRequests})
			}
			startLogger.Debug("done-requesting-start-auctions")
		}
	}()

	defer func() {
		h.lrpStatMetronNotifier.RecordConvergenceDuration(h.clock.Since(start))

		domains, err := h.domainDB.FreshDomains(ctx, logger)
		if err != nil {
			logger.Error("failed-getting-fresh-domains", err)
		}
		h.lrpStatMetronNotifier.RecordFreshDomains(domains)

		claimed, unclaimed, running, crashed, crashingDesired := h.lrpDB.CountActualLRPsByState(ctx, logger)
		desired := h.lrpDB.CountDesiredInstances(ctx, logger)

		h.lrpStatMetronNotifier.RecordLRPCounts(
			unclaimed, claimed, running, crashed,
			len(convergenceResult.MissingLRPKeys), len(convergenceResult.KeysToRetire),
			len(convergenceResult.SuspectRunningKeys), len(convergenceResult.SuspectClaimedKeys),
			desired, crashingDesired,
		)

		h.lrpStatMetronNotifier.RecordCellCounts(len(cellSet), len(convergenceResult.MissingCellIds))
	}()

	for _, key := range convergenceResult.MissingLRPKeys {
		dereferencedKey := *key
		works = append(works, func() {
			lrp, err := h.lrpDB.CreateUnclaimedActualLRP(ctx, logger, dereferencedKey.Key)
			if err != nil {
				logger.Error("failed-to-create-unclaimed-lrp", err, lager.Data{"key": dereferencedKey.Key})
				return
			}

			//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
			go h.actualHub.Emit(models.NewActualLRPCreatedEvent(lrp.ToActualLRPGroup()))
			go h.actualLRPInstanceHub.Emit(models.NewActualLRPInstanceCreatedEvent(lrp, traceId))

			startRequest := models.NewLRPStartRequestFromSchedulingInfo(dereferencedKey.SchedulingInfo, int(dereferencedKey.Key.Index))
			startRequestLock.Lock()
			startRequests = append(startRequests, &startRequest)
			startRequestLock.Unlock()
		})
	}

	for _, lrpKey := range convergenceResult.UnstartedLRPKeys {
		dereferencedKey := *lrpKey
		works = append(works, func() {
			before, after, err := h.lrpDB.UnclaimActualLRP(ctx, logger, true, dereferencedKey.Key)
			if err != nil && err != models.ErrActualLRPCannotBeUnclaimed {
				logger.Error("cannot-unclaim-lrp", err, lager.Data{"key": dereferencedKey})
				return
			} else if !after.Equal(before) {
				logger.Info("emitting-changed-event", lager.Data{"before": before, "after": after})
				//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
				go h.actualHub.Emit(models.NewActualLRPChangedEvent(before.ToActualLRPGroup(), after.ToActualLRPGroup()))
				go func() {
					h.actualLRPInstanceHub.Emit(models.NewActualLRPInstanceCreatedEvent(after, traceId))
					h.actualLRPInstanceHub.Emit(models.NewActualLRPInstanceRemovedEvent(before, traceId))
				}()
			}

			startRequest := models.NewLRPStartRequestFromSchedulingInfo(dereferencedKey.SchedulingInfo, int(dereferencedKey.Key.Index))
			startRequestLock.Lock()
			startRequests = append(startRequests, &startRequest)
			startRequestLock.Unlock()
		})
	}

	suspectKeyMap := map[models.ActualLRPKey]int{}
	for _, suspectKey := range convergenceResult.SuspectRunningKeys {
		suspectKeyMap[*suspectKey] = 0
	}
	for _, suspectKey := range convergenceResult.SuspectClaimedKeys {
		suspectKeyMap[*suspectKey] = 0
	}

	for _, key := range convergenceResult.KeysWithMissingCells {
		dereferencedKey := *key
		handleLRP := func() {
			logger := logger.Session("keys-with-missing-cells")

			_, existingSuspect := suspectKeyMap[*dereferencedKey.Key]
			if existingSuspect {
				// there is a Suspect LRP already, unclaim this previously created
				// replacement and reauction it
				logger.Debug("found-suspect-lrp-unclaiming", lager.Data{"key": dereferencedKey.Key})
				before, after, err := h.lrpDB.UnclaimActualLRP(ctx, logger, false, dereferencedKey.Key)
				if err != nil {
					logger.Error("failed-unclaiming-lrp", err)
					return
				}

				//emit instance events for removing suspect and creating unclaimed
				go func() {
					h.actualLRPInstanceHub.Emit(models.NewActualLRPInstanceCreatedEvent(after, traceId))
					h.actualLRPInstanceHub.Emit(models.NewActualLRPInstanceRemovedEvent(before, traceId))
				}()

				return
			}

			before, after, err := h.lrpDB.ChangeActualLRPPresence(ctx, logger, dereferencedKey.Key, models.ActualLRP_Ordinary, models.ActualLRP_Suspect)
			if err != nil {
				logger.Error("cannot-change-lrp-presence", err, lager.Data{"key": dereferencedKey})
				return
			}
			go h.actualLRPInstanceHub.Emit(models.NewActualLRPInstanceChangedEvent(before, after, traceId))

			unclaimed, err := h.lrpDB.CreateUnclaimedActualLRP(ctx, logger.Session("create-unclaimed-actual"), dereferencedKey.Key)
			if err != nil {
				logger.Error("cannot-unclaim-lrp", err)
				return
			}
			go h.actualLRPInstanceHub.Emit(models.NewActualLRPInstanceCreatedEvent(unclaimed, traceId))

			startRequest := models.NewLRPStartRequestFromSchedulingInfo(dereferencedKey.SchedulingInfo, int(dereferencedKey.Key.Index))
			startRequestLock.Lock()
			startRequests = append(startRequests, &startRequest)
			startRequestLock.Unlock()
			logger.Info("creating-start-request",
				lager.Data{"reason": "missing-cell", "process_guid": dereferencedKey.Key.ProcessGuid, "index": dereferencedKey.Key.Index})
		}

		works = append(works, handleLRP)
	}

	for _, key := range convergenceResult.SuspectKeysWithExistingCells {
		dereferencedKey := *key
		works = append(works, func() {
			logger := logger.Session("suspect-keys-with-existing-cells")
			beforeLRP, afterLRP, removedLRP, err := h.suspectDB.PromoteSuspectActualLRP(ctx, logger, dereferencedKey.ProcessGuid, dereferencedKey.Index)
			if err != nil {
				logger.Error("cannot-promote-suspect-lrp", err, lager.Data{"key": dereferencedKey})
				return
			}
			if removedLRP != nil {
				//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
				go h.actualHub.Emit(models.NewActualLRPRemovedEvent(removedLRP.ToActualLRPGroup()))
				go h.actualLRPInstanceHub.Emit(models.NewActualLRPInstanceRemovedEvent(removedLRP, traceId))

			}
			go h.actualLRPInstanceHub.Emit(models.NewActualLRPInstanceChangedEvent(beforeLRP, afterLRP, traceId))
		})
	}

	for _, key := range convergenceResult.SuspectLRPKeysToRetire {
		dereferencedKey := *key
		works = append(works, func() {
			logger := logger.Session("suspect-keys-to-retire")
			suspectLRP, err := h.suspectDB.RemoveSuspectActualLRP(ctx, logger, &dereferencedKey)
			if err != nil {
				logger.Error("cannot-remove-suspect-lrp", err, lager.Data{"key": dereferencedKey})
				return
			}

			//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
			go h.actualHub.Emit(models.NewActualLRPRemovedEvent(suspectLRP.ToActualLRPGroup()))
			go h.actualLRPInstanceHub.Emit(models.NewActualLRPInstanceRemovedEvent(suspectLRP, traceId))
		})
	}

	for _, lrpKey := range convergenceResult.KeysWithInternalRouteChanges {
		dereferencedLRPKey := *lrpKey
		works = append(works, func() {
			cellPresence, err := h.serviceClient.CellById(logger, dereferencedLRPKey.InstanceKey.CellId)
			if err != nil {
				logger.Error("failed-fetching-cell-presence", err)
				return
			}

			repClient, err := h.repClientFactory.CreateClient(cellPresence.RepAddress, cellPresence.RepUrl, "")
			if err != nil {
				logger.Error("create-rep-client-failed", err)
				return
			}

			var internalRoutes models.InternalRoutes
			for _, ir := range dereferencedLRPKey.DesiredInternalRoutes {
				internalRoutes = append(internalRoutes, models.InternalRoute{Hostname: ir.Hostname})
			}
			lrpUpdate := models.NewLRPUpdate(dereferencedLRPKey.InstanceKey.InstanceGuid, *dereferencedLRPKey.Key, internalRoutes, nil)
			err = repClient.UpdateLRPInstance(logger, lrpUpdate)
			if err != nil {
				logger.Error("updating-lrp-instance", err)
			}
		})
	}

	for _, lrpKey := range convergenceResult.KeysWithMetricTagChanges {
		dereferencedLRPKey := *lrpKey
		works = append(works, func() {
			cellPresence, err := h.serviceClient.CellById(logger, dereferencedLRPKey.InstanceKey.CellId)
			if err != nil {
				logger.Error("failed-fetching-cell-presence", err)
				return
			}

			repClient, err := h.repClientFactory.CreateClient(cellPresence.RepAddress, cellPresence.RepUrl, "")
			if err != nil {
				logger.Error("create-rep-client-failed", err)
				return
			}

			lrpUpdate := models.NewLRPUpdate(dereferencedLRPKey.InstanceKey.InstanceGuid, *dereferencedLRPKey.Key, nil, lrpKey.DesiredMetricTags)
			err = repClient.UpdateLRPInstance(logger, lrpUpdate)
			if err != nil {
				logger.Error("updating-lrp-instance", err)
			}
		})
	}

	var throttler *workpool.Throttler
	throttler, err = workpool.NewThrottler(h.convergenceWorkersSize, works)
	if err != nil {
		logger.Error("failed-constructing-throttler", err, lager.Data{"max_workers": h.convergenceWorkersSize, "num_works": len(works)})
		return
	}

	retireLogger.Debug("retiring-actual-lrps")
	throttler.Work()
	retireLogger.Debug("done-retiring-actual-lrps")
}
