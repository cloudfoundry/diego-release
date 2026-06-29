package tcp_routes

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/models"
)

const TCP_ROUTER = "tcp-router"

type TCPRoutes []TCPRoute

type TCPRoute struct {
	RouterGroupGuid      string  `json:"router_group_guid"`
	ExternalPort         uint32  `json:"external_port"`
	ContainerPort        uint32  `json:"container_port"`
	SniHostname          *string `json:"backend_sni_hostname,omitempty"`
	TerminateFrontendTLS bool    `json:"terminate_frontend_tls,omitempty"`
}

func (c TCPRoutes) RoutingInfo() *models.Routes {
	data, _ := json.Marshal(c)
	routingInfo := json.RawMessage(data)
	return &models.Routes{
		TCP_ROUTER: &routingInfo,
	}
}

func TCPRoutesFromRoutingInfo(routingInfoPtr *models.Routes) (TCPRoutes, error) {
	if routingInfoPtr == nil {
		return nil, nil
	}

	routingInfo := *routingInfoPtr

	data, found := routingInfo[TCP_ROUTER]
	if !found {
		return nil, nil
	}

	if data == nil {
		return nil, nil
	}

	routes := TCPRoutes{}
	err := json.Unmarshal(*data, &routes)

	return routes, err
}
