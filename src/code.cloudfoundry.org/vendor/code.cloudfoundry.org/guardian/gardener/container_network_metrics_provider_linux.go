package gardener

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
)

type SysFSContainerNetworkMetricsProvider struct {
	containerizer   Containerizer
	propertyManager PropertyManager
}

func NewSysFSContainerNetworkMetricsProvider(
	containerizer Containerizer,
	propertyManager PropertyManager,
) *SysFSContainerNetworkMetricsProvider {
	return &SysFSContainerNetworkMetricsProvider{
		containerizer:   containerizer,
		propertyManager: propertyManager,
	}
}

func (l *SysFSContainerNetworkMetricsProvider) Get(logger lager.Logger, handle string) (*garden.ContainerNetworkStat, error) {
	log := logger.Session("container-network-metrics")

	ifName, found := l.propertyManager.Get(handle, ContainerInterfaceKey)
	if !found || ifName == "" {
		return nil, nil
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	process, err := l.containerizer.Run(log, handle, garden.ProcessSpec{
		Path: "cat",
		Args: []string{
			networkStatPath(ifName, "rx_bytes"),
			networkStatPath(ifName, "tx_bytes"),
		},
	}, garden.ProcessIO{
		Stdout: stdout,
		Stderr: stderr,
	})

	if err != nil {
		return nil, fmt.Errorf("running process failed, %w", err)
	}

	exitStatus, err := process.Wait()
	if err != nil {
		return nil, err
	}

	if exitStatus != 0 {
		return nil, fmt.Errorf("running process failed with exit status %d, error %q", exitStatus, stderr.String())
	}

	stats := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(stats) != 2 {
		return nil, fmt.Errorf("expected two values but got %q", stdout.String())
	}

	for idx, s := range stats {
		stats[idx] = strings.TrimSpace(s)
	}

	rxBytes, err := strconv.ParseUint(stats[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("could not parse rx_bytes value %q, %w", stats[0], err)
	}

	txBytes, err := strconv.ParseUint(stats[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("could not parse tx_bytes value %q, %w", stats[1], err)
	}

	return &garden.ContainerNetworkStat{
		RxBytes: rxBytes,
		TxBytes: txBytes,
	}, nil
}

func networkStatPath(ifName, stat string) string {
	return filepath.Join("/sys/class/net", ifName, "statistics", stat)
}
