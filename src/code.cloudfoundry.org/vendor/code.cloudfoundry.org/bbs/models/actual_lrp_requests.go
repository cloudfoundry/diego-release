package models

import "encoding/json"

func (request *ActualLRPsRequest) Validate() error {
	return nil
}

func (request *ActualLRPsRequest) SetIndex(index int32) {
	request.OptionalIndex = &ActualLRPsRequest_Index{Index: index}
}

func (request ActualLRPsRequest) IndexExists() bool {
	_, ok := request.GetOptionalIndex().(*ActualLRPsRequest_Index)
	return ok
}

type internalActualLRPsRequest struct {
	Domain      string `json:"domain"`
	CellId      string `json:"cell_id"`
	ProcessGuid string `json:"process_guid"`
	Index       *int32 `json:"index,omitempty"`
}

func (request *ActualLRPsRequest) UnmarshalJSON(data []byte) error {
	var internalRequest internalActualLRPsRequest
	if err := json.Unmarshal(data, &internalRequest); err != nil {
		return err
	}

	request.Domain = internalRequest.Domain
	request.CellId = internalRequest.CellId
	request.ProcessGuid = internalRequest.ProcessGuid
	if internalRequest.Index != nil {
		request.SetIndex(*internalRequest.Index)
	}

	return nil
}

func (request ActualLRPsRequest) MarshalJSON() ([]byte, error) {
	internalRequest := internalActualLRPsRequest{
		Domain:      request.Domain,
		CellId:      request.CellId,
		ProcessGuid: request.ProcessGuid,
	}

	if request.IndexExists() {
		i := request.GetIndex()
		internalRequest.Index = &i
	}
	return json.Marshal(internalRequest)
}

// DEPRECATED
func (request *ActualLRPGroupsRequest) Validate() error {
	return nil
}

// DEPRECATED
func (request *ActualLRPGroupsByProcessGuidRequest) Validate() error {
	var validationError ValidationError

	if request.ProcessGuid == "" {
		validationError = validationError.Append(ErrInvalidField{"process_guid"})
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

// DEPRECATED
func (request *ActualLRPGroupByProcessGuidAndIndexRequest) Validate() error {
	var validationError ValidationError

	if request.ProcessGuid == "" {
		validationError = validationError.Append(ErrInvalidField{"process_guid"})
	}

	if request.Index < 0 {
		validationError = validationError.Append(ErrInvalidField{"index"})
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (request *RemoveActualLRPRequest) Validate() error {
	var validationError ValidationError

	if request.ProcessGuid == "" {
		validationError = validationError.Append(ErrInvalidField{"process_guid"})
	}

	if request.Index < 0 {
		validationError = validationError.Append(ErrInvalidField{"index"})
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (request *ClaimActualLRPRequest) Validate() error {
	var validationError ValidationError

	if request.ProcessGuid == "" {
		validationError = validationError.Append(ErrInvalidField{"process_guid"})
	}

	if request.ActualLrpInstanceKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_instance_key"})
	} else if err := request.ActualLrpInstanceKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (request *StartActualLRPRequest) Validate() error {
	var validationError ValidationError

	if request.ActualLrpKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_key"})
	} else if err := request.ActualLrpKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if request.ActualLrpInstanceKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_instance_key"})
	} else if err := request.ActualLrpInstanceKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if request.ActualLrpNetInfo == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_net_info"})
	} else if err := request.ActualLrpNetInfo.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (request *StartActualLRPRequest) SetRoutable(routable bool) {
	request.OptionalRoutable = &StartActualLRPRequest_Routable{
		Routable: routable,
	}
}

func (request *StartActualLRPRequest) RoutableExists() bool {
	_, ok := request.GetOptionalRoutable().(*StartActualLRPRequest_Routable)
	return ok
}

func (request *CrashActualLRPRequest) Validate() error {
	var validationError ValidationError

	if request.ActualLrpKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_key"})
	} else if err := request.ActualLrpKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if request.ActualLrpInstanceKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_instance_key"})
	} else if err := request.ActualLrpInstanceKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (request *FailActualLRPRequest) Validate() error {
	var validationError ValidationError

	if request.ActualLrpKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_key"})
	} else if err := request.ActualLrpKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if request.ErrorMessage == "" {
		validationError = validationError.Append(ErrInvalidField{"error_message"})
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (request *RetireActualLRPRequest) Validate() error {
	var validationError ValidationError

	if request.ActualLrpKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_key"})
	} else if err := request.ActualLrpKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (request *RemoveEvacuatingActualLRPRequest) Validate() error {
	var validationError ValidationError

	if request.ActualLrpKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_key"})
	} else if err := request.ActualLrpKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if request.ActualLrpInstanceKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_instance_key"})
	} else if err := request.ActualLrpInstanceKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (request *EvacuateClaimedActualLRPRequest) Validate() error {
	var validationError ValidationError

	if request.ActualLrpKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_key"})
	} else if err := request.ActualLrpKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if request.ActualLrpInstanceKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_instance_key"})
	} else if err := request.ActualLrpInstanceKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (request *EvacuateCrashedActualLRPRequest) Validate() error {
	var validationError ValidationError

	if request.ActualLrpKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_key"})
	} else if err := request.ActualLrpKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if request.ActualLrpInstanceKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_instance_key"})
	} else if err := request.ActualLrpInstanceKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if request.ErrorMessage == "" {
		validationError = validationError.Append(ErrInvalidField{"error_message"})
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (request *EvacuateStoppedActualLRPRequest) Validate() error {
	var validationError ValidationError

	if request.ActualLrpKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_key"})
	} else if err := request.ActualLrpKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if request.ActualLrpInstanceKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_instance_key"})
	} else if err := request.ActualLrpInstanceKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (request *EvacuateRunningActualLRPRequest) Validate() error {
	var validationError ValidationError

	if request.ActualLrpKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_key"})
	} else if err := request.ActualLrpKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if request.ActualLrpInstanceKey == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_instance_key"})
	} else if err := request.ActualLrpInstanceKey.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	if request.ActualLrpNetInfo == nil {
		validationError = validationError.Append(ErrInvalidField{"actual_lrp_net_info"})
	} else if err := request.ActualLrpNetInfo.Validate(); err != nil {
		validationError = validationError.Append(err)
	}
	if !validationError.Empty() {
		return validationError
	}

	return nil
}
