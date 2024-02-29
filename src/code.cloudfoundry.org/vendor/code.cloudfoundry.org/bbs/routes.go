package bbs

import "github.com/tedsuo/rata"

const (
	// Ping
	PingRoute_r0 = "Ping"

	// Domains
	DomainsRoute_r0      = "Domains"
	UpsertDomainRoute_r0 = "UpsertDomain"

	// Actual LRPs
	ActualLRPsRoute_r0                          = "ActualLRPs"
	ActualLRPGroupsRoute_r0                     = "ActualLRPGroups"                      // DEPRECATED
	ActualLRPGroupsByProcessGuidRoute_r0        = "ActualLRPGroupsByProcessGuid"         // DEPRECATED
	ActualLRPGroupByProcessGuidAndIndexRoute_r0 = "ActualLRPGroupsByProcessGuidAndIndex" // DEPRECATED

	// Actual LRP Lifecycle
	ClaimActualLRPRoute_r0  = "ClaimActualLRP"
	StartActualLRPRoute_r1  = "StartActualLRP"
	StartActualLRPRoute_r0  = "StartActualLRP_r0" // DEPRECATED
	CrashActualLRPRoute_r0  = "CrashActualLRP"
	FailActualLRPRoute_r0   = "FailActualLRP"
	RemoveActualLRPRoute_r0 = "RemoveActualLRP"
	RetireActualLRPRoute_r0 = "RetireActualLRP"

	// Evacuation
	RemoveEvacuatingActualLRPRoute_r0 = "RemoveEvacuatingActualLRP"
	EvacuateClaimedActualLRPRoute_r0  = "EvacuateClaimedActualLRP"
	EvacuateCrashedActualLRPRoute_r0  = "EvacuateCrashedActualLRP"
	EvacuateStoppedActualLRPRoute_r0  = "EvacuateStoppedActualLRP"
	EvacuateRunningActualLRPRoute_r1  = "EvacuateRunningActualLRP"
	EvacuateRunningActualLRPRoute_r0  = "EvacuateRunningActualLRP_r0" // DEPRECATED

	// Desired LRPs
	DesiredLRPsRoute_r3               = "DesiredLRPs"
	DesiredLRPSchedulingInfosRoute_r0 = "DesiredLRPSchedulingInfos"
	DesiredLRPRoutingInfosRoute_r0    = "DesiredLRPRoutingInfos"
	DesiredLRPByProcessGuidRoute_r3   = "DesiredLRPByProcessGuid"
	DesiredLRPsRoute_r2               = "DesiredLRPs_r2"             // DEPRECATED
	DesiredLRPByProcessGuidRoute_r2   = "DesiredLRPByProcessGuid_r2" // DEPRECATED

	// Desire LRP Lifecycle
	DesireDesiredLRPRoute_r2 = "DesireDesiredLRP"
	UpdateDesiredLRPRoute_r0 = "UpdateDesireLRP"
	RemoveDesiredLRPRoute_r0 = "RemoveDesiredLRP"

	// Tasks
	TasksRoute_r3         = "Tasks"
	TaskByGuidRoute_r3    = "TaskByGuid"
	DesireTaskRoute_r2    = "DesireTask"
	StartTaskRoute_r0     = "StartTask"
	CancelTaskRoute_r0    = "CancelTask"
	FailTaskRoute_r0      = "FailTask" // DEPRECATED
	RejectTaskRoute_r0    = "RejectTask"
	CompleteTaskRoute_r0  = "CompleteTask"
	ResolvingTaskRoute_r0 = "ResolvingTask"
	DeleteTaskRoute_r0    = "DeleteTask"
	TasksRoute_r2         = "Tasks_r2"      // DEPRECATED
	TaskByGuidRoute_r2    = "TaskByGuid_r2" // DEPRECATED

	// Event Streaming
	LRPGroupEventStreamRoute_r1    = "EventStream" // DEPRECATED
	TaskEventStreamRoute_r1        = "TaskEventStream"
	LRPInstanceEventStreamRoute_r1 = "LRPInstanceEventStream"
	EventStreamRoute_r0            = "EventStream_r0"            // DEPRECATED
	TaskEventStreamRoute_r0        = "TaskEventStream_r0"        // DEPRECATED
	LrpInstanceEventStreamRoute_r0 = "LrpInstanceEventStream_r0" // DEPRECATED

	// Cell Presence
	CellsRoute_r0 = "Cells"
)

var Routes = rata.Routes{
	// Ping
	{Path: "/v1/ping", Method: "POST", Name: PingRoute_r0},

	// Domains
	{Path: "/v1/domains/list", Method: "POST", Name: DomainsRoute_r0},
	{Path: "/v1/domains/upsert", Method: "POST", Name: UpsertDomainRoute_r0},

	// Actual LRPs
	{Path: "/v1/actual_lrps/list", Method: "POST", Name: ActualLRPsRoute_r0},
	{Path: "/v1/actual_lrp_groups/list", Method: "POST", Name: ActualLRPGroupsRoute_r0},                                              // DEPRECATED
	{Path: "/v1/actual_lrp_groups/list_by_process_guid", Method: "POST", Name: ActualLRPGroupsByProcessGuidRoute_r0},                 // DEPRECATED
	{Path: "/v1/actual_lrp_groups/get_by_process_guid_and_index", Method: "POST", Name: ActualLRPGroupByProcessGuidAndIndexRoute_r0}, // DEPRECATED

	// Actual LRP Lifecycle
	{Path: "/v1/actual_lrps/claim", Method: "POST", Name: ClaimActualLRPRoute_r0},
	{Path: "/v1/actual_lrps/start.r1", Method: "POST", Name: StartActualLRPRoute_r1},
	{Path: "/v1/actual_lrps/start", Method: "POST", Name: StartActualLRPRoute_r0},
	{Path: "/v1/actual_lrps/crash", Method: "POST", Name: CrashActualLRPRoute_r0},
	{Path: "/v1/actual_lrps/fail", Method: "POST", Name: FailActualLRPRoute_r0},
	{Path: "/v1/actual_lrps/remove", Method: "POST", Name: RemoveActualLRPRoute_r0},
	{Path: "/v1/actual_lrps/retire", Method: "POST", Name: RetireActualLRPRoute_r0},

	// Evacuation
	{Path: "/v1/actual_lrps/remove_evacuating", Method: "POST", Name: RemoveEvacuatingActualLRPRoute_r0},
	{Path: "/v1/actual_lrps/evacuate_claimed", Method: "POST", Name: EvacuateClaimedActualLRPRoute_r0},
	{Path: "/v1/actual_lrps/evacuate_crashed", Method: "POST", Name: EvacuateCrashedActualLRPRoute_r0},
	{Path: "/v1/actual_lrps/evacuate_stopped", Method: "POST", Name: EvacuateStoppedActualLRPRoute_r0},
	{Path: "/v1/actual_lrps/evacuate_running.r1", Method: "POST", Name: EvacuateRunningActualLRPRoute_r1},
	{Path: "/v1/actual_lrps/evacuate_running", Method: "POST", Name: EvacuateRunningActualLRPRoute_r0},

	// Desired LRPs
	{Path: "/v1/desired_lrp_scheduling_infos/list", Method: "POST", Name: DesiredLRPSchedulingInfosRoute_r0},
	{Path: "/v1/desired_lrp_routing_infos/list", Method: "POST", Name: DesiredLRPRoutingInfosRoute_r0},

	{Path: "/v1/desired_lrps/list.r3", Method: "POST", Name: DesiredLRPsRoute_r3},
	{Path: "/v1/desired_lrps/get_by_process_guid.r3", Method: "POST", Name: DesiredLRPByProcessGuidRoute_r3},
	{Path: "/v1/desired_lrps/list.r2", Method: "POST", Name: DesiredLRPsRoute_r2},                            // DEPRECATED
	{Path: "/v1/desired_lrps/get_by_process_guid.r2", Method: "POST", Name: DesiredLRPByProcessGuidRoute_r2}, // DEPRECATED

	// Desire LPR Lifecycle
	{Path: "/v1/desired_lrp/desire.r2", Method: "POST", Name: DesireDesiredLRPRoute_r2},
	{Path: "/v1/desired_lrp/update", Method: "POST", Name: UpdateDesiredLRPRoute_r0},
	{Path: "/v1/desired_lrp/remove", Method: "POST", Name: RemoveDesiredLRPRoute_r0},

	// Tasks
	{Path: "/v1/tasks/list.r3", Method: "POST", Name: TasksRoute_r3},
	{Path: "/v1/tasks/get_by_task_guid.r3", Method: "POST", Name: TaskByGuidRoute_r3},
	{Path: "/v1/tasks/list.r2", Method: "POST", Name: TasksRoute_r2},                  // DEPRECATED
	{Path: "/v1/tasks/get_by_task_guid.r2", Method: "POST", Name: TaskByGuidRoute_r2}, // DEPRECATED

	// Task Lifecycle
	{Path: "/v1/tasks/desire.r2", Method: "POST", Name: DesireTaskRoute_r2},
	{Path: "/v1/tasks/start", Method: "POST", Name: StartTaskRoute_r0},
	{Path: "/v1/tasks/cancel", Method: "POST", Name: CancelTaskRoute_r0},
	{Path: "/v1/tasks/fail", Method: "POST", Name: FailTaskRoute_r0}, // DEPRECATED
	{Path: "/v1/tasks/reject", Method: "POST", Name: RejectTaskRoute_r0},
	{Path: "/v1/tasks/complete", Method: "POST", Name: CompleteTaskRoute_r0},
	{Path: "/v1/tasks/resolving", Method: "POST", Name: ResolvingTaskRoute_r0},
	{Path: "/v1/tasks/delete", Method: "POST", Name: DeleteTaskRoute_r0},

	// Event Streaming
	{Path: "/v1/events.r1", Method: "GET", Name: LRPGroupEventStreamRoute_r1}, // DEPRECATED
	{Path: "/v1/events/tasks.r1", Method: "POST", Name: TaskEventStreamRoute_r1},
	{Path: "/v1/events/lrp_instances.r1", Method: "POST", Name: LRPInstanceEventStreamRoute_r1},
	{Path: "/v1/events", Method: "GET", Name: EventStreamRoute_r0},                           // DEPRECATED
	{Path: "/v1/events/tasks", Method: "POST", Name: TaskEventStreamRoute_r0},                // DEPRECATED
	{Path: "/v1/events/lrp_instances", Method: "POST", Name: LrpInstanceEventStreamRoute_r0}, // DEPRECATED

	// Cells
	{Path: "/v1/cells/list.r1", Method: "POST", Name: CellsRoute_r0},
}
