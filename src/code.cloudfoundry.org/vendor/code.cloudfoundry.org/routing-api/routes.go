package routing_api

import "github.com/tedsuo/rata"

const (
	UpsertRoute           = "UpsertRoute"
	DeleteRoute           = "Delete"
	ListRoute             = "List"
	EventStreamRoute      = "EventStream"
	ListRouterGroups      = "ListRouterGroups"
	UpdateRouterGroup     = "UpdateRouterGroup"
	CreateRouterGroup     = "CreateRouterGroup"
	DeleteRouterGroup     = "DeleteRouterGroup"
	UpsertTcpRouteMapping = "UpsertTcpRouteMapping"
	DeleteTcpRouteMapping = "DeleteTcpRouteMapping"
	ListTcpRouteMapping   = "ListTcpRouteMapping"
	EventStreamTcpRoute   = "TcpRouteEventStream"
)

var RoutesMap = map[string]rata.Route{UpsertRoute: {Path: "/routing/v1/routes", Method: "POST", Name: UpsertRoute},
	DeleteRoute:           {Path: "/routing/v1/routes", Method: "DELETE", Name: DeleteRoute},
	ListRoute:             {Path: "/routing/v1/routes", Method: "GET", Name: ListRoute},
	EventStreamRoute:      {Path: "/routing/v1/events", Method: "GET", Name: EventStreamRoute},
	CreateRouterGroup:     {Path: "/routing/v1/router_groups", Method: "POST", Name: CreateRouterGroup},
	DeleteRouterGroup:     {Path: "/routing/v1/router_groups/:guid", Method: "DELETE", Name: DeleteRouterGroup},
	ListRouterGroups:      {Path: "/routing/v1/router_groups", Method: "GET", Name: ListRouterGroups},
	UpdateRouterGroup:     {Path: "/routing/v1/router_groups/:guid", Method: "PUT", Name: UpdateRouterGroup},
	UpsertTcpRouteMapping: {Path: "/routing/v1/tcp_routes/create", Method: "POST", Name: UpsertTcpRouteMapping},
	DeleteTcpRouteMapping: {Path: "/routing/v1/tcp_routes/delete", Method: "POST", Name: DeleteTcpRouteMapping},
	ListTcpRouteMapping:   {Path: "/routing/v1/tcp_routes", Method: "GET", Name: ListTcpRouteMapping},
	EventStreamTcpRoute:   {Path: "/routing/v1/tcp_routes/events", Method: "GET", Name: EventStreamTcpRoute},
}

func Routes() rata.Routes {
	var routes rata.Routes
	for _, r := range RoutesMap {
		routes = append(routes, r)
	}

	return routes
}
