package routingtable

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/routing-info/cfroutes"
	"code.cloudfoundry.org/routing-info/internalroutes"
	"code.cloudfoundry.org/routing-info/tcp_routes"
)

func DesiredLRPData(lrp *models.DesiredLRP) lager.Data {
	if lrp == nil || lrp.Routes == nil {
		return lager.Data{}
	}

	logRoutes := make(models.Routes)
	logRoutes[cfroutes.CF_ROUTER] = (*lrp.Routes)[cfroutes.CF_ROUTER]
	logRoutes[tcp_routes.TCP_ROUTER] = (*lrp.Routes)[tcp_routes.TCP_ROUTER]
	logRoutes[internalroutes.INTERNAL_ROUTER] = (*lrp.Routes)[internalroutes.INTERNAL_ROUTER]

	return lager.Data{
		"process-guid": lrp.ProcessGuid,
		"routes":       logRoutes,
		"domain":       lrp.Domain,
		"instances":    lrp.GetInstances(),
	}
}

func ActualLRPData(actualLRP *models.ActualLRP) lager.Data {
	return lager.Data{
		"process-guid":  actualLRP.ActualLrpKey.ProcessGuid,
		"index":         actualLRP.ActualLrpKey.Index,
		"domain":        actualLRP.ActualLrpKey.Domain,
		"instance-guid": actualLRP.ActualLrpInstanceKey.InstanceGuid,
		"cell-id":       actualLRP.ActualLrpInstanceKey.CellId,
		"address":       actualLRP.ActualLrpNetInfo.Address,
		"ports":         actualLRP.ActualLrpNetInfo.Ports,
		"evacuating":    actualLRP.Presence == models.ActualLRP_Evacuating,
		"state":         actualLRP.State,
	}
}
