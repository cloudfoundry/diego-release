package auctioneer

import (
	"errors"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/rep"
)

type TaskStartRequest struct {
	rep.Task
}

func NewTaskStartRequest(task rep.Task) TaskStartRequest {
	return TaskStartRequest{task}
}

func NewTaskStartRequestFromModel(taskGuid, domain string, taskDef *models.TaskDefinition) TaskStartRequest {
	volumeMounts := []string{}
	for _, volumeMount := range taskDef.VolumeMounts {
		volumeMounts = append(volumeMounts, volumeMount.Driver)
	}
	return TaskStartRequest{
		rep.NewTask(
			taskGuid,
			domain,
			rep.NewResource(taskDef.MemoryMb, taskDef.DiskMb, taskDef.MaxPids),
			rep.NewPlacementConstraint(taskDef.RootFs, taskDef.PlacementTags, volumeMounts),
		),
	}
}

func (t *TaskStartRequest) Validate() error {
	switch {
	case t.TaskGuid == "":
		return errors.New("task guid is empty")
	case !t.Resource.Valid():
		return errors.New("resources cannot be less than zero")
	case !t.PlacementConstraint.Valid():
		return errors.New("placement constraint cannot be empty")
	default:
		return nil
	}
}

type LRPStartRequest struct {
	ProcessGuid string `json:"process_guid"`
	Domain      string `json:"domain"`
	Indices     []int  `json:"indices"`
	rep.PlacementConstraint
	rep.Resource
}

func NewLRPStartRequest(processGuid, domain string, indices []int, res rep.Resource, pl rep.PlacementConstraint) LRPStartRequest {
	return LRPStartRequest{
		ProcessGuid:         processGuid,
		Domain:              domain,
		Indices:             indices,
		Resource:            res,
		PlacementConstraint: pl,
	}
}

func NewLRPStartRequestFromModel(d *models.DesiredLRP, indices ...int) LRPStartRequest {
	volumeDrivers := []string{}
	for _, volumeMount := range d.VolumeMounts {
		volumeDrivers = append(volumeDrivers, volumeMount.Driver)
	}

	return NewLRPStartRequest(
		d.ProcessGuid,
		d.Domain,
		indices,
		rep.NewResource(d.MemoryMb, d.DiskMb, d.MaxPids),
		rep.NewPlacementConstraint(d.RootFs, d.PlacementTags, volumeDrivers),
	)
}

func NewLRPStartRequestFromSchedulingInfo(s *models.DesiredLRPSchedulingInfo, indices ...int) LRPStartRequest {
	return NewLRPStartRequest(
		s.DesiredLrpKey.ProcessGuid,
		s.DesiredLrpKey.Domain,
		indices,
		rep.NewResource(s.DesiredLrpResource.MemoryMb, s.DesiredLrpResource.DiskMb, s.DesiredLrpResource.MaxPids),
		rep.NewPlacementConstraint(s.DesiredLrpResource.RootFs, s.PlacementTags, s.VolumePlacement.DriverNames),
	)
}

func (lrpstart *LRPStartRequest) Validate() error {
	switch {
	case lrpstart.ProcessGuid == "":
		return errors.New("proccess guid is empty")
	case lrpstart.Domain == "":
		return errors.New("domain is empty")
	case len(lrpstart.Indices) == 0:
		return errors.New("indices must not be empty")
	case !lrpstart.Resource.Valid():
		return errors.New("resources cannot be less than 0")
	case !lrpstart.PlacementConstraint.Valid():
		return errors.New("placement constraint cannot be empty")
	default:
		return nil
	}
}
