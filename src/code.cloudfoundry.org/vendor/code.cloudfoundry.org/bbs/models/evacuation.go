package models

func (request *EvacuateRunningActualLRPRequest) SetRoutable(routable bool) {
	request.OptionalRoutable = &EvacuateRunningActualLRPRequest_Routable{
		Routable: routable,
	}
}

func (request *EvacuateRunningActualLRPRequest) RoutableExists() bool {
	_, ok := request.GetOptionalRoutable().(*EvacuateRunningActualLRPRequest_Routable)
	return ok
}
