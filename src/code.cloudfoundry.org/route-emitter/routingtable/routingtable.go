package routingtable

import (
	"sync"

	"code.cloudfoundry.org/bbs/models"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
	tcpmodels "code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/routing-info/cfroutes"
	"code.cloudfoundry.org/routing-info/internalroutes"
	"code.cloudfoundry.org/routing-info/tcp_routes"
)

type TCPRouteMappings struct {
	Registrations   []tcpmodels.TcpRouteMapping
	Unregistrations []tcpmodels.TcpRouteMapping
}

func (mappings TCPRouteMappings) Merge(other TCPRouteMappings) TCPRouteMappings {
	var result TCPRouteMappings
	result.Registrations = append(mappings.Registrations, other.Registrations...)
	result.Unregistrations = append(mappings.Unregistrations, other.Unregistrations...)
	return result
}

const addressCollisionsCounter = "AddressCollisions"

//go:generate counterfeiter -o fakeroutingtable/fake_routingtable.go . RoutingTable
type RoutingTable interface {
	// table modification
	SetRoutes(logger lager.Logger, beforeLRP, afterLRP *models.DesiredLRP) (TCPRouteMappings, MessagesToEmit)
	RemoveRoutes(logger lager.Logger, desiredLRP *models.DesiredLRP) (TCPRouteMappings, MessagesToEmit)
	AddEndpoint(logger lager.Logger, actualLRP *models.ActualLRP) (TCPRouteMappings, MessagesToEmit)
	RemoveEndpoint(logger lager.Logger, actualLRP *models.ActualLRP) (TCPRouteMappings, MessagesToEmit)
	Swap(logger lager.Logger, t RoutingTable, domains models.DomainSet) (TCPRouteMappings, MessagesToEmit)
	GetInternalRoutingEvents() (TCPRouteMappings, MessagesToEmit)
	GetExternalRoutingEvents() (TCPRouteMappings, MessagesToEmit)

	// routes

	HasExternalRoutes(actualLRP *models.ActualLRP) bool
	HTTPAssociationsCount() int     // return number of associations desired-lrp-http-routes * actual-lrps
	InternalAssociationsCount() int // return number of associations desired-lrp-internal-routes * 2 * actual-lrps
	TCPAssociationsCount() int      // return number of associations desired-lrp-tcp-routes * actual-lrps
	TableSize() int
}

type internalRoutingTable struct {
	endpointGenerator        func(*models.ActualLRP) []Endpoint
	routesGenerator          func(*models.DesiredLRP) map[RoutingKey][]routeMapping
	entries                  map[RoutingKey]RoutableEndpoints
	addressEntries           map[Address]EndpointKey
	addressGenerator         func(endpoint Endpoint) Address
	directInstanceRoute      bool
	metronClient             loggingclient.IngressClient
	suppressAddressCollision bool
	sync.Locker
}

type routingTable struct {
	tcpRoutesRoutingTable      *internalRoutingTable
	httpRoutesRoutingTable     *internalRoutingTable
	internalRoutesRoutingTable *internalRoutingTable
}

func NewRoutingTable(directInstanceRoute bool, tcpTLSEnabled bool, metronClient loggingclient.IngressClient) RoutingTable {
	addressGenerator := func(endpoint Endpoint) Address {
		if endpoint.IsDirectInstanceRoute(directInstanceRoute) {
			return Address{Host: endpoint.ContainerIP, Port: endpoint.ContainerPort}
		}
		return Address{Host: endpoint.Host, Port: endpoint.Port}
	}

	httpRoutingTable := &internalRoutingTable{
		endpointGenerator:   NewEndpointsFromActual,
		routesGenerator:     httpRoutesFrom,
		entries:             make(map[RoutingKey]RoutableEndpoints),
		addressEntries:      make(map[Address]EndpointKey),
		directInstanceRoute: directInstanceRoute,
		addressGenerator:    addressGenerator,
		metronClient:        metronClient,
		Locker:              &sync.Mutex{},
	}
	tcpRoutingTable := &internalRoutingTable{
		endpointGenerator:        NewEndpointsFromActual,
		routesGenerator:          tcpRoutesGenerator{tcpTLSEnabled}.RoutesFrom,
		entries:                  make(map[RoutingKey]RoutableEndpoints),
		addressEntries:           make(map[Address]EndpointKey),
		directInstanceRoute:      directInstanceRoute,
		addressGenerator:         addressGenerator,
		metronClient:             metronClient,
		suppressAddressCollision: true,
		Locker:                   &sync.Mutex{},
	}
	internalRoutingTable := &internalRoutingTable{
		endpointGenerator:        internalEndpointsFromActualLRP,
		routesGenerator:          internalRoutesFrom,
		entries:                  make(map[RoutingKey]RoutableEndpoints),
		addressEntries:           make(map[Address]EndpointKey),
		directInstanceRoute:      directInstanceRoute,
		addressGenerator:         addressGenerator,
		metronClient:             metronClient,
		suppressAddressCollision: true,
		Locker:                   &sync.Mutex{},
	}

	return &routingTable{
		tcpRoutesRoutingTable:      tcpRoutingTable,
		httpRoutesRoutingTable:     httpRoutingTable,
		internalRoutesRoutingTable: internalRoutingTable,
	}
}

func internalEndpointsFromActualLRP(actualLRP *models.ActualLRP) []Endpoint {
	return []Endpoint{
		{
			InstanceGUID:    actualLRP.ActualLrpInstanceKey.InstanceGuid,
			Index:           actualLRP.ActualLrpKey.Index,
			Host:            actualLRP.ActualLrpNetInfo.Address,
			ContainerIP:     actualLRP.ActualLrpNetInfo.InstanceAddress,
			Presence:        actualLRP.Presence,
			Since:           actualLRP.Since,
			ModificationTag: &actualLRP.ModificationTag,
		},
	}
}

func (table *routingTable) AddEndpoint(logger lager.Logger, actualLRP *models.ActualLRP) (TCPRouteMappings, MessagesToEmit) {
	httpMappings, httpMessages, httpChanged := table.httpRoutesRoutingTable.AddEndpoint(logger, actualLRP)
	tcpMappings, tcpMessages, tcpChanged := table.tcpRoutesRoutingTable.AddEndpoint(logger, actualLRP)
	internalMappings, internalMessages, internalChanged := table.internalRoutesRoutingTable.AddEndpoint(logger, actualLRP)

	mappings := httpMappings.Merge(tcpMappings).Merge(internalMappings)
	messages := httpMessages.Merge(tcpMessages).Merge(internalMessages)

	changed := httpChanged || tcpChanged || internalChanged
	if changed {
		logger.Info("added", ActualLRPData(actualLRP))
	}

	return mappings, messages
}

func (table *routingTable) RemoveEndpoint(logger lager.Logger, actualLRP *models.ActualLRP) (TCPRouteMappings, MessagesToEmit) {
	httpMappings, httpMessages, httpChanged := table.httpRoutesRoutingTable.RemoveEndpoint(logger, actualLRP)
	tcpMappings, tcpMessages, tcpChanged := table.tcpRoutesRoutingTable.RemoveEndpoint(logger, actualLRP)
	internalMappings, internalMessages, internalChanged := table.internalRoutesRoutingTable.RemoveEndpoint(logger, actualLRP)

	mappings := httpMappings.Merge(tcpMappings).Merge(internalMappings)
	messages := httpMessages.Merge(tcpMessages).Merge(internalMessages)

	changed := httpChanged || tcpChanged || internalChanged
	if changed {
		logger.Info("removed", ActualLRPData(actualLRP))
	}

	return mappings, messages
}

func (t *routingTable) Swap(logger lager.Logger, other RoutingTable, domains models.DomainSet) (TCPRouteMappings, MessagesToEmit) {
	table, ok := other.(*routingTable)
	if !ok {
		logger.Error("failed-to-convert-to-routing-table", nil)
		return TCPRouteMappings{}, MessagesToEmit{}
	}
	logger = logger.Session("swap")
	logger.Info("starting", lager.Data{"domains": domains})
	defer logger.Info("finished")

	httpMappings, httpMessages := t.httpRoutesRoutingTable.Swap(table.httpRoutesRoutingTable, domains)
	tcpMappings, tcpMessages := t.tcpRoutesRoutingTable.Swap(table.tcpRoutesRoutingTable, domains)
	internalMappings, internalMessages := t.internalRoutesRoutingTable.Swap(table.internalRoutesRoutingTable, domains)

	mappings := httpMappings.Merge(tcpMappings).Merge(internalMappings)
	messages := httpMessages.Merge(tcpMessages).Merge(internalMessages)
	return mappings, messages
}

func (t *routingTable) GetExternalRoutingEvents() (TCPRouteMappings, MessagesToEmit) {
	httpMappings, httpMessages := t.httpRoutesRoutingTable.GetRoutingEvents()
	tcpMappings, tcpMessages := t.tcpRoutesRoutingTable.GetRoutingEvents()

	mappings := httpMappings.Merge(tcpMappings)
	messages := httpMessages.Merge(tcpMessages)
	return mappings, messages
}

func (t *routingTable) GetInternalRoutingEvents() (TCPRouteMappings, MessagesToEmit) {
	return t.internalRoutesRoutingTable.GetRoutingEvents()
}

func (t *routingTable) SetRoutes(logger lager.Logger, before, after *models.DesiredLRP) (TCPRouteMappings, MessagesToEmit) {
	httpMappings, httpMessages, httpChanged := t.httpRoutesRoutingTable.SetRoutes(before, after)
	tcpMappings, tcpMessages, tcpChanged := t.tcpRoutesRoutingTable.SetRoutes(before, after)
	internalMappings, internalMessages, internalChanged := t.internalRoutesRoutingTable.SetRoutes(before, after)

	mappings := httpMappings.Merge(tcpMappings).Merge(internalMappings)
	messages := httpMessages.Merge(tcpMessages).Merge(internalMessages)

	if httpChanged || tcpChanged || internalChanged {
		logger.Info("set-routes", lager.Data{"before": DesiredLRPData(before), "after": DesiredLRPData(after)})
	}

	return mappings, messages
}

func (t *routingTable) RemoveRoutes(logger lager.Logger, desiredLRP *models.DesiredLRP) (TCPRouteMappings, MessagesToEmit) {
	httpMappings, httpMessages, httpChanged := t.httpRoutesRoutingTable.RemoveRoutes(desiredLRP)
	tcpMappings, tcpMessages, tcpChanged := t.tcpRoutesRoutingTable.RemoveRoutes(desiredLRP)
	internalMappings, internalMessages, internalChanged := t.internalRoutesRoutingTable.RemoveRoutes(desiredLRP)

	mappings := httpMappings.Merge(tcpMappings).Merge(internalMappings)
	messages := httpMessages.Merge(tcpMessages).Merge(internalMessages)

	if httpChanged || tcpChanged || internalChanged {
		logger.Info("remove-routes", DesiredLRPData(desiredLRP))
	}

	return mappings, messages
}

func (table *internalRoutingTable) AddEndpoint(logger lager.Logger, actualLRP *models.ActualLRP) (TCPRouteMappings, MessagesToEmit, bool) {
	table.Lock()
	defer table.Unlock()

	changeDetected := false
	endpoints := table.endpointGenerator(actualLRP)

	// collision detection

	if !table.suppressAddressCollision {
		for _, endpoint := range endpoints {
			address := table.addressGenerator(endpoint)
			// if the address exists and the instance guid doesn't match then we have a collision
			if existingEndpointKey, ok := table.addressEntries[address]; ok && existingEndpointKey.InstanceGUID != endpoint.InstanceGUID {
				metricErr := table.metronClient.IncrementCounter(addressCollisionsCounter)
				if metricErr != nil {
					logger.Debug("error-emitting-address-collision-metric", lager.Data{"error": metricErr})
				}
				existingInstanceGuid := existingEndpointKey.InstanceGUID
				logger.Info("collision-detected-with-endpoint", lager.Data{
					"instance_guid_a": existingInstanceGuid,
					"instance_guid_b": endpoint.InstanceGUID,
					"Address":         address,
				})
			}

			table.addressEntries[address] = endpoint.key()
		}
	}

	// add endpoints

	var messagesToEmit MessagesToEmit
	var mappings TCPRouteMappings

	for _, routingEndpoint := range endpoints {
		key := RoutingKey{
			ProcessGUID:   actualLRP.ActualLrpKey.ProcessGuid,
			ContainerPort: routingEndpoint.ContainerPort,
		}
		currentEntry := table.entries[key]
		// Since desiredLRP is same, only need to check one entry
		if currentEntry.DesiredInstances > 0 && routingEndpoint.Index >= currentEntry.DesiredInstances {
			logger.Debug("skipping-undesired-instance")
			continue
		}
		currentEndpoint, ok := currentEntry.Endpoints[routingEndpoint.key()]
		if ok && !currentEndpoint.ModificationTag.SucceededBy(routingEndpoint.ModificationTag) {
			continue
		}
		newEntry := currentEntry.copy()
		newEntry.Endpoints[routingEndpoint.key()] = routingEndpoint
		table.entries[key] = newEntry
		mapping, message, changed := table.emitDiffMessages(key, currentEntry, newEntry)
		mappings = mappings.Merge(mapping)
		messagesToEmit = messagesToEmit.Merge(message)
		changeDetected = changeDetected || changed
	}

	return mappings, messagesToEmit, changeDetected
}

func (table *internalRoutingTable) RemoveEndpoint(logger lager.Logger, actualLRP *models.ActualLRP) (TCPRouteMappings, MessagesToEmit, bool) {
	table.Lock()
	defer table.Unlock()

	changeDetected := false
	endpoints := table.endpointGenerator(actualLRP)

	// remove address
	if !table.suppressAddressCollision {
		for _, endpoint := range endpoints {
			address := table.addressGenerator(endpoint)
			currentEntry, ok := table.addressEntries[address]
			if ok && currentEntry.InstanceGUID != endpoint.InstanceGUID {
				logger.Info("collision-detected-with-endpoint", lager.Data{
					"instance_guid_a": currentEntry.InstanceGUID,
					"instance_guid_b": endpoint.InstanceGUID,
					"Address":         address,
				})
				continue
			}
			delete(table.addressEntries, address)
		}
	}

	// remove endpoint
	var messagesToEmit MessagesToEmit
	var mappings TCPRouteMappings
	for _, routingEndpoint := range endpoints {
		key := RoutingKey{
			ProcessGUID:   actualLRP.ActualLrpKey.ProcessGuid,
			ContainerPort: routingEndpoint.ContainerPort,
		}

		currentEntry := table.entries[key]
		endpointKey := routingEndpoint.key()
		currentEndpoint, ok := currentEntry.Endpoints[endpointKey]

		if !ok ||
			(!currentEndpoint.ModificationTag.Equal(routingEndpoint.ModificationTag) &&
				!currentEndpoint.ModificationTag.SucceededBy(routingEndpoint.ModificationTag)) {
			continue
		}

		newEntry := currentEntry.copy()
		delete(newEntry.Endpoints, endpointKey)

		table.entries[key] = newEntry
		table.deleteEntryIfEmpty(key)

		mapping, message, changed := table.emitDiffMessages(key, currentEntry, newEntry)
		messagesToEmit = messagesToEmit.Merge(message)
		mappings = mappings.Merge(mapping)
		changeDetected = changeDetected || changed
	}

	return mappings, messagesToEmit, changeDetected
}

func (t *internalRoutingTable) Swap(otherTable *internalRoutingTable, domains models.DomainSet) (TCPRouteMappings, MessagesToEmit) {
	t.Lock()
	defer t.Unlock()

	t.addressEntries = otherTable.addressEntries

	var messagesToEmit MessagesToEmit
	var mappings TCPRouteMappings

	mergedRoutingKeys := map[RoutingKey]struct{}{}
	for key := range otherTable.entries {
		mergedRoutingKeys[key] = struct{}{}
	}
	for key := range t.entries {
		mergedRoutingKeys[key] = struct{}{}
	}

	for key := range mergedRoutingKeys {
		existingEntry, ok := t.entries[key]
		newEntry := otherTable.entries[key]
		if !ok {
			// routing key only exist in the new table
			mapping, message, _ := t.emitDiffMessages(key, RoutableEndpoints{}, newEntry)
			messagesToEmit = messagesToEmit.Merge(message)
			mappings = mappings.Merge(mapping)
			continue
		}

		// entry exists in both tables or in old table, merge the two entries to ensure non-fresh domain endpoints aren't removed
		merged := mergeUnfreshRoutes(existingEntry, newEntry, domains)
		otherTable.entries[key] = merged
		otherTable.deleteEntryIfEmpty(key)
		mapping, message, _ := t.emitDiffMessages(key, existingEntry, merged)
		messagesToEmit = messagesToEmit.Merge(message)
		mappings = mappings.Merge(mapping)
	}

	t.entries = otherTable.entries

	return mappings, messagesToEmit
}

// merge the routes from both endpoints, ensuring that non-fresh routes aren't removed
func mergeUnfreshRoutes(before, after RoutableEndpoints, domains models.DomainSet) RoutableEndpoints {
	merged := after.copy()
	merged.Domain = before.Domain

	if !domains.Contains(before.Domain) {
		// non-fresh domain, append routes from older endpoint
		for _, oldRoute := range before.Routes {
			routeExistInNewLRP := func() bool {
				for _, newRoute := range after.Routes {
					if newRoute.Hash() == oldRoute.Hash() {
						return true
					}
				}
				return false
			}
			if !routeExistInNewLRP() {
				merged.Routes = append(merged.Routes, oldRoute)
			}
		}
	}

	return merged
}

func (t *internalRoutingTable) GetRoutingEvents() (TCPRouteMappings, MessagesToEmit) {
	t.Lock()
	defer t.Unlock()

	var messagesToEmit MessagesToEmit
	var mappings TCPRouteMappings
	for key, route := range t.entries {
		mapping, message, _ := t.emitDiffMessages(key, RoutableEndpoints{}, route)

		mappings = mappings.Merge(mapping)
		messagesToEmit = messagesToEmit.Merge(message)
	}

	return mappings, messagesToEmit
}

type routeMapping interface {
	MessageFor(endpoint Endpoint, directInstanceAddress, emitEndpointUpdatedAt bool) (*RegistryMessage, *tcpmodels.TcpRouteMapping, *RegistryMessage)
	Hash() interface{}
}

func httpRoutesFrom(lrp *models.DesiredLRP) map[RoutingKey][]routeMapping {
	if lrp == nil || lrp.Routes == nil {
		return nil
	}

	routes, _ := cfroutes.CFRoutesFromRoutingInfo(*lrp.Routes)
	routeEntries := make(map[RoutingKey][]routeMapping)
	for _, route := range routes {
		key := RoutingKey{ProcessGUID: lrp.ProcessGuid, ContainerPort: route.Port}

		routes := []routeMapping{}
		for _, hostname := range route.Hostnames {
			route := Route{
				Hostname:         hostname,
				LogGUID:          lrp.LogGuid,
				RouteServiceUrl:  route.RouteServiceUrl,
				IsolationSegment: route.IsolationSegment,
				MetricTags:       lrp.MetricTags,
				Protocol:         route.Protocol,
				Options:          route.Options,
			}
			routes = append(routes, route)
		}
		routeEntries[key] = append(routeEntries[key], routes...)
	}
	return routeEntries
}

type tcpRoutesGenerator struct {
	tlsEnabled bool
}

func (g tcpRoutesGenerator) RoutesFrom(lrp *models.DesiredLRP) map[RoutingKey][]routeMapping {
	if lrp == nil {
		return nil
	}

	routes, _ := tcp_routes.TCPRoutesFromRoutingInfo(lrp.Routes)

	routeEntries := make(map[RoutingKey][]routeMapping)
	for _, route := range routes {
		key := RoutingKey{ProcessGUID: lrp.ProcessGuid, ContainerPort: route.ContainerPort}

		routeEntries[key] = append(routeEntries[key], ExternalEndpointInfo{
			RouterGroupGUID: route.RouterGroupGuid,
			Port:            route.ExternalPort,
			TLSEnabled:      g.tlsEnabled,
		})
	}
	return routeEntries
}

func internalRoutesFrom(lrp *models.DesiredLRP) map[RoutingKey][]routeMapping {
	if lrp == nil || lrp.Routes == nil {
		return nil
	}

	routes, _ := internalroutes.InternalRoutesFromRoutingInfo(lrp.Routes)

	routeEntries := make(map[RoutingKey][]routeMapping)
	for _, route := range routes {
		key := RoutingKey{ProcessGUID: lrp.ProcessGuid}
		routeEntries[key] = append(routeEntries[key], InternalRoute{
			Hostname: route.Hostname,
			LogGUID:  lrp.LogGuid,
		})
	}
	return routeEntries
}

func (table *internalRoutingTable) SetRoutes(before, after *models.DesiredLRP) (TCPRouteMappings, MessagesToEmit, bool) {
	table.Lock()
	defer table.Unlock()

	// update routes
	removedRouteEntries := table.routesGenerator(before)
	routeEntries := table.routesGenerator(after)
	// logger.Info("internal-route-table", lager.Data{"numEntries": len(table.internalEntries)})

	messagesToEmit := MessagesToEmit{}
	var mappings TCPRouteMappings

	changedDetected := false

	for key, routes := range routeEntries {
		currentEntry := table.entries[key]
		// if modification tag is old, ignore the new lrp
		if !currentEntry.ModificationTag.SucceededBy(after.ModificationTag) {
			continue
		}

		newEntry := currentEntry.copy()
		newEntry.Domain = after.Domain
		newEntry.Routes = routes
		newEntry.ModificationTag = after.ModificationTag
		newEntry.DesiredInstances = after.Instances

		// check if scaling down
		if before != nil && after != nil && before.Instances > after.Instances {
			newEndpoints := make(map[EndpointKey]Endpoint)

			for endpointKey, endpoint := range newEntry.Endpoints {
				if endpoint.Index < after.Instances {
					newEndpoints[endpointKey] = endpoint
				}
			}
			newEntry.Endpoints = newEndpoints
		}

		table.entries[key] = newEntry

		mapping, message, changed := table.emitDiffMessages(key, currentEntry, newEntry)
		messagesToEmit = messagesToEmit.Merge(message)
		mappings = mappings.Merge(mapping)
		changedDetected = changedDetected || changed
	}

	for key := range removedRouteEntries {
		if _, ok := routeEntries[key]; ok {
			continue
		}

		currentEntry := table.entries[key]
		if after == nil {
			// this is a delete (after == nil), then before lrp modification tag must be >=
			if !currentEntry.ModificationTag.Equal(before.ModificationTag) && !currentEntry.ModificationTag.SucceededBy(before.ModificationTag) {
				continue
			}
		} else if !currentEntry.ModificationTag.SucceededBy(after.ModificationTag) {
			// this is an update, then after lrp modification tag must be >
			continue
		}

		newEntry := currentEntry.copy()
		newEntry.Routes = nil
		if after != nil {
			newEntry.Domain = after.Domain
			newEntry.ModificationTag = after.ModificationTag
			newEntry.DesiredInstances = after.Instances
		}

		table.entries[key] = newEntry

		table.deleteEntryIfEmpty(key)

		mapping, message, changed := table.emitDiffMessages(key, currentEntry, newEntry)
		messagesToEmit = messagesToEmit.Merge(message)
		mappings = mappings.Merge(mapping)
		changedDetected = changedDetected || changed
	}

	return mappings, messagesToEmit, changedDetected
}

func (table *internalRoutingTable) deleteEntryIfEmpty(key RoutingKey) {
	entry := table.entries[key]
	if len(entry.Endpoints) == 0 && len(entry.Routes) == 0 {
		delete(table.entries, key)
	}
}

func (table *internalRoutingTable) emitDiffMessages(key RoutingKey, oldEntry, newEntry RoutableEndpoints) (TCPRouteMappings, MessagesToEmit, bool) {
	routesDiff := diffRoutes(oldEntry.Routes, newEntry.Routes)
	endpointsDiff := diffEndpoints(oldEntry.Endpoints, newEntry.Endpoints)

	changed := false
	if len(routesDiff.added) > 0 || len(routesDiff.removed) > 0 ||
		len(endpointsDiff.added) > 0 || len(endpointsDiff.removed) > 0 {
		changed = true
	}

	mappings, messages := table.messages(routesDiff, endpointsDiff)
	return mappings, messages, changed
}

type routesDiff struct {
	before, after, removed, added []routeMapping
}

type endpointsDiff struct {
	before, after, removed, added map[EndpointKey]Endpoint
}

func diffRoutes(before, after []routeMapping) routesDiff {
	existingRoutes := map[interface{}]routeMapping{}
	newRoutes := map[interface{}]routeMapping{}
	for _, route := range before {
		existingRoutes[route.Hash()] = route
	}
	for _, route := range after {
		newRoutes[route.Hash()] = route
	}

	diff := routesDiff{
		before: before,
		after:  after,
	}
	// generate the diff
	for routeHash := range existingRoutes {
		if _, ok := newRoutes[routeHash]; !ok {
			diff.removed = append(diff.removed, existingRoutes[routeHash])
		}
	}

	for routeHash := range newRoutes {
		if _, ok := existingRoutes[routeHash]; !ok {
			diff.added = append(diff.added, newRoutes[routeHash])
		}
	}

	return diff
}

// endpoints are different if any field is different other than the following:
// 1. ModificationTag
// 2. Presence
// 3. Since
func endpointDifferent(before, after Endpoint) bool {
	endpoint := before
	endpoint.ModificationTag = after.ModificationTag
	endpoint.Presence = after.Presence
	endpoint.Since = after.Since
	return endpoint != after
}

func diffEndpoints(before, after map[EndpointKey]Endpoint) endpointsDiff {
	diff := endpointsDiff{
		before:  before,
		after:   after,
		removed: make(map[EndpointKey]Endpoint),
		added:   make(map[EndpointKey]Endpoint),
	}
	// generate the diff
	for key, endpoint := range before {
		newEndpoint, ok := after[key]
		if !ok {
			key.Evacuating = !key.Evacuating
			newEndpoint, ok = after[key]
		}
		if !ok || endpointDifferent(newEndpoint, endpoint) {
			diff.removed[key] = endpoint
		}
	}
	for key, endpoint := range after {
		oldEndpoint, ok := before[key]
		if !ok {
			key.Evacuating = !key.Evacuating
			oldEndpoint, ok = before[key]
		}
		if !ok || endpointDifferent(endpoint, oldEndpoint) {
			diff.added[key] = endpoint
		}
	}

	return diff
}

func (table *internalRoutingTable) messages(routesDiff routesDiff, endpointDiff endpointsDiff) (TCPRouteMappings, MessagesToEmit) {
	type registrationMetadata struct {
		emitEndpointUpdatedAt bool
		route                 routeMapping
	}

	type unregistrationMetadata struct {
		route routeMapping
	}

	// maps used to remove duplicates
	unregistrations := map[interface{}]map[Endpoint]*unregistrationMetadata{}
	registrations := map[interface{}]map[Endpoint]*registrationMetadata{}

	// for removed routes remove endpoints previously registered
	for _, route := range routesDiff.removed {
		rh := route.Hash()
		for _, container := range endpointDiff.before {
			if unregistrations[rh] != nil && unregistrations[rh][container] != nil {
				continue
			}
			if unregistrations[rh] == nil {
				unregistrations[rh] = map[Endpoint]*unregistrationMetadata{}
			}
			unregistrations[rh][container] = &unregistrationMetadata{
				route: route,
			}
		}
	}

	// for added routes add all currently known endpoints
	for _, route := range routesDiff.added {
		rh := route.Hash()
		for _, container := range endpointDiff.after {
			if registrations[rh] != nil && registrations[rh][container] != nil {
				continue
			}
			if registrations[rh] == nil {
				registrations[rh] = map[Endpoint]*registrationMetadata{}
			}
			registrations[rh][container] = &registrationMetadata{
				emitEndpointUpdatedAt: false,
				route:                 route,
			}
		}
	}

	// for removed endpoints remove routes previously registered
	for _, container := range endpointDiff.removed {
		for _, route := range routesDiff.before {
			rh := route.Hash()
			if unregistrations[rh] != nil && unregistrations[rh][container] != nil {
				continue
			}
			if unregistrations[rh] == nil {
				unregistrations[rh] = map[Endpoint]*unregistrationMetadata{}
			}
			unregistrations[rh][container] = &unregistrationMetadata{
				route: route,
			}
		}
	}

	// for added endpoints register all current routes
	for _, container := range endpointDiff.added {
		for _, route := range routesDiff.after {
			rh := route.Hash()
			if registrations[rh] != nil && registrations[rh][container] != nil {
				continue
			}
			if registrations[rh] == nil {
				registrations[rh] = map[Endpoint]*registrationMetadata{}
			}
			registrations[rh][container] = &registrationMetadata{
				emitEndpointUpdatedAt: true,
				route:                 route,
			}
		}
	}

	messages := MessagesToEmit{}
	mappings := TCPRouteMappings{}

	for _, es := range registrations {
		for e, metadata := range es {
			msg, mapping, internalMsg := metadata.route.MessageFor(e, table.directInstanceRoute, metadata.emitEndpointUpdatedAt)
			if msg != nil {
				messages.RegistrationMessages = append(messages.RegistrationMessages, *msg)
			}
			if mapping != nil {
				mappings.Registrations = append(mappings.Registrations, *mapping)
			}
			if internalMsg != nil {
				messages.InternalRegistrationMessages = append(messages.InternalRegistrationMessages, *internalMsg)
			}
		}
	}

	for _, es := range unregistrations {
		for e, metadata := range es {
			msg, mapping, internalMsg := metadata.route.MessageFor(e, table.directInstanceRoute, false)
			if msg != nil {
				messages.UnregistrationMessages = append(messages.UnregistrationMessages, *msg)
			}
			if mapping != nil {
				mappings.Unregistrations = append(mappings.Unregistrations, *mapping)
			}
			if internalMsg != nil {
				messages.InternalUnregistrationMessages = append(messages.InternalUnregistrationMessages, *internalMsg)
			}
		}
	}
	return mappings, messages
}

func (table *internalRoutingTable) RemoveRoutes(desiredLRP *models.DesiredLRP) (TCPRouteMappings, MessagesToEmit, bool) {
	return table.SetRoutes(desiredLRP, nil)
}

func (t *internalRoutingTable) AssociationsCount() int {
	t.Lock()
	defer t.Unlock()

	count := 0
	for _, entry := range t.entries {
		count += len(entry.Routes) * len(entry.Endpoints)
	}

	return count
}

func (t *internalRoutingTable) TableSize() int {
	t.Lock()
	defer t.Unlock()

	return len(t.entries)
}

func (t *internalRoutingTable) HasExternalRoutes(actualLRP *models.ActualLRP) bool {
	for _, key := range NewRoutingKeysFromActual(actualLRP) {
		if len(t.entries[key].Routes) > 0 {
			return true
		}
	}

	return false
}

func (t *routingTable) HTTPAssociationsCount() int {
	return t.httpRoutesRoutingTable.AssociationsCount()
}

func (t *routingTable) TCPAssociationsCount() int {
	return t.tcpRoutesRoutingTable.AssociationsCount()
}

func (t *routingTable) InternalAssociationsCount() int {
	return 2 * t.internalRoutesRoutingTable.AssociationsCount()
}

func (t *routingTable) TableSize() int {
	return t.httpRoutesRoutingTable.TableSize() + t.tcpRoutesRoutingTable.TableSize() + t.internalRoutesRoutingTable.TableSize()
}

func (t *routingTable) HasExternalRoutes(actualLRP *models.ActualLRP) bool {
	return t.httpRoutesRoutingTable.HasExternalRoutes(actualLRP) || t.tcpRoutesRoutingTable.HasExternalRoutes(actualLRP)
}
