package rep

import "github.com/tedsuo/rata"

const (
	StateRoute            = "STATE"
	ContainerMetricsRoute = "ContainerMetrics"
	PerformRoute          = "PERFORM"

	UpdateLRPInstanceRoute    = "UpdateLRPInstance"
	UpdateLRPInstanceRoute_r0 = "UpdateLRPInstance_r0"
	StopLRPInstanceRoute      = "StopLRPInstance"
	CancelTaskRoute           = "CancelTask"

	SimResetRoute = "RESET"

	PingRoute     = "Ping"
	EvacuateRoute = "Evacuate"
)

func NewRoutes(networkAccessible bool) rata.Routes {
	var routes rata.Routes

	if networkAccessible {
		routes = append(routes,
			rata.Route{Path: "/state", Method: "GET", Name: StateRoute},
			rata.Route{Path: "/container_metrics", Method: "GET", Name: ContainerMetricsRoute},
			rata.Route{Path: "/work", Method: "POST", Name: PerformRoute},

			rata.Route{Path: "/v2/lrps/:process_guid/instances/:instance_guid", Method: "PUT", Name: UpdateLRPInstanceRoute},
			rata.Route{Path: "/v1/lrps/:process_guid/instances/:instance_guid", Method: "PUT", Name: UpdateLRPInstanceRoute_r0},
			rata.Route{Path: "/v1/lrps/:process_guid/instances/:instance_guid/stop", Method: "POST", Name: StopLRPInstanceRoute},
			rata.Route{Path: "/v1/tasks/:task_guid/cancel", Method: "POST", Name: CancelTaskRoute},

			rata.Route{Path: "/sim/reset", Method: "POST", Name: SimResetRoute},
		)
	} else {
		routes = append(routes,
			rata.Route{Path: "/ping", Method: "GET", Name: PingRoute},
			rata.Route{Path: "/evacuate", Method: "POST", Name: EvacuateRoute},
		)
	}
	return routes

}

var RoutesLocalhostOnly = NewRoutes(false)
var RoutesNetworkAccessible = NewRoutes(true)
var Routes = append(RoutesLocalhostOnly, RoutesNetworkAccessible...)
