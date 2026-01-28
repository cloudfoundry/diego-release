package routingtable

import (
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
	tcpmodels "code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/routing-info/tcp_routes"
)

type EndpointKey struct {
	InstanceGUID string
	Evacuating   bool
}

func (key *EndpointKey) String() string {
	return fmt.Sprintf(`{"InstanceGUID": "%s", "Evacuating": %t}`, key.InstanceGUID, key.Evacuating)
}

func NewEndpointKey(instanceGUID string, evacuating bool) EndpointKey {
	return EndpointKey{
		InstanceGUID: instanceGUID,
		Evacuating:   evacuating,
	}
}

type Address struct {
	Host string
	Port uint32
}

type Endpoint struct {
	InstanceGUID          string
	Index                 int32
	Host                  string
	ContainerIP           string
	Port                  uint32
	ContainerPort         uint32
	TlsProxyPort          uint32
	ContainerTlsProxyPort uint32
	Presence              models.ActualLRP_Presence
	IsolationSegment      string
	Since                 int64
	ModificationTag       *models.ModificationTag
	PreferredAddress      models.ActualLRPNetInfo_PreferredAddress
	AvailabilityZone      string
}

func (e Endpoint) key() EndpointKey {
	return EndpointKey{InstanceGUID: e.InstanceGUID, Evacuating: e.Presence == models.ActualLRP_Evacuating}
}

func (e Endpoint) IsDirectInstanceRoute(defaultIsDirectInstanceRoute bool) bool {
	switch e.PreferredAddress {
	case models.ActualLRPNetInfo_PreferredAddressInstance:
		return true
	case models.ActualLRPNetInfo_PreferredAddressHost:
		return false
	case models.ActualLRPNetInfo_PreferredAddressUnknown:
		if defaultIsDirectInstanceRoute {
			return true
		}
	}
	return false
}

func NewEndpoint(
	instanceGUID string, presence models.ActualLRP_Presence,
	host, containerIP string,
	port, containerPort uint32,
	preferredAddress models.ActualLRPNetInfo_PreferredAddress,
	modificationTag *models.ModificationTag,
	availabiltiyZone string,
) Endpoint {
	return Endpoint{
		InstanceGUID:     instanceGUID,
		Presence:         presence,
		Host:             host,
		ContainerIP:      containerIP,
		Port:             port,
		ContainerPort:    containerPort,
		PreferredAddress: preferredAddress,
		ModificationTag:  modificationTag,
		AvailabilityZone: availabiltiyZone,
	}
}

type ExternalEndpointInfo struct {
	RouterGroupGUID string
	Port            uint32
	TLSEnabled      bool
}

func (info ExternalEndpointInfo) Hash() interface{} {
	return info
}

func (info ExternalEndpointInfo) MessageFor(e Endpoint, directInstanceRoute, _ bool) (*RegistryMessage, *tcpmodels.TcpRouteMapping, *RegistryMessage) {
	tlsHostPort := -1
	tlsContainerPort := -1
	instanceGUID := ""
	if info.TLSEnabled {
		tlsHostPort = int(e.TlsProxyPort)
		tlsContainerPort = int(e.ContainerTlsProxyPort)
		instanceGUID = e.InstanceGUID
	}
	mapping := tcpmodels.NewTcpRouteMapping(
		info.RouterGroupGUID,
		uint16(info.Port),
		e.Host,
		uint16(e.Port),
		tlsHostPort,
		instanceGUID,
		nil,
		0,
		tcpmodels.ModificationTag{},
	)
	if e.IsDirectInstanceRoute(directInstanceRoute) {
		mapping = tcpmodels.NewTcpRouteMapping(
			info.RouterGroupGUID,
			uint16(info.Port),
			e.ContainerIP,
			uint16(e.ContainerPort),
			tlsContainerPort,
			instanceGUID,
			nil,
			0,
			tcpmodels.ModificationTag{},
		)
	}
	return nil, &mapping, nil
}

type ExternalEndpointInfos []ExternalEndpointInfo

type Route struct {
	Hostname         string
	RouteServiceUrl  string
	IsolationSegment string
	LogGUID          string
	Protocol         string
	MetricTags       map[string]*models.MetricTagValue
	Options          json.RawMessage
}

type routeHash struct {
	Hostname         string
	RouteServiceUrl  string
	IsolationSegment string
	LogGUID          string
	Protocol         string
}

// route hash is used to find route differences
// it needs to be dereferenced so that it can be used as a key in a hash map
func (r Route) Hash() interface{} {
	return routeHash{
		Hostname:         r.Hostname,
		RouteServiceUrl:  r.RouteServiceUrl,
		IsolationSegment: r.IsolationSegment,
		LogGUID:          r.LogGUID,
		Protocol:         r.Protocol,
	}
}

func (r Route) MessageFor(endpoint Endpoint, directInstanceAddress, emitEndpointUpdatedAt bool) (*RegistryMessage, *tcpmodels.TcpRouteMapping, *RegistryMessage) {
	generator := RegistryMessageFor
	if endpoint.IsDirectInstanceRoute(directInstanceAddress) {
		generator = InternalAddressRegistryMessageFor
	}
	msg := generator(endpoint, r, emitEndpointUpdatedAt)
	return &msg, nil, nil
}

type InternalRoute struct {
	Hostname    string
	ContainerIP string
	LogGUID     string
}

func (r InternalRoute) Hash() interface{} {
	return r
}

func (r InternalRoute) MessageFor(endpoint Endpoint, _, emitEndpointUpdatedAt bool) (*RegistryMessage, *tcpmodels.TcpRouteMapping, *RegistryMessage) {
	generator := InternalEndpointRegistryMessageFor
	msg := generator(endpoint, r, emitEndpointUpdatedAt)
	return nil, nil, &msg
}

func (entry RoutableEndpoints) copy() RoutableEndpoints {

	clone := RoutableEndpoints{
		Domain:           entry.Domain,
		Endpoints:        map[EndpointKey]Endpoint{},
		Routes:           make([]routeMapping, len(entry.Routes)),
		DesiredInstances: entry.DesiredInstances,
		ModificationTag:  entry.ModificationTag,
	}

	copy(clone.Routes, entry.Routes)

	for k, v := range entry.Endpoints {
		clone.Endpoints[k] = v
	}

	return clone
}

type RoutableEndpoints struct {
	Domain           string
	Routes           []routeMapping
	Endpoints        map[EndpointKey]Endpoint
	DesiredInstances int32
	ModificationTag  *models.ModificationTag
}

type InternalRoutableEndpoints struct {
	Routes           []InternalRoute
	Endpoints        map[EndpointKey]Endpoint
	DesiredInstances int32
	ModificationTag  *models.ModificationTag
}

func NewEndpointsFromActual(actualLRP *models.ActualLRP) []Endpoint {
	endpoints := []Endpoint{}
	for _, portMapping := range actualLRP.ActualLrpNetInfo.Ports {
		if portMapping != nil {
			endpoint := Endpoint{
				InstanceGUID:          actualLRP.ActualLrpInstanceKey.InstanceGuid,
				Index:                 actualLRP.ActualLrpKey.Index,
				Host:                  actualLRP.ActualLrpNetInfo.Address,
				ContainerIP:           actualLRP.ActualLrpNetInfo.InstanceAddress,
				Port:                  portMapping.HostPort,
				ContainerPort:         portMapping.ContainerPort,
				Presence:              actualLRP.Presence,
				ModificationTag:       &actualLRP.ModificationTag,
				TlsProxyPort:          portMapping.HostTlsProxyPort,
				ContainerTlsProxyPort: portMapping.ContainerTlsProxyPort,
				Since:                 actualLRP.Since,
				PreferredAddress:      actualLRP.ActualLrpNetInfo.PreferredAddress,
				AvailabilityZone:      actualLRP.AvailabilityZone,
			}
			endpoints = append(endpoints, endpoint)
		}
	}

	return endpoints
}

func NewRoutingKeysFromActual(actualLRP *models.ActualLRP) RoutingKeys {
	keys := RoutingKeys{}
	for _, portMapping := range actualLRP.ActualLrpNetInfo.Ports {
		keys = append(keys, NewRoutingKey(actualLRP.ActualLrpKey.ProcessGuid, portMapping.ContainerPort))

		if portMapping.HostTlsProxyPort != 0 && portMapping.ContainerTlsProxyPort != 0 {
			keys = append(keys, NewRoutingKey(actualLRP.ActualLrpKey.ProcessGuid, portMapping.ContainerTlsProxyPort))
		}
	}

	return keys
}

func NewRoutingKeysFromDesired(desired *models.DesiredLRP) RoutingKeys {
	keys := RoutingKeys{}
	routes, err := tcp_routes.TCPRoutesFromRoutingInfo(desired.Routes)
	if err != nil {
		return keys
	}
	for _, r := range routes {
		keys = append(keys, NewRoutingKey(desired.ProcessGuid, r.ContainerPort))
	}

	return keys
}

func (e ExternalEndpointInfos) HasNoExternalPorts(logger lager.Logger) bool {
	if len(e) == 0 {
		logger.Debug("no-external-port")
		return true
	}
	// This originally checked if Port was 0, I think to see if it was a zero value, check and make sure
	return false
}

type RoutingKeys []RoutingKey

type RoutingKey struct {
	ProcessGUID   string
	ContainerPort uint32
}

func NewRoutingKey(processGUID string, containerPort uint32) RoutingKey {
	return RoutingKey{
		ProcessGUID:   processGUID,
		ContainerPort: containerPort,
	}
}

func (e ExternalEndpointInfos) ContainsExternalPort(port uint32) bool {
	for _, existing := range e {
		if existing.Port == port {
			return true
		}
	}
	return false
}

func (lhs RoutingKeys) Remove(rhs RoutingKeys) RoutingKeys {
	result := RoutingKeys{}
	for _, lhsKey := range lhs {
		if !rhs.containsRoutingKey(lhsKey) {
			result = append(result, lhsKey)
		}
	}
	return result
}

func (lhs RoutingKeys) containsRoutingKey(routingKey RoutingKey) bool {
	for _, lhsKey := range lhs {
		if lhsKey == routingKey {
			return true
		}
	}
	return false
}
