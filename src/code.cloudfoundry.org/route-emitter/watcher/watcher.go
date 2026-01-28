package watcher

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
)

const (
	routeSyncDuration = "RouteEmitterSyncDuration"
)

//go:generate counterfeiter -o fakes/fake_routehandler.go . RouteHandler
type RouteHandler interface {
	HandleEvent(logger lager.Logger, event models.Event)
	Sync(
		logger lager.Logger,
		desired []*models.DesiredLRP,
		runningActual []*models.ActualLRP,
		domains models.DomainSet,
		cachedEvents map[string]models.Event,
	)
	EmitExternal(logger lager.Logger)
	EmitInternal(logger lager.Logger)
	ShouldRefreshDesired(*models.ActualLRP) bool
	RefreshDesired(lager.Logger, []*models.DesiredLRP)
}

type Watcher struct {
	cellID         string
	bbsClient      bbs.Client
	clock          clock.Clock
	routeHandler   RouteHandler
	syncCh         chan struct{}
	emitExternalCh chan struct{}
	emitInternalCh chan struct{}
	logger         lager.Logger
	metronClient   loggingclient.IngressClient
}

func NewWatcher(
	cellID string,
	bbsClient bbs.Client,
	clock clock.Clock,
	routeHandler RouteHandler,
	syncCh chan struct{},
	emitExternalCh chan struct{},
	emitInternalCh chan struct{},
	logger lager.Logger,
	metronClient loggingclient.IngressClient,
) *Watcher {
	return &Watcher{
		cellID:         cellID,
		bbsClient:      bbsClient,
		clock:          clock,
		routeHandler:   routeHandler,
		syncCh:         syncCh,
		emitExternalCh: emitExternalCh,
		emitInternalCh: emitInternalCh,
		logger:         logger.Session("watcher"),
		metronClient:   metronClient,
	}
}

type syncEventResult struct {
	startTime     time.Time
	desired       []*models.DesiredLRP
	runningActual []*models.ActualLRP
	domains       models.DomainSet
	err           error
}

func (watcher *Watcher) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	watcher.logger.Debug("starting", lager.Data{"cell-id": watcher.cellID})
	defer watcher.logger.Debug("finished")

	eventChan := make(chan models.Event)
	resubscribeChannel := make(chan error)

	eventSource := &atomic.Value{}
	var stopEventSource int32

	go watcher.checkForEvents(resubscribeChannel, eventChan, eventSource, watcher.logger)
	watcher.logger.Debug("listening-on-channels")
	close(ready)
	watcher.logger.Debug("started")

	cachedEvents := make(map[string]models.Event)
	syncEnd := make(chan *syncEventResult)
	syncing := false

	for {
		select {
		case event := <-eventChan:
			if syncing {
				watcher.logger.Info("caching-event", lager.Data{
					"type": event.EventType(),
				})
				cachedEvents[event.Key()] = event
				continue
			}
			logger := watcher.logger.Session("handling-event")
			watcher.handleEvent(logger, event)
		case <-watcher.emitExternalCh:
			logger := watcher.logger.Session("emit-external")
			watcher.routeHandler.EmitExternal(logger)
		case <-watcher.emitInternalCh:
			logger := watcher.logger.Session("emit-internal")
			watcher.routeHandler.EmitInternal(logger)
		case syncEvent := <-syncEnd:
			syncing = false
			logger := watcher.logger.Session("sync")
			if syncEvent.err != nil {
				logger.Error("failed-to-sync-events", syncEvent.err)
				continue
			}

			var cachedDesired []*models.DesiredLRP
			for _, e := range cachedEvents {
				desired := watcher.retrieveDesiredWhileSyncing(logger, e, syncEvent.desired)
				if len(desired) > 0 {
					cachedDesired = append(cachedDesired, desired...)
				}
			}

			if len(cachedDesired) > 0 {
				syncEvent.desired = append(syncEvent.desired, cachedDesired...)
			}

			logger.Debug("calling-handler-sync")
			watcher.routeHandler.Sync(logger,
				syncEvent.desired,
				syncEvent.runningActual,
				syncEvent.domains,
				cachedEvents,
			)

			after := watcher.clock.Now()
			if err := watcher.metronClient.SendDuration(routeSyncDuration, after.Sub(syncEvent.startTime)); err != nil {
				watcher.logger.Error("failed-to-send-route-sync-duration-metric", err)
			}

			cachedEvents = make(map[string]models.Event)
			logger.Info("complete")
		case <-watcher.syncCh:
			if syncing {
				watcher.logger.Debug("sync-already-in-progress")
				continue
			}
			logger := watcher.logger.Session("sync")
			logger.Info("starting")
			go watcher.sync(logger, syncEnd)
			syncing = true
		case err := <-resubscribeChannel:
			watcher.logger.Error("event-source-error", err)
			if es := eventSource.Load(); es != nil {
				err := es.(events.EventSource).Close()
				if err != nil {
					watcher.logger.Error("failed-closing-event-source", err)
				}
			}
			go watcher.checkForEvents(resubscribeChannel, eventChan, eventSource, watcher.logger)

		case <-signals:
			watcher.logger.Info("stopping")
			atomic.StoreInt32(&stopEventSource, 1)
			if es := eventSource.Load(); es != nil {
				err := es.(events.EventSource).Close()
				if err != nil {
					watcher.logger.Error("failed-closing-event-source", err)
				}
			}
			return nil
		}
	}
}

func (w *Watcher) retrieveDesiredInternal(logger lager.Logger, event models.Event, currentDesireds []*models.DesiredLRP, syncing bool) []*models.DesiredLRP {
	var err error
	var actualLRP *models.ActualLRP
	var traceId string
	switch event := event.(type) {
	case *models.ActualLRPInstanceCreatedEvent:
		actualLRP = event.ActualLrp
		traceId = event.TraceId
	case *models.ActualLRPInstanceChangedEvent:
		actualLRP = event.After.ToActualLRP(event.ActualLrpKey, event.ActualLrpInstanceKey)
		traceId = event.TraceId
	default:
		return nil
	}
	if actualLRP == nil {
		logger.Error("nil-actual-lrp", nil, lager.Data{"event-type": event.EventType()})
		return nil
	}
	var desiredLRPs []*models.DesiredLRP
	if actualLRP.State != models.ActualLRPStateRunning {
		return nil
	}
	if w.routeHandler.ShouldRefreshDesired(actualLRP) || (syncing && !foundInCurrentDesireds(actualLRP.ActualLrpKey.ProcessGuid, currentDesireds)) {
		logger.Info("refreshing-desired-lrp-routing-info", lager.Data{"process-guid": actualLRP.ActualLrpKey.ProcessGuid})
		desiredLRPs, err = getDesiredLRPs(logger, w.bbsClient, traceId, []string{actualLRP.ActualLrpKey.ProcessGuid})

		if err != nil {
			logger.Error("failed-getting-desired-lrps-routing-info-for-missing-actual-lrp", err)
		}
	}

	return desiredLRPs
}

func (w *Watcher) retrieveDesired(logger lager.Logger, event models.Event) []*models.DesiredLRP {
	return w.retrieveDesiredInternal(logger, event, nil, false)
}

func (w *Watcher) retrieveDesiredWhileSyncing(logger lager.Logger, event models.Event, currentDesireds []*models.DesiredLRP) []*models.DesiredLRP {
	return w.retrieveDesiredInternal(logger, event, currentDesireds, true)
}

func foundInCurrentDesireds(guid string, currentDesireds []*models.DesiredLRP) bool {
	for _, d := range currentDesireds {
		if d.ProcessGuid == guid {
			return true
		}
	}

	return false
}

func (w *Watcher) handleEvent(logger lager.Logger, event models.Event) {
	desiredLRPs := w.retrieveDesired(logger, event)
	if len(desiredLRPs) > 0 {
		w.routeHandler.RefreshDesired(logger, desiredLRPs)
	}
	w.routeHandler.HandleEvent(logger, event)
}

func (w *Watcher) sync(logger lager.Logger, ch chan<- *syncEventResult) {
	var runningActualLRPs []*models.ActualLRP
	var desiredLRPs []*models.DesiredLRP
	var domains models.DomainSet

	var actualErr, desiredErr, domainsErr error
	before := w.clock.Now()

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Debug("getting-actual-lrps")
		var actualLRPs []*models.ActualLRP
		actualLRPs, actualErr = w.bbsClient.ActualLRPs(logger, "", models.ActualLRPFilter{CellID: w.cellID})
		if actualErr != nil {
			logger.Error("failed-getting-actual-lrps", actualErr)
			return
		}
		logger.Debug("succeeded-getting-actual-lrps", lager.Data{"num-actual-responses": len(actualLRPs)})

		for _, actualLRP := range actualLRPs {
			if actualLRP.State == models.ActualLRPStateRunning && (!actualLRP.RoutableExists() || *actualLRP.GetRoutable()) {
				runningActualLRPs = append(runningActualLRPs, actualLRP)
			}
		}

		if w.cellID != "" {
			guids := make([]string, 0, len(actualLRPs))
			// filter the desired lrp scheduling info by process guids
			for _, actualLRP := range actualLRPs {
				guids = append(guids, actualLRP.ActualLrpKey.ProcessGuid)
			}
			if len(guids) > 0 {
				desiredLRPs, desiredErr = getDesiredLRPs(logger, w.bbsClient, "", guids)
			}
		}
	}()

	if w.cellID == "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			desiredLRPs, desiredErr = getDesiredLRPs(logger, w.bbsClient, "", nil)
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		var domainArray []string
		logger.Debug("getting-domains")
		domainArray, domainsErr = w.bbsClient.Domains(logger, "")
		if domainsErr != nil {
			logger.Error("failed-getting-domains", domainsErr)
			return
		}

		domains = models.NewDomainSet(domainArray)
		logger.Debug("succeeded-getting-domains", lager.Data{"num-domains": len(domains)})
	}()

	wg.Wait()

	var err error
	if actualErr != nil || desiredErr != nil || domainsErr != nil {
		err = fmt.Errorf("failed to sync: %s, %s, %s", actualErr, desiredErr, domainsErr)
	}

	ch <- &syncEventResult{
		startTime:     before,
		desired:       desiredLRPs,
		runningActual: runningActualLRPs,
		domains:       domains,
		err:           err,
	}
}

func (w *Watcher) checkForEvents(resubscribeChannel chan error, eventChan chan models.Event, eventSource *atomic.Value, logger lager.Logger) {
	var err error
	var es events.EventSource

	logger.Info("subscribing-to-bbs-events")
	es, err = w.bbsClient.SubscribeToInstanceEventsByCellID(logger, w.cellID)
	if err != nil {
		resubscribeChannel <- err
		return
	}
	logger.Info("subscribed-to-bbs-events")

	eventSource.Store(es)

	var event models.Event
	for {
		event, err = es.Next()
		if err != nil {
			switch err {
			case events.ErrUnrecognizedEventType:
				logger.Error("failed-getting-next-event", err)
			default:
				resubscribeChannel <- err
				return
			}
		}

		if event != nil {
			eventChan <- event
		}
	}
}

func getDesiredLRPs(logger lager.Logger, bbsClient bbs.Client, traceId string, guids []string) ([]*models.DesiredLRP, error) {
	logger.Debug("getting-desired-lrps-routing-info", lager.Data{"guids-length": len(guids)})
	filter := models.DesiredLRPFilter{ProcessGuids: guids}

	desiredLRPs, err := bbsClient.DesiredLRPRoutingInfos(logger, traceId, filter)
	if err == bbs.EndpointNotFoundErr {
		desiredLRPs, err = bbsClient.DesiredLRPs(logger, traceId, filter)
	}

	if err != nil {
		logger.Error("failed-getting-desired-lrps-routing-info", err)
		return nil, err
	}

	logger.Debug("succeeded-getting-desired-lrps-routing-info", lager.Data{"num-desired-responses": len(desiredLRPs)})
	return desiredLRPs, nil
}
