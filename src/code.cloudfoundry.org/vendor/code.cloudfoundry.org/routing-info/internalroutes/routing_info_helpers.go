package internalroutes

import (
	"code.cloudfoundry.org/bbs/models"
)

const INTERNAL_ROUTER = models.InternalRouter

// InternalRoute and InternalRoutes are type aliases for the canonical types
// now defined in code.cloudfoundry.org/bbs/models to break the bbs→internalroutes
// import cycle while keeping all existing callers building unchanged.
type InternalRoute = models.InternalRoute
type InternalRoutes = models.InternalRoutes

// InternalRoutesFromRoutingInfo delegates to the canonical implementation in bbs/models.
func InternalRoutesFromRoutingInfo(routingInfo models.Routes) (InternalRoutes, error) {
	return models.InternalRoutesFromRoutingInfo(routingInfo)
}
