package handlers_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	executorfakes "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"

	"code.cloudfoundry.org/rep/handlers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("UpdateLRPInstanceHandler", func() {
	var (
		updateInstanceHandler *handlers.UpdateLRPInstanceHandler
		fakeClient            *executorfakes.FakeClient
		resp                  *httptest.ResponseRecorder
		req                   *http.Request
		logger                *lagertest.TestLogger
		processGuid           string
		instanceGuid          string
		internalRoutes        models.InternalRoutes
		metricTags            map[string]string

		requestIdHeader   string
		b3RequestIdHeader string
	)

	BeforeEach(func() {
		var err error

		fakeClient = &executorfakes.FakeClient{}

		logger = lagertest.NewTestLogger("test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))

		updateInstanceHandler = handlers.NewUpdateLRPInstanceHandler(fakeClient, fakeRequestMetrics)

		resp = httptest.NewRecorder()

		processGuid = "process-guid"
		instanceGuid = "instance-guid"
		internalRoutes = models.InternalRoutes{
			{Hostname: "a.apps.internal"},
			{Hostname: "b.apps.internal"},
		}
		metricTags = map[string]string{"some-key": "some-value"}
		k := models.NewActualLRPKey(processGuid, 2, "test-domain")
		lrpUpdate := models.NewLRPUpdate(instanceGuid, k, internalRoutes, metricTags)
		req, err = http.NewRequest("PUT", "", JSONReaderFor(lrpUpdate))
		Expect(err).NotTo(HaveOccurred())

		requestIdHeader = "fa89bde2-3607-419f-a4b3-151312f5515c"
		req.Header.Set(lager.RequestIdHeader, requestIdHeader)
		b3RequestIdHeader = fmt.Sprintf(`"trace-id":"%s"`, strings.Replace(requestIdHeader, "-", "", -1))
	})

	JustBeforeEach(func() {
		updateInstanceHandler.ServeHTTP(resp, req, logger)
	})

	Context("when the request is valid", func() {
		BeforeEach(func() {
			values := make(url.Values)
			values.Set(":process_guid", processGuid)
			values.Set(":instance_guid", instanceGuid)
			req.URL.RawQuery = values.Encode()
		})

		Context("and UpdateContainer succeeds", func() {
			var requestLatency time.Duration

			BeforeEach(func() {
				requestLatency = 50 * time.Millisecond
				fakeClient.UpdateContainerStub = func(logger lager.Logger, updateReq *executor.UpdateRequest) error {
					time.Sleep(requestLatency)
					return nil
				}
			})

			It("responds with 202 Accepted", func() {
				Expect(resp.Code).To(Equal(http.StatusAccepted))
			})

			It("eventually updates the instance", func() {
				Eventually(fakeClient.UpdateContainerCallCount).Should(Equal(1))

				_, updateReq := fakeClient.UpdateContainerArgsForCall(0)
				Expect(updateReq.Guid).To(Equal(instanceGuid))
				Expect(updateReq.InternalRoutes).To(Equal(internalRoutes))
				Expect(updateReq.MetricTags).To(Equal(metricTags))
			})

			It("emits the request metrics", func() {
				Expect(fakeRequestMetrics.IncrementRequestsStartedCounterCallCount()).To(Equal(1))
				calledRequestType, delta := fakeRequestMetrics.IncrementRequestsStartedCounterArgsForCall(0)
				Expect(delta).To(Equal(1))
				Expect(calledRequestType).To(Equal("UpdateLRPInstance"))

				Expect(fakeRequestMetrics.IncrementRequestsInFlightCounterCallCount()).To(Equal(1))
				calledRequestType, delta = fakeRequestMetrics.IncrementRequestsInFlightCounterArgsForCall(0)
				Expect(delta).To(Equal(1))
				Expect(calledRequestType).To(Equal("UpdateLRPInstance"))

				Expect(fakeRequestMetrics.DecrementRequestsInFlightCounterCallCount()).To(Equal(1))
				calledRequestType, delta = fakeRequestMetrics.DecrementRequestsInFlightCounterArgsForCall(0)
				Expect(delta).To(Equal(1))
				Expect(calledRequestType).To(Equal("UpdateLRPInstance"))

				Expect(fakeRequestMetrics.UpdateLatencyCallCount()).To(Equal(1))
				calledRequestType, calledLatency := fakeRequestMetrics.UpdateLatencyArgsForCall(0)
				Expect(calledRequestType).To(Equal("UpdateLRPInstance"))
				Expect(calledLatency).To(BeNumerically("~", requestLatency, 25*time.Millisecond))

				Expect(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).To(Equal(1))
				calledRequestType, delta = fakeRequestMetrics.IncrementRequestsSucceededCounterArgsForCall(0)
				Expect(delta).To(Equal(1))
				Expect(calledRequestType).To(Equal("UpdateLRPInstance"))

				Expect(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).To(Equal(0))
			})
		})

		Context("but UpdateContainer fails", func() {
			BeforeEach(func() {
				fakeClient.UpdateContainerReturns(errors.New("fail"))
			})

			It("responds with 500 Internal Server Error", func() {
				Expect(resp.Code).To(Equal(http.StatusInternalServerError))
			})

			It("emits the failed request metrics", func() {
				Expect(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).To(Equal(0))

				Expect(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).To(Equal(1))
				calledRequestType, delta := fakeRequestMetrics.IncrementRequestsFailedCounterArgsForCall(0)
				Expect(delta).To(Equal(1))
				Expect(calledRequestType).To(Equal("UpdateLRPInstance"))
			})

			It("logs the event-id", func() {
				Eventually(logger).Should(gbytes.Say(b3RequestIdHeader))
			})
		})
	})

	Context("when the request is invalid", func() {
		BeforeEach(func() {
			req.Body = io.NopCloser(bytes.NewBufferString("foo"))
		})

		It("responds with 400 Bad Request", func() {
			Expect(resp.Code).To(Equal(http.StatusBadRequest))
		})

		It("does not attempt to update the instance", func() {
			Expect(fakeClient.UpdateContainerCallCount()).To(Equal(0))
		})

		It("emits the failed request metrics", func() {
			Expect(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).To(Equal(0))

			Expect(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).To(Equal(1))
			calledRequestType, delta := fakeRequestMetrics.IncrementRequestsFailedCounterArgsForCall(0)
			Expect(delta).To(Equal(1))
			Expect(calledRequestType).To(Equal("UpdateLRPInstance"))
		})

		It("logs the event-id", func() {
			Eventually(logger).Should(gbytes.Say(b3RequestIdHeader))
		})
	})
})
