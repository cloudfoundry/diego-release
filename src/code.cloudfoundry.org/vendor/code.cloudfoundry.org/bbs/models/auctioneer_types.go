package models

// Auctioneer types moved here from code.cloudfoundry.org/auctioneer
// to break the bbs <-> auctioneer module cycle.

import (
	"errors"

	"code.cloudfoundry.org/lager/v3"
)

// AuctioneerClient is the interface bbs uses to communicate with the auctioneer.
// auctioneer implements this interface. Defined here to break the bbs <-> auctioneer cycle.
//
//go:generate counterfeiter -o fakes/fake_auctioneer_client.go . AuctioneerClient
type AuctioneerClient interface {
	RequestLRPAuctions(logger lager.Logger, traceID string, lrpStart []*LRPStartRequest) error
	RequestTaskAuctions(logger lager.Logger, traceID string, tasks []*TaskStartRequest) error
}

type TaskStartRequest struct {
	Task SchedulingTask
}

func NewTaskStartRequest(task SchedulingTask) TaskStartRequest {
	return TaskStartRequest{Task: task}
}

func NewTaskStartRequestFromModel(taskGuid, domain string, taskDef *TaskDefinition) TaskStartRequest {
	volumeMounts := []string{}
	for _, volumeMount := range taskDef.VolumeMounts {
		volumeMounts = append(volumeMounts, volumeMount.Driver)
	}
	return TaskStartRequest{
		Task: NewSchedulingTask(
			taskGuid,
			domain,
			NewResource(taskDef.MemoryMb, taskDef.DiskMb, taskDef.MaxPids),
			NewPlacementConstraint(taskDef.RootFs, taskDef.PlacementTags, volumeMounts),
		),
	}
}

func (t *TaskStartRequest) Validate() error {
	switch {
	case t.Task.TaskGuid == "":
		return errors.New("task guid is empty")
	case !t.Task.Resource.Valid():
		return errors.New("resources cannot be less than zero")
	case !t.Task.PlacementConstraint.Valid():
		return errors.New("placement constraint cannot be empty")
	default:
		return nil
	}
}

type LRPStartRequest struct {
	ProcessGuid string `json:"process_guid"`
	Domain      string `json:"domain"`
	Indices     []int  `json:"indices"`
	PlacementConstraint
	Resource
}

func NewLRPStartRequest(processGuid, domain string, indices []int, res Resource, pl PlacementConstraint) LRPStartRequest {
	return LRPStartRequest{
		ProcessGuid:         processGuid,
		Domain:              domain,
		Indices:             indices,
		Resource:            res,
		PlacementConstraint: pl,
	}
}

func NewLRPStartRequestFromModel(d *DesiredLRP, indices ...int) LRPStartRequest {
	volumeDrivers := []string{}
	for _, volumeMount := range d.VolumeMounts {
		volumeDrivers = append(volumeDrivers, volumeMount.Driver)
	}

	return NewLRPStartRequest(
		d.ProcessGuid,
		d.Domain,
		indices,
		NewResource(d.MemoryMb, d.DiskMb, d.MaxPids),
		NewPlacementConstraint(d.RootFs, d.PlacementTags, volumeDrivers),
	)
}

func NewLRPStartRequestFromSchedulingInfo(s *DesiredLRPSchedulingInfo, indices ...int) LRPStartRequest {
	return NewLRPStartRequest(
		s.ProcessGuid,
		s.Domain,
		indices,
		NewResource(s.MemoryMb, s.DiskMb, s.MaxPids),
		NewPlacementConstraint(s.RootFs, s.PlacementTags, s.VolumePlacement.DriverNames),
	)
}

func (lrpstart *LRPStartRequest) Validate() error {
	switch {
	case lrpstart.ProcessGuid == "":
		return errors.New("process guid is empty")
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
