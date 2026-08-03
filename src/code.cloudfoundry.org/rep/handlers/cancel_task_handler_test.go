package handlers_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/rep"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("CancelTask", func() {
	var (
		params            map[string]string
		requestIdHeader   string
		b3RequestIdHeader string
	)
	// var server *httptest.Server
	BeforeEach(func() {
		requestIdHeader = "fa89bcf8-3607-419f-a4b3-151312f5154b"
		b3RequestIdHeader = fmt.Sprintf(`"trace-id":"%s"`, strings.Replace(requestIdHeader, "-", "", -1))
		params = map[string]string{"task_guid": "some-guid"}
		// server = httptest.NewServer(exportedHandler)
	})

	Context("when the container deletion succeeds", func() {
		BeforeEach(func() {
			fakeExecutorClient.DeleteContainerReturns(nil)
		})

		It("responds with Accepted status", func() {
			status, _ := RequestTracing(rep.CancelTaskRoute, params, nil, requestIdHeader)

			Expect(status).To(Equal(http.StatusAccepted))

			Eventually(fakeExecutorClient.DeleteContainerCallCount).Should(Equal(1))
			_, traceID, taskGuidArg := fakeExecutorClient.DeleteContainerArgsForCall(0)
			Expect(traceID).To(Equal(requestIdHeader))
			Expect(taskGuidArg).To(Equal("some-guid"))
		})

		It("emits request metrics", func() {
			RequestTracing(rep.CancelTaskRoute, params, nil, requestIdHeader)
			Eventually(logger).Should(gbytes.Say("deleting-container"))
			Eventually(logger).Should(gbytes.Say(b3RequestIdHeader))

			Expect(fakeRequestMetrics.IncrementRequestsStartedCounterCallCount()).To(Equal(1))
			calledRequestType, delta := fakeRequestMetrics.IncrementRequestsStartedCounterArgsForCall(0)
			Expect(delta).To(Equal(1))
			Expect(calledRequestType).To(Equal("CancelTask"))

			Expect(fakeRequestMetrics.IncrementRequestsInFlightCounterCallCount()).To(Equal(1))
			calledRequestType, delta = fakeRequestMetrics.IncrementRequestsInFlightCounterArgsForCall(0)
			Expect(delta).To(Equal(1))
			Expect(calledRequestType).To(Equal("CancelTask"))

			Expect(fakeRequestMetrics.DecrementRequestsInFlightCounterCallCount()).To(Equal(1))
			calledRequestType, delta = fakeRequestMetrics.DecrementRequestsInFlightCounterArgsForCall(0)
			Expect(delta).To(Equal(1))
			Expect(calledRequestType).To(Equal("CancelTask"))

			Expect(fakeRequestMetrics.UpdateLatencyCallCount()).To(Equal(1))
			calledRequestType, _ = fakeRequestMetrics.UpdateLatencyArgsForCall(0)
			Expect(calledRequestType).To(Equal("CancelTask"))

			Consistently(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).Should(Equal(0))

			Eventually(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).Should(Equal(1))
			calledRequestType, delta = fakeRequestMetrics.IncrementRequestsSucceededCounterArgsForCall(0)
			Expect(delta).To(Equal(1))
			Expect(calledRequestType).To(Equal("CancelTask"))
		})
	})

	Context("when the container does not exist", func() {
		BeforeEach(func() {
			fakeExecutorClient.DeleteContainerReturns(executor.ErrContainerNotFound)
		})

		It("responds with Accepted status", func() {
			status, _ := RequestTracing(rep.CancelTaskRoute, params, nil, requestIdHeader)

			Expect(status).To(Equal(http.StatusAccepted))

			Eventually(fakeExecutorClient.DeleteContainerCallCount).Should(Equal(1))
			_, traceID, taskGuidArg := fakeExecutorClient.DeleteContainerArgsForCall(0)
			Expect(traceID).To(Equal(requestIdHeader))
			Expect(taskGuidArg).To(Equal("some-guid"))

			Eventually(logger).Should(gbytes.Say("cancel-task"))
			Eventually(logger).Should(gbytes.Say("deleting-container"))
			Eventually(logger).Should(gbytes.Say("container-not-found"))
			Eventually(logger).Should(gbytes.Say(b3RequestIdHeader))
		})

		It("emits success request metric", func() {
			Request(rep.CancelTaskRoute, params, nil)

			Consistently(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).Should(Equal(0))

			Eventually(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).Should(Equal(1))
			calledRequestType, delta := fakeRequestMetrics.IncrementRequestsSucceededCounterArgsForCall(0)
			Expect(delta).To(Equal(1))
			Expect(calledRequestType).To(Equal("CancelTask"))
		})
	})

	Context("when the container deletion fails", func() {
		BeforeEach(func() {
			fakeExecutorClient.DeleteContainerReturns(errors.New("uh-oh"))
		})

		It("responds with Accepted status", func() {
			status, _ := RequestTracing(rep.CancelTaskRoute, params, nil, requestIdHeader)
			Expect(status).To(Equal(http.StatusAccepted))

			Eventually(fakeExecutorClient.DeleteContainerCallCount).Should(Equal(1))
			_, traceID, taskGuidArg := fakeExecutorClient.DeleteContainerArgsForCall(0)
			Expect(traceID).To(Equal(requestIdHeader))
			Expect(taskGuidArg).To(Equal("some-guid"))
		})

		It("emits failed request metric", func() {
			RequestTracing(rep.CancelTaskRoute, params, nil, requestIdHeader)

			Consistently(fakeRequestMetrics.IncrementRequestsSucceededCounterCallCount()).Should(Equal(0))

			Eventually(fakeRequestMetrics.IncrementRequestsFailedCounterCallCount()).Should(Equal(1))
			calledRequestType, delta := fakeRequestMetrics.IncrementRequestsFailedCounterArgsForCall(0)
			Expect(delta).To(Equal(1))
			Expect(calledRequestType).To(Equal("CancelTask"))

			Eventually(logger).Should(gbytes.Say("deleting-container"))
			Eventually(logger).Should(gbytes.Say("failed-deleting-container"))
			Eventually(logger).Should(gbytes.Say(b3RequestIdHeader))
		})
	})
})
