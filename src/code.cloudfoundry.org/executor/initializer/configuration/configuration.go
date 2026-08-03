package configuration

import (
	"fmt"
	"net/url"
	"strconv"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/gardenhealth"
	"code.cloudfoundry.org/executor/guidgen"
	"code.cloudfoundry.org/garden"
	garden_client "code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/lager/v3"
)

const (
	Automatic = "auto"
)

var (
	ErrMemoryFlagInvalid       = fmt.Errorf("memory limit must be a positive number or '%s'", Automatic)
	ErrDiskFlagInvalid         = fmt.Errorf("disk limit must be a positive number or '%s'", Automatic)
	ErrAutoDiskCapacityInvalid = fmt.Errorf("auto disk limit must result in a positive number")
)

func ConfigureCapacity(
	gardenClient garden_client.Client,
	memoryMBFlag string,
	diskMBFlag string,
	maxCacheSizeInBytes uint64,
	autoDiskMBOverhead int,
	useSchedulableDiskSize bool,
) (executor.ExecutorResources, error) {
	gardenCapacity, err := gardenClient.Capacity()
	if err != nil {
		return executor.ExecutorResources{}, err
	}

	memory, err := memoryInMB(gardenCapacity, memoryMBFlag)
	if err != nil {
		return executor.ExecutorResources{}, err
	}

	disk, err := diskInMB(gardenCapacity, diskMBFlag, useSchedulableDiskSize, maxCacheSizeInBytes, autoDiskMBOverhead)
	if err != nil {
		return executor.ExecutorResources{}, err
	}

	return executor.ExecutorResources{
		MemoryMB:   memory,
		DiskMB:     disk,
		Containers: int(gardenCapacity.MaxContainers) - 1,
	}, nil
}

//go:generate counterfeiter -o configurationfakes/fake_rootfssizer.go . RootFSSizer
type RootFSSizer interface {
	RootFSSizeFromPath(path string) uint64
}

func GetRootFSSizes(
	logger lager.Logger,
	gardenClient garden_client.Client,
	guidGenerator guidgen.Generator,
	containerOwnerName string,
	rootFSes map[string]string,
) (RootFSSizer, error) {
	rootFSSizes := make(map[string]uint64)

	handles := []string{}
	guidToRootFSPath := make(map[string]string)
	for _, rootFSURI := range rootFSes {
		guid := fmt.Sprintf("rootfs-c-%s", guidGenerator.Guid(logger))
		_, err := gardenClient.Create(garden.ContainerSpec{
			Handle: guid,
			Image:  garden.ImageRef{URI: rootFSURI},
			Properties: garden.Properties{
				executor.ContainerOwnerProperty:         containerOwnerName,
				gardenhealth.HealthcheckNetworkProperty: "true",
			},
		})
		if err != nil {
			return nil, err
		}
		uri, err := url.Parse(rootFSURI)
		if err != nil {
			return nil, err
		}
		guidToRootFSPath[guid] = uri.Path
		handles = append(handles, guid)

		defer func(handle string) {
			destroyErr := gardenClient.Destroy(handle)
			if destroyErr != nil {
				logger.Error(fmt.Sprintf("failed to delete container '%s'", handle), destroyErr)
			}
		}(guid)
	}

	metrics, err := gardenClient.BulkMetrics(handles)
	if err != nil {
		return nil, err
	}

	for guid, metric := range metrics {
		rootFSSize := uint64(metric.Metrics.DiskStat.TotalBytesUsed - metric.Metrics.DiskStat.ExclusiveBytesUsed)
		rootFSSizes[guidToRootFSPath[guid]] = rootFSSize
	}

	return &rootFSSizeMap{
		rootFSSizes: rootFSSizes,
	}, nil
}

func memoryInMB(capacity garden.Capacity, memoryMBFlag string) (int, error) {
	if memoryMBFlag == Automatic {
		return int(capacity.MemoryInBytes / (1024 * 1024)), nil
	} else {
		memoryMB, err := strconv.Atoi(memoryMBFlag)
		if err != nil || memoryMB <= 0 {
			return 0, ErrMemoryFlagInvalid
		}
		return memoryMB, nil
	}
}

func diskInMB(capacity garden.Capacity, diskMBFlag string, useSchedulableDiskSize bool, maxCacheSizeInBytes uint64, autoDiskMBOverhead int) (int, error) {
	if diskMBFlag == Automatic {
		var diskMB int
		if useSchedulableDiskSize && capacity.SchedulableDiskInBytes > 0 {
			diskMB = int(capacity.SchedulableDiskInBytes/(1024*1024)) - autoDiskMBOverhead
		} else {
			diskMB = ((int(capacity.DiskInBytes) - int(maxCacheSizeInBytes)) / (1024 * 1024)) - autoDiskMBOverhead
		}
		if diskMB <= 0 {
			return 0, ErrAutoDiskCapacityInvalid
		}
		return diskMB, nil
	} else {
		diskMB, err := strconv.Atoi(diskMBFlag)
		if err != nil || diskMB <= 0 {
			return 0, ErrDiskFlagInvalid
		}
		return diskMB, nil
	}
}

type rootFSSizeMap struct {
	rootFSSizes map[string]uint64
}

func (r *rootFSSizeMap) RootFSSizeFromPath(path string) uint64 {
	url, err := url.Parse(path)
	if err != nil {
		return 0
	}
	return r.rootFSSizes[url.Path]
}
