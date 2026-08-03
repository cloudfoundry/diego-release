package containermetrics

import (
	"time"

	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/v3"
)

type MetricsProcessor interface {
	ProcessAndSend(logger lager.Logger, metronClient loggingclient.IngressClient, metricsConfig executor.MetricsConfig, containerMetrics executor.ContainerMetrics, previousInfo *CPUInfo, now time.Time) (CPUInfo, *CachedContainerMetrics)
}

type DefaultMetricsProcessor struct{}

func (p *DefaultMetricsProcessor) ProcessAndSend(
	logger lager.Logger,
	metronClient loggingclient.IngressClient,
	metricsConfig executor.MetricsConfig,
	containerMetrics executor.ContainerMetrics,
	previousInfo *CPUInfo,
	now time.Time,
) (CPUInfo, *CachedContainerMetrics) {
	currentInfo, cpuPercent, cpuEntitlementPercent := calculateInfo(containerMetrics, previousInfo, now)

	if len(metricsConfig.Tags) == 0 {
		metricsConfig.Tags = map[string]string{}
	}

	applicationId := metricsConfig.Guid
	if sourceID, ok := metricsConfig.Tags["source_id"]; ok {
		applicationId = sourceID
	} else {
		metricsConfig.Tags["source_id"] = applicationId
	}

	if applicationId != "" {
		err := metronClient.SendAppMetrics(loggingclient.ContainerMetric{
			CpuPercentage:            cpuPercent,
			CpuEntitlementPercentage: cpuEntitlementPercent,
			MemoryBytes:              containerMetrics.MemoryUsageInBytes,
			DiskBytes:                containerMetrics.DiskUsageInBytes,
			MemoryBytesQuota:         containerMetrics.MemoryLimitInBytes,
			DiskBytesQuota:           containerMetrics.DiskLimitInBytes,
			AbsoluteCPUUsage:         uint64(containerMetrics.TimeSpentInCPU.Nanoseconds()),
			AbsoluteCPUEntitlement:   containerMetrics.AbsoluteCPUEntitlementInNanoseconds,
			ContainerAge:             containerMetrics.ContainerAgeInNanoseconds,
			Tags:                     metricsConfig.Tags,
			RxBytes:                  containerMetrics.RxInBytes,
			TxBytes:                  containerMetrics.TxInBytes,
		})
		if err != nil {
			logger.Error("failed-to-send-container-metrics", err, lager.Data{
				"metrics_guid":  applicationId,
				"metrics_index": metricsConfig.Index,
				"tags":          metricsConfig.Tags,
			})
		}

		if err := metronClient.SendAppLogRate(0, 0, metricsConfig.Tags); err != nil {
			logger.Error("failed-to-send-log-rate", err, lager.Data{
				"metrics_guid":  applicationId,
				"metrics_index": metricsConfig.Index,
				"tags":          metricsConfig.Tags,
			})
		}
	}

	return currentInfo, &CachedContainerMetrics{
		MetricGUID:       applicationId,
		CPUUsageFraction: cpuPercent / 100,
		DiskUsageBytes:   containerMetrics.DiskUsageInBytes,
		DiskQuotaBytes:   containerMetrics.DiskLimitInBytes,
		MemoryUsageBytes: containerMetrics.MemoryUsageInBytes,
		MemoryQuotaBytes: containerMetrics.MemoryLimitInBytes,
		RxBytes:          containerMetrics.RxInBytes,
		TxBytes:          containerMetrics.TxInBytes,
	}
}
