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

	executorfakes "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/rep/handlers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("StopLRPInstanceHandler", func() {
	var (
		stopInstanceHandler *handlers.StopLRPInstanceHandler
		fakeClient          *executorfakes.FakeClient
		resp                *httptest.ResponseRecorder
		req                 *http.Request
		logger              *lagertest.TestLogger
		requestIdHeader     string
		b3RequestIdHeader   string
	)

	BeforeEach(func() {
		var err error

		fakeClient = &executorfakes.FakeClient{}

		logger = lagertest.NewTestLogger("test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))

		stopInstanceHandler = handlers.NewStopLRPInstanceHandler(fakeClient, fakeRequestMetrics)

		resp = httptest.NewRecorder()

		req, err = http.NewRequest("POST", "", nil)
		Expect(err).NotTo(HaveOccurred())

		requestIdHeader = "fa89bde2-3607-419f-a4b3-151312f5154b"
		req.Header.Set(lager.RequestIdHeader, requestIdHeader)
		b3RequestIdHeader = fmt.Sprintf(`"trace-id":"%s"`, strings.Replace(requestIdHeader, "-", "", -1))
	})

	JustBeforeEach(func() {
		stopInstanceHandler.ServeHTTP(resp, req, logger)
	})

	Context("when the request is valid", func() {
		var processGuid, instanceGuid string

		BeforeEach(func() {
			processGuid = "process-guid"
			instanceGuid = "instance-guid"

			values := make(url.Values)
			values.Set(":process_guid", processGuid)
			values.Set(":instance_guid", instanceGuid)
			req.URL.RawQuery = values.Encode()
		})

		Context("and StopContainer succeeds", func() {
			var requestLatency time.Duration

			BeforeEach(func() {
				requestLatency = 50 * time.Millisecond
				fakeClient.StopContainerStub = func(logger lager.Logger, traceID string, guid string) error {
					time.Sleep(requestLatency)
					return nil
				}
			})

			It("responds with 202 Accepted", func() {
				Expect(resp.Code).To(Equal(http.StatusAccepted))
			})

			It("eventually stops the instance", func() {
				Eventually(fakeClient.StopContainerCallCount).Should(Equal(1))

				processGuid, traceID, instanceGuid := fakeClient.StopContainerArgsForCall(0)
				Expect(traceID).To(Equal(requestIdHeader))
				Expect(processGuid).To(Equal(processGuid))
				Expect(instanceGuid).To(Equal(instanceGuid))
			})

			It("emits the request metrics", func() {
				Expect(fakeRequestMetrics.IncrementRequestsStartedCounterCallCount()).To(Equal(1))
				calledRequestType, delta := fakeRequestMetrics.IncrementRequestsStartedCounterArgsForCall(0)
				Expect(delta).To(Equal(1))
				Expect(calledRequestType).To(Equal("StopLRPInstance"))

				Expect(fakeRequestMetrics.IncrementRequestsInFlightCounterCallCount()).To(Equal(1))
				calledRequestType, delta = fakeRequestMetrics.IncrementRequestsInFlightCounterArgsForCall(0)
				Expect(delta).To(Equal(1))
				Expect(calledRequestType).To(Equal("StopLRPInstance"))

				Expect(fakeRequestMetrics.DecrementRequestsInFlightCounterCallCount()).To(Equal(1))
				calledRequestType, delta = fakeRequestMetrics.DecrementRequestsInFlightCounterArgsForCall(0)
				Expect(delta).To(Equal(1))
				Expect(calledRequestType).To(Equal("StopLRPInstance"))

				Expect(fakeRequestMetrics.UpdateLatencyCallCount()).To(Equal(1))
				calledRequestType, calledLatency := fakeRequestMetrics.UpdateLatencyArgsForCall(0)
				Expect(calledRequestType).To(Equal("StopLRPInstance"))
				Expect(calledLatency).To(BeNumerically("~", requestLatency, 25*time.Millisecond))

				Expect(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).To(Equal(1))
				calledRequestType, delta = fakeRequestMetrics.IncrementRequestsSucceededCounterArgsForCall(0)
				Expect(delta).To(Equal(1))
				Expect(calledRequestType).To(Equal("StopLRPInstance"))

				Expect(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).To(Equal(0))
			})
		})

		Context("but StopContainer fails", func() {
			BeforeEach(func() {
				fakeClient.StopContainerReturns(errors.New("fail"))
			})

			It("responds with 500 Internal Server Error", func() {
				Expect(resp.Code).To(Equal(http.StatusInternalServerError))
			})

			It("emits the failed request metrics", func() {
				Expect(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).To(Equal(0))

				Expect(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).To(Equal(1))
				calledRequestType, delta := fakeRequestMetrics.IncrementRequestsFailedCounterArgsForCall(0)
				Expect(delta).To(Equal(1))
				Expect(calledRequestType).To(Equal("StopLRPInstance"))

				Eventually(logger).Should(gbytes.Say("failed-to-stop-container"))
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

		It("does not attempt to stop the instance", func() {
			Expect(fakeClient.StopContainerCallCount()).To(Equal(0))
		})

		It("emits the failed request metrics", func() {
			Expect(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).To(Equal(0))

			Expect(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).To(Equal(1))
			calledRequestType, delta := fakeRequestMetrics.IncrementRequestsFailedCounterArgsForCall(0)
			Expect(delta).To(Equal(1))
			Expect(calledRequestType).To(Equal("StopLRPInstance"))
		})
		It("logs the error with trace-id", func() {
			Eventually(logger).Should(gbytes.Say("missing-process-guid"))
			Eventually(logger).Should(gbytes.Say(b3RequestIdHeader))
		})
	})
})
