package log_streamer

import (
	"context"
	"io"
	"time"

	"code.cloudfoundry.org/bbs/models"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
)

const (
	MAX_MESSAGE_SIZE = 61440

	DefaultLogSource = "LOG"
)

//go:generate counterfeiter -o fake_log_streamer/fake_log_streamer.go . LogStreamer
type LogStreamer interface {
	Stdout() io.Writer
	Stderr() io.Writer

	UpdateTags(map[string]string)
	Flush()

	WithSource(sourceName string) LogStreamer
	SourceName() string

	Stop()
}

type logStreamer struct {
	ctx        context.Context
	cancelFunc context.CancelFunc
	stdout     *streamDestination
	stderr     *streamDestination
}

func New(config models.LogConfig, metronClient loggingclient.IngressClient, maxLogLinesPerSecond int, maxLogBytesPerSecond int64, metricReportInterval time.Duration) LogStreamer {
	if config.Guid == "" {
		return noopStreamer{}
	}

	sourceName, tags := config.GetSourceNameAndTagsForLogging()

	ctx, cancelFunc := context.WithCancel(context.Background())
	logRateLimiter := NewLogRateLimiter(ctx, metronClient, tags, maxLogLinesPerSecond, maxLogBytesPerSecond, metricReportInterval)

	return &logStreamer{
		ctx:        ctx,
		cancelFunc: cancelFunc,
		stdout: newStreamDestination(
			ctx,
			sourceName,
			tags,
			loggregator_v2.Log_OUT,
			metronClient,
			logRateLimiter,
		),

		stderr: newStreamDestination(
			ctx,
			sourceName,
			tags,
			loggregator_v2.Log_ERR,
			metronClient,
			logRateLimiter,
		),
	}
}

func (e *logStreamer) Stdout() io.Writer {
	return e.stdout
}

func (e *logStreamer) Stderr() io.Writer {
	return e.stderr
}

func (e *logStreamer) UpdateTags(tags map[string]string) {
	e.stdout.updateTags(tags)
	e.stderr.updateTags(tags)
}

func (e *logStreamer) Flush() {
	e.stdout.lockAndFlush()
	e.stderr.lockAndFlush()
}

func (e *logStreamer) WithSource(sourceName string) LogStreamer {
	if sourceName == "" {
		sourceName = e.SourceName()
	}

	ctx, cancelFunc := context.WithCancel(e.ctx)

	return &logStreamer{
		ctx:        ctx,
		cancelFunc: cancelFunc,
		stdout:     e.stdout.withSource(ctx, sourceName),
		stderr:     e.stderr.withSource(ctx, sourceName),
	}
}

func (e *logStreamer) SourceName() string {
	return e.stdout.sourceName
}

func (e *logStreamer) Stop() {
	e.cancelFunc()
}
