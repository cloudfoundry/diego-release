package cgroups

import (
	"github.com/opencontainers/cgroups"
)

func IsCgroup2UnifiedMode() bool {
	return cgroups.IsCgroup2UnifiedMode()
}

func ConvertCPUSharesToCgroupV2Value(cpuShares uint64) uint64 {
	return cgroups.ConvertCPUSharesToCgroupV2Value(cpuShares)
}
