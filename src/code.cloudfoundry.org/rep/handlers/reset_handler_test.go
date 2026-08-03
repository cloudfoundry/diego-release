package handlers_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/rep"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Reset", func() {
	Context("when the reset succeeds", func() {
		var requestLatency time.Duration

		BeforeEach(func() {
			requestLatency = 50 * time.Millisecond
			fakeLocalRep.ResetStub = func() error {
				time.Sleep(requestLatency)
				return nil
			}
		})

		It("succeeds", func() {
			status, body := Request(rep.SimResetRoute, nil, nil)
			Expect(status).To(Equal(http.StatusOK))
			Expect(body).To(BeEmpty())

			Expect(fakeLocalRep.ResetCallCount()).To(Equal(1))
		})

		It("emits the request metrics", func() {
			Request(rep.SimResetRoute, nil, nil)

			Expect(fakeRequestMetrics.IncrementRequestsStartedCounterCallCount()).To(Equal(1))
			calledRequestType, delta := fakeRequestMetrics.IncrementRequestsStartedCounterArgsForCall(0)
			Expect(delta).To(Equal(1))
			Expect(calledRequestType).To(Equal("Reset"))

			Expect(fakeRequestMetrics.IncrementRequestsInFlightCounterCallCount()).To(Equal(1))
			calledRequestType, delta = fakeRequestMetrics.IncrementRequestsInFlightCounterArgsForCall(0)
			Expect(delta).To(Equal(1))
			Expect(calledRequestType).To(Equal("Reset"))

			Expect(fakeRequestMetrics.DecrementRequestsInFlightCounterCallCount()).To(Equal(1))
			calledRequestType, delta = fakeRequestMetrics.DecrementRequestsInFlightCounterArgsForCall(0)
			Expect(delta).To(Equal(1))
			Expect(calledRequestType).To(Equal("Reset"))

			Expect(fakeRequestMetrics.UpdateLatencyCallCount()).To(Equal(1))
			calledRequestType, calledLatency := fakeRequestMetrics.UpdateLatencyArgsForCall(0)
			Expect(calledRequestType).To(Equal("Reset"))
			Expect(calledLatency).To(BeNumerically("~", requestLatency, 25*time.Millisecond))

			Expect(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).To(Equal(1))
			calledRequestType, delta = fakeRequestMetrics.IncrementRequestsSucceededCounterArgsForCall(0)
			Expect(delta).To(Equal(1))
			Expect(calledRequestType).To(Equal("Reset"))

			Expect(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).To(Equal(0))
		})
	})

	Context("when the reset fails", func() {
		var (
			requestIdHeader   string
			b3RequestIdHeader string
		)

		BeforeEach(func() {
			fakeLocalRep.ResetReturns(errors.New("boom"))

			requestIdHeader = "fa89bcf8-3607-419f-a4b3-151312f5154b"
			b3RequestIdHeader = fmt.Sprintf(`"trace-id":"%s"`, strings.Replace(requestIdHeader, "-", "", -1))
		})

		It("fails", func() {
			status, body := RequestTracing(rep.SimResetRoute, nil, nil, requestIdHeader)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(BeEmpty())

			Expect(fakeLocalRep.ResetCallCount()).To(Equal(1))

			Eventually(logger).Should(gbytes.Say("failed-to-reset"))
			Eventually(logger).Should(gbytes.Say(b3RequestIdHeader))
		})

		It("emits the failed request metrics", func() {
			RequestTracing(rep.SimResetRoute, nil, nil, requestIdHeader)

			Expect(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).To(Equal(0))

			Expect(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).To(Equal(1))
			calledRequestType, delta := fakeRequestMetrics.IncrementRequestsFailedCounterArgsForCall(0)
			Expect(delta).To(Equal(1))
			Expect(calledRequestType).To(Equal("Reset"))

			Eventually(logger).Should(gbytes.Say("failed-to-reset"))
			Eventually(logger).Should(gbytes.Say(b3RequestIdHeader))
		})
	})
})
