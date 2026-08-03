package handlers_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/executor/containermetrics"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ContainerMetrics", func() {
	var (
		containerMetrics  *rep.ContainerMetricsCollection
		requestLatency    time.Duration
		requestIdHeader   = "fa89bcf8-3607-419f-a4b3-151312f5154b"
		b3RequestIdHeader = fmt.Sprintf(`"trace-id":"%s"`, strings.Replace(requestIdHeader, "-", "", -1))
	)

	BeforeEach(func() {
		one := uint64(1)
		containerMetrics = &rep.ContainerMetricsCollection{
			CellID: "some-cell-id",
			LRPs: []rep.LRPMetric{
				{
					ProcessGUID:  "some-process-guid",
					InstanceGUID: "some-instance-guid",
					CachedContainerMetrics: containermetrics.CachedContainerMetrics{
						RxBytes: &one,
						TxBytes: &one,
					},
				},
			},
			Tasks: []rep.TaskMetric{
				{
					TaskGUID: "some-guid",
				},
			},
		}
		requestLatency = 50 * time.Millisecond

		fakeMetricCollector.MetricsStub = func(logger lager.Logger) (*rep.ContainerMetricsCollection, error) {
			time.Sleep(requestLatency)
			return containerMetrics, nil
		}
	})

	It("has the right field names", func() {
		status, body := Request(rep.ContainerMetricsRoute, nil, nil)
		Expect(status).To(Equal(http.StatusOK))

		// part of the response up to "tasks"
		lrps := strings.Split(string(body), "tasks")[0]
		// part of the response after "tasks"
		tasks := strings.Split(string(body), "tasks")[1]

		Expect(lrps).To(ContainSubstring(`process_guid`))
		Expect(lrps).To(ContainSubstring(`instance_guid`))
		Expect(lrps).To(ContainSubstring(`index`))
		Expect(lrps).To(ContainSubstring(`metric_guid`))
		Expect(lrps).To(ContainSubstring(`cpu_usage_fraction`))
		Expect(lrps).To(ContainSubstring(`disk_usage_bytes`))
		Expect(lrps).To(ContainSubstring(`disk_quota_bytes`))
		Expect(lrps).To(ContainSubstring(`memory_usage_bytes`))
		Expect(lrps).To(ContainSubstring(`memory_quota_bytes`))
		Expect(lrps).To(ContainSubstring(`rx_bytes`))
		Expect(lrps).To(ContainSubstring(`tx_bytes`))

		Expect(tasks).To(ContainSubstring(`task_guid`))
		Expect(tasks).To(ContainSubstring(`metric_guid`))
		Expect(tasks).To(ContainSubstring(`cpu_usage_fraction`))
		Expect(tasks).To(ContainSubstring(`disk_usage_bytes`))
		Expect(tasks).To(ContainSubstring(`disk_quota_bytes`))
		Expect(tasks).To(ContainSubstring(`memory_usage_bytes`))
		Expect(tasks).To(ContainSubstring(`memory_quota_bytes`))
		Expect(tasks).ToNot(ContainSubstring(`rx_bytes`))
		Expect(tasks).ToNot(ContainSubstring(`tx_bytes`))
	})

	It("it returns whatever the container_metrics call returns", func() {
		status, body := Request(rep.ContainerMetricsRoute, nil, nil)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(JSONFor(containerMetrics)))
		Expect(fakeMetricCollector.MetricsCallCount()).To(Equal(1))
	})

	It("emits the request metrics", func() {
		Request(rep.ContainerMetricsRoute, nil, nil)

		Expect(fakeRequestMetrics.IncrementRequestsStartedCounterCallCount()).To(Equal(1))
		calledRequestType, delta := fakeRequestMetrics.IncrementRequestsStartedCounterArgsForCall(0)
		Expect(delta).To(Equal(1))
		Expect(calledRequestType).To(Equal("ContainerMetrics"))

		Expect(fakeRequestMetrics.IncrementRequestsInFlightCounterCallCount()).To(Equal(1))
		calledRequestType, delta = fakeRequestMetrics.IncrementRequestsInFlightCounterArgsForCall(0)
		Expect(delta).To(Equal(1))
		Expect(calledRequestType).To(Equal("ContainerMetrics"))

		Expect(fakeRequestMetrics.DecrementRequestsInFlightCounterCallCount()).To(Equal(1))
		calledRequestType, delta = fakeRequestMetrics.DecrementRequestsInFlightCounterArgsForCall(0)
		Expect(delta).To(Equal(1))
		Expect(calledRequestType).To(Equal("ContainerMetrics"))

		Expect(fakeRequestMetrics.UpdateLatencyCallCount()).To(Equal(1))
		calledRequestType, calledLatency := fakeRequestMetrics.UpdateLatencyArgsForCall(0)
		Expect(calledRequestType).To(Equal("ContainerMetrics"))
		Expect(calledLatency).To(BeNumerically("~", requestLatency, 25*time.Millisecond))

		Expect(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).To(Equal(1))
		calledRequestType, delta = fakeRequestMetrics.IncrementRequestsSucceededCounterArgsForCall(0)
		Expect(delta).To(Equal(1))
		Expect(calledRequestType).To(Equal("ContainerMetrics"))

		Expect(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).To(Equal(0))
	})

	Context("when the container_metrics call fails", func() {
		BeforeEach(func() {
			fakeMetricCollector.MetricsReturns(nil, errors.New("some-err"))
		})

		It("fails", func() {
			Expect(fakeMetricCollector.MetricsCallCount()).To(Equal(0))

			status, body := RequestTracing(rep.ContainerMetricsRoute, nil, nil, requestIdHeader)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(BeEmpty())
			Expect(fakeMetricCollector.MetricsCallCount()).To(Equal(1))
			Eventually(logger).Should(gbytes.Say(b3RequestIdHeader))
		})

		It("emits the failed request metrics", func() {
			RequestTracing(rep.ContainerMetricsRoute, nil, nil, requestIdHeader)

			Expect(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).To(Equal(0))

			Expect(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).To(Equal(1))
			calledRequestType, delta := fakeRequestMetrics.IncrementRequestsFailedCounterArgsForCall(0)
			Expect(delta).To(Equal(1))
			Expect(calledRequestType).To(Equal("ContainerMetrics"))
			Eventually(logger).Should(gbytes.Say("failed-to-fetch-container-metrics"))
			Eventually(logger).Should(gbytes.Say(b3RequestIdHeader))
		})
	})
})
