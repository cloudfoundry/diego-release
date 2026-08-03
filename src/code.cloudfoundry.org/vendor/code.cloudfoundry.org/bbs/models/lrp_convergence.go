package models

type ActualLRPKeyWithSchedulingInfo struct {
	Key            *ActualLRPKey
	SchedulingInfo *DesiredLRPSchedulingInfo
}
