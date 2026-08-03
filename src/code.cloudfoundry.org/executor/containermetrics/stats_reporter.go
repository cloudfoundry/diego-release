package containermetrics

import (
	"strconv"
	"sync/atomic"
	"time"

	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/v3"
)

type CPUInfo struct {
	TimeSpentInCPU                      time.Duration
	TimeOfSample                        time.Time
	AbsoluteCPUEntitlementInNanoseconds uint64
}

type StatsReporter struct {
	CPUInfos              map[string]*CPUInfo
	metronClient          loggingclient.IngressClient
	enableContainerProxy  bool
	proxyMemoryAllocation float64
	metricsCache          *atomic.Value
	processor             MetricsProcessor
}

func NewStatsReporter(metronClient loggingclient.IngressClient, enableContainerProxy bool, proxyMemoryAllocation float64, metricsCache *atomic.Value) *StatsReporter {
	return NewStatsReporterWithProcessor(metronClient, enableContainerProxy, proxyMemoryAllocation, metricsCache, &DefaultMetricsProcessor{})
}

func NewStatsReporterWithProcessor(metronClient loggingclient.IngressClient, enableContainerProxy bool, proxyMemoryAllocation float64, metricsCache *atomic.Value, processor MetricsProcessor) *StatsReporter {
	return &StatsReporter{
		CPUInfos:              make(map[string]*CPUInfo),
		enableContainerProxy:  enableContainerProxy,
		metronClient:          metronClient,
		proxyMemoryAllocation: proxyMemoryAllocation,
		metricsCache:          metricsCache,
		processor:             processor,
	}
}

func (reporter *StatsReporter) Metrics() map[string]*CachedContainerMetrics {
	if v := reporter.metricsCache.Load(); v != nil {
		return v.(map[string]*CachedContainerMetrics)
	}
	return nil
}

func (reporter *StatsReporter) Report(logger lager.Logger, containers []executor.Container, metrics map[string]executor.Metrics, timeStamp time.Time) error {
	CPUInfos := map[string]*CPUInfo{}
	repMetricsMap := make(map[string]*CachedContainerMetrics)

	for _, container := range containers {

		guid := container.Guid
		metric, ok := metrics[guid]
		if !ok {
			continue
		}

		CPUInfos[guid] = reporter.CPUInfos[guid]

		previousCPUInfo := CPUInfos[guid]

		if reporter.enableContainerProxy && container.EnableContainerProxy {
			metric.MemoryUsageInBytes = uint64(float64(metric.MemoryUsageInBytes) * reporter.scaleMemory(container))
			metric.MemoryLimitInBytes = uint64(float64(metric.MemoryLimitInBytes) - reporter.proxyMemoryAllocation)
		}

		if len(metric.MetricsConfig.Tags) == 0 {
			metric.MetricsConfig.Tags = map[string]string{}
		}

		index := strconv.Itoa(metric.MetricsConfig.Index)
		if _, ok := metric.MetricsConfig.Tags["instance_id"]; !ok {
			metric.MetricsConfig.Tags["instance_id"] = index
		}

		cpu, repMetrics := reporter.processor.ProcessAndSend(logger, reporter.metronClient, metric.MetricsConfig, metric.ContainerMetrics, previousCPUInfo, timeStamp)
		CPUInfos[guid] = &cpu

		if repMetrics != nil {
			repMetricsMap[guid] = repMetrics
		}

	}

	reporter.CPUInfos = CPUInfos
	reporter.metricsCache.Store(repMetricsMap)
	return nil
}

func calculateInfo(containerMetrics executor.ContainerMetrics, previousInfo *CPUInfo, now time.Time) (CPUInfo, float64, float64) {
	TimeOfSample := now
	if containerMetrics.ContainerAgeInNanoseconds != 0 {
		TimeOfSample = time.Unix(0, int64(containerMetrics.ContainerAgeInNanoseconds))
	}

	currentInfo := CPUInfo{
		TimeSpentInCPU:                      containerMetrics.TimeSpentInCPU,
		TimeOfSample:                        TimeOfSample,
		AbsoluteCPUEntitlementInNanoseconds: containerMetrics.AbsoluteCPUEntitlementInNanoseconds,
	}

	var cpuPercent float64
	var cpuEntitlementPercent float64
	if previousInfo == nil {
		cpuPercent = 0.0
		cpuEntitlementPercent = 0.0
	} else {
		cpuPercent = computeCPUPercent(
			previousInfo.TimeSpentInCPU,
			currentInfo.TimeSpentInCPU,
			previousInfo.TimeOfSample,
			currentInfo.TimeOfSample,
		)
		cpuEntitlementPercent = computeCPUEntitlementPercent(
			previousInfo.TimeSpentInCPU,
			currentInfo.TimeSpentInCPU,
			previousInfo.AbsoluteCPUEntitlementInNanoseconds,
			currentInfo.AbsoluteCPUEntitlementInNanoseconds,
		)
	}

	return currentInfo, cpuPercent, cpuEntitlementPercent
}

func computeCPUEntitlementPercent(previousTimeSpentInCPU, currentTimeSpentInCPU time.Duration, previousAbsoluteEntitlementInNanoseconds, currentAbsoluteEntitlementInNanoseconds uint64) float64 {
	return (float64(currentTimeSpentInCPU-previousTimeSpentInCPU) / float64(currentAbsoluteEntitlementInNanoseconds-previousAbsoluteEntitlementInNanoseconds)) * 100.0
}

// scale from (0 - 100) * runtime.NumCPU()
func computeCPUPercent(timeSpentA, timeSpentB time.Duration, sampleTimeA, sampleTimeB time.Time) float64 {
	// divide change in time spent in CPU over time between samples.
	// result is out of 100 * runtime.NumCPU()
	//
	// don't worry about overflowing int64. it's like, 30 years.
	return float64((timeSpentB-timeSpentA)*100) / float64(sampleTimeB.UnixNano()-sampleTimeA.UnixNano())
}

func (reporter *StatsReporter) scaleMemory(container executor.Container) float64 {
	memFloat := float64(container.MemoryLimit)
	return (memFloat - reporter.proxyMemoryAllocation) / memFloat
}
