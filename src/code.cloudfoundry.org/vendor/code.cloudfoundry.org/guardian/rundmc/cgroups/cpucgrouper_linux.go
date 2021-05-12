package cgroups

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fs"
)

type CPUCgrouper struct {
	cgroupRoot string
}

func NewCPUCgrouper(cgroupRoot string) CPUCgrouper {
	return CPUCgrouper{
		cgroupRoot: cgroupRoot,
	}
}

func (c CPUCgrouper) CreateBadCgroup(handle string) error {
	if err := os.MkdirAll(filepath.Join(c.cgroupRoot, BadCgroupName, handle), 0755); err != nil {
		return err
	}
	return nil
}

func (c CPUCgrouper) DestroyBadCgroup(handle string) error {
	if err := os.RemoveAll(filepath.Join(c.cgroupRoot, BadCgroupName, handle)); err != nil {
		return err
	}
	return nil
}

func (c CPUCgrouper) ReadBadCgroupUsage(handle string) (garden.ContainerCPUStat, error) {
	stats := cgroups.Stats{}
	cpuactCgroup := &fs.CpuacctGroup{}

	path := filepath.Join(c.cgroupRoot, BadCgroupName, handle)
	if _, err := os.Stat(path); err != nil {
		return garden.ContainerCPUStat{}, err
	}

	if err := cpuactCgroup.GetStats(path, &stats); err != nil {
		return garden.ContainerCPUStat{}, err
	}

	cpuStats := garden.ContainerCPUStat{
		Usage:  stats.CpuStats.CpuUsage.TotalUsage,
		System: stats.CpuStats.CpuUsage.UsageInKernelmode,
		User:   stats.CpuStats.CpuUsage.UsageInUsermode,
	}
	return cpuStats, nil
}
