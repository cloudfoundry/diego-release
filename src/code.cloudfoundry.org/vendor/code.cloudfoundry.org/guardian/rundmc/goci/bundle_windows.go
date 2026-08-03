package goci

import specs "github.com/opencontainers/runtime-spec/specs-go"

func (b Bndl) setCPUShares(shares specs.LinuxCPU) Bndl {
	resources := b.Resources()
	if resources == nil {
		resources = &specs.LinuxResources{}
	}

	resources.CPU = &shares

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
