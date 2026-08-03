package metrics_test

import (
	"errors"
	"os"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock/fakeclock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/metrics"
	"code.cloudfoundry.org/executor/fakes"
	loggregator "code.cloudfoundry.org/go-loggregator/v9"
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/tedsuo/ifrit"
)

type metricEnvelope struct {
	value int
	tags  map[string]string
}

var _ = Describe("Reporter", func() {
	var (
		reportInterval   time.Duration
		executorClient   *fakes.FakeClient
		fakeClock        *fakeclock.FakeClock
		fakeMetronClient *mfakes.FakeIngressClient

		reporter  ifrit.Process
		logger    *lagertest.TestLogger
		metricMap map[string]metricEnvelope
		m         sync.RWMutex
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		reportInterval = 1 * time.Millisecond
		executorClient = new(fakes.FakeClient)

		fakeClock = fakeclock.NewFakeClock(time.Now())
		fakeMetronClient = new(mfakes.FakeIngressClient)

		executorClient.GetBulkMetricsReturns(map[string]executor.Metrics{
			"container-1": executor.Metrics{
				MetricsConfig: executor.MetricsConfig{},
				ContainerMetrics: executor.ContainerMetrics{
					MemoryUsageInBytes: 256 * 1024 * 1024,
					DiskUsageInBytes:   800 * 1024 * 1024,
				},
			},
			"container-2": executor.Metrics{
				MetricsConfig: executor.MetricsConfig{},
				ContainerMetrics: executor.ContainerMetrics{
					MemoryUsageInBytes: 300 * 1024 * 1024,
					DiskUsageInBytes:   512 * 1024 * 1024,
				},
			},
		}, nil)

		executorClient.TotalResourcesReturns(executor.ExecutorResources{
			MemoryMB:   1024,
			DiskMB:     2048,
			Containers: 4096,
		}, nil)

		executorClient.RemainingResourcesReturns(executor.ExecutorResources{
			MemoryMB:   128,
			DiskMB:     256,
			Containers: 512,
		}, nil)

		executorClient.ListContainersReturns([]executor.Container{
			{Guid: "container-1", State: executor.StateInitializing},
			{Guid: "container-2", State: executor.StateReserved},
			{Guid: "container-3", State: executor.StateCreated},
			{Guid: "container-4", State: executor.StateRunning},
			{Guid: "container-5"},
		}, nil)

		m = sync.RWMutex{}
	})

	JustBeforeEach(func() {
		metricMap = make(map[string]metricEnvelope)

		sendStub := func(name string, value int, opts ...loggregator.EmitGaugeOption) error {
			m.Lock()
			envelope := metricEnvelope{value: value, tags: map[string]string{}}
			e := &loggregator_v2.Envelope{Tags: map[string]string{}}
			for _, opt := range opts {
				opt(e)
			}
			envelope.tags = e.Tags
			metricMap[name] = envelope
			m.Unlock()
			return nil
		}
		fakeMetronClient.SendMetricStub = sendStub
		fakeMetronClient.SendMebiBytesStub = sendStub

		reporter = ifrit.Invoke(&metrics.Reporter{
			ExecutorSource: executorClient,
			Interval:       reportInterval,
			Clock:          fakeClock,
			Logger:         logger,
			MetronClient:   fakeMetronClient,
			Tags:           map[string]string{"foo": "bar"},
		})
		fakeClock.WaitForWatcherAndIncrement(reportInterval)

	})

	AfterEach(func() {
		reporter.Signal(os.Interrupt)
		Eventually(reporter.Wait()).Should(Receive())
	})

	It("reports the current capacity on the given interval", func() {
		Eventually(fakeMetronClient.SendMebiBytesCallCount).Should(Equal(8))
		Eventually(fakeMetronClient.SendMetricCallCount).Should(Equal(4))

		m.RLock()
		remainingMemory := metricMap["CapacityRemainingMemory"]
		totalMemory := metricMap["CapacityTotalMemory"]
		remainingDisk := metricMap["CapacityRemainingDisk"]
		totalDisk := metricMap["CapacityTotalDisk"]

		expectedTags := map[string]string{"foo": "bar"}

		Eventually(totalMemory.value).Should(Equal(1024))
		Eventually(totalMemory.tags).Should(Equal(expectedTags))
		Eventually(metricMap["CapacityTotalContainers"].value).Should(Equal(4096))
		Eventually(metricMap["CapacityTotalContainers"].tags).Should(Equal(expectedTags))
		Eventually(totalDisk.value).Should(Equal(2048))
		Eventually(totalDisk.tags).Should(Equal(expectedTags))

		Eventually(remainingMemory.value).Should(Equal(128))
		Eventually(remainingMemory.tags).Should(Equal(expectedTags))
		Eventually(remainingDisk.value).Should(Equal(256))
		Eventually(remainingDisk.tags).Should(Equal(expectedTags))
		Eventually(metricMap["CapacityRemainingContainers"].value).Should(Equal(512))
		Eventually(metricMap["CapacityRemainingContainers"].tags).Should(Equal(expectedTags))

		Eventually(metricMap["CapacityAllocatedMemory"].value).Should(Equal(totalMemory.value - remainingMemory.value))
		Eventually(metricMap["CapacityAllocatedMemory"].tags).Should(Equal(expectedTags))
		Eventually(metricMap["CapacityAllocatedDisk"].value).Should(Equal(totalDisk.value - remainingDisk.value))
		Eventually(metricMap["CapacityAllocatedDisk"].tags).Should(Equal(expectedTags))

		Eventually(metricMap["ContainerUsageMemory"].value).Should(Equal(556))
		Eventually(metricMap["ContainerUsageMemory"].tags).Should(Equal(expectedTags))
		Eventually(metricMap["ContainerUsageDisk"].value).Should(Equal(1312))
		Eventually(metricMap["ContainerUsageDisk"].tags).Should(Equal(expectedTags))

		Eventually(metricMap["ContainerCount"].value).Should(Equal(5))
		Eventually(metricMap["ContainerCount"].tags).Should(Equal(expectedTags))
		Eventually(metricMap["StartingContainerCount"].value).Should(Equal(3))
		Eventually(metricMap["StartingContainerCount"].tags).Should(Equal(expectedTags))

		executorClient.GetBulkMetricsReturns(map[string]executor.Metrics{
			"container-1": executor.Metrics{
				MetricsConfig: executor.MetricsConfig{},
				ContainerMetrics: executor.ContainerMetrics{
					MemoryUsageInBytes: 300 * 1024 * 1024,
					DiskUsageInBytes:   400 * 1024 * 1024,
				},
			},
			"container-2": executor.Metrics{
				MetricsConfig: executor.MetricsConfig{},
				ContainerMetrics: executor.ContainerMetrics{
					MemoryUsageInBytes: 200 * 1024 * 1024,
					DiskUsageInBytes:   300 * 1024 * 1024,
				},
			},
		}, nil)

		executorClient.RemainingResourcesReturns(executor.ExecutorResources{
			MemoryMB:   129,
			DiskMB:     257,
			Containers: 513,
		}, nil)

		executorClient.ListContainersReturns([]executor.Container{
			{Guid: "container-1"},
			{Guid: "container-2"},
		}, nil)

		fakeClock.WaitForWatcherAndIncrement(reportInterval)

		m.RUnlock()

		Eventually(fakeMetronClient.SendMebiBytesCallCount).Should(Equal(16))
		Eventually(fakeMetronClient.SendMetricCallCount).Should(Equal(8))

		m.RLock()

		remainingMemory = metricMap["CapacityRemainingMemory"]
		remainingDisk = metricMap["CapacityRemainingDisk"]

		Eventually(metricMap["CapacityRemainingMemory"].value).Should(Equal(129))
		Eventually(metricMap["CapacityRemainingDisk"].value).Should(Equal(257))
		Eventually(metricMap["CapacityRemainingContainers"].value).Should(Equal(513))
		Eventually(metricMap["CapacityAllocatedMemory"].value).Should(Equal(totalMemory.value - remainingMemory.value))
		Eventually(metricMap["CapacityAllocatedDisk"].value).Should(Equal(totalDisk.value - remainingDisk.value))

		Eventually(metricMap["ContainerUsageMemory"].value).Should(Equal(500))
		Eventually(metricMap["ContainerUsageDisk"].value).Should(Equal(700))

		Eventually(metricMap["ContainerCount"].value).Should(Equal(2))
		Eventually(metricMap["StartingContainerCount"].value).Should(Equal(0))

		m.RUnlock()
	})

	Context("when getting remaining resources fails", func() {
		BeforeEach(func() {
			executorClient.RemainingResourcesReturns(executor.ExecutorResources{}, errors.New("oh no!"))
		})

		It("sends missing remaining resources", func() {
			Eventually(fakeMetronClient.SendMebiBytesCallCount).Should(Equal(8))

			m.RLock()
			Eventually(metricMap["CapacityRemainingMemory"].value).Should(Equal(-1))
			Eventually(metricMap["CapacityRemainingDisk"].value).Should(Equal(-1))
			Eventually(metricMap["CapacityRemainingContainers"].value).Should(Equal(-1))
			m.RUnlock()
		})

		It("sends missing allocated resources", func() {
			Eventually(fakeMetronClient.SendMebiBytesCallCount).Should(Equal(8))

			m.RLock()
			Eventually(metricMap["CapacityAllocatedMemory"].value).Should(Equal(-1))
			Eventually(metricMap["CapacityAllocatedDisk"].value).Should(Equal(-1))
			m.RUnlock()
		})
	})

	Context("when getting total resources fails", func() {
		BeforeEach(func() {
			executorClient.TotalResourcesReturns(executor.ExecutorResources{}, errors.New("oh no!"))
		})

		It("sends missing total resources", func() {
			Eventually(fakeMetronClient.SendMebiBytesCallCount).Should(Equal(8))

			m.RLock()
			Eventually(metricMap["CapacityTotalMemory"].value).Should(Equal(-1))
			Eventually(metricMap["CapacityTotalContainers"].value).Should(Equal(-1))
			Eventually(metricMap["CapacityTotalDisk"].value).Should(Equal(-1))
			m.RUnlock()
		})

		It("sends missing allocated resources", func() {
			Eventually(fakeMetronClient.SendMebiBytesCallCount).Should(Equal(8))

			m.RLock()
			Eventually(metricMap["CapacityAllocatedMemory"].value).Should(Equal(-1))
			Eventually(metricMap["CapacityAllocatedDisk"].value).Should(Equal(-1))
			m.RUnlock()
		})
	})

	Context("when getting the containers fails", func() {
		BeforeEach(func() {
			executorClient.ListContainersReturns(nil, errors.New("oh no!"))
		})

		It("reports garden.containers as -1", func() {
			Eventually(fakeMetronClient.SendMetricCallCount).Should(Equal(4))

			m.RLock()
			Eventually(metricMap["ContainerCount"].value).Should(Equal(-1))
			Eventually(metricMap["StartingContainerCount"].value).Should(Equal(0))
			m.RUnlock()
		})
	})

	Context("when getting the bulk metrics fails", func() {
		BeforeEach(func() {
			executorClient.GetBulkMetricsReturns(nil, errors.New("oh no!"))
		})

		It("reports container usage as -1", func() {
			Eventually(fakeMetronClient.SendMebiBytesCallCount).Should(Equal(8))

			m.RLock()
			Eventually(metricMap["ContainerUsageDisk"].value).Should(Equal(-1))
			Eventually(metricMap["ContainerUsageMemory"].value).Should(Equal(-1))
			m.RUnlock()
		})
	})
})
