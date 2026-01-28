package routingtable

import (
	"encoding/json"
	"fmt"
	"strconv"

	"code.cloudfoundry.org/bbs/models"
)

type RegistryMessage struct {
	Host                 string            `json:"host"`
	Port                 uint32            `json:"port"`
	TlsPort              uint32            `json:"tls_port,omitempty"`
	URIs                 []string          `json:"uris"`
	Protocol             string            `json:"protocol,omitempty"`
	App                  string            `json:"app,omitempty" hash:"ignore"`
	RouteServiceUrl      string            `json:"route_service_url,omitempty" hash:"ignore"`
	PrivateInstanceId    string            `json:"private_instance_id,omitempty" hash:"ignore"`
	PrivateInstanceIndex string            `json:"private_instance_index,omitempty" hash:"ignore"`
	ServerCertDomainSAN  string            `json:"server_cert_domain_san,omitempty" hash:"ignore"`
	IsolationSegment     string            `json:"isolation_segment,omitempty" hash:"ignore"`
	EndpointUpdatedAtNs  int64             `json:"endpoint_updated_at_ns,omitempty" hash:"ignore"`
	Tags                 map[string]string `json:"tags,omitempty" hash:"ignore"`
	AvailabilityZone     string            `json:"availability_zone,omitempty" hash:"ignore"`
	Options              json.RawMessage   `json:"options,omitempty" hash:"ignore"`
}

func RegistryMessageFor(endpoint Endpoint, route Route, emitEndpointUpdatedAt bool) RegistryMessage {
	var index string
	if endpoint.InstanceGUID != "" {
		index = fmt.Sprintf("%d", endpoint.Index)
	}
	since := endpoint.Since
	if !emitEndpointUpdatedAt {
		since = 0
	}
	// route.MetricTags -> Populate Tags
	return RegistryMessage{
		URIs:                []string{route.Hostname},
		Host:                endpoint.Host,
		Port:                endpoint.Port,
		TlsPort:             endpoint.TlsProxyPort,
		Protocol:            route.Protocol,
		App:                 route.LogGUID,
		IsolationSegment:    route.IsolationSegment,
		Tags:                populateMetricTags(route.MetricTags, endpoint),
		AvailabilityZone:    endpoint.AvailabilityZone,
		EndpointUpdatedAtNs: since,

		PrivateInstanceId:    endpoint.InstanceGUID,
		PrivateInstanceIndex: index,
		ServerCertDomainSAN:  endpoint.InstanceGUID,
		RouteServiceUrl:      route.RouteServiceUrl,
		Options:              route.Options,
	}
}

// This is used when RE is emitting container ip addr/port as opposed to host ip add/port
func InternalAddressRegistryMessageFor(endpoint Endpoint, route Route, emitEndpointUpdatedAt bool) RegistryMessage {
	var index string
	if endpoint.InstanceGUID != "" {
		index = fmt.Sprintf("%d", endpoint.Index)
	}
	since := endpoint.Since
	if !emitEndpointUpdatedAt {
		since = 0
	}
	return RegistryMessage{
		URIs:             []string{route.Hostname},
		Host:             endpoint.ContainerIP,
		Port:             endpoint.ContainerPort,
		TlsPort:          endpoint.ContainerTlsProxyPort,
		Protocol:         route.Protocol,
		App:              route.LogGUID,
		IsolationSegment: route.IsolationSegment,
		Tags:             populateMetricTags(route.MetricTags, endpoint),
		AvailabilityZone: endpoint.AvailabilityZone,

		ServerCertDomainSAN:  endpoint.InstanceGUID,
		PrivateInstanceId:    endpoint.InstanceGUID,
		PrivateInstanceIndex: index,
		EndpointUpdatedAtNs:  since,
		RouteServiceUrl:      route.RouteServiceUrl,
	}
}

// This is used to generate registry messages for Internal routes
func InternalEndpointRegistryMessageFor(endpoint Endpoint, route InternalRoute, emitEndpointUpdatedAt bool) RegistryMessage {
	var index string
	if endpoint.InstanceGUID != "" {
		index = fmt.Sprintf("%d", endpoint.Index)
	}
	since := endpoint.Since
	if !emitEndpointUpdatedAt {
		since = 0
	}
	return RegistryMessage{
		URIs:                []string{route.Hostname, fmt.Sprintf("%s.%s", index, route.Hostname)},
		Host:                endpoint.ContainerIP,
		App:                 route.LogGUID,
		Tags:                map[string]string{"component": "route-emitter"},
		AvailabilityZone:    endpoint.AvailabilityZone,
		EndpointUpdatedAtNs: since,

		PrivateInstanceIndex: index,
	}
}

type ExternalServiceGreetingMessage struct {
	MinimumRegisterInterval int `json:"minimumRegisterIntervalInSeconds"`
	PruneThresholdInSeconds int `json:"pruneThresholdInSeconds"`
}

func populateMetricTags(input map[string]*models.MetricTagValue, endpoint Endpoint) map[string]string {
	tags := map[string]string{}
	for k, v := range input {
		var value string
		if v.Dynamic > 0 {
			switch v.Dynamic {
			case models.MetricTagValue_MetricTagDynamicValueIndex:
				value = strconv.FormatInt(int64(endpoint.Index), 10)
			case models.MetricTagValue_MetricTagDynamicValueInstanceGuid:
				value = endpoint.InstanceGUID
			}
		} else {
			value = v.Static
		}
		tags[k] = value
	}
	tags["component"] = "route-emitter"
	return tags
}
