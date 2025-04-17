package goci

import (
	"fmt"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func (b Bndl) setCPUShares(shares specs.LinuxCPU) Bndl {
	resources := b.Resources()
	if resources == nil {
		resources = &specs.LinuxResources{}
	}

	if cgroups.IsCgroup2UnifiedMode() {
		if resources.Unified == nil {
			resources.Unified = make(map[string]string)
		}
		if shares.Quota != nil && shares.Period != nil {
			resources.Unified["cpu.max"] = fmt.Sprintf("%d %d", *shares.Quota, *shares.Period)
		}
		if shares.Shares != nil && *shares.Shares > 0 {
			resources.Unified["cpu.weight"] = fmt.Sprintf("%d", cgroups.ConvertCPUSharesToCgroupV2Value(*shares.Shares))
		}
	} else {
		resources.CPU = &shares
	}

	b.CloneLinux().Spec.Linux.Resources = resources

	return b
}

func (b Bndl) setMemoryLimit(limit specs.LinuxMemory) Bndl {
	resources := b.Resources()
	if resources == nil {
		resources = &specs.LinuxResources{}
	}

	resources.Memory = &limit

	b.CloneLinux().Spec.Linux.Resources = resources

	return b
}
