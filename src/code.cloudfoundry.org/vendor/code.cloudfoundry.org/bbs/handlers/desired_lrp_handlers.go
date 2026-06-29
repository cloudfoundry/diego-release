package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/events/calculator"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/serviceclient"
	"code.cloudfoundry.org/bbs/trace"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/workpool"
)

const BbsLogSource = "DIEGO-API"

type DesiredLRPHandler struct {
	desiredLRPDB         db.DesiredLRPDB
	actualLRPDB          db.ActualLRPDB
	desiredHub           events.Hub
	actualHub            events.Hub
	actualLRPInstanceHub events.Hub
	auctioneerClient     auctioneer.Client
	repClientFactory     rep.ClientFactory
	serviceClient        serviceclient.ServiceClient
	updateWorkersCount   int
	exitChan             chan<- struct{}
	metronClient         loggingclient.IngressClient
}

func NewDesiredLRPHandler(
	updateWorkersCount int,
	desiredLRPDB db.DesiredLRPDB,
	actualLRPDB db.ActualLRPDB,
	desiredHub events.Hub,
	actualHub events.Hub,
	actualLRPInstanceHub events.Hub,
	auctioneerClient auctioneer.Client,
	repClientFactory rep.ClientFactory,
	serviceClient serviceclient.ServiceClient,
	exitChan chan<- struct{},
	metronClient loggingclient.IngressClient,
) *DesiredLRPHandler {
	return &DesiredLRPHandler{
		desiredLRPDB:         desiredLRPDB,
		actualLRPDB:          actualLRPDB,
		desiredHub:           desiredHub,
		actualHub:            actualHub,
		actualLRPInstanceHub: actualLRPInstanceHub,
		auctioneerClient:     auctioneerClient,
		repClientFactory:     repClientFactory,
		serviceClient:        serviceClient,
		updateWorkersCount:   updateWorkersCount,
		exitChan:             exitChan,
		metronClient:         metronClient,
	}
}

func (h *DesiredLRPHandler) commonDesiredLRPs(logger lager.Logger, targetVersion format.Version, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("desired-lrps").WithTraceInfo(req)

	request := &models.DesiredLRPsRequest{}
	response := &models.DesiredLRPsResponse{}

	err = parseRequest(logger, req, request)
	if err == nil {
		filter := models.DesiredLRPFilter{Domain: request.Domain, ProcessGuids: request.ProcessGuids, AppGuids: request.AppGuids}

		var desiredLRPs []*models.DesiredLRP
		desiredLRPs, err = h.desiredLRPDB.DesiredLRPs(req.Context(), logger, filter)
		for i, d := range desiredLRPs {
			desiredLRPs[i] = d.VersionDownTo(targetVersion).PopulateMetricsGuid()
			if len(desiredLRPs[i].CachedDependencies) == 0 {
				desiredLRPs[i].CachedDependencies = nil
			}
		}

		response.DesiredLrps = desiredLRPs
	}

	response.Error = models.ConvertError(err)
	writeResponse(w, response)
	exitIfUnrecoverable(logger, h.exitChan, response.Error)

}

func (h *DesiredLRPHandler) DesiredLRPs(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonDesiredLRPs(logger, format.V3, w, req)
}

func (h *DesiredLRPHandler) DesiredLRPs_r2(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonDesiredLRPs(logger, format.V2, w, req)
}

func (h *DesiredLRPHandler) commonDesiredLRPByProcessGuid(logger lager.Logger, targetVersion format.Version, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("desired-lrp-by-process-guid").WithTraceInfo(req)

	request := &models.DesiredLRPByProcessGuidRequest{}
	response := &models.DesiredLRPResponse{}

	err = parseRequest(logger, req, request)
	if err == nil {
		var desiredLRP *models.DesiredLRP
		desiredLRP, err = h.desiredLRPDB.DesiredLRPByProcessGuid(req.Context(), logger, request.ProcessGuid)
		if desiredLRP != nil {
			desiredLRP = desiredLRP.VersionDownTo(targetVersion).PopulateMetricsGuid()
		}
		response.DesiredLrp = desiredLRP
	}

	response.Error = models.ConvertError(err)
	writeResponse(w, response)
	exitIfUnrecoverable(logger, h.exitChan, response.Error)

}

func (h *DesiredLRPHandler) DesiredLRPByProcessGuid(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonDesiredLRPByProcessGuid(logger, format.V3, w, req)
}

func (h *DesiredLRPHandler) DesiredLRPByProcessGuid_r2(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonDesiredLRPByProcessGuid(logger, format.V2, w, req)
}

func (h *DesiredLRPHandler) DesiredLRPSchedulingInfos(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("desired-lrp-scheduling-infos").WithTraceInfo(req)
	logger.Debug("starting")
	defer logger.Debug("complete")

	request := &models.DesiredLRPsRequest{}
	response := &models.DesiredLRPSchedulingInfosResponse{}

	err = parseRequest(logger, req, request)
	if err == nil {
		filter := models.DesiredLRPFilter{
			Domain:       request.Domain,
			ProcessGuids: request.ProcessGuids,
			AppGuids:     request.AppGuids,
		}
		response.DesiredLrpSchedulingInfos, err = h.desiredLRPDB.DesiredLRPSchedulingInfos(req.Context(), logger, filter)
	}

	response.Error = models.ConvertError(err)
	writeResponse(w, response)
	exitIfUnrecoverable(logger, h.exitChan, response.Error)
}

func (h *DesiredLRPHandler) DesiredLRPSchedulingInfoByProcessGuid(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("desired-lrp-scheduling-info-by-process-guid").WithTraceInfo(req)
	logger.Debug("starting")
	defer logger.Debug("complete")

	request := &models.DesiredLRPByProcessGuidRequest{}
	response := &models.DesiredLRPSchedulingInfoByProcessGuidResponse{}

	err = parseRequest(logger, req, request)
	if err == nil {
		response.DesiredLrpSchedulingInfo, err = h.desiredLRPDB.DesiredLRPSchedulingInfoByProcessGuid(req.Context(), logger, request.ProcessGuid)
	}

	response.Error = models.ConvertError(err)
	writeResponse(w, response)
	exitIfUnrecoverable(logger, h.exitChan, response.Error)
}

func (h *DesiredLRPHandler) DesiredLRPRoutingInfos(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("desired-lrp-routing-infos")
	logger.Debug("starting")
	defer logger.Debug("complete")

	request := &models.DesiredLRPsRequest{}
	response := &models.DesiredLRPsResponse{}

	err = parseRequest(logger, req, request)
	if err == nil {
		filter := models.DesiredLRPFilter{
			Domain:       request.Domain,
			ProcessGuids: request.ProcessGuids,
		}
		response.DesiredLrps, err = h.desiredLRPDB.DesiredLRPRoutingInfos(req.Context(), logger, filter)
	}

	response.Error = models.ConvertError(err)
	writeResponse(w, response)
	exitIfUnrecoverable(logger, h.exitChan, response.Error)
}

func (h *DesiredLRPHandler) DesireDesiredLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("desire-lrp").WithTraceInfo(req)

	request := &models.DesireLRPRequest{}
	response := &models.DesiredLRPLifecycleResponse{}
	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err := parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		if err = h.logDesiredLrpParsingErrors(response.Error, request.GetDesiredLrp().GetProcessGuid()); err != nil {
			logger.Error("failed-sending-app-logs", err)
		}
		return
	}

	err = h.desiredLRPDB.DesireLRP(req.Context(), logger, request.DesiredLrp)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	desiredLRP, err := h.desiredLRPDB.DesiredLRPByProcessGuid(req.Context(), logger, request.DesiredLrp.ProcessGuid)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	go h.desiredHub.Emit(models.NewDesiredLRPCreatedEvent(desiredLRP, trace.RequestIdFromRequest(req)))

	schedulingInfo := request.DesiredLrp.DesiredLRPSchedulingInfo()
	if schedulingInfo.Instances > 0 {
		h.startInstanceRange(trace.ContextWithRequestId(req), logger, 0, schedulingInfo.Instances, &schedulingInfo)
	}
}

func (h *DesiredLRPHandler) UpdateDesiredLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("update-desired-lrp").WithTraceInfo(req)

	request := &models.UpdateDesiredLRPRequest{}
	response := &models.DesiredLRPLifecycleResponse{}
	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err := parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		if err = h.logDesiredLrpParsingErrors(response.Error, request.GetProcessGuid()); err != nil {
			logger.Error("failed-sending-app-logs", err)
		}
		return
	}

	logger = logger.WithData(lager.Data{"guid": request.ProcessGuid})

	logger.Debug("updating-desired-lrp")
	beforeDesiredLRP, err := h.desiredLRPDB.UpdateDesiredLRP(req.Context(), logger, request.ProcessGuid, request.Update)
	if err != nil {
		logger.Debug("failed-updating-desired-lrp")
		response.Error = models.ConvertError(err)
		return
	}
	logger.Debug("completed-updating-desired-lrp")

	desiredLRP, err := h.desiredLRPDB.DesiredLRPByProcessGuid(req.Context(), logger, request.ProcessGuid)
	if err != nil {
		logger.Error("failed-fetching-desired-lrp", err)
		return
	}

	if request.Update.InstancesExists() {
		logger.Debug("updating-lrp-instances")
		previousInstanceCount := beforeDesiredLRP.Instances

		requestedInstances := request.Update.GetInstances() - previousInstanceCount

		logger = logger.WithData(lager.Data{"instances_delta": requestedInstances})
		if requestedInstances > 0 {
			logger.Debug("increasing-the-instances")
			schedulingInfo := desiredLRP.DesiredLRPSchedulingInfo()
			h.startInstanceRange(trace.ContextWithRequestId(req), logger, previousInstanceCount, request.Update.GetInstances(), &schedulingInfo)
		}

		if requestedInstances < 0 {
			logger.Debug("decreasing-the-instances")
			numExtraActualLRP := previousInstanceCount + requestedInstances
			h.stopInstancesFrom(trace.ContextWithRequestId(req), logger, request.ProcessGuid, int(numExtraActualLRP))
		}
	}

	internalRoutesUpdated := request.Update.IsRoutesGroupUpdated(beforeDesiredLRP.Routes, models.InternalRouter)
	metricTagsUpdated := request.Update.IsMetricTagsUpdated(beforeDesiredLRP.MetricTags)

	if internalRoutesUpdated || metricTagsUpdated {
		h.updateInstances(trace.ContextWithRequestId(req), logger, request.ProcessGuid, request.Update, internalRoutesUpdated, metricTagsUpdated)
	}

	go h.desiredHub.Emit(models.NewDesiredLRPChangedEvent(beforeDesiredLRP, desiredLRP, trace.RequestIdFromRequest(req)))
}

func (h *DesiredLRPHandler) RemoveDesiredLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("remove-desired-lrp").WithTraceInfo(req)

	request := &models.RemoveDesiredLRPRequest{}
	response := &models.DesiredLRPLifecycleResponse{}
	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err := parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}
	logger = logger.WithData(lager.Data{"process_guid": request.ProcessGuid})

	desiredLRP, err := h.desiredLRPDB.DesiredLRPByProcessGuid(req.Context(), logger.Session("fetch-desired"), request.ProcessGuid)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	err = h.desiredLRPDB.RemoveDesiredLRP(req.Context(), logger.Session("remove-desired"), request.ProcessGuid)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	go h.desiredHub.Emit(models.NewDesiredLRPRemovedEvent(desiredLRP, trace.RequestIdFromRequest(req)))

	h.stopInstancesFrom(trace.ContextWithRequestId(req), logger, request.ProcessGuid, 0)
}

func (h *DesiredLRPHandler) startInstanceRange(ctx context.Context, logger lager.Logger, lower, upper int32, schedulingInfo *models.DesiredLRPSchedulingInfo) {
	logger = logger.Session("start-instance-range", lager.Data{"lower": lower, "upper": upper})
	logger.Info("starting")
	defer logger.Info("complete")

	keys := []*models.ActualLRPKey{}
	for actualIndex := lower; actualIndex < upper; actualIndex++ {
		key := models.NewActualLRPKey(schedulingInfo.ProcessGuid, int32(actualIndex), schedulingInfo.Domain)
		keys = append(keys, &key)
	}

	createdIndices := h.createUnclaimedActualLRPs(ctx, logger, keys)
	start := auctioneer.NewLRPStartRequestFromSchedulingInfo(schedulingInfo, createdIndices...)

	logger.Info("start-lrp-auction-request", lager.Data{"app_guid": schedulingInfo.ProcessGuid, "indices": createdIndices})
	err := h.auctioneerClient.RequestLRPAuctions(logger, trace.RequestIdFromContext(ctx), []*auctioneer.LRPStartRequest{&start})
	logger.Info("finished-lrp-auction-request", lager.Data{"app_guid": schedulingInfo.ProcessGuid, "indices": createdIndices})
	if err != nil {
		logger.Error("failed-to-request-auction", err)
	}
}

func (h *DesiredLRPHandler) createUnclaimedActualLRPs(ctx context.Context, logger lager.Logger, keys []*models.ActualLRPKey) []int {
	count := len(keys)
	createdIndicesChan := make(chan int, count)

	eventCalculator := calculator.ActualLRPEventCalculator{
		ActualLRPGroupHub:    h.actualHub,
		ActualLRPInstanceHub: h.actualLRPInstanceHub,
	}

	works := make([]func(), count)
	logger = logger.Session("create-unclaimed-actual-lrp")
	for i, key := range keys {
		key := key
		works[i] = func() {
			logger.Info("starting", lager.Data{"actual_lrp_key": key})
			actualLRP, err := h.actualLRPDB.CreateUnclaimedActualLRP(ctx, logger, key)
			if err != nil {
				logger.Info("failed", lager.Data{"actual_lrp_key": key, "err_message": err.Error()})
				return
			}

			lrps := eventCalculator.RecordChange(nil, actualLRP, nil)
			go eventCalculator.EmitEvents(trace.RequestIdFromContext(ctx), nil, lrps)
			createdIndicesChan <- int(key.Index)
		}
	}

	throttlerSize := h.updateWorkersCount
	throttler, err := workpool.NewThrottler(throttlerSize, works)
	if err != nil {
		logger.Error("failed-constructing-throttler", err, lager.Data{"max_workers": throttlerSize, "num_works": len(works)})
		return []int{}
	}

	go func() {
		throttler.Work()
		close(createdIndicesChan)
	}()

	createdIndices := make([]int, 0, count)
	for createdIndex := range createdIndicesChan {
		createdIndices = append(createdIndices, createdIndex)
	}

	return createdIndices
}

func (h *DesiredLRPHandler) stopInstancesFrom(ctx context.Context, logger lager.Logger, processGuid string, index int) {
	logger = logger.Session("stop-instances-from", lager.Data{"process_guid": processGuid, "index": index})
	actualLRPs, err := h.actualLRPDB.ActualLRPs(ctx, logger.Session("fetch-actuals"), models.ActualLRPFilter{ProcessGuid: processGuid})
	if err != nil {
		logger.Error("failed-fetching-actual-lrps", err)
		return
	}

	for i := 0; i < len(actualLRPs); i++ {
		lrp := actualLRPs[i]

		if lrp.Presence != models.ActualLRP_Evacuating {
			if lrp.Index >= int32(index) {
				switch lrp.State {
				case models.ActualLRPStateUnclaimed, models.ActualLRPStateCrashed:
					err = h.actualLRPDB.RemoveActualLRP(ctx, logger.Session("remove-actual"), lrp.ProcessGuid, lrp.Index, nil)
					if err != nil {
						logger.Error("failed-removing-lrp-instance", err)
					} else {
						//lint:ignore SA1019 - implementing deprecated logic until it is removed
						go h.actualHub.Emit(models.NewActualLRPRemovedEvent(lrp.ToActualLRPGroup()))
						go h.actualLRPInstanceHub.Emit(models.NewActualLRPInstanceRemovedEvent(lrp, trace.RequestIdFromContext(ctx)))
					}
				default:
					cellPresence, err := h.serviceClient.CellById(logger, lrp.CellId)
					if err != nil {
						logger.Error("failed-fetching-cell-presence", err)
						continue
					}
					repClient, err := h.repClientFactory.CreateClient(cellPresence.RepAddress, cellPresence.RepUrl, trace.RequestIdFromContext(ctx))
					if err != nil {
						logger.Error("create-rep-client-failed", err)
						continue
					}
					logger.Debug("stopping-lrp-instance")
					go func() {
						err := repClient.StopLRPInstance(logger, lrp.ActualLRPKey, lrp.ActualLRPInstanceKey)
						if err != nil {
							logger.Error("failed-stopping-lrp-instance", err)
						}
					}()
				}
			}
		}
	}
}

func (h *DesiredLRPHandler) updateInstances(ctx context.Context, logger lager.Logger, processGuid string, update *models.DesiredLRPUpdate, internalRoutesUpdated, metricTagsUpdated bool) {
	logger = logger.Session("updating-instances", lager.Data{"process_guid": processGuid})
	actualLRPs, err := h.actualLRPDB.ActualLRPs(ctx, logger.Session("fetch-actuals"), models.ActualLRPFilter{ProcessGuid: processGuid})
	if err != nil {
		logger.Error("failed-fetching-actual-lrps", err)
		return
	}

	for i := 0; i < len(actualLRPs); i++ {
		lrp := actualLRPs[i]

		if lrp.Presence != models.ActualLRP_Evacuating && lrp.State != models.ActualLRPStateUnclaimed && lrp.State != models.ActualLRPStateCrashed {
			cellPresence, err := h.serviceClient.CellById(logger, lrp.CellId)
			if err != nil {
				logger.Error("failed-fetching-cell-presence", err)
				continue
			}
			repClient, err := h.repClientFactory.CreateClient(cellPresence.RepAddress, cellPresence.RepUrl, trace.RequestIdFromContext(ctx))
			if err != nil {
				logger.Error("create-rep-client-failed", err)
				continue
			}
			logger.Debug("updating-lrp-instance")

			var internalRoutes models.InternalRoutes
			if internalRoutesUpdated {
				internalRoutes, err = models.InternalRoutesFromRoutingInfo(*update.Routes)
				if err != nil {
					logger.Error("getting-internal-routes-failed", err)
					continue
				}
			}

			var metricTags map[string]string
			if metricTagsUpdated {
				metricTags, err = models.ConvertMetricTags(update.MetricTags, map[models.MetricTagValue_DynamicValue]interface{}{
					models.MetricTagDynamicValueIndex:        lrp.Index,
					models.MetricTagDynamicValueInstanceGuid: lrp.InstanceGuid,
				})
				if err != nil {
					logger.Error("converting-metric-tags-failed", err)
					continue
				}
			}

			lrpUpdate := rep.NewLRPUpdate(lrp.ActualLRPInstanceKey.InstanceGuid, lrp.ActualLRPKey, internalRoutes, metricTags)
			go func() {
				err := repClient.UpdateLRPInstance(logger, lrpUpdate)
				if err != nil {
					logger.Error("updating-lrp-instance", err)
				}
			}()
		}
	}
}

func (h *DesiredLRPHandler) logDesiredLrpParsingErrors(err *models.Error, processGuid string) error {
	appGuid := parseAppGuidFromProcessGuid(processGuid)
	if appGuid == "" {
		return errors.New("app guid is empty")
	}

	tags := map[string]string{
		"source_id": appGuid,
	}

	return h.metronClient.SendAppErrorLog(
		fmt.Sprintf("Error parsing request for app with guid %s, %s, %s", appGuid, err.GetType(), err.GetMessage()),
		BbsLogSource,
		tags)
}

// Parses a ProcessGuid which is in the format: 'UUID-UUID' and returns the first part which is the AppGuid
func parseAppGuidFromProcessGuid(processGuid string) string {
	const uuidParts = 5 // a valid UUID contains 5 parts separated by '-'

	parts := strings.Split(processGuid, "-")
	if len(parts) < uuidParts {
		return ""
	}

	return strings.Join(parts[:uuidParts], "-")
}
