package cgroups

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/cgroups/fs"
	"github.com/opencontainers/cgroups/fs2"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type CPUCgrouper struct {
	cgroupRoot string
}

func NewCPUCgrouper(cgroupRoot string) CPUCgrouper {
	return CPUCgrouper{
		cgroupRoot: cgroupRoot,
	}
}

func (c CPUCgrouper) PrepareCgroups(handle string) error {
	badCgroupPath := filepath.Join(c.cgroupRoot, BadCgroupName, handle)
	if err := os.MkdirAll(badCgroupPath, 0755); err != nil {
		return err
	}
	if cgroups.IsCgroup2UnifiedMode() {
		if err := enableSupportedControllers(badCgroupPath); err != nil {
			return err
		}
	}
	return nil
}

func (c CPUCgrouper) CleanupCgroups(handle string) error {
	if err := os.RemoveAll(filepath.Join(c.cgroupRoot, BadCgroupName, handle)); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(c.cgroupRoot, GoodCgroupName, handle)); err != nil {
		return err
	}
	return nil
}

func (c CPUCgrouper) ReadTotalCgroupUsage(handle string, containerCPUStats garden.ContainerCPUStat) (garden.ContainerCPUStat, error) {
	badPath := filepath.Join(c.cgroupRoot, BadCgroupName, handle)
	badCPUStats, err := readCPUstatsFromPath(badPath)
	if err != nil {
		return garden.ContainerCPUStat{}, err
	}

	goodPath := filepath.Join(c.cgroupRoot, GoodCgroupName, handle)
	goodCPUStats, err := readCPUstatsFromPath(goodPath)
	if err != nil {
		return garden.ContainerCPUStat{}, err
	}

	cpuStats := garden.ContainerCPUStat{
		Usage:  badCPUStats.CpuStats.CpuUsage.TotalUsage + goodCPUStats.CpuStats.CpuUsage.TotalUsage,
		System: badCPUStats.CpuStats.CpuUsage.UsageInKernelmode + goodCPUStats.CpuStats.CpuUsage.UsageInKernelmode,
		User:   badCPUStats.CpuStats.CpuUsage.UsageInUsermode + goodCPUStats.CpuStats.CpuUsage.UsageInUsermode,
	}

	return cpuStats, nil
}

func readCPUstatsFromPath(path string) (cgroups.Stats, error) {
	stats := &cgroups.Stats{}

	if cgroups.IsCgroup2UnifiedMode() {
		cgroupManager, err := fs2.NewManager(&configs.Cgroup{}, path)
		if err != nil {
			return cgroups.Stats{}, err
		}
		stats, err = cgroupManager.GetStats()
		if err != nil {
			return cgroups.Stats{}, err
		}
	} else {
		cpuactCgroup := &fs.CpuacctGroup{}

		if _, err := os.Stat(path); err != nil {
			return cgroups.Stats{}, err
		}

		if err := cpuactCgroup.GetStats(path, stats); err != nil {
			return cgroups.Stats{}, err
		}
	}

	return *stats, nil
}
