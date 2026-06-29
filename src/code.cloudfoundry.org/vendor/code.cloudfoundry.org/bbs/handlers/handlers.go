package handlers

import (
	"io"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/cmd/bbs/config"
	"code.cloudfoundry.org/bbs/controllers"
	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/handlers/middleware"
	"code.cloudfoundry.org/bbs/metrics"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/serviceclient"
	"code.cloudfoundry.org/bbs/taskworkpool"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"
	"github.com/gogo/protobuf/proto"
	"github.com/tedsuo/rata"
)

func New(
	logger,
	accessLogger lager.Logger,
	updateWorkers int,
	convergenceWorkersSize int,
	maxTaskPlacementRetries int,
	advancedMetricsConfig config.AdvancedMetrics,
	emitter middleware.Emitter,
	db db.DB,
	desiredHub, actualHub, actualLRPInstanceHub, taskHub events.Hub,
	taskCompletionClient taskworkpool.TaskCompletionClient,
	serviceClient serviceclient.ServiceClient,
	auctioneerClient auctioneer.Client,
	repClientFactory rep.ClientFactory,
	taskStatMetronNotifier metrics.TaskStatMetronNotifier,
	migrationsDone <-chan struct{},
	exitChan chan struct{},
	metronClient loggingclient.IngressClient,
) http.Handler {
	pingHandler := NewPingHandler()
	domainHandler := NewDomainHandler(db, exitChan)
	actualLRPHandler := NewActualLRPHandler(db, exitChan)
	actualLRPController := controllers.NewActualLRPLifecycleController(
		db, db, db, db,
		auctioneerClient,
		serviceClient,
		repClientFactory,
		actualHub,
		actualLRPInstanceHub,
	)
	evacuationController := controllers.NewEvacuationController(
		db, db, db, db,
		auctioneerClient,
		actualHub,
		actualLRPInstanceHub,
	)
	actualLRPLifecycleHandler := NewActualLRPLifecycleHandler(actualLRPController, exitChan)
	evacuationHandler := NewEvacuationHandler(evacuationController, exitChan)
	desiredLRPHandler := NewDesiredLRPHandler(updateWorkers, db, db, desiredHub, actualHub, actualLRPInstanceHub, auctioneerClient, repClientFactory, serviceClient, exitChan, metronClient)
	taskController := controllers.NewTaskController(db, taskCompletionClient, auctioneerClient, serviceClient, repClientFactory, taskHub, taskStatMetronNotifier, maxTaskPlacementRetries)
	taskHandler := NewTaskHandler(taskController, exitChan)
	lrpGroupEventsHandler := NewLRPGroupEventsHandler(desiredHub, actualHub)
	taskEventsHandler := NewTaskEventHandler(taskHub)
	lrpInstanceEventsHandler := NewLRPInstanceEventHandler(desiredHub, actualLRPInstanceHub)
	cellsHandler := NewCellHandler(serviceClient, exitChan)

	metricsAndLoggingWrap := func(loggableHandlerFunc middleware.LoggableHandlerFunc, routeName string) http.HandlerFunc {
		return middleware.RecordMetrics(middleware.LogWrap(logger, accessLogger, loggableHandlerFunc), emitter, advancedMetricsConfig, routeName)
	}

	actions := rata.Handlers{
		// Ping
		bbs.PingRoute_r0: metricsAndLoggingWrap(pingHandler.Ping, bbs.PingRoute_r0),

		// Domains
		bbs.DomainsRoute_r0:      metricsAndLoggingWrap(domainHandler.Domains, bbs.DomainsRoute_r0),
		bbs.UpsertDomainRoute_r0: metricsAndLoggingWrap(domainHandler.Upsert, bbs.UpsertDomainRoute_r0),

		// Actual LRPs
		bbs.ActualLRPsRoute_r0: metricsAndLoggingWrap(actualLRPHandler.ActualLRPs, bbs.ActualLRPsRoute_r0),
		// Multiple Actual LRPs by multiple Process GUIDs
		bbs.ActualLRPsByProcessGuidsRoute_r0: metricsAndLoggingWrap(actualLRPHandler.ActualLRPsByProcessGuids, bbs.ActualLRPsByProcessGuidsRoute_r0),
		//lint:ignore SA1019 - implementing deprecated logic until it is removed
		bbs.ActualLRPGroupsRoute_r0: metricsAndLoggingWrap(actualLRPHandler.ActualLRPGroups, bbs.ActualLRPGroupsRoute_r0),
		//lint:ignore SA1019 - implementing deprecated logic until it is removed
		bbs.ActualLRPGroupsByProcessGuidRoute_r0: metricsAndLoggingWrap(actualLRPHandler.ActualLRPGroupsByProcessGuid, bbs.ActualLRPGroupsByProcessGuidRoute_r0),
		//lint:ignore SA1019 - implementing deprecated logic until it is removed
		bbs.ActualLRPGroupByProcessGuidAndIndexRoute_r0: metricsAndLoggingWrap(actualLRPHandler.ActualLRPGroupByProcessGuidAndIndex, bbs.ActualLRPGroupByProcessGuidAndIndexRoute_r0),

		// Actual LRP Lifecycle
		bbs.ClaimActualLRPRoute_r0: metricsAndLoggingWrap(actualLRPLifecycleHandler.ClaimActualLRP, bbs.ClaimActualLRPRoute_r0),
		//lint:ignore SA1019 - implementing deprecated logic until it is removed
		bbs.StartActualLRPRoute_r0:  metricsAndLoggingWrap(actualLRPLifecycleHandler.StartActualLRP_r0, bbs.StartActualLRPRoute_r0), // DEPRECATED
		bbs.StartActualLRPRoute_r1:  metricsAndLoggingWrap(actualLRPLifecycleHandler.StartActualLRP, bbs.StartActualLRPRoute_r1),
		bbs.CrashActualLRPRoute_r0:  metricsAndLoggingWrap(actualLRPLifecycleHandler.CrashActualLRP, bbs.CrashActualLRPRoute_r0),
		bbs.RetireActualLRPRoute_r0: metricsAndLoggingWrap(actualLRPLifecycleHandler.RetireActualLRP, bbs.RetireActualLRPRoute_r0),
		bbs.FailActualLRPRoute_r0:   metricsAndLoggingWrap(actualLRPLifecycleHandler.FailActualLRP, bbs.FailActualLRPRoute_r0),
		bbs.RemoveActualLRPRoute_r0: metricsAndLoggingWrap(actualLRPLifecycleHandler.RemoveActualLRP, bbs.RemoveActualLRPRoute_r0),

		// Evacuation
		bbs.RemoveEvacuatingActualLRPRoute_r0: metricsAndLoggingWrap(evacuationHandler.RemoveEvacuatingActualLRP, bbs.RemoveEvacuatingActualLRPRoute_r0),
		bbs.EvacuateClaimedActualLRPRoute_r0:  metricsAndLoggingWrap(evacuationHandler.EvacuateClaimedActualLRP, bbs.EvacuateClaimedActualLRPRoute_r0),
		bbs.EvacuateCrashedActualLRPRoute_r0:  metricsAndLoggingWrap(evacuationHandler.EvacuateCrashedActualLRP, bbs.EvacuateCrashedActualLRPRoute_r0),
		bbs.EvacuateStoppedActualLRPRoute_r0:  metricsAndLoggingWrap(evacuationHandler.EvacuateStoppedActualLRP, bbs.EvacuateStoppedActualLRPRoute_r0),
		//lint:ignore SA1019 - implementing deprecated logic until it is removed
		bbs.EvacuateRunningActualLRPRoute_r0: metricsAndLoggingWrap(evacuationHandler.EvacuateRunningActualLRP_r0, bbs.EvacuateRunningActualLRPRoute_r0), // DEPRECATED
		bbs.EvacuateRunningActualLRPRoute_r1: metricsAndLoggingWrap(evacuationHandler.EvacuateRunningActualLRP, bbs.EvacuateRunningActualLRPRoute_r1),

		// Desired LRPs
		bbs.DesiredLRPsRoute_r3:             metricsAndLoggingWrap(desiredLRPHandler.DesiredLRPs, bbs.DesiredLRPsRoute_r3),
		bbs.DesiredLRPByProcessGuidRoute_r3: metricsAndLoggingWrap(desiredLRPHandler.DesiredLRPByProcessGuid, bbs.DesiredLRPByProcessGuidRoute_r3),
		//lint:ignore SA1019 - implementing deprecated logic until it is removed
		bbs.DesiredLRPsRoute_r2: metricsAndLoggingWrap(desiredLRPHandler.DesiredLRPs_r2, bbs.DesiredLRPsRoute_r2), // DEPRECATED
		//lint:ignore SA1019 - implementing deprecated logic until it is removed
		bbs.DesiredLRPByProcessGuidRoute_r2:          metricsAndLoggingWrap(desiredLRPHandler.DesiredLRPByProcessGuid_r2, bbs.DesiredLRPByProcessGuidRoute_r2), // DEPRECATED
		bbs.DesiredLRPSchedulingInfosRoute_r0:        metricsAndLoggingWrap(desiredLRPHandler.DesiredLRPSchedulingInfos, bbs.DesiredLRPSchedulingInfosRoute_r0),
		bbs.DesiredLRPSchedulingInfoByProcessGuid_r0: metricsAndLoggingWrap(desiredLRPHandler.DesiredLRPSchedulingInfoByProcessGuid, bbs.DesiredLRPSchedulingInfoByProcessGuid_r0),
		bbs.DesiredLRPRoutingInfosRoute_r0:           metricsAndLoggingWrap(desiredLRPHandler.DesiredLRPRoutingInfos, bbs.DesiredLRPRoutingInfosRoute_r0),
		bbs.DesireDesiredLRPRoute_r2:                 metricsAndLoggingWrap(desiredLRPHandler.DesireDesiredLRP, bbs.DesireDesiredLRPRoute_r2),
		bbs.UpdateDesiredLRPRoute_r0:                 metricsAndLoggingWrap(desiredLRPHandler.UpdateDesiredLRP, bbs.UpdateDesiredLRPRoute_r0),
		bbs.RemoveDesiredLRPRoute_r0:                 metricsAndLoggingWrap(desiredLRPHandler.RemoveDesiredLRP, bbs.RemoveDesiredLRPRoute_r0),

		// Tasks
		//lint:ignore SA1019 - implementing deprecated logic until it is removed
		bbs.TasksRoute_r2: metricsAndLoggingWrap(taskHandler.Tasks_r2, bbs.TasksRoute_r2), // DEPRECATED
		//lint:ignore SA1019 - implementing deprecated logic until it is removed
		bbs.TaskByGuidRoute_r2: metricsAndLoggingWrap(taskHandler.TaskByGuid_r2, bbs.TaskByGuidRoute_r2), // DEPRECATED
		bbs.TasksRoute_r3:      metricsAndLoggingWrap(taskHandler.Tasks, bbs.TasksRoute_r3),
		bbs.TaskByGuidRoute_r3: metricsAndLoggingWrap(taskHandler.TaskByGuid, bbs.TaskByGuidRoute_r3),
		bbs.DesireTaskRoute_r2: metricsAndLoggingWrap(taskHandler.DesireTask, bbs.DesireTaskRoute_r2),
		bbs.StartTaskRoute_r0:  metricsAndLoggingWrap(taskHandler.StartTask, bbs.StartTaskRoute_r0),
		bbs.CancelTaskRoute_r0: metricsAndLoggingWrap(taskHandler.CancelTask, bbs.CancelTaskRoute_r0),
		//lint:ignore SA1019 - implementing deprecated logic until it is removed
		bbs.FailTaskRoute_r0:      metricsAndLoggingWrap(taskHandler.FailTask, bbs.FailTaskRoute_r0), // DEPRECATED
		bbs.RejectTaskRoute_r0:    metricsAndLoggingWrap(taskHandler.RejectTask, bbs.RejectTaskRoute_r0),
		bbs.CompleteTaskRoute_r0:  metricsAndLoggingWrap(taskHandler.CompleteTask, bbs.CompleteTaskRoute_r0),
		bbs.ResolvingTaskRoute_r0: metricsAndLoggingWrap(taskHandler.ResolvingTask, bbs.ResolvingTaskRoute_r0),
		bbs.DeleteTaskRoute_r0:    metricsAndLoggingWrap(taskHandler.DeleteTask, bbs.DeleteTaskRoute_r0),

		// Events
		//lint:ignore SA1019 - implementing deprecated logic until it is removed
		bbs.EventStreamRoute_r0: middleware.RecordRequestCount(middleware.LogWrap(logger, accessLogger, lrpGroupEventsHandler.Subscribe_r0), emitter), // DEPRECATED
		//lint:ignore SA1019 - implementing deprecated logic until it is removed
		bbs.TaskEventStreamRoute_r0: middleware.RecordRequestCount(middleware.LogWrap(logger, accessLogger, taskEventsHandler.Subscribe_r0), emitter), // DEPRECATED
		//lint:ignore SA1019 - implementing deprecated logic until it is removed
		bbs.LrpInstanceEventStreamRoute_r0: middleware.RecordRequestCount(middleware.LogWrap(logger, accessLogger, lrpInstanceEventsHandler.Subscribe_r0), emitter), // DEPRECATED
		bbs.LRPGroupEventStreamRoute_r1:    middleware.RecordRequestCount(middleware.LogWrap(logger, accessLogger, lrpGroupEventsHandler.Subscribe_r1), emitter),
		bbs.TaskEventStreamRoute_r1:        middleware.RecordRequestCount(middleware.LogWrap(logger, accessLogger, taskEventsHandler.Subscribe_r1), emitter),
		bbs.LRPInstanceEventStreamRoute_r1: middleware.RecordRequestCount(middleware.LogWrap(logger, accessLogger, lrpInstanceEventsHandler.Subscribe_r1), emitter),

		// Cells
		bbs.CellsRoute_r0: metricsAndLoggingWrap(cellsHandler.Cells, bbs.CellsRoute_r0),
	}

	handler, err := rata.NewRouter(bbs.Routes, actions)
	if err != nil {
		panic("unable to create router: " + err.Error())
	}

	return UnavailableWrap(handler,
		migrationsDone,
	)
}

func parseRequest(logger lager.Logger, req *http.Request, request MessageValidator) error {
	data, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Error("failed-to-read-body", err)
		return models.ErrUnknownError
	}

	err = request.Unmarshal(data)
	if err != nil {
		logger.Error("failed-to-parse-request-body", err)
		return models.ErrBadRequest
	}

	if err := request.Validate(); err != nil {
		logger.Error("invalid-request", err)
		return models.NewError(models.Error_InvalidRequest, err.Error())
	}

	return nil
}

func exitIfUnrecoverable(logger lager.Logger, exitCh chan<- struct{}, err *models.Error) {
	if err != nil && err.Type == models.Error_Unrecoverable {
		logger.Error("unrecoverable-error", err)
		select {
		case exitCh <- struct{}{}:
		default:
		}
	}
}

func writeResponse(w http.ResponseWriter, message proto.Message) {
	responseBytes, err := proto.Marshal(message)
	if err != nil {
		panic("Unable to encode Proto: " + err.Error())
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(responseBytes)))
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	// #nosec G104 - ignore errors when writing HTTP responses so we don't spam our logs during a DoS
	w.Write(responseBytes)
}
