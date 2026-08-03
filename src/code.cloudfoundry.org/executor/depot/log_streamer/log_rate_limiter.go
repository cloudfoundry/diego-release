package log_streamer

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"golang.org/x/time/rate"
)

const (
	AppInstanceExceededLogRateLimitCount = "AppInstanceExceededLogRateLimitCount"
	LogRateLimitExceededLogInterval      = time.Second
)

// A logRateLimiter is used by the streamDestination to limit logs from an app instance.
// This can be done by limiting the number of lines per second,
// or by limiting the number of bytes per second.
type logRateLimiter struct {
	ctx          context.Context
	metronClient loggingclient.IngressClient

	maxLogLinesPerSecond        int
	maxLogBytesPerSecond        int64
	maxLogLinesPerSecondLimiter *rate.Limiter
	maxLogBytesPerSecondLimiter *rate.Limiter
	logMetricsEmitInterval      time.Duration
	lastOverage                 time.Time
	bytesEmittedLastInterval    uint64
	tags                        map[string]string
}

func NewLogRateLimiter(
	ctx context.Context,
	metronClient loggingclient.IngressClient,
	tags map[string]string,
	maxLogLinesPerSecond int,
	maxLogBytesPerSecond int64,
	logMetricsEmitInterval time.Duration,
) *logRateLimiter {
	limiter := &logRateLimiter{
		ctx:                      ctx,
		metronClient:             metronClient,
		maxLogLinesPerSecond:     maxLogLinesPerSecond,
		maxLogBytesPerSecond:     maxLogBytesPerSecond,
		logMetricsEmitInterval:   logMetricsEmitInterval,
		bytesEmittedLastInterval: 0,
		tags:                     tags,
		lastOverage:              time.Now().Add(-time.Second),
	}

	if maxLogLinesPerSecond > 0 {
		limiter.maxLogLinesPerSecondLimiter = rate.NewLimiter(rate.Limit(maxLogLinesPerSecond), maxLogLinesPerSecond)
	} else {
		limiter.maxLogLinesPerSecondLimiter = rate.NewLimiter(rate.Inf, 0)
	}
	if limiter.maxLogBytesPerSecond > -1 {
		limiter.maxLogBytesPerSecondLimiter = rate.NewLimiter(rate.Limit(maxLogBytesPerSecond), int(maxLogBytesPerSecond))
	} else {
		limiter.maxLogBytesPerSecondLimiter = rate.NewLimiter(rate.Inf, 0)
	}
	go limiter.emitMetrics()
	return limiter
}

// Limit is called before logging to determine if the log should be dropped (returns err) or logged (returns nil).
func (r *logRateLimiter) Limit(sourceName string, logLength int) error {
	if r.maxLogBytesPerSecond == 0 {
		return fmt.Errorf("Not allowed to log")
	}
	if r.lastOverage.Add(time.Second).After(time.Now()) {
		return fmt.Errorf("timeout for overage")
	}

	if !r.maxLogBytesPerSecondLimiter.AllowN(time.Now(), logLength) {
		reportMessage := fmt.Sprintf("app instance exceeded log rate limit (%d bytes/sec)", r.maxLogBytesPerSecond)
		r.reportOverlimit(sourceName, reportMessage)
		r.lastOverage = time.Now()
		return errors.New(reportMessage)
	}

	if !r.maxLogLinesPerSecondLimiter.Allow() {
		reportMessage := fmt.Sprintf("app instance exceeded log rate limit (%d log-lines/sec) set by platform operator", r.maxLogLinesPerSecond)
		r.reportOverlimit(sourceName, reportMessage)
		r.lastOverage = time.Now()
		return errors.New(reportMessage)
	}

	atomic.AddUint64(&r.bytesEmittedLastInterval, uint64(logLength))
	return nil
}

func (r *logRateLimiter) emitMetrics() {
	if r.logMetricsEmitInterval <= 0 {
		return
	}
	t := time.NewTicker(r.logMetricsEmitInterval)
	defer t.Stop()
	intervalDivider := r.logMetricsEmitInterval.Seconds()
	for {
		select {
		case <-t.C:
			lastIntervalEmitted := atomic.SwapUint64(&r.bytesEmittedLastInterval, 0)
			perSecondValue := float64(lastIntervalEmitted) / intervalDivider
			// #nosec G104 - ignore error returned by sending metric here
			r.metronClient.SendAppLogRate(perSecondValue, float64(r.maxLogBytesPerSecond), r.tags)
		case <-r.ctx.Done():
			return
		}
	}
}

func (r *logRateLimiter) reportOverlimit(sourceName string, reportMessage string) {
	r.reportLogRateLimitExceededMetric()
	r.reportLogRateLimitExceededLog(sourceName, reportMessage)
}

func (r *logRateLimiter) reportLogRateLimitExceededMetric() {
	_ = r.metronClient.IncrementCounter(AppInstanceExceededLogRateLimitCount)
}

func (r *logRateLimiter) reportLogRateLimitExceededLog(sourceName string, reportMessage string) {
	_ = r.metronClient.SendAppLog(reportMessage, sourceName, r.tags)
}
