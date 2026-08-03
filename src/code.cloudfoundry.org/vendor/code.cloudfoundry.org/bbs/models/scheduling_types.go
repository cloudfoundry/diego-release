package models

// Scheduling and placement types moved here from code.cloudfoundry.org/rep
// to break the bbs <-> rep module cycle.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const InternalRouter = "internal-router"

var ErrorIncompatibleRootfs = errors.New("rootfs not found")

const StackVersionFile = "/etc/stack-version"

type Resources struct {
	MemoryMB   int32
	DiskMB     int32
	Containers int
}

func NewResources(memoryMb, diskMb int32, containerCount int) Resources {
	return Resources{memoryMb, diskMb, containerCount}
}

func (r *Resources) Copy() Resources {
	return *r
}

func (r *Resources) Subtract(res *Resource) {
	r.MemoryMB -= res.MemoryMB
	r.DiskMB -= res.DiskMB
	r.Containers -= 1
}

func (r *Resources) ComputeScore(total *Resources) float64 {
	fractionUsedMemory := 1.0 - float64(r.MemoryMB)/float64(total.MemoryMB)
	fractionUsedDisk := 1.0 - float64(r.DiskMB)/float64(total.DiskMB)
	fractionUsedContainers := 1.0 - float64(r.Containers)/float64(total.Containers)
	return (fractionUsedMemory + fractionUsedDisk + fractionUsedContainers) / 3.0
}

type Resource struct {
	MemoryMB int32
	DiskMB   int32
	MaxPids  int32
}

func NewResource(memoryMb, diskMb int32, maxPids int32) Resource {
	return Resource{MemoryMB: memoryMb, DiskMB: diskMb, MaxPids: maxPids}
}

func (r *Resource) Valid() bool {
	return r.DiskMB >= 0 && r.MemoryMB >= 0
}

func (r *Resource) Copy() Resource {
	return NewResource(r.MemoryMB, r.DiskMB, r.MaxPids)
}

type PlacementConstraint struct {
	PlacementTags []string
	VolumeDrivers []string
	RootFs        string
}

func NewPlacementConstraint(rootFs string, placementTags, volumeDrivers []string) PlacementConstraint {
	return PlacementConstraint{PlacementTags: placementTags, VolumeDrivers: volumeDrivers, RootFs: rootFs}
}

func (p *PlacementConstraint) Valid() bool {
	return p.RootFs != ""
}

// SchedulingLRP is the rep-facing LRP type used in scheduling.
// Named SchedulingLRP to avoid conflict with protobuf-generated ActualLRP/DesiredLRP.
type SchedulingLRP struct {
	InstanceGUID string `json:"instance_guid"`
	ActualLRPKey
	PlacementConstraint
	Resource
	State string `json:"state"`
}

func NewSchedulingLRP(instanceGUID string, key ActualLRPKey, res Resource, pc PlacementConstraint) SchedulingLRP {
	return SchedulingLRP{instanceGUID, key, pc, res, ""}
}

func (lrp *SchedulingLRP) Identifier() string {
	return fmt.Sprintf("%s.%d", lrp.ProcessGuid, lrp.Index)
}

func (lrp *SchedulingLRP) Copy() SchedulingLRP {
	return NewSchedulingLRP(lrp.InstanceGUID, lrp.ActualLRPKey, lrp.Resource, lrp.PlacementConstraint)
}

// InternalRoute is a hostname for internal routing.
// Inlined from code.cloudfoundry.org/routing-info/internalroutes to avoid an import cycle.
type InternalRoute struct {
	Hostname string `json:"hostname"`
}

// InternalRoutes is a slice of InternalRoute entries.
type InternalRoutes []InternalRoute

// RoutingInfo serialises the routes for use in a Routes map.
func (r InternalRoutes) RoutingInfo() Routes {
	data, _ := json.Marshal(r)
	routingInfo := json.RawMessage(data)
	return Routes{
		InternalRouter: &routingInfo,
	}
}

// Equal returns true if the two InternalRoutes slices contain the same hostnames.
func (r InternalRoutes) Equal(other InternalRoutes) bool {
	if len(r) != len(other) {
		return false
	}
	set := make(map[string]bool, len(r))
	for _, route := range r {
		set[route.Hostname] = true
	}
	for _, route := range other {
		if !set[route.Hostname] {
			return false
		}
	}
	return true
}

// InternalRoutesFromRoutingInfo deserialises InternalRoutes from a Routes map.
func InternalRoutesFromRoutingInfo(routingInfo Routes) (InternalRoutes, error) {
	if routingInfo == nil {
		return nil, nil
	}
	data, found := routingInfo[InternalRouter]
	if !found || data == nil {
		return nil, nil
	}
	var routes InternalRoutes
	err := json.Unmarshal(*data, &routes)
	return routes, err
}

// LRPUpdate carries internal-route and metric-tag updates for a running LRP.
type LRPUpdate struct {
	InstanceGUID string `json:"instance_guid"`
	ActualLRPKey
	InternalRoutes InternalRoutes    `json:"internal_routes"`
	MetricTags     map[string]string `json:"metric_tags"`
}

func NewLRPUpdate(instanceGUID string, key ActualLRPKey, internalRoutes InternalRoutes, metricTags map[string]string) LRPUpdate {
	return LRPUpdate{
		InstanceGUID:   instanceGUID,
		ActualLRPKey:   key,
		InternalRoutes: internalRoutes,
		MetricTags:     metricTags,
	}
}

// SchedulingTask is the rep-facing task type used in scheduling.
// Named SchedulingTask to avoid conflict with the protobuf-generated Task type.
type SchedulingTask struct {
	TaskGuid string
	Domain   string
	PlacementConstraint
	Resource
	State  Task_State `json:"state"`
	Failed bool       `json:"failed"`
}

func NewSchedulingTask(guid string, domain string, res Resource, pc PlacementConstraint) SchedulingTask {
	return SchedulingTask{guid, domain, pc, res, Task_Invalid, false}
}

func (task *SchedulingTask) Identifier() string {
	return task.TaskGuid
}

func (task SchedulingTask) Copy() SchedulingTask {
	return task
}

// Work represents a bundle of LRPs and tasks for a rep cell.
type Work struct {
	LRPs   []SchedulingLRP
	Tasks  []SchedulingTask
	CellID string `json:"cell_id,omitempty"`
}

type InsufficientResourcesError struct {
	Problems map[string]struct{}
}

func (i InsufficientResourcesError) Error() string {
	if len(i.Problems) == 0 {
		return "insufficient resources"
	}

	keys := []string{}
	for key := range i.Problems {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return fmt.Sprintf("insufficient resources: %s", strings.Join(keys, ", "))
}

type CellState struct {
	RepURL                  string `json:"rep_url"`
	CellID                  string `json:"cell_id"`
	CellIndex               int    `json:"cell_index"`
	RootFSProviders         RootFSProviders
	AvailableResources      Resources
	TotalResources          Resources
	LRPs                    []SchedulingLRP
	Tasks                   []SchedulingTask
	StartingContainerCount  int
	Zone                    string
	Evacuating              bool
	VolumeDrivers           []string
	PlacementTags           []string
	OptionalPlacementTags   []string
	ProxyMemoryAllocationMB int
}

func NewCellState(
	cellID string,
	cellIndex int,
	repURL string,
	root RootFSProviders,
	avail Resources,
	total Resources,
	lrps []SchedulingLRP,
	tasks []SchedulingTask,
	zone string,
	startingContainerCount int,
	isEvac bool,
	volumeDrivers []string,
	placementTags []string,
	optionalPlacementTags []string,
	proxyMemoryAllocation int,
) CellState {
	return CellState{
		CellID:                  cellID,
		CellIndex:               cellIndex,
		RepURL:                  repURL,
		RootFSProviders:         root,
		AvailableResources:      avail,
		TotalResources:          total,
		LRPs:                    lrps,
		Tasks:                   tasks,
		Zone:                    zone,
		StartingContainerCount:  startingContainerCount,
		Evacuating:              isEvac,
		VolumeDrivers:           volumeDrivers,
		PlacementTags:           placementTags,
		OptionalPlacementTags:   optionalPlacementTags,
		ProxyMemoryAllocationMB: proxyMemoryAllocation,
	}
}

func (c *CellState) AddLRP(lrp *SchedulingLRP) {
	c.AvailableResources.Subtract(&lrp.Resource)
	c.StartingContainerCount += 1
	c.LRPs = append(c.LRPs, *lrp)
}

func (c *CellState) AddTask(task *SchedulingTask) {
	c.AvailableResources.Subtract(&task.Resource)
	c.StartingContainerCount += 1
	c.Tasks = append(c.Tasks, *task)
}

func (c *CellState) ResourceMatch(res *Resource) error {
	problems := map[string]struct{}{}

	if c.AvailableResources.DiskMB < res.DiskMB {
		problems["disk"] = struct{}{}
	}
	if c.AvailableResources.MemoryMB < res.MemoryMB {
		problems["memory"] = struct{}{}
	}
	if c.AvailableResources.Containers < 1 {
		problems["containers"] = struct{}{}
	}
	if len(problems) == 0 {
		return nil
	}

	return InsufficientResourcesError{Problems: problems}
}

func (c CellState) ComputeScore(res *Resource, startingContainerWeight float64) float64 {
	remainingResources := c.AvailableResources.Copy()
	remainingResources.Subtract(res)
	startingContainerScore := float64(c.StartingContainerCount) * startingContainerWeight
	return remainingResources.ComputeScore(&c.TotalResources) + startingContainerScore
}

func (c *CellState) MatchRootFS(rootfs string) bool {
	rootFSURL, err := url.Parse(rootfs)
	if err != nil {
		return false
	}
	return c.RootFSProviders.Match(*rootFSURL)
}

func (c *CellState) MatchVolumeDrivers(volumeDrivers []string) bool {
	for _, requestedDriver := range volumeDrivers {
		found := false
		for _, actualDriver := range c.VolumeDrivers {
			if requestedDriver == actualDriver {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (c *CellState) MatchPlacementTags(desiredPlacementTags []string) bool {
	desiredTags := toPlacementSet(desiredPlacementTags)
	optionalTags := toPlacementSet(c.OptionalPlacementTags)
	requiredTags := toPlacementSet(c.PlacementTags)
	allTags := requiredTags.union(optionalTags)
	return requiredTags.isSubset(desiredTags) && desiredTags.isSubset(allTags)
}

type placementTagSet map[string]struct{}

func (set placementTagSet) union(other placementTagSet) placementTagSet {
	tags := placementTagSet{}
	for k := range set {
		tags[k] = struct{}{}
	}
	for k := range other {
		tags[k] = struct{}{}
	}
	return tags
}

func (set placementTagSet) isSubset(other placementTagSet) bool {
	for k := range set {
		if _, ok := other[k]; !ok {
			return false
		}
	}
	return true
}

func toPlacementSet(slice []string) placementTagSet {
	tags := placementTagSet{}
	for _, k := range slice {
		tags[k] = struct{}{}
	}
	return tags
}
