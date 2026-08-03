package log_streamer_test

import (
	"context"
	"time"

	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/executor/depot/log_streamer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LogRateLimiter", func() {
	It("limits logs by byte", func() {
		ctx := context.Background()
		fakeClient := &mfakes.FakeIngressClient{}
		logRateLimiter := log_streamer.NewLogRateLimiter(ctx, fakeClient, map[string]string{}, 2, 5, time.Hour)

		Expect(logRateLimiter.Limit("test", 5)).ToNot(HaveOccurred())
		Expect(logRateLimiter.Limit("test", 5)).To(MatchError("app instance exceeded log rate limit (5 bytes/sec)"))
	})
	It("limits logs by line", func() {
		ctx := context.Background()
		fakeClient := &mfakes.FakeIngressClient{}
		logRateLimiter := log_streamer.NewLogRateLimiter(ctx, fakeClient, map[string]string{}, 1, -1, time.Hour)

		Expect(logRateLimiter.Limit("test", 5)).ToNot(HaveOccurred())
		Expect(logRateLimiter.Limit("test", 5)).To(MatchError("app instance exceeded log rate limit (1 log-lines/sec) set by platform operator"))
	})
	It("cannot log if log limit by byte is 0", func() {
		ctx := context.Background()
		fakeClient := &mfakes.FakeIngressClient{}
		logRateLimiter := log_streamer.NewLogRateLimiter(ctx, fakeClient, map[string]string{}, 2, 0, time.Hour)

		Expect(logRateLimiter.Limit("test", 5)).To(MatchError("Not allowed to log"))
	})
	It("send the over limit message to loggregator", func() {
		ctx := context.Background()
		fakeClient := &mfakes.FakeIngressClient{}
		logRateLimiter := log_streamer.NewLogRateLimiter(ctx, fakeClient, map[string]string{"a": "value"}, 2, 1, time.Hour)

		Expect(logRateLimiter.Limit("test", 5)).To(HaveOccurred())

		Expect(fakeClient.SendAppLogCallCount()).To(Equal(1))
		reportMessage, sourceName, tags := fakeClient.SendAppLogArgsForCall(0)
		Expect(sourceName).To(Equal("test"))
		Expect(reportMessage).To(Equal("app instance exceeded log rate limit (1 bytes/sec)"))
		Expect(tags).To(Equal(map[string]string{"a": "value"}))
	})
	It("send the over limit counter to loggregator", func() {
		ctx := context.Background()
		fakeClient := &mfakes.FakeIngressClient{}
		logRateLimiter := log_streamer.NewLogRateLimiter(ctx, fakeClient, map[string]string{}, 2, 1, time.Hour)

		Expect(logRateLimiter.Limit("test", 5)).To(HaveOccurred())

		Expect(fakeClient.IncrementCounterCallCount()).To(Equal(1))
		counterName := fakeClient.IncrementCounterArgsForCall(0)
		Expect(counterName).To(Equal("AppInstanceExceededLogRateLimitCount"))
	})

	It("blocks messages for an entire second with only one report if limit reached", func() {
		ctx := context.Background()
		fakeClient := &mfakes.FakeIngressClient{}
		logRateLimiter := log_streamer.NewLogRateLimiter(ctx, fakeClient, map[string]string{}, 2, 100, time.Hour)

		Expect(logRateLimiter.Limit("test", 5)).ToNot(HaveOccurred())
		Expect(fakeClient.SendAppLogCallCount()).To(Equal(0))
		Expect(logRateLimiter.Limit("test", 100000)).To(HaveOccurred())
		Expect(fakeClient.SendAppLogCallCount()).To(Equal(1))
		Expect(logRateLimiter.Limit("test", 100000)).To(HaveOccurred())
		Expect(fakeClient.SendAppLogCallCount()).To(Equal(1))
		Expect(logRateLimiter.Limit("test", 5)).To(HaveOccurred())
		Expect(fakeClient.SendAppLogCallCount()).To(Equal(1))
		Expect(logRateLimiter.Limit("test", 100000)).To(HaveOccurred())
		Expect(fakeClient.SendAppLogCallCount()).To(Equal(1))
		time.Sleep(time.Millisecond * 1100)
		Expect(logRateLimiter.Limit("test", 100000)).To(HaveOccurred())
		Expect(fakeClient.SendAppLogCallCount()).To(Equal(2))
	})

	It("emits the log rate usage and limit", func() {
		ctx := context.Background()
		fakeClient := &mfakes.FakeIngressClient{}
		logRateLimiter := log_streamer.NewLogRateLimiter(ctx, fakeClient, map[string]string{"a": "value"}, 2, 30, time.Second/2)

		Expect(logRateLimiter.Limit("test", 5)).ToNot(HaveOccurred())
		Expect(logRateLimiter.Limit("test", 5)).ToNot(HaveOccurred())

		Eventually(fakeClient.SendAppLogRateCallCount).Should(Equal(1))
		logRate, logRateLimit, tags := fakeClient.SendAppLogRateArgsForCall(0)
		Expect(logRateLimit).To(BeEquivalentTo(30))
		Expect(logRate).To(BeEquivalentTo(20))
		Expect(tags).To(Equal(map[string]string{"a": "value"}))
	})

	It("stop emitting metrics when the context is done", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		fakeClient := &mfakes.FakeIngressClient{}
		logRateLimiter := log_streamer.NewLogRateLimiter(ctx, fakeClient, map[string]string{}, 2, 15, time.Millisecond)

		Expect(logRateLimiter.Limit("test", 5)).ToNot(HaveOccurred())

		Consistently(fakeClient.SendAppLogRateCallCount).Should(Equal(0))
	})

	It("does not emit metrics if the rate is zero", func() {
		ctx := context.Background()
		fakeClient := &mfakes.FakeIngressClient{}
		logRateLimiter := log_streamer.NewLogRateLimiter(ctx, fakeClient, map[string]string{}, 2, 15, 0)

		Expect(logRateLimiter.Limit("test", 5)).ToNot(HaveOccurred())

		Consistently(fakeClient.SendAppLogRateCallCount).Should(Equal(0))
	})
})
