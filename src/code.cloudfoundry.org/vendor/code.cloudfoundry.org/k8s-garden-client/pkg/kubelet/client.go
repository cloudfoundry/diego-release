package kubelet

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"time"

	"code.cloudfoundry.org/lager/v3"
	statsapi "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
)

//go:generate go tool counterfeiter -generate

const summaryPath = "/stats/summary"

type Metrics struct {
	ContainerAgeInNanoseconds uint64
	TimeSpentInCPU            time.Duration
	MemoryUsageInBytes        uint64
	DiskUsageInBytes          uint64
	RxInBytes                 *uint64
	TxInBytes                 *uint64
}

//counterfeiter:generate . Client
type Client interface {
	// GetMetrics returns the resource metrics for the given node.
	GetMetrics(logger lager.Logger, guids []string) (map[string]Metrics, error)
}

type kubeletClient struct {
	client *http.Client
	url    url.URL
}

func NewClient(client *http.Client, address, port string) Client {
	return &kubeletClient{
		client: client,
		url: url.URL{
			Scheme: "https",
			Host:   net.JoinHostPort(address, port),
			Path:   summaryPath,
		},
	}
}

// GetMetrics implements client.KubeletMetricsGetter
func (kc *kubeletClient) GetMetrics(logger lager.Logger, guids []string) (map[string]Metrics, error) {
	response, err := kc.client.Get(kc.url.String())
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			logger.Error("failed-to-close-response-body", err)
		}
	}()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed, status: %q", response.Status)
	}

	summary := &statsapi.Summary{}
	decoder := json.NewDecoder(response.Body)
	if err = decoder.Decode(summary); err != nil {
		logger.Error("failed-to-decode-summary", err)
		return nil, err
	}

	return kc.toContainerMetrics(logger, summary.Pods, guids), nil
}

func (kc *kubeletClient) toContainerMetrics(logger lager.Logger, podStats []statsapi.PodStats, guids []string) map[string]Metrics {
	containerMetrics := make(map[string]Metrics, len(podStats))

	for _, podStat := range podStats {
		if !slices.Contains(guids, podStat.PodRef.Name) {
			logger.Debug("skipping-pod-not-in-guid-list", lager.Data{"name": podStat.PodRef.Name, "namespace": podStat.PodRef.Namespace})
			continue
		}

		containerStat, found := getAppContainerStats(podStat)
		if !found {
			logger.Info("skipping-pod-no-app-container", lager.Data{"name": podStat.PodRef.Name, "namespace": podStat.PodRef.Namespace})
			continue
		}

		metricsPoint := Metrics{
			ContainerAgeInNanoseconds: uint64(time.Since(podStat.StartTime.Time).Nanoseconds()),
		}

		if containerStat.CPU != nil && containerStat.CPU.UsageCoreNanoSeconds != nil {
			metricsPoint.TimeSpentInCPU = time.Duration(*containerStat.CPU.UsageCoreNanoSeconds)
		} else {
			logger.Info("skipping-pod-no-cpu-usage", lager.Data{"name": podStat.PodRef.Name, "namespace": podStat.PodRef.Namespace})
		}

		if containerStat.Memory != nil && containerStat.Memory.WorkingSetBytes != nil {
			metricsPoint.MemoryUsageInBytes = *containerStat.Memory.WorkingSetBytes
		} else {
			logger.Info("skipping-pod-no-memory-usage", lager.Data{"name": podStat.PodRef.Name, "namespace": podStat.PodRef.Namespace})
		}

		if podStat.EphemeralStorage != nil && podStat.EphemeralStorage.UsedBytes != nil {
			metricsPoint.DiskUsageInBytes = *podStat.EphemeralStorage.UsedBytes
		} else {
			logger.Info("skipping-pod-no-ephemeral-storage-usage", lager.Data{"name": podStat.PodRef.Name, "namespace": podStat.PodRef.Namespace})
		}

		if podStat.Network != nil {
			if podStat.Network.RxBytes != nil {
				metricsPoint.RxInBytes = podStat.Network.RxBytes
			}

			if podStat.Network.TxBytes != nil {
				metricsPoint.TxInBytes = podStat.Network.TxBytes
			}
		}

		containerMetrics[podStat.PodRef.Name] = metricsPoint
	}

	return containerMetrics
}

func getAppContainerStats(podStat statsapi.PodStats) (statsapi.ContainerStats, bool) {
	for _, container := range podStat.Containers {
		if container.Name == "app" {
			return container, true
		}
	}
	return statsapi.ContainerStats{}, false
}
