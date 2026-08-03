package handlers_test

import (
	"bytes"
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

var _ = Describe("Perform", func() {
	Context("with valid JSON", func() {
		var (
			requestedWork, failedWork models.Work
			requestLatency            time.Duration
			requestIdHeader           string
			b3RequestIdHeader         string
		)

		BeforeEach(func() {
			resourceA := models.NewResource(128, 256, 256)
			placementContraintA := models.NewPlacementConstraint("some-rootfs", nil, nil)
			resourceB := models.NewResource(256, 512, 256)
			placementContraintB := models.NewPlacementConstraint("some-rootfs", nil, nil)
			resourceC := models.NewResource(512, 1024, 256)
			placementContraintC := models.NewPlacementConstraint("some-rootfs", nil, nil)

			requestedWork = models.Work{
				Tasks: []models.SchedulingTask{
					models.NewSchedulingTask("a", "domain", resourceA, placementContraintA),
					models.NewSchedulingTask("b", "domain", resourceB, placementContraintB),
				},
			}

			failedWork = models.Work{
				Tasks: []models.SchedulingTask{
					models.NewSchedulingTask("c", "domain", resourceC, placementContraintC),
				},
			}

			requestLatency = 50 * time.Millisecond

			requestIdHeader = "eb89bcf8-3901-ff0f-a4b3-151312f5154b"
			b3RequestIdHeader = fmt.Sprintf(`"trace-id":"%s"`, strings.Replace(requestIdHeader, "-", "", -1))
		})

		Context("and no perform error", func() {
			BeforeEach(func() {
				fakeLocalRep.PerformStub = func(logger lager.Logger, traceID string, work models.Work) (models.Work, error) {
					time.Sleep(requestLatency)
					return failedWork, nil
				}
			})

			It("succeeds, returning any failed work", func() {
				status, body := RequestTracing(rep.PerformRoute, nil, JSONReaderFor(requestedWork), requestIdHeader)
				Expect(status).To(Equal(http.StatusOK))
				Expect(body).To(MatchJSON(JSONFor(failedWork)))

				Expect(fakeLocalRep.PerformCallCount()).To(Equal(1))
				_, traceID, actualWork := fakeLocalRep.PerformArgsForCall(0)
				Expect(traceID).To(Equal(requestIdHeader))
				Expect(actualWork).To(Equal(requestedWork))

			})

			It("emits the request metrics", func() {
				Request(rep.PerformRoute, nil, JSONReaderFor(requestedWork))

				Expect(fakeRequestMetrics.IncrementRequestsStartedCounterCallCount()).To(Equal(1))
				calledRequestType, delta := fakeRequestMetrics.IncrementRequestsStartedCounterArgsForCall(0)
				Expect(delta).To(Equal(1))
				Expect(calledRequestType).To(Equal("Perform"))

				Expect(fakeRequestMetrics.IncrementRequestsInFlightCounterCallCount()).To(Equal(1))
				calledRequestType, delta = fakeRequestMetrics.IncrementRequestsInFlightCounterArgsForCall(0)
				Expect(delta).To(Equal(1))
				Expect(calledRequestType).To(Equal("Perform"))

				Expect(fakeRequestMetrics.DecrementRequestsInFlightCounterCallCount()).To(Equal(1))
				calledRequestType, delta = fakeRequestMetrics.DecrementRequestsInFlightCounterArgsForCall(0)
				Expect(delta).To(Equal(1))
				Expect(calledRequestType).To(Equal("Perform"))

				Expect(fakeRequestMetrics.UpdateLatencyCallCount()).To(Equal(1))
				calledRequestType, calledLatency := fakeRequestMetrics.UpdateLatencyArgsForCall(0)
				Expect(calledRequestType).To(Equal("Perform"))
				Expect(calledLatency).To(BeNumerically("~", requestLatency, 2*requestLatency))

				Expect(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).To(Equal(1))
				calledRequestType, delta = fakeRequestMetrics.IncrementRequestsSucceededCounterArgsForCall(0)
				Expect(delta).To(Equal(1))
				Expect(calledRequestType).To(Equal("Perform"))

				Expect(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).To(Equal(0))
			})
		})

		Context("and a perform error", func() {
			BeforeEach(func() {
				fakeLocalRep.PerformReturns(failedWork, errors.New("kaboom"))
				requestIdHeader = "eb89bcf8-3901-ff0f-a4b3-151312f5154b"
				b3RequestIdHeader = fmt.Sprintf(`"trace-id":"%s"`, strings.Replace(requestIdHeader, "-", "", -1))
			})

			It("fails, returning nothing", func() {
				status, body := RequestTracing(rep.PerformRoute, nil, JSONReaderFor(requestedWork), requestIdHeader)
				Expect(status).To(Equal(http.StatusInternalServerError))
				Expect(body).To(BeEmpty())

				Expect(fakeLocalRep.PerformCallCount()).To(Equal(1))
				_, traceID, actualWork := fakeLocalRep.PerformArgsForCall(0)
				Expect(traceID).To(Equal(requestIdHeader))
				Expect(actualWork).To(Equal(requestedWork))

				Eventually(logger).Should(gbytes.Say("failed-to-perform-work"))
				Eventually(logger).Should(gbytes.Say(b3RequestIdHeader))
			})

			It("emits the failed request metrics", func() {
				RequestTracing(rep.PerformRoute, nil, JSONReaderFor(requestedWork), requestIdHeader)

				Expect(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).To(Equal(0))

				Expect(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).To(Equal(1))
				calledRequestType, delta := fakeRequestMetrics.IncrementRequestsFailedCounterArgsForCall(0)
				Expect(delta).To(Equal(1))
				Expect(calledRequestType).To(Equal("Perform"))

				Eventually(logger).Should(gbytes.Say("failed-to-perform-work"))
				Eventually(logger).Should(gbytes.Say(b3RequestIdHeader))
			})
		})
	})

	Context("with invalid JSON", func() {
		var (
			requestIdHeader   string
			b3RequestIdHeader string
		)
		BeforeEach(func() {
			requestIdHeader = "eb89bcf8-3901-ff0f-a4b3-151312f5154b"
			b3RequestIdHeader = fmt.Sprintf(`"trace-id":"%s"`, strings.Replace(requestIdHeader, "-", "", -1))
		})
		It("fails", func() {
			status, body := RequestTracing(rep.PerformRoute, nil, bytes.NewBufferString("∆"), requestIdHeader)
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(body).To(BeEmpty())

			Expect(fakeLocalRep.PerformCallCount()).To(Equal(0))

			Eventually(logger).Should(gbytes.Say("failed-to-unmarshal"))
			Eventually(logger).Should(gbytes.Say(b3RequestIdHeader))
		})

		It("emits the failed request metric", func() {
			Request(rep.PerformRoute, nil, bytes.NewBufferString("∆"))

			Expect(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).To(Equal(0))

			Expect(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).To(Equal(1))
			calledRequestType, delta := fakeRequestMetrics.IncrementRequestsFailedCounterArgsForCall(0)
			Expect(delta).To(Equal(1))
			Expect(calledRequestType).To(Equal("Perform"))
		})
	})
})
