package handlers_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("State", func() {
	var (
		repState       models.CellState
		requestLatency time.Duration
	)

	BeforeEach(func() {
		repState = models.CellState{
			RootFSProviders: models.RootFSProviders{"docker": models.ArbitraryRootFSProvider{}},
		}
		requestLatency = 50 * time.Millisecond
		fakeLocalRep.StateStub = func(logger lager.Logger) (models.CellState, bool, error) {
			time.Sleep(requestLatency)
			return repState, true, nil
		}
	})

	It("it returns whatever the state call returns", func() {
		status, body := Request(rep.StateRoute, nil, nil)
		Expect(status).To(Equal(http.StatusOK))
		Expect(body).To(MatchJSON(JSONFor(repState)))
		Expect(fakeLocalRep.StateCallCount()).To(Equal(1))
	})

	It("emits the request metrics", func() {
		Request(rep.StateRoute, nil, nil)

		Expect(fakeRequestMetrics.IncrementRequestsStartedCounterCallCount()).To(Equal(1))
		calledRequestType, delta := fakeRequestMetrics.IncrementRequestsStartedCounterArgsForCall(0)
		Expect(delta).To(Equal(1))
		Expect(calledRequestType).To(Equal("State"))

		Expect(fakeRequestMetrics.IncrementRequestsInFlightCounterCallCount()).To(Equal(1))
		calledRequestType, delta = fakeRequestMetrics.IncrementRequestsInFlightCounterArgsForCall(0)
		Expect(delta).To(Equal(1))
		Expect(calledRequestType).To(Equal("State"))

		Expect(fakeRequestMetrics.DecrementRequestsInFlightCounterCallCount()).To(Equal(1))
		calledRequestType, delta = fakeRequestMetrics.DecrementRequestsInFlightCounterArgsForCall(0)
		Expect(delta).To(Equal(1))
		Expect(calledRequestType).To(Equal("State"))

		Expect(fakeRequestMetrics.UpdateLatencyCallCount()).To(Equal(1))
		calledRequestType, calledLatency := fakeRequestMetrics.UpdateLatencyArgsForCall(0)
		Expect(calledRequestType).To(Equal("State"))
		Expect(calledLatency).To(BeNumerically("~", requestLatency, 25*time.Millisecond))

		Expect(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).To(Equal(1))
		calledRequestType, delta = fakeRequestMetrics.IncrementRequestsSucceededCounterArgsForCall(0)
		Expect(delta).To(Equal(1))
		Expect(calledRequestType).To(Equal("State"))

		Expect(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).To(Equal(0))
	})

	Context("when the state call is not healthy", func() {
		var (
			requestIdHeader   string
			b3RequestIdHeader string
		)

		BeforeEach(func() {
			fakeLocalRep.StateReturns(repState, false, nil)

			requestIdHeader = "fa89bcf8-3607-419f-a4b3-151312f5154b"
			b3RequestIdHeader = fmt.Sprintf(`"trace-id":"%s"`, strings.Replace(requestIdHeader, "-", "", -1))
		})

		It("returns a StatusServiceUnavailable", func() {
			status, body := RequestTracing(rep.StateRoute, nil, nil, requestIdHeader)
			Expect(status).To(Equal(http.StatusServiceUnavailable))
			Expect(body).To(MatchJSON(JSONFor(repState)))
			Expect(fakeLocalRep.StateCallCount()).To(Equal(1))

			Eventually(logger).Should(gbytes.Say("cell-not-healthy"))
			Eventually(logger).Should(gbytes.Say(b3RequestIdHeader))
		})
	})

	Context("when the state call fails", func() {
		var (
			requestIdHeader   string
			b3RequestIdHeader string
		)

		BeforeEach(func() {
			fakeLocalRep.StateReturns(models.CellState{}, false, errors.New("boom"))

			requestIdHeader = "fa89bcf8-3607-419f-a4b3-151312f5154b"
			b3RequestIdHeader = fmt.Sprintf(`"trace-id":"%s"`, strings.Replace(requestIdHeader, "-", "", -1))
		})

		It("fails", func() {
			status, body := RequestTracing(rep.StateRoute, nil, nil, requestIdHeader)
			Expect(status).To(Equal(http.StatusInternalServerError))
			Expect(body).To(BeEmpty())
			Expect(fakeLocalRep.StateCallCount()).To(Equal(1))
			Eventually(logger).Should(gbytes.Say("failed-to-fetch-state"))
			Eventually(logger).Should(gbytes.Say(b3RequestIdHeader))
		})

		It("emits the failed request metrics", func() {
			RequestTracing(rep.StateRoute, nil, nil, requestIdHeader)

			Expect(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).To(Equal(0))

			Expect(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).To(Equal(1))
			calledRequestType, delta := fakeRequestMetrics.IncrementRequestsFailedCounterArgsForCall(0)
			Expect(delta).To(Equal(1))
			Expect(calledRequestType).To(Equal("State"))

			Eventually(logger).Should(gbytes.Say("failed-to-fetch-state"))
			Eventually(logger).Should(gbytes.Say(b3RequestIdHeader))
		})
	})
})
