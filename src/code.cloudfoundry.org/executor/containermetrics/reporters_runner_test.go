package containermetrics_test

import (
	"errors"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	logging "code.cloudfoundry.org/diego-logging-client"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/containermetrics"
	efakes "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
)

func megsToBytes(mem int) uint64 {
	return uint64(mem * 1024 * 1024)
}

type cpuUsage struct {
	applicationID            string
	instanceIndex            int32
	absoluteUsage            uint64
	absoluteEntitlement      uint64
	containerAge             uint64
	cpuEntitlementPercentage float64
}

var _ = Describe("ReportersRunner", func() {
	var (
		logger *lagertest.TestLogger

		interval           time.Duration
		fakeClock          *fakeclock.FakeClock
		fakeExecutorClient *efakes.FakeClient
		fakeMetronClient   *mfakes.FakeIngressClient

		testStart time.Time
		process   ifrit.Process

		enableContainerProxy    bool
		proxyMemoryAllocationMB int
		reportersRunner         *containermetrics.ReportersRunner
		statsReporter           *containermetrics.StatsReporter

		metricsCache *atomic.Value
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger(fmt.Sprintf("test-%d", GinkgoParallelProcess()))

		interval = 10 * time.Second
		testStart = time.Now()
		fakeClock = fakeclock.NewFakeClock(testStart)
		fakeExecutorClient = new(efakes.FakeClient)
		fakeMetronClient = new(mfakes.FakeIngressClient)

		enableContainerProxy = false
		proxyMemoryAllocationMB = 5

		metricsCache = &atomic.Value{}
	})

	JustBeforeEach(func() {
		statsReporter = containermetrics.NewStatsReporter(fakeMetronClient, enableContainerProxy, float64(proxyMemoryAllocationMB*1024*1024), metricsCache)
		cpuSpikeReporter := containermetrics.NewCPUSpikeReporter(fakeMetronClient)
		reportersRunner = containermetrics.NewReportersRunner(logger, interval, fakeClock, fakeExecutorClient, statsReporter, cpuSpikeReporter)
		process = ifrit.Invoke(reportersRunner)
		fakeClock.WaitForWatcherAndIncrement(interval)
		Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(1))
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	one := uint64(1)
	two := uint64(2)
	three := uint64(3)

	sentCPUUsage := func() []cpuUsage {
		usage := []cpuUsage{}

		for i := 0; i < fakeMetronClient.SendAppMetricsCallCount(); i++ {
			metrics := fakeMetronClient.SendAppMetricsArgsForCall(i)
			index, _ := strconv.Atoi(metrics.Tags["instance_id"])
			usage = append(usage, cpuUsage{
				applicationID:            metrics.Tags["source_id"],
				instanceIndex:            int32(index),
				absoluteUsage:            metrics.AbsoluteCPUUsage,
				absoluteEntitlement:      metrics.AbsoluteCPUEntitlement,
				containerAge:             metrics.ContainerAge,
				cpuEntitlementPercentage: metrics.CpuEntitlementPercentage,
			})
		}
		return usage
	}

	sentMetrics := func() []logging.ContainerMetric {
		evs := []logging.ContainerMetric{}
		for i := 0; i < fakeMetronClient.SendAppMetricsCallCount(); i++ {
			arg := fakeMetronClient.SendAppMetricsArgsForCall(i)
			evs = append(evs, arg)
		}
		return evs
	}

	Context("when the interval elapses", func() {
		var metricsAtT0, metricsAtT10, metricsAtT20 map[string]executor.Metrics

		BeforeEach(func() {
			containers1 := []executor.Container{
				{Guid: "container-guid-without-index"},
				{Guid: "container-guid-with-index"},
				{Guid: "container-guid-without-preloaded-rootfs"},
				{Guid: "container-guid-without-source-id"},
				{Guid: "container-guid-without-age"},
				{Guid: "container-guid-without-metrics-at-t0"},
			}
			containers2 := []executor.Container{
				{Guid: "container-guid-without-index"},
				{Guid: "container-guid-with-index"},
				{Guid: "container-guid-without-preloaded-rootfs"},
				{Guid: "container-guid-without-source-id"},
				{Guid: "container-guid-without-age"},
				{Guid: "container-guid-without-metrics-at-t0"},
			}
			containers3 := []executor.Container{
				{Guid: "container-guid-without-index"},
				{Guid: "container-guid-with-index"},
				{Guid: "container-guid-without-preloaded-rootfs"},
				{Guid: "container-guid-without-source-id"},
				{Guid: "container-guid-without-age"},
				{Guid: "container-guid-without-metrics-at-t0"},
			}

			fakeExecutorClient.ListContainersReturnsOnCall(0, containers1, nil)
			fakeExecutorClient.ListContainersReturnsOnCall(1, containers2, nil)
			fakeExecutorClient.ListContainersReturnsOnCall(2, containers3, nil)

			metricsAtT0 = map[string]executor.Metrics{
				"container-guid-without-source-id": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(123),
						DiskUsageInBytes:                    megsToBytes(456),
						TimeSpentInCPU:                      0,
						MemoryLimitInBytes:                  megsToBytes(789),
						DiskLimitInBytes:                    megsToBytes(1024),
						ContainerAgeInNanoseconds:           1000,
						AbsoluteCPUEntitlementInNanoseconds: 0,
						RxInBytes:                           &one,
						TxInBytes:                           &one,
					},
				},
				"container-guid-without-index": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-without-index"}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(123),
						DiskUsageInBytes:                    megsToBytes(456),
						TimeSpentInCPU:                      0,
						MemoryLimitInBytes:                  megsToBytes(789),
						DiskLimitInBytes:                    megsToBytes(1024),
						ContainerAgeInNanoseconds:           1001,
						AbsoluteCPUEntitlementInNanoseconds: 0,
						RxInBytes:                           &one,
						TxInBytes:                           &one,
					},
				},
				"container-guid-with-index": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-with-index", "instance_id": "1"}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(321),
						DiskUsageInBytes:                    megsToBytes(654),
						TimeSpentInCPU:                      0,
						MemoryLimitInBytes:                  megsToBytes(987),
						DiskLimitInBytes:                    megsToBytes(2048),
						ContainerAgeInNanoseconds:           1002,
						AbsoluteCPUEntitlementInNanoseconds: 0,
						RxInBytes:                           &one,
						TxInBytes:                           &one,
					},
				},
				"container-guid-without-preloaded-rootfs": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-without-preloaded-rootfs"}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(345),
						DiskUsageInBytes:                    megsToBytes(456),
						TimeSpentInCPU:                      0,
						MemoryLimitInBytes:                  megsToBytes(678),
						DiskLimitInBytes:                    megsToBytes(2048),
						ContainerAgeInNanoseconds:           1003,
						AbsoluteCPUEntitlementInNanoseconds: 0,
						RxInBytes:                           &one,
						TxInBytes:                           &one,
					},
				},
				"container-guid-without-age": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-without-age"}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(123),
						DiskUsageInBytes:                    megsToBytes(456),
						TimeSpentInCPU:                      0,
						MemoryLimitInBytes:                  megsToBytes(789),
						DiskLimitInBytes:                    megsToBytes(1024),
						ContainerAgeInNanoseconds:           0,
						AbsoluteCPUEntitlementInNanoseconds: 0,
						RxInBytes:                           &one,
						TxInBytes:                           &one,
					},
				},
			}

			metricsAtT10 = map[string]executor.Metrics{
				"container-guid-without-source-id": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(123),
						DiskUsageInBytes:                    megsToBytes(456),
						TimeSpentInCPU:                      0.10 * 10 * time.Second,
						MemoryLimitInBytes:                  megsToBytes(789),
						DiskLimitInBytes:                    megsToBytes(1024),
						ContainerAgeInNanoseconds:           1000 + uint64(10*time.Second),
						AbsoluteCPUEntitlementInNanoseconds: uint64(5 * time.Second),
						RxInBytes:                           &two,
						TxInBytes:                           &two,
					},
				},

				"container-guid-without-index": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-without-index"}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(1230),
						DiskUsageInBytes:                    4560,
						TimeSpentInCPU:                      0.20 * 10 * time.Second,
						MemoryLimitInBytes:                  megsToBytes(7890),
						DiskLimitInBytes:                    4096,
						ContainerAgeInNanoseconds:           1001 + uint64(10*time.Second),
						AbsoluteCPUEntitlementInNanoseconds: uint64(5 * time.Second),
						RxInBytes:                           &two,
						TxInBytes:                           &two,
					},
				},
				"container-guid-with-index": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-with-index", "instance_id": "1"}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(3210),
						DiskUsageInBytes:                    6540,
						TimeSpentInCPU:                      0.30 * 10 * time.Second,
						MemoryLimitInBytes:                  megsToBytes(9870),
						DiskLimitInBytes:                    512,
						ContainerAgeInNanoseconds:           1002 + uint64(10*time.Second),
						AbsoluteCPUEntitlementInNanoseconds: uint64(5 * time.Second),
						RxInBytes:                           &two,
						TxInBytes:                           &two,
					}},
				"container-guid-without-preloaded-rootfs": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-without-preloaded-rootfs"}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(3450),
						DiskUsageInBytes:                    4560,
						TimeSpentInCPU:                      0.40 * 10 * time.Second,
						MemoryLimitInBytes:                  megsToBytes(6780),
						DiskLimitInBytes:                    2048,
						ContainerAgeInNanoseconds:           1003 + uint64(10*time.Second),
						AbsoluteCPUEntitlementInNanoseconds: uint64(5 * time.Second),
						RxInBytes:                           &two,
						TxInBytes:                           &two,
					},
				},
				"container-guid-without-age": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-without-age"}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(123),
						DiskUsageInBytes:                    megsToBytes(456),
						TimeSpentInCPU:                      0.50 * 10 * time.Second,
						MemoryLimitInBytes:                  megsToBytes(789),
						DiskLimitInBytes:                    megsToBytes(1024),
						ContainerAgeInNanoseconds:           0,
						AbsoluteCPUEntitlementInNanoseconds: uint64(5 * time.Second),
						RxInBytes:                           &two,
						TxInBytes:                           &two,
					},
				},
				"container-guid-without-metrics-at-t0": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-without-metrics-at-t0", "instance_id": "1"}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(3210),
						DiskUsageInBytes:                    6540,
						TimeSpentInCPU:                      0.60 * 10 * time.Second,
						MemoryLimitInBytes:                  megsToBytes(9870),
						DiskLimitInBytes:                    512,
						ContainerAgeInNanoseconds:           1002 + uint64(10*time.Second),
						AbsoluteCPUEntitlementInNanoseconds: uint64(5 * time.Second),
						RxInBytes:                           &two,
						TxInBytes:                           &two,
					},
				},
			}

			metricsAtT20 = map[string]executor.Metrics{
				"container-guid-without-source-id": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(123),
						DiskUsageInBytes:                    megsToBytes(456),
						TimeSpentInCPU:                      0.10 * 20 * time.Second,
						MemoryLimitInBytes:                  megsToBytes(789),
						DiskLimitInBytes:                    megsToBytes(1024),
						ContainerAgeInNanoseconds:           1000 + uint64(20*time.Second),
						AbsoluteCPUEntitlementInNanoseconds: uint64(10 * time.Second),
						RxInBytes:                           &three,
						TxInBytes:                           &three,
					},
				},

				"container-guid-without-index": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-without-index"}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(12300),
						DiskUsageInBytes:                    45600,
						TimeSpentInCPU:                      0.20 * 20 * time.Second,
						MemoryLimitInBytes:                  megsToBytes(7890),
						DiskLimitInBytes:                    234,
						ContainerAgeInNanoseconds:           1001 + uint64(20*time.Second),
						AbsoluteCPUEntitlementInNanoseconds: uint64(10 * time.Second),
						RxInBytes:                           &three,
						TxInBytes:                           &three,
					},
				},
				"container-guid-with-index": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-with-index", "instance_id": "1"}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(32100),
						DiskUsageInBytes:                    65400,
						TimeSpentInCPU:                      0.30 * 20 * time.Second,
						MemoryLimitInBytes:                  megsToBytes(98700),
						DiskLimitInBytes:                    43200,
						ContainerAgeInNanoseconds:           1002 + uint64(20*time.Second),
						AbsoluteCPUEntitlementInNanoseconds: uint64(10 * time.Second),
						RxInBytes:                           &three,
						TxInBytes:                           &three,
					},
				},
				"container-guid-without-preloaded-rootfs": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-without-preloaded-rootfs"}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(34500),
						DiskUsageInBytes:                    45600,
						TimeSpentInCPU:                      0.40 * 20 * time.Second,
						MemoryLimitInBytes:                  megsToBytes(6780),
						DiskLimitInBytes:                    2048,
						ContainerAgeInNanoseconds:           1003 + uint64(20*time.Second),
						AbsoluteCPUEntitlementInNanoseconds: uint64(10 * time.Second),
						RxInBytes:                           &three,
						TxInBytes:                           &three,
					},
				},
				"container-guid-without-age": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-without-age"}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(123),
						DiskUsageInBytes:                    megsToBytes(456),
						TimeSpentInCPU:                      0.50 * 20 * time.Second,
						MemoryLimitInBytes:                  megsToBytes(789),
						DiskLimitInBytes:                    megsToBytes(1024),
						ContainerAgeInNanoseconds:           0,
						AbsoluteCPUEntitlementInNanoseconds: uint64(10 * time.Second),
						RxInBytes:                           &three,
						TxInBytes:                           &three,
					},
				},
				"container-guid-without-metrics-at-t0": {
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-without-metrics-at-t0", "instance_id": "1"}},
					ContainerMetrics: executor.ContainerMetrics{
						MemoryUsageInBytes:                  megsToBytes(3210),
						DiskUsageInBytes:                    6540,
						TimeSpentInCPU:                      0.60 * 20 * time.Second,
						MemoryLimitInBytes:                  megsToBytes(9870),
						DiskLimitInBytes:                    512,
						ContainerAgeInNanoseconds:           1002 + uint64(20*time.Second),
						AbsoluteCPUEntitlementInNanoseconds: uint64(10 * time.Second),
						RxInBytes:                           &three,
						TxInBytes:                           &three,
					},
				},
			}

			fakeExecutorClient.GetBulkMetricsReturnsOnCall(0, metricsAtT0, nil)
			fakeExecutorClient.GetBulkMetricsReturnsOnCall(1, metricsAtT10, nil)
			fakeExecutorClient.GetBulkMetricsReturnsOnCall(2, metricsAtT20, nil)
		})

		It("emits memory and disk usage for each container, but no CPU", func() {
			Eventually(sentMetrics).Should(ConsistOf([]logging.ContainerMetric{
				{
					CpuPercentage:            0.0,
					CpuEntitlementPercentage: 0.0,
					MemoryBytes:              metricsAtT0["container-guid-without-index"].ContainerMetrics.MemoryUsageInBytes,
					DiskBytes:                metricsAtT0["container-guid-without-index"].ContainerMetrics.DiskUsageInBytes,
					MemoryBytesQuota:         metricsAtT0["container-guid-without-index"].ContainerMetrics.MemoryLimitInBytes,
					DiskBytesQuota:           metricsAtT0["container-guid-without-index"].ContainerMetrics.DiskLimitInBytes,
					AbsoluteCPUUsage:         uint64(metricsAtT0["container-guid-without-index"].ContainerMetrics.TimeSpentInCPU),
					AbsoluteCPUEntitlement:   metricsAtT0["container-guid-without-index"].ContainerMetrics.AbsoluteCPUEntitlementInNanoseconds,
					ContainerAge:             metricsAtT0["container-guid-without-index"].ContainerMetrics.ContainerAgeInNanoseconds,
					RxBytes:                  metricsAtT0["container-guid-without-index"].ContainerMetrics.RxInBytes,
					TxBytes:                  metricsAtT0["container-guid-without-index"].ContainerMetrics.TxInBytes,
					Tags: map[string]string{
						"source_id":   "source-id-without-index",
						"instance_id": "0",
					},
				},
				{
					CpuPercentage:            0.0,
					CpuEntitlementPercentage: 0.0,
					MemoryBytes:              metricsAtT0["container-guid-with-index"].ContainerMetrics.MemoryUsageInBytes,
					DiskBytes:                metricsAtT0["container-guid-with-index"].ContainerMetrics.DiskUsageInBytes,
					MemoryBytesQuota:         metricsAtT0["container-guid-with-index"].ContainerMetrics.MemoryLimitInBytes,
					DiskBytesQuota:           metricsAtT0["container-guid-with-index"].ContainerMetrics.DiskLimitInBytes,
					AbsoluteCPUUsage:         uint64(metricsAtT0["container-guid-with-index"].ContainerMetrics.TimeSpentInCPU),
					AbsoluteCPUEntitlement:   metricsAtT0["container-guid-with-index"].ContainerMetrics.AbsoluteCPUEntitlementInNanoseconds,
					ContainerAge:             metricsAtT0["container-guid-with-index"].ContainerMetrics.ContainerAgeInNanoseconds,
					RxBytes:                  metricsAtT0["container-guid-with-index"].ContainerMetrics.RxInBytes,
					TxBytes:                  metricsAtT0["container-guid-with-index"].ContainerMetrics.TxInBytes,
					Tags: map[string]string{
						"source_id":   "source-id-with-index",
						"instance_id": "1",
					},
				},
				{
					CpuPercentage:            0.0,
					CpuEntitlementPercentage: 0.0,
					MemoryBytes:              metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.MemoryUsageInBytes,
					DiskBytes:                metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.DiskUsageInBytes,
					MemoryBytesQuota:         metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.MemoryLimitInBytes,
					DiskBytesQuota:           metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.DiskLimitInBytes,
					AbsoluteCPUUsage:         uint64(metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.TimeSpentInCPU),
					AbsoluteCPUEntitlement:   metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.AbsoluteCPUEntitlementInNanoseconds,
					ContainerAge:             metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.ContainerAgeInNanoseconds,
					RxBytes:                  metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.RxInBytes,
					TxBytes:                  metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.TxInBytes,
					Tags: map[string]string{
						"source_id":   "source-id-without-preloaded-rootfs",
						"instance_id": "0",
					},
				},
				{
					CpuPercentage:            0.0,
					CpuEntitlementPercentage: 0.0,
					MemoryBytes:              metricsAtT0["container-guid-without-age"].ContainerMetrics.MemoryUsageInBytes,
					DiskBytes:                metricsAtT0["container-guid-without-age"].ContainerMetrics.DiskUsageInBytes,
					MemoryBytesQuota:         metricsAtT0["container-guid-without-age"].ContainerMetrics.MemoryLimitInBytes,
					DiskBytesQuota:           metricsAtT0["container-guid-without-age"].ContainerMetrics.DiskLimitInBytes,
					AbsoluteCPUUsage:         uint64(metricsAtT0["container-guid-without-age"].ContainerMetrics.TimeSpentInCPU),
					AbsoluteCPUEntitlement:   metricsAtT0["container-guid-without-age"].ContainerMetrics.AbsoluteCPUEntitlementInNanoseconds,
					ContainerAge:             metricsAtT0["container-guid-without-age"].ContainerMetrics.ContainerAgeInNanoseconds,
					RxBytes:                  metricsAtT0["container-guid-without-age"].ContainerMetrics.RxInBytes,
					TxBytes:                  metricsAtT0["container-guid-without-age"].ContainerMetrics.TxInBytes,
					Tags: map[string]string{
						"source_id":   "source-id-without-age",
						"instance_id": "0",
					},
				},
			}))
		})

		It("emits container cpu usage for each container", func() {
			Eventually(sentCPUUsage).Should(ConsistOf(
				cpuUsage{applicationID: "source-id-without-index", absoluteUsage: 0, absoluteEntitlement: 0, containerAge: 1001, cpuEntitlementPercentage: 0.0},
				cpuUsage{applicationID: "source-id-with-index", instanceIndex: 1, absoluteUsage: 0, absoluteEntitlement: 0, containerAge: 1002, cpuEntitlementPercentage: 0.0},
				cpuUsage{applicationID: "source-id-without-preloaded-rootfs", absoluteUsage: 0, absoluteEntitlement: 0, containerAge: 1003, cpuEntitlementPercentage: 0.0},
				cpuUsage{applicationID: "source-id-without-age", absoluteUsage: 0, absoluteEntitlement: 0, containerAge: 0, cpuEntitlementPercentage: 0.0},
			))
		})

		Context("when containers EnableContainerProxy is set", func() {
			BeforeEach(func() {
				containers := []executor.Container{
					{
						Guid:        "container-guid-without-index",
						MemoryLimit: megsToBytes(200 + proxyMemoryAllocationMB),
						RunInfo: executor.RunInfo{
							EnableContainerProxy: true,
							MetricsConfig:        executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-without-index"}},
						},
					},
					{
						Guid:        "container-guid-with-index",
						MemoryLimit: megsToBytes(400 + proxyMemoryAllocationMB),
						RunInfo: executor.RunInfo{
							EnableContainerProxy: true,
							MetricsConfig:        executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-with-index", "instance_id": "1"}},
						},
					},
					{
						Guid:        "container-guid-without-preloaded-rootfs",
						MemoryLimit: megsToBytes(200),
						RunInfo: executor.RunInfo{
							EnableContainerProxy: false,
							MetricsConfig:        executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-without-preloaded-rootfs"}},
						},
					},
				}
				fakeExecutorClient.ListContainersReturnsOnCall(0, containers, nil)
			})

			Context("when enableContainerProxy not set", func() {
				It("does not change the memory usage reported by garden", func() {
					appMemory := megsToBytes(123)
					expectedMemoryUsageWithoutIndex := float64(appMemory)
					Eventually(sentMetrics).Should(ContainElement(
						logging.ContainerMetric{
							CpuPercentage:          0.0,
							MemoryBytes:            uint64(expectedMemoryUsageWithoutIndex),
							DiskBytes:              metricsAtT0["container-guid-without-index"].ContainerMetrics.DiskUsageInBytes,
							MemoryBytesQuota:       metricsAtT0["container-guid-without-index"].ContainerMetrics.MemoryLimitInBytes,
							DiskBytesQuota:         metricsAtT0["container-guid-without-index"].ContainerMetrics.DiskLimitInBytes,
							AbsoluteCPUUsage:       uint64(metricsAtT0["container-guid-without-index"].ContainerMetrics.TimeSpentInCPU),
							AbsoluteCPUEntitlement: metricsAtT0["container-guid-without-index"].ContainerMetrics.AbsoluteCPUEntitlementInNanoseconds,
							ContainerAge:           metricsAtT0["container-guid-without-index"].ContainerMetrics.ContainerAgeInNanoseconds,
							RxBytes:                metricsAtT0["container-guid-without-index"].ContainerMetrics.RxInBytes,
							TxBytes:                metricsAtT0["container-guid-without-index"].ContainerMetrics.TxInBytes,
							Tags: map[string]string{
								"source_id":   "source-id-without-index",
								"instance_id": "0",
							},
						},
					))
				})
			})

			Context("when enableContainerProxy is set", func() {
				BeforeEach(func() {
					enableContainerProxy = true
				})

				It("emits a rescaled memory usage based on the additional memory allocation for the proxy", func() {
					appMemory := megsToBytes(123)
					expectedMemoryUsageWithoutIndex := float64(appMemory) * 200.0 / (200.0 + float64(proxyMemoryAllocationMB))
					expectedMemoryLimitWithoutIndex := float64(metricsAtT0["container-guid-without-index"].ContainerMetrics.MemoryLimitInBytes) - float64(megsToBytes(proxyMemoryAllocationMB))
					Eventually(sentMetrics).Should(ContainElement(
						logging.ContainerMetric{
							CpuPercentage:          0.0,
							MemoryBytes:            uint64(expectedMemoryUsageWithoutIndex),
							DiskBytes:              metricsAtT0["container-guid-without-index"].ContainerMetrics.DiskUsageInBytes,
							MemoryBytesQuota:       uint64(expectedMemoryLimitWithoutIndex),
							DiskBytesQuota:         metricsAtT0["container-guid-without-index"].ContainerMetrics.DiskLimitInBytes,
							AbsoluteCPUUsage:       uint64(metricsAtT0["container-guid-without-index"].ContainerMetrics.TimeSpentInCPU),
							AbsoluteCPUEntitlement: metricsAtT0["container-guid-without-index"].ContainerMetrics.AbsoluteCPUEntitlementInNanoseconds,
							ContainerAge:           metricsAtT0["container-guid-without-index"].ContainerMetrics.ContainerAgeInNanoseconds,
							RxBytes:                metricsAtT0["container-guid-without-index"].ContainerMetrics.RxInBytes,
							TxBytes:                metricsAtT0["container-guid-without-index"].ContainerMetrics.TxInBytes,
							Tags: map[string]string{
								"source_id":   "source-id-without-index",
								"instance_id": "0",
							},
						},
					))
				})

				Context("when there is a container without preloaded rootfs", func() {
					It("should emit memory usage that is not rescaled", func() {
						Eventually(sentMetrics).Should(ContainElement(
							logging.ContainerMetric{
								CpuPercentage:          0.0,
								MemoryBytes:            metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.MemoryUsageInBytes,
								DiskBytes:              metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.DiskUsageInBytes,
								MemoryBytesQuota:       metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.MemoryLimitInBytes,
								DiskBytesQuota:         metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.DiskLimitInBytes,
								AbsoluteCPUUsage:       uint64(metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.TimeSpentInCPU),
								AbsoluteCPUEntitlement: metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.AbsoluteCPUEntitlementInNanoseconds,
								ContainerAge:           metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.ContainerAgeInNanoseconds,
								RxBytes:                metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.RxInBytes,
								TxBytes:                metricsAtT0["container-guid-without-preloaded-rootfs"].ContainerMetrics.TxInBytes,
								Tags: map[string]string{
									"source_id":   "source-id-without-preloaded-rootfs",
									"instance_id": "0",
								},
							},
						))
					})
				})
			})
		})

		It("does not emit anything for containers with no metrics guid", func() {
			Consistently(sentMetrics).ShouldNot(ContainElement(WithTransform(func(m logging.ContainerMetric) string {
				return m.Tags["source_id"]
			}, BeEmpty())))
			Consistently(sentCPUUsage).ShouldNot(ContainElement(WithTransform(func(m cpuUsage) string {
				return m.applicationID
			}, BeEmpty())))
		})

		Context("and the interval elapses again", func() {
			JustBeforeEach(func() {
				fakeClock.WaitForWatcherAndIncrement(interval)
				Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(2))
			})

			It("emits the new memory and disk usage, and the computed CPU percent", func() {
				Eventually(sentMetrics).Should(ContainElement(logging.ContainerMetric{
					CpuPercentage:            20.0,
					CpuEntitlementPercentage: 40.0,
					MemoryBytes:              metricsAtT10["container-guid-without-index"].ContainerMetrics.MemoryUsageInBytes,
					DiskBytes:                metricsAtT10["container-guid-without-index"].ContainerMetrics.DiskUsageInBytes,
					MemoryBytesQuota:         metricsAtT10["container-guid-without-index"].ContainerMetrics.MemoryLimitInBytes,
					DiskBytesQuota:           metricsAtT10["container-guid-without-index"].ContainerMetrics.DiskLimitInBytes,
					AbsoluteCPUUsage:         uint64(metricsAtT10["container-guid-without-index"].ContainerMetrics.TimeSpentInCPU),
					AbsoluteCPUEntitlement:   metricsAtT10["container-guid-without-index"].ContainerMetrics.AbsoluteCPUEntitlementInNanoseconds,
					ContainerAge:             metricsAtT10["container-guid-without-index"].ContainerMetrics.ContainerAgeInNanoseconds,
					RxBytes:                  metricsAtT10["container-guid-without-index"].ContainerMetrics.RxInBytes,
					TxBytes:                  metricsAtT10["container-guid-without-index"].ContainerMetrics.TxInBytes,
					Tags: map[string]string{
						"source_id":   "source-id-without-index",
						"instance_id": "0",
					},
				}))

				Eventually(sentMetrics).Should(ContainElement(logging.ContainerMetric{
					CpuPercentage:            30.0,
					CpuEntitlementPercentage: 60.0,
					MemoryBytes:              metricsAtT10["container-guid-with-index"].ContainerMetrics.MemoryUsageInBytes,
					DiskBytes:                metricsAtT10["container-guid-with-index"].ContainerMetrics.DiskUsageInBytes,
					MemoryBytesQuota:         metricsAtT10["container-guid-with-index"].ContainerMetrics.MemoryLimitInBytes,
					DiskBytesQuota:           metricsAtT10["container-guid-with-index"].ContainerMetrics.DiskLimitInBytes,
					AbsoluteCPUUsage:         uint64(metricsAtT10["container-guid-with-index"].ContainerMetrics.TimeSpentInCPU),
					AbsoluteCPUEntitlement:   metricsAtT10["container-guid-with-index"].ContainerMetrics.AbsoluteCPUEntitlementInNanoseconds,
					ContainerAge:             metricsAtT10["container-guid-with-index"].ContainerMetrics.ContainerAgeInNanoseconds,
					RxBytes:                  metricsAtT10["container-guid-with-index"].ContainerMetrics.RxInBytes,
					TxBytes:                  metricsAtT10["container-guid-with-index"].ContainerMetrics.TxInBytes,
					Tags: map[string]string{
						"source_id":   "source-id-with-index",
						"instance_id": "1",
					},
				}))

				Eventually(sentMetrics).Should(ContainElement(logging.ContainerMetric{
					CpuPercentage:            40.0,
					CpuEntitlementPercentage: 80.0,
					MemoryBytes:              metricsAtT10["container-guid-without-preloaded-rootfs"].ContainerMetrics.MemoryUsageInBytes,
					DiskBytes:                metricsAtT10["container-guid-without-preloaded-rootfs"].ContainerMetrics.DiskUsageInBytes,
					MemoryBytesQuota:         metricsAtT10["container-guid-without-preloaded-rootfs"].ContainerMetrics.MemoryLimitInBytes,
					DiskBytesQuota:           metricsAtT10["container-guid-without-preloaded-rootfs"].ContainerMetrics.DiskLimitInBytes,
					AbsoluteCPUUsage:         uint64(metricsAtT10["container-guid-without-preloaded-rootfs"].ContainerMetrics.TimeSpentInCPU),
					AbsoluteCPUEntitlement:   metricsAtT10["container-guid-without-preloaded-rootfs"].ContainerMetrics.AbsoluteCPUEntitlementInNanoseconds,
					ContainerAge:             metricsAtT10["container-guid-without-preloaded-rootfs"].ContainerMetrics.ContainerAgeInNanoseconds,
					RxBytes:                  metricsAtT10["container-guid-without-preloaded-rootfs"].ContainerMetrics.RxInBytes,
					TxBytes:                  metricsAtT10["container-guid-without-preloaded-rootfs"].ContainerMetrics.TxInBytes,
					Tags: map[string]string{
						"source_id":   "source-id-without-preloaded-rootfs",
						"instance_id": "0",
					},
				}))

				Eventually(sentMetrics).Should(ContainElement(logging.ContainerMetric{
					CpuPercentage:            0.0,
					CpuEntitlementPercentage: 0.0,
					MemoryBytes:              metricsAtT10["container-guid-without-metrics-at-t0"].ContainerMetrics.MemoryUsageInBytes,
					DiskBytes:                metricsAtT10["container-guid-without-metrics-at-t0"].ContainerMetrics.DiskUsageInBytes,
					MemoryBytesQuota:         metricsAtT10["container-guid-without-metrics-at-t0"].ContainerMetrics.MemoryLimitInBytes,
					DiskBytesQuota:           metricsAtT10["container-guid-without-metrics-at-t0"].ContainerMetrics.DiskLimitInBytes,
					AbsoluteCPUUsage:         uint64(metricsAtT10["container-guid-without-metrics-at-t0"].ContainerMetrics.TimeSpentInCPU),
					AbsoluteCPUEntitlement:   metricsAtT10["container-guid-without-metrics-at-t0"].ContainerMetrics.AbsoluteCPUEntitlementInNanoseconds,
					ContainerAge:             metricsAtT10["container-guid-without-metrics-at-t0"].ContainerMetrics.ContainerAgeInNanoseconds,
					RxBytes:                  metricsAtT10["container-guid-without-metrics-at-t0"].ContainerMetrics.RxInBytes,
					TxBytes:                  metricsAtT10["container-guid-without-metrics-at-t0"].ContainerMetrics.TxInBytes,
					Tags: map[string]string{
						"source_id":   "source-id-without-metrics-at-t0",
						"instance_id": "1",
					},
				}))
			})

			Context("Metrics", func() {
				It("returns the cached metrics last emitted", func() {
					containerMetrics := statsReporter.Metrics()
					Expect(containerMetrics).To(HaveLen(6))
					Expect(containerMetrics).To(HaveKeyWithValue("container-guid-without-index", &containermetrics.CachedContainerMetrics{
						MetricGUID:       "source-id-without-index",
						CPUUsageFraction: 0.2,
						MemoryUsageBytes: megsToBytes(1230),
						DiskUsageBytes:   4560,
						MemoryQuotaBytes: megsToBytes(7890),
						DiskQuotaBytes:   4096,
						RxBytes:          &two,
						TxBytes:          &two,
					}))
					Expect(containerMetrics).To(HaveKeyWithValue("container-guid-with-index", &containermetrics.CachedContainerMetrics{
						MetricGUID:       "source-id-with-index",
						CPUUsageFraction: 0.3,
						MemoryUsageBytes: megsToBytes(3210),
						DiskUsageBytes:   6540,
						MemoryQuotaBytes: megsToBytes(9870),
						DiskQuotaBytes:   512,
						RxBytes:          &two,
						TxBytes:          &two,
					}))
					Expect(containerMetrics).To(HaveKeyWithValue("container-guid-without-source-id", &containermetrics.CachedContainerMetrics{
						MetricGUID:       "",
						CPUUsageFraction: 0.1,
						MemoryUsageBytes: megsToBytes(123),
						DiskUsageBytes:   megsToBytes(456),
						MemoryQuotaBytes: megsToBytes(789),
						DiskQuotaBytes:   megsToBytes(1024),
						RxBytes:          &two,
						TxBytes:          &two,
					}))
					Expect(containerMetrics).To(HaveKeyWithValue("container-guid-without-preloaded-rootfs", &containermetrics.CachedContainerMetrics{
						MetricGUID:       "source-id-without-preloaded-rootfs",
						CPUUsageFraction: 0.4,
						MemoryUsageBytes: megsToBytes(3450),
						DiskUsageBytes:   4560,
						MemoryQuotaBytes: megsToBytes(6780),
						DiskQuotaBytes:   2048,
						RxBytes:          &two,
						TxBytes:          &two,
					}))
					Expect(containerMetrics).To(HaveKeyWithValue("container-guid-without-age", &containermetrics.CachedContainerMetrics{
						MetricGUID:       "source-id-without-age",
						CPUUsageFraction: 0.5,
						MemoryUsageBytes: megsToBytes(123),
						DiskUsageBytes:   megsToBytes(456),
						MemoryQuotaBytes: megsToBytes(789),
						DiskQuotaBytes:   megsToBytes(1024),
						RxBytes:          &two,
						TxBytes:          &two,
					}))
					Expect(containerMetrics).To(HaveKeyWithValue("container-guid-without-metrics-at-t0", &containermetrics.CachedContainerMetrics{
						MetricGUID:       "source-id-without-metrics-at-t0",
						CPUUsageFraction: 0.0,
						MemoryUsageBytes: megsToBytes(3210),
						DiskUsageBytes:   6540,
						MemoryQuotaBytes: megsToBytes(9870),
						DiskQuotaBytes:   512,
						RxBytes:          &two,
						TxBytes:          &two,
					}))
				})
			})

			Context("and the interval elapses again", func() {
				JustBeforeEach(func() {
					fakeClock.WaitForWatcherAndIncrement(interval)
					Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(3))
				})

				It("emits the new memory and disk usage, and the computed CPU percent", func() {
					Eventually(sentMetrics).Should(ContainElement(logging.ContainerMetric{
						CpuEntitlementPercentage: 40.0,
						CpuPercentage:            20.0,
						MemoryBytes:              metricsAtT20["container-guid-without-index"].ContainerMetrics.MemoryUsageInBytes,
						DiskBytes:                metricsAtT20["container-guid-without-index"].ContainerMetrics.DiskUsageInBytes,
						MemoryBytesQuota:         metricsAtT20["container-guid-without-index"].ContainerMetrics.MemoryLimitInBytes,
						DiskBytesQuota:           metricsAtT20["container-guid-without-index"].ContainerMetrics.DiskLimitInBytes,
						AbsoluteCPUUsage:         uint64(metricsAtT20["container-guid-without-index"].ContainerMetrics.TimeSpentInCPU),
						AbsoluteCPUEntitlement:   metricsAtT20["container-guid-without-index"].ContainerMetrics.AbsoluteCPUEntitlementInNanoseconds,
						ContainerAge:             metricsAtT20["container-guid-without-index"].ContainerMetrics.ContainerAgeInNanoseconds,
						RxBytes:                  metricsAtT20["container-guid-without-index"].ContainerMetrics.RxInBytes,
						TxBytes:                  metricsAtT20["container-guid-without-index"].ContainerMetrics.TxInBytes,
						Tags: map[string]string{
							"source_id":   "source-id-without-index",
							"instance_id": "0",
						},
					}))

					Eventually(sentMetrics).Should(ContainElement(logging.ContainerMetric{
						CpuEntitlementPercentage: 60.0,
						CpuPercentage:            30.0,
						MemoryBytes:              metricsAtT20["container-guid-with-index"].ContainerMetrics.MemoryUsageInBytes,
						DiskBytes:                metricsAtT20["container-guid-with-index"].ContainerMetrics.DiskUsageInBytes,
						MemoryBytesQuota:         metricsAtT20["container-guid-with-index"].ContainerMetrics.MemoryLimitInBytes,
						DiskBytesQuota:           metricsAtT20["container-guid-with-index"].ContainerMetrics.DiskLimitInBytes,
						AbsoluteCPUUsage:         uint64(metricsAtT20["container-guid-with-index"].ContainerMetrics.TimeSpentInCPU),
						AbsoluteCPUEntitlement:   metricsAtT20["container-guid-with-index"].ContainerMetrics.AbsoluteCPUEntitlementInNanoseconds,
						ContainerAge:             metricsAtT20["container-guid-with-index"].ContainerMetrics.ContainerAgeInNanoseconds,
						RxBytes:                  metricsAtT20["container-guid-with-index"].ContainerMetrics.RxInBytes,
						TxBytes:                  metricsAtT20["container-guid-with-index"].ContainerMetrics.TxInBytes,
						Tags: map[string]string{
							"source_id":   "source-id-with-index",
							"instance_id": "1",
						},
					}))

					Eventually(sentMetrics).Should(ContainElement(logging.ContainerMetric{
						CpuEntitlementPercentage: 80.0,
						CpuPercentage:            40.0,
						MemoryBytes:              metricsAtT20["container-guid-without-preloaded-rootfs"].ContainerMetrics.MemoryUsageInBytes,
						DiskBytes:                metricsAtT20["container-guid-without-preloaded-rootfs"].ContainerMetrics.DiskUsageInBytes,
						MemoryBytesQuota:         metricsAtT20["container-guid-without-preloaded-rootfs"].ContainerMetrics.MemoryLimitInBytes,
						DiskBytesQuota:           metricsAtT20["container-guid-without-preloaded-rootfs"].ContainerMetrics.DiskLimitInBytes,
						AbsoluteCPUUsage:         uint64(metricsAtT20["container-guid-without-preloaded-rootfs"].ContainerMetrics.TimeSpentInCPU),
						AbsoluteCPUEntitlement:   metricsAtT20["container-guid-without-preloaded-rootfs"].ContainerMetrics.AbsoluteCPUEntitlementInNanoseconds,
						ContainerAge:             metricsAtT20["container-guid-without-preloaded-rootfs"].ContainerMetrics.ContainerAgeInNanoseconds,
						RxBytes:                  metricsAtT20["container-guid-without-preloaded-rootfs"].ContainerMetrics.RxInBytes,
						TxBytes:                  metricsAtT20["container-guid-without-preloaded-rootfs"].ContainerMetrics.TxInBytes,
						Tags: map[string]string{
							"source_id":   "source-id-without-preloaded-rootfs",
							"instance_id": "0",
						},
					}))

					Eventually(sentMetrics).Should(ContainElement(logging.ContainerMetric{
						CpuEntitlementPercentage: 120.0,
						CpuPercentage:            60.0,
						MemoryBytes:              metricsAtT20["container-guid-without-metrics-at-t0"].ContainerMetrics.MemoryUsageInBytes,
						DiskBytes:                metricsAtT20["container-guid-without-metrics-at-t0"].ContainerMetrics.DiskUsageInBytes,
						MemoryBytesQuota:         metricsAtT20["container-guid-without-metrics-at-t0"].ContainerMetrics.MemoryLimitInBytes,
						DiskBytesQuota:           metricsAtT20["container-guid-without-metrics-at-t0"].ContainerMetrics.DiskLimitInBytes,
						AbsoluteCPUUsage:         uint64(metricsAtT20["container-guid-without-metrics-at-t0"].ContainerMetrics.TimeSpentInCPU),
						AbsoluteCPUEntitlement:   metricsAtT20["container-guid-without-metrics-at-t0"].ContainerMetrics.AbsoluteCPUEntitlementInNanoseconds,
						ContainerAge:             metricsAtT20["container-guid-without-metrics-at-t0"].ContainerMetrics.ContainerAgeInNanoseconds,
						RxBytes:                  metricsAtT20["container-guid-without-metrics-at-t0"].ContainerMetrics.RxInBytes,
						TxBytes:                  metricsAtT20["container-guid-without-metrics-at-t0"].ContainerMetrics.TxInBytes,
						Tags: map[string]string{
							"source_id":   "source-id-without-metrics-at-t0",
							"instance_id": "1",
						},
					}))
				})
			})
		})
	})

	Context("CPU Spikes", func() {
		var metricsAtT0, metricsAtT10, metricsAtT20 map[string]executor.Metrics
		createMetrics := func(cpuTime time.Duration, entitlement uint64) executor.Metrics {
			return executor.Metrics{
				MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "app", "instance_id": "0"}},
				ContainerMetrics: executor.ContainerMetrics{
					TimeSpentInCPU:                      cpuTime,
					AbsoluteCPUEntitlementInNanoseconds: entitlement,
				},
			}
		}

		BeforeEach(func() {
			fakeExecutorClient.ListContainersReturns([]executor.Container{{Guid: "container-guid"}}, nil)
			metricsAtT0 = make(map[string]executor.Metrics)
			metricsAtT10 = make(map[string]executor.Metrics)
			metricsAtT20 = make(map[string]executor.Metrics)
			fakeExecutorClient.GetBulkMetricsReturnsOnCall(0, metricsAtT0, nil)
			fakeExecutorClient.GetBulkMetricsReturnsOnCall(1, metricsAtT10, nil)
			fakeExecutorClient.GetBulkMetricsReturnsOnCall(2, metricsAtT20, nil)
		})

		JustBeforeEach(func() {
			fakeClock.WaitForWatcherAndIncrement(interval)
			fakeClock.WaitForWatcherAndIncrement(interval)
		})

		Context("when there are multiple containers", func() {
			BeforeEach(func() {
				fakeExecutorClient.ListContainersReturns([]executor.Container{{Guid: "container-guid-1"}, {Guid: "container-guid-2"}}, nil)
				metricsAtT0["container-guid-1"] = executor.Metrics{
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "app", "instance_id": "0"}},
					ContainerMetrics: executor.ContainerMetrics{
						TimeSpentInCPU:                      1300 * time.Nanosecond,
						AbsoluteCPUEntitlementInNanoseconds: 1000,
					},
				}

				metricsAtT0["container-guid-2"] = executor.Metrics{
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "app", "instance_id": "1"}},
					ContainerMetrics: executor.ContainerMetrics{
						TimeSpentInCPU:                      2300 * time.Nanosecond,
						AbsoluteCPUEntitlementInNanoseconds: 2000,
					},
				}
			})

			It("sends a metric per container", func() {
				Eventually(fakeMetronClient.SendSpikeMetricsCallCount).Should(Equal(2))

				spikeMetric := fakeMetronClient.SendSpikeMetricsArgsForCall(0)
				Expect(spikeMetric.Tags).To(HaveKeyWithValue("source_id", "app"))
				Expect(spikeMetric.Tags).To(HaveKeyWithValue("instance_id", "0"))

				spikeMetric = fakeMetronClient.SendSpikeMetricsArgsForCall(1)
				Expect(spikeMetric.Tags).To(HaveKeyWithValue("source_id", "app"))
				Expect(spikeMetric.Tags).To(HaveKeyWithValue("instance_id", "1"))
			})

			Context("when sending the spike metric for a container fails", func() {
				BeforeEach(func() {
					fakeMetronClient.SendSpikeMetricsReturnsOnCall(0, errors.New("send-error-1"))
				})

				It("reports the error", func() {
					Eventually(func() []lager.LogFormat {
						return logger.Logs()
					}, 5*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Data": HaveKeyWithValue("error", "send-error-1")})))
				})

				It("does not send metrics for the other containers", func() {
					Expect(fakeMetronClient.SendSpikeMetricsCallCount()).To(Equal(1))
				})
			})
		})

		Context("when the container has never spiked", func() {
			BeforeEach(func() {
				metricsAtT0["container-guid"] = createMetrics(300*time.Nanosecond, 1000)
				metricsAtT10["container-guid"] = createMetrics(600*time.Nanosecond, 2000)
				metricsAtT20["container-guid"] = createMetrics(1200*time.Nanosecond, 4000)
			})

			It("never sends a spike metric", func() {
				Consistently(fakeMetronClient.SendSpikeMetricsCallCount).Should(Equal(0))
			})
		})

		Context("when the container is starting a spike", func() {
			BeforeEach(func() {
				metricsAtT0["container-guid"] = createMetrics(300*time.Nanosecond, 1000)
				metricsAtT10["container-guid"] = createMetrics(600*time.Nanosecond, 2000)
				metricsAtT20["container-guid"] = createMetrics(5000*time.Nanosecond, 4000)
			})

			It("sends a spike metric with start and zero end", func() {
				Eventually(fakeMetronClient.SendSpikeMetricsCallCount).Should(Equal(1))
				spikeMetric := fakeMetronClient.SendSpikeMetricsArgsForCall(0)

				Expect(spikeMetric.Tags).To(HaveKeyWithValue("source_id", "app"))
				Expect(spikeMetric.Tags).To(HaveKeyWithValue("instance_id", "0"))
				Expect(spikeMetric.Start).To(Equal(testStart.Add(30 * time.Second)))
				Expect(spikeMetric.End).To(Equal(time.Time{}))
			})
		})

		Context("when the container is finishing a spike", func() {
			BeforeEach(func() {
				metricsAtT0["container-guid"] = createMetrics(700*time.Nanosecond, 1000)
				metricsAtT10["container-guid"] = createMetrics(2100*time.Nanosecond, 2000)
				metricsAtT20["container-guid"] = createMetrics(3000*time.Nanosecond, 4000)
			})

			It("sends a spike metric with start as spike start and end as spike end", func() {
				Eventually(fakeMetronClient.SendSpikeMetricsCallCount).Should(Equal(2))
				spikeMetric := fakeMetronClient.SendSpikeMetricsArgsForCall(1)

				Expect(spikeMetric.Start).To(Equal(testStart.Add(20 * time.Second)))
				Expect(spikeMetric.End).To(Equal(testStart.Add(30 * time.Second)))
			})
		})

		Context("while the container is spiking", func() {
			BeforeEach(func() {
				metricsAtT0["container-guid"] = createMetrics(1100*time.Nanosecond, 1000)
				metricsAtT10["container-guid"] = createMetrics(2200*time.Nanosecond, 2000)
				metricsAtT20["container-guid"] = createMetrics(4400*time.Nanosecond, 4000)
			})

			It("sends spike metric on every iteration", func() {
				Eventually(fakeMetronClient.SendSpikeMetricsCallCount).Should(Equal(3))

				spikeMetric := fakeMetronClient.SendSpikeMetricsArgsForCall(0)
				Expect(spikeMetric.Start).To(Equal(testStart.Add(10 * time.Second)))
				Expect(spikeMetric.End).To(Equal(time.Time{}))

				spikeMetric = fakeMetronClient.SendSpikeMetricsArgsForCall(1)
				Expect(spikeMetric.Start).To(Equal(testStart.Add(10 * time.Second)))
				Expect(spikeMetric.End).To(Equal(time.Time{}))

				spikeMetric = fakeMetronClient.SendSpikeMetricsArgsForCall(2)
				Expect(spikeMetric.Start).To(Equal(testStart.Add(10 * time.Second)))
				Expect(spikeMetric.End).To(Equal(time.Time{}))
			})
		})

		Context("when a non-spiking container has a previous spike", func() {
			BeforeEach(func() {
				metricsAtT0["container-guid"] = createMetrics(1100*time.Nanosecond, 1000)
				metricsAtT10["container-guid"] = createMetrics(1800*time.Nanosecond, 2000)
				metricsAtT20["container-guid"] = createMetrics(2000*time.Nanosecond, 4000)
			})

			It("sends spike metric with start and end", func() {
				fakeClock.WaitForWatcherAndIncrement(interval)
				fakeClock.WaitForWatcherAndIncrement(interval)

				Eventually(fakeMetronClient.SendSpikeMetricsCallCount).Should(Equal(3))
				spikeMetric := fakeMetronClient.SendSpikeMetricsArgsForCall(2)
				Expect(spikeMetric.Start).To(Equal(testStart.Add(10 * time.Second)))
				Expect(spikeMetric.End).To(Equal(testStart.Add(20 * time.Second)))
			})
		})

		Context("when the container is starting a subsequent spike", func() {
			BeforeEach(func() {
				metricsAtT0["container-guid"] = createMetrics(1100*time.Nanosecond, 1000)
				metricsAtT10["container-guid"] = createMetrics(1800*time.Nanosecond, 2000)
				metricsAtT20["container-guid"] = createMetrics(4200*time.Nanosecond, 4000)
			})

			It("sends a spike metric with start and zero end", func() {
				fakeClock.WaitForWatcherAndIncrement(interval)
				fakeClock.WaitForWatcherAndIncrement(interval)
				Eventually(fakeMetronClient.SendSpikeMetricsCallCount).Should(Equal(3))
				spikeMetric := fakeMetronClient.SendSpikeMetricsArgsForCall(2)

				Expect(spikeMetric.Start).To(Equal(testStart.Add(30 * time.Second)))
				Expect(spikeMetric.End).To(Equal(time.Time{}))
			})
		})
	})

	Context("Reporters reset the container list", func() {
		var metricsAtT0, metricsAtT10, metricsAtT20 map[string]executor.Metrics

		BeforeEach(func() {
			containers1 := []executor.Container{
				{Guid: "container-guid"},
			}
			containers2 := []executor.Container{}
			containers3 := []executor.Container{
				{Guid: "container-guid"},
			}

			fakeExecutorClient.ListContainersReturnsOnCall(0, containers1, nil)
			fakeExecutorClient.ListContainersReturnsOnCall(1, containers2, nil)
			fakeExecutorClient.ListContainersReturnsOnCall(2, containers3, nil)

			metricsAtT0 = map[string]executor.Metrics{
				"container-guid": executor.Metrics{
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-with-index", "instance_id": "1"}},
					ContainerMetrics: executor.ContainerMetrics{
						TimeSpentInCPU:                      2000 * time.Nanosecond,
						ContainerAgeInNanoseconds:           1000,
						AbsoluteCPUEntitlementInNanoseconds: 1000,
					},
				},
			}

			metricsAtT10 = map[string]executor.Metrics{}

			metricsAtT20 = map[string]executor.Metrics{
				"container-guid": executor.Metrics{
					MetricsConfig: executor.MetricsConfig{Tags: map[string]string{"source_id": "source-id-with-index", "instance_id": "1"}},
					ContainerMetrics: executor.ContainerMetrics{
						TimeSpentInCPU:                      2600 * time.Nanosecond,
						ContainerAgeInNanoseconds:           2000,
						AbsoluteCPUEntitlementInNanoseconds: 3000,
					},
				},
			}

			fakeExecutorClient.GetBulkMetricsReturnsOnCall(0, metricsAtT0, nil)
			fakeExecutorClient.GetBulkMetricsReturnsOnCall(1, metricsAtT10, nil)
			fakeExecutorClient.GetBulkMetricsReturnsOnCall(2, metricsAtT20, nil)
		})

		It("does not send a spike metric when the container is gone", func() {
			fakeClock.WaitForWatcherAndIncrement(interval)
			fakeClock.WaitForWatcherAndIncrement(interval)
			Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(3))
			Consistently(fakeMetronClient.SendSpikeMetricsCallCount).Should(Equal(1))
		})

		It("should report 0 CPU", func() {
			fakeClock.WaitForWatcherAndIncrement(interval)
			fakeClock.WaitForWatcherAndIncrement(interval)
			Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(3))
			Eventually(sentMetrics).Should(ContainElement(logging.ContainerMetric{
				CpuPercentage:          0.0,
				AbsoluteCPUUsage:       uint64(metricsAtT20["container-guid"].ContainerMetrics.TimeSpentInCPU),
				AbsoluteCPUEntitlement: metricsAtT20["container-guid"].ContainerMetrics.AbsoluteCPUEntitlementInNanoseconds,
				ContainerAge:           metricsAtT20["container-guid"].ContainerMetrics.ContainerAgeInNanoseconds,
				Tags: map[string]string{
					"source_id":   "source-id-with-index",
					"instance_id": "1",
				},
			}))
		})
	})

	Context("when the metric tags are provided", func() {
		BeforeEach(func() {
			containers := []executor.Container{
				{Guid: "container-0"},
			}
			fakeExecutorClient.ListContainersReturnsOnCall(0, containers, nil)
		})

		Context("when the source_id tag is set in metrics config for a container", func() {
			BeforeEach(func() {
				metricsMap := map[string]executor.Metrics{
					"container-0": {
						MetricsConfig:    executor.MetricsConfig{Tags: map[string]string{"source_id": "some-source-id"}, Guid: "some-metric-guid"},
						ContainerMetrics: executor.ContainerMetrics{},
					},
				}
				fakeExecutorClient.GetBulkMetricsReturns(metricsMap, nil)
			})

			It("sends a container metric with the correct application id value", func() {
				fakeClock.WaitForWatcherAndIncrement(interval)
				Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(2))
				Eventually(fakeMetronClient.SendAppMetricsCallCount).Should(Equal(1))
				Expect(fakeMetronClient.SendAppMetricsArgsForCall(0).Tags).To(HaveKeyWithValue("source_id", "some-source-id"))
			})
		})

		Context("when there is no source_id tag set in metrics config for a container and the metricGuid is not set", func() {
			BeforeEach(func() {
				metricsMap := map[string]executor.Metrics{
					"container-0": {
						MetricsConfig:    executor.MetricsConfig{Tags: map[string]string{"some-key": "some-value"}},
						ContainerMetrics: executor.ContainerMetrics{},
					},
				}
				fakeExecutorClient.GetBulkMetricsReturns(metricsMap, nil)
			})

			It("will not emit any metrics", func() {
				fakeClock.WaitForWatcherAndIncrement(interval)
				Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(2))
				Consistently(fakeMetronClient.SendAppMetricsCallCount).Should(Equal(0))
			})
		})

		Context("when instance id tag set in metrics config for container", func() {
			BeforeEach(func() {
				metricsMap := map[string]executor.Metrics{
					"container-0": {
						MetricsConfig:    executor.MetricsConfig{Tags: map[string]string{"source_id": "some-source-id", "instance_id": "99"}, Index: 1},
						ContainerMetrics: executor.ContainerMetrics{},
					},
				}
				fakeExecutorClient.GetBulkMetricsReturns(metricsMap, nil)
			})

			It("sends a container metric with the correct instance id value", func() {
				fakeClock.WaitForWatcherAndIncrement(interval)
				Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(2))
				Eventually(fakeMetronClient.SendAppMetricsCallCount).Should(Equal(1))
				Expect(fakeMetronClient.SendAppMetricsArgsForCall(0).Tags).To(HaveKeyWithValue("instance_id", "99"))
			})
		})

		Context("when instance id tag not set in metrics config for container", func() {
			BeforeEach(func() {
				metricsMap := map[string]executor.Metrics{
					"container-0": {
						MetricsConfig:    executor.MetricsConfig{Tags: map[string]string{"source_id": "some-source-id"}, Index: 1},
						ContainerMetrics: executor.ContainerMetrics{},
					},
				}
				fakeExecutorClient.GetBulkMetricsReturns(metricsMap, nil)
			})

			It("sends a container metric with the correct instance id value", func() {
				fakeClock.WaitForWatcherAndIncrement(interval)
				Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(2))
				Eventually(fakeMetronClient.SendAppMetricsCallCount).Should(Equal(1))
				Expect(fakeMetronClient.SendAppMetricsArgsForCall(0).Tags).To(HaveKeyWithValue("instance_id", "1"))
			})
		})

		Context("when instance id tag set to non-integer value in metrics config for container", func() {
			BeforeEach(func() {
				metricsMap := map[string]executor.Metrics{
					"container-0": {
						MetricsConfig:    executor.MetricsConfig{Tags: map[string]string{"source_id": "some-source-id", "instance_id": "some-instance-id"}, Index: 1},
						ContainerMetrics: executor.ContainerMetrics{},
					},
				}
				fakeExecutorClient.GetBulkMetricsReturns(metricsMap, nil)
			})

			It("keeps the metrics tag value of instance id", func() {
				fakeClock.WaitForWatcherAndIncrement(interval)
				Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(2))
				Eventually(fakeMetronClient.SendAppMetricsCallCount).Should(Equal(1))
				Expect(fakeMetronClient.SendAppMetricsArgsForCall(0).Tags).To(HaveKeyWithValue("instance_id", "some-instance-id"))
			})
		})
	})

	Context("when metric tags are not provided", func() {
		BeforeEach(func() {
			containers := []executor.Container{
				{Guid: "container-0"},
			}
			fakeExecutorClient.ListContainersReturnsOnCall(0, containers, nil)
		})

		Context("when the metrics guid is provided", func() {
			BeforeEach(func() {
				metricsMap := map[string]executor.Metrics{
					"container-0": {
						MetricsConfig:    executor.MetricsConfig{Guid: "some-metric-guid"},
						ContainerMetrics: executor.ContainerMetrics{},
					},
				}
				fakeExecutorClient.GetBulkMetricsReturns(metricsMap, nil)
			})

			It("sends a container metric with the correct application id value", func() {
				fakeClock.WaitForWatcherAndIncrement(interval)
				Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(2))
				Eventually(fakeMetronClient.SendAppMetricsCallCount).Should(Equal(1))
				Expect(fakeMetronClient.SendAppMetricsArgsForCall(0).Tags).To(HaveKeyWithValue("source_id", "some-metric-guid"))
			})
		})

		Context("when the metrics guid is not provided", func() {
			BeforeEach(func() {
				metricsMap := map[string]executor.Metrics{
					"container-0": {
						MetricsConfig:    executor.MetricsConfig{Guid: ""},
						ContainerMetrics: executor.ContainerMetrics{},
					},
				}
				fakeExecutorClient.GetBulkMetricsReturns(metricsMap, nil)
			})

			It("will not emit any metrics", func() {
				fakeClock.WaitForWatcherAndIncrement(interval)
				Eventually(fakeExecutorClient.GetBulkMetricsCallCount).Should(Equal(2))
				Consistently(fakeMetronClient.SendAppMetricsCallCount).Should(Equal(0))
			})
		})
	})

})
