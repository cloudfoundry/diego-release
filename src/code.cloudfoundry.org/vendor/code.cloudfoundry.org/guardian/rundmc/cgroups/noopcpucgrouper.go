package cgroups

import "code.cloudfoundry.org/garden"

type NoopCPUCgrouper struct{}

func (NoopCPUCgrouper) CreateBadCgroup(string) error {
	return nil
}

func (NoopCPUCgrouper) DestroyBadCgroup(string) error {
	return nil
}

func (NoopCPUCgrouper) ReadBadCgroupUsage(string) (garden.ContainerCPUStat, error) {
	return garden.ContainerCPUStat{}, nil
}
