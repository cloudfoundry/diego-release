package routehandlers

import (
	"errors"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/trace"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/route-emitter/emitter"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/unregistration"
	"code.cloudfoundry.org/route-emitter/watcher"
)

const (
	routesTotalMetric         = "RoutesTotal"
	routesSyncedCounter       = "RoutesSynced"
	routesRegisteredCounter   = "RoutesRegistered"
	routesUnregisteredCounter = "RoutesUnregistered"
	httpRouteCount            = "HTTPRouteCount"
	tcpRouteCount             = "TCPRouteCount"
)

type Handler struct {
	routingTable        routingtable.RoutingTable
	natsEmitter         emitter.NATSEmitter
	routingAPIEmitter   emitter.RoutingAPIEmitter
	localMode           bool
	tcpTLSEnabled       bool
	metronClient        loggingclient.IngressClient
	unregistrationCache unregistration.Cache
}

var _ watcher.RouteHandler = new(Handler)

func NewHandler(
	routingTable routingtable.RoutingTable,
	natsEmitter emitter.NATSEmitter,
	routingAPIEmitter emitter.RoutingAPIEmitter,
	localMode bool,
	tcpTLSEnabled bool,
	metronClient loggingclient.IngressClient,
	unregistrationCache unregistration.Cache,
) *Handler {
	return &Handler{
		routingTable:        routingTable,
		natsEmitter:         natsEmitter,
		routingAPIEmitter:   routingAPIEmitter,
		localMode:           localMode,
		tcpTLSEnabled:       tcpTLSEnabled,
		metronClient:        metronClient,
		unregistrationCache: unregistrationCache,
	}
}

func (handler *Handler) HandleEvent(logger lager.Logger, event models.Event) {

	switch event := event.(type) {
	case *models.DesiredLRPCreatedEvent:
		logger = trace.LoggerWithTraceInfo(logger, event.TraceId)
		handler.handleDesiredCreate(logger, event.DesiredLrp)
	case *models.DesiredLRPChangedEvent:
		logger = trace.LoggerWithTraceInfo(logger, event.TraceId)
		err := handler.handleDesiredUpdate(logger, event.Before, event.After)
		if err != nil {
			logger.Error("failed-to-handle-desired-update", err)
		}
	case *models.DesiredLRPRemovedEvent:
		logger = trace.LoggerWithTraceInfo(logger, event.TraceId)
		handler.handleDesiredDelete(logger, event.DesiredLrp)
	case *models.ActualLRPInstanceCreatedEvent:
		logger = trace.LoggerWithTraceInfo(logger, event.TraceId)
		if event.ActualLrp == nil {
			logger.Error("nil-actual-lrp", nil, lager.Data{"event-type": event.EventType()})
			return
		}
		handler.handleActualCreate(logger, event.ActualLrp)
	case *models.ActualLRPInstanceChangedEvent:
		logger = trace.LoggerWithTraceInfo(logger, event.TraceId)
		before := event.Before.ToActualLRP(event.ActualLrpKey, event.ActualLrpInstanceKey)
		after := event.After.ToActualLRP(event.ActualLrpKey, event.ActualLrpInstanceKey)
		logger.Debug("received-actual-lrp-changed-event", lager.Data{"before": before, "after": after})
		if before == nil || after == nil {
			logger.Error("nil-actual-lrp", nil, lager.Data{"event-type": event.EventType()})
			return
		}
		err := handler.handleActualUpdate(logger, before, after)
		if err != nil {
			logger.Error("failed-to-handle-actual-update", err)
		}
	case *models.ActualLRPInstanceRemovedEvent:
		logger = trace.LoggerWithTraceInfo(logger, event.TraceId)
		logger.Debug("received-actual-lrp-instance-removed-event", lager.Data{"lrp": event.ActualLrp})
		if event.ActualLrp == nil {
			logger.Error("nil-actual-lrp", nil, lager.Data{"event-type": event.EventType()})
			return
		}
		handler.handleActualDelete(logger, event.ActualLrp)
	default:
		logger.Error("did-not-handle-unrecognizable-event", errors.New("unrecognizable-event"), lager.Data{"event-type": event.EventType()})
	}
}

func (handler *Handler) EmitExternal(logger lager.Logger) {
	routingEvents, messagesToEmit := handler.routingTable.GetExternalRoutingEvents()

	logger.Debug("emitting-nats-messages", lager.Data{"messages": messagesToEmit})
	if handler.natsEmitter != nil {
		err := handler.natsEmitter.Emit(messagesToEmit)
		if err != nil {
			logger.Error("failed-to-emit-nats-routes", err)
		}
	}

	logger.Debug("emitting-routing-api-messages", lager.Data{"messages": routingEvents})
	if handler.routingAPIEmitter != nil {
		err := handler.routingAPIEmitter.Emit(routingEvents)
		if err != nil {
			logger.Error("failed-to-emit-tcp-routes", err)
		}
	}

	err := handler.metronClient.IncrementCounterWithDelta(routesSyncedCounter, messagesToEmit.RouteRegistrationCount())
	if err != nil {
		logger.Error("failed-send-routes-synced-count-metric", err)
	}
	err = handler.metronClient.SendMetric(routesTotalMetric, handler.routingTable.HTTPAssociationsCount())
	if err != nil {
		logger.Error("failed-to-send-total-route-count-metric", err)
	}
}

func (handler *Handler) EmitInternal(logger lager.Logger) {
	_, messagesToEmit := handler.routingTable.GetInternalRoutingEvents()

	logger.Debug("emitting-nats-messages", lager.Data{"messages": messagesToEmit})
	if handler.natsEmitter != nil {
		err := handler.natsEmitter.Emit(messagesToEmit)
		if err != nil {
			logger.Error("failed-to-emit-nats-routes", err)
		}
	}
}

func (handler *Handler) Sync(
	logger lager.Logger,
	desired []*models.DesiredLRP,
	actuals []*models.ActualLRP,
	domains models.DomainSet,
	cachedEvents map[string]models.Event,
) {
	logger = logger.Session("sync")
	logger.Debug("starting")
	defer logger.Debug("completed")

	nullLogger := lager.NewLogger("null-logger") // ignore log messsages from the routing table
	// The newTable is only used for Swap call which only replaces table entries and does not
	// update config parameters
	newTable := routingtable.NewRoutingTable(false, handler.tcpTLSEnabled, handler.metronClient)

	for _, lrp := range desired {
		newTable.SetRoutes(nullLogger, nil, lrp)
	}

	for _, lrp := range actuals {
		newTable.AddEndpoint(nullLogger, lrp)
	}

	natsEmitter := handler.natsEmitter
	routingAPIEmitter := handler.routingAPIEmitter
	table := handler.routingTable

	handler.natsEmitter = nil
	handler.routingAPIEmitter = nil
	handler.routingTable = newTable

	for _, event := range cachedEvents {
		handler.HandleEvent(logger, event)
	}

	handler.routingTable = table
	handler.natsEmitter = natsEmitter
	handler.routingAPIEmitter = routingAPIEmitter

	routeMappings, messages := handler.routingTable.Swap(nullLogger, newTable, domains)
	logger.Debug("start-emitting-messages", lager.Data{
		"num-registration-messages":            len(messages.RegistrationMessages),
		"num-unregistration-messages":          len(messages.UnregistrationMessages),
		"num-internal-registration-messages":   len(messages.InternalRegistrationMessages),
		"num-internal-unregistration-messages": len(messages.InternalUnregistrationMessages),
	})
	err := handler.unregistrationCache.Add(messages.UnregistrationMessages)
	if err != nil {
		logger.Error("failed-to-add-messages-to-cache", err, lager.Data{"messages": messages.UnregistrationMessages})
	}
	err = handler.unregistrationCache.Remove(messages.RegistrationMessages)
	if err != nil {
		logger.Error("failed-to-remove-messages-from-cache", err, lager.Data{"messages": messages.RegistrationMessages})
	}
	handler.emitMessages(logger, messages, routeMappings)
	logger.Debug("done-emitting-messages", lager.Data{
		"num-registration-messages":            len(messages.RegistrationMessages),
		"num-unregistration-messages":          len(messages.UnregistrationMessages),
		"num-internal-registration-messages":   len(messages.InternalRegistrationMessages),
		"num-internal-unregistration-messages": len(messages.InternalUnregistrationMessages),
	})

	if handler.localMode {
		err := handler.metronClient.SendMetric(httpRouteCount, handler.routingTable.HTTPAssociationsCount())
		if err != nil {
			logger.Error("failed-to-send-http-routes-count-metric", err)
		}
		err = handler.metronClient.SendMetric(tcpRouteCount, handler.routingTable.TCPAssociationsCount())
		if err != nil {
			logger.Error("failed-to-send-tcp-route-count-metric", err)
		}
	}
}

func (handler *Handler) RefreshDesired(logger lager.Logger, desiredLRPs []*models.DesiredLRP) {
	for _, desiredLRP := range desiredLRPs {
		routeMappings, messagesToEmit := handler.routingTable.SetRoutes(logger, nil, desiredLRP)
		handler.emitMessages(logger, messagesToEmit, routeMappings)
	}
}

func (handler *Handler) ShouldRefreshDesired(actualLRP *models.ActualLRP) bool {
	return !handler.routingTable.HasExternalRoutes(actualLRP)
}

func (handler *Handler) handleDesiredCreate(logger lager.Logger, desiredLRP *models.DesiredLRP) {
	routeMappings, messagesToEmit := handler.routingTable.SetRoutes(logger, nil, desiredLRP)
	handler.emitMessages(logger, messagesToEmit, routeMappings)
}

func (handler *Handler) handleDesiredUpdate(logger lager.Logger, before, after *models.DesiredLRP) error {
	routeMappings, messagesToEmit := handler.routingTable.SetRoutes(logger, before, after)
	err := handler.unregistrationCache.Add(messagesToEmit.UnregistrationMessages)
	if err != nil {
		return err
	}
	err = handler.unregistrationCache.Remove(messagesToEmit.RegistrationMessages)
	if err != nil {
		return err
	}
	handler.emitMessages(logger, messagesToEmit, routeMappings)
	return nil
}

func (handler *Handler) handleDesiredDelete(logger lager.Logger, desiredLRP *models.DesiredLRP) {
	routeMappings, messagesToEmit := handler.routingTable.RemoveRoutes(logger, desiredLRP)
	handler.emitMessages(logger, messagesToEmit, routeMappings)
}

func (handler *Handler) handleActualCreate(logger lager.Logger, actualLRP *models.ActualLRP) {
	if actualLRP.State != models.ActualLRPStateRunning {
		return
	}
	routeMappings, messagesToEmit := handler.routingTable.AddEndpoint(logger, actualLRP)
	handler.emitMessages(logger, messagesToEmit, routeMappings)
}

func (handler *Handler) handleActualUpdate(logger lager.Logger, before, after *models.ActualLRP) error {
	var (
		messagesToEmit routingtable.MessagesToEmit
		routeMappings  routingtable.TCPRouteMappings
	)
	switch {
	case after.State == models.ActualLRPStateRunning:
		if !after.RoutableExists() || *after.GetRoutable() {
			routeMappings, messagesToEmit = handler.routingTable.AddEndpoint(logger, after)
		} else if *before.GetRoutable() {
			routeMappings, messagesToEmit = handler.routingTable.RemoveEndpoint(logger, after)
		}

		if before.State == models.ActualLRPStateRunning &&
			before.Presence == models.ActualLRP_Ordinary && after.Presence == models.ActualLRP_Evacuating {
			removeRouteMappings, removeMessagesToEmit := handler.routingTable.RemoveEndpoint(logger, before)
			routeMappings = routeMappings.Merge(removeRouteMappings)
			messagesToEmit = messagesToEmit.Merge(removeMessagesToEmit)
		}
	case before.State == models.ActualLRPStateRunning && after.State != models.ActualLRPStateRunning:
		routeMappings, messagesToEmit = handler.routingTable.RemoveEndpoint(logger, before)
	}

	err := handler.unregistrationCache.Add(messagesToEmit.UnregistrationMessages)
	if err != nil {
		return err
	}

	err = handler.unregistrationCache.Remove(messagesToEmit.RegistrationMessages)
	if err != nil {
		return err
	}
	handler.emitMessages(logger, messagesToEmit, routeMappings)
	return nil
}

func (handler *Handler) handleActualDelete(logger lager.Logger, actualLRP *models.ActualLRP) {
	if actualLRP == nil || actualLRP.State != models.ActualLRPStateRunning {
		return
	}
	routeMappings, messagesToEmit := handler.routingTable.RemoveEndpoint(logger, actualLRP)
	handler.emitMessages(logger, messagesToEmit, routeMappings)
}

func (handler *Handler) emitMessages(logger lager.Logger, messagesToEmit routingtable.MessagesToEmit, routeMappings routingtable.TCPRouteMappings) {
	if handler.natsEmitter != nil {
		logger.Debug("emit-messages", lager.Data{"messages": messagesToEmit})
		err := handler.natsEmitter.Emit(messagesToEmit)
		if err != nil {
			logger.Error("failed-to-emit-http-routes", err)
		}
		err = handler.metronClient.IncrementCounterWithDelta(routesRegisteredCounter, messagesToEmit.RouteRegistrationCount())
		if err != nil {
			logger.Error("failed-to-emit-registration-message-count", err)
		}
		err = handler.metronClient.IncrementCounterWithDelta(routesUnregisteredCounter, messagesToEmit.RouteUnregistrationCount())
		if err != nil {
			logger.Error("failed-to-emit-unregistration-message-count", err)
		}
	} else {
		logger.Info("no-emitter-configured-skipping-emit-messages", lager.Data{"messages": messagesToEmit})
	}

	if handler.routingAPIEmitter != nil {
		err := handler.routingAPIEmitter.Emit(routeMappings)
		if err != nil {
			logger.Error("failed-to-emit-http-routes", err)
		}
	}
}
