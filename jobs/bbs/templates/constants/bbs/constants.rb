# frozen_string_literal: true

module Constants
  # BBS Endpoints for Advanced Metrics
  ENDPOINTS = %w[
    Ping
    Domains
    UpsertDomain
    ActualLRPs
    ActualLRPGroups
    ActualLRPGroupsByProcessGuid
    ActualLRPGroupByProcessGuidAndIndex
    ClaimActualLRP
    StartActualLRP
    StartActualLRP_r0
    CrashActualLRP
    FailActualLRP
    RemoveActualLRP
    RetireActualLRP
    RemoveEvacuatingActualLRP
    EvacuateClaimedActualLRP
    EvacuateCrashedActualLRP
    EvacuateStoppedActualLRP
    EvacuateRunningActualLRP
    EvacuateRunningActualLRP_r0
    DesiredLRPs
    DesiredLRPSchedulingInfos
    DesiredLRPSchedulingInfoByProcessGuid
    DesiredLRPRoutingInfos
    DesiredLRPByProcessGuid
    DesiredLRPs_r2
    DesiredLRPByProcessGuid_r2
    DesireDesiredLRP
    UpdateDesiredLRP
    RemoveDesiredLRP
    Tasks
    TaskByGuid
    DesireTask
    StartTask
    CancelTask
    FailTask
    RejectTask
    CompleteTask
    ResolvingTask
    DeleteTask
    Tasks_r2
    TaskByGuid_r2
    Cells
  ].freeze
end
