package log_streamer

import (
	"context"
	"sync"
	"unicode/utf8"

	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
)

type streamDestination struct {
	ctx            context.Context
	sourceName     string
	tags           map[string]string
	messageType    loggregator_v2.Log_Type
	buffer         []byte
	processLock    sync.Mutex
	metronClient   loggingclient.IngressClient
	logRateLimiter *logRateLimiter
}

func newStreamDestination(
	ctx context.Context,
	sourceName string,
	tags map[string]string,
	messageType loggregator_v2.Log_Type,
	metronClient loggingclient.IngressClient,
	limiter *logRateLimiter,
) *streamDestination {
	return &streamDestination{
		ctx:            ctx,
		sourceName:     sourceName,
		tags:           tags,
		messageType:    messageType,
		buffer:         make([]byte, 0, MAX_MESSAGE_SIZE),
		metronClient:   metronClient,
		logRateLimiter: limiter,
	}
}

func (destination *streamDestination) updateTags(tags map[string]string) {
	destination.processLock.Lock()
	defer destination.processLock.Unlock()

	for k := range destination.tags {
		delete(destination.tags, k)
	}

	for k, v := range tags {
		destination.tags[k] = v
	}
}

func (destination *streamDestination) lockAndFlush() {
	destination.processLock.Lock()
	defer destination.processLock.Unlock()
	destination.flush()
}

func (destination *streamDestination) Write(data []byte) (int, error) {
	destination.processMessage(string(data))
	return len(data), nil
}

func (destination *streamDestination) flush() {
	msg := destination.copyAndResetBuffer()

	if len(msg) > 0 {
		if destination.logRateLimiter.Limit(destination.sourceName, len(msg)) != nil {
			return
		}
		switch destination.messageType {
		case loggregator_v2.Log_OUT:
			_ = destination.metronClient.SendAppLog(string(msg), destination.sourceName, destination.tags)
		case loggregator_v2.Log_ERR:
			_ = destination.metronClient.SendAppErrorLog(string(msg), destination.sourceName, destination.tags)
		}
	}
}

// Not thread safe.  should only be called when holding the processLock
func (destination *streamDestination) copyAndResetBuffer() []byte {
	if len(destination.buffer) > 0 {
		msg := make([]byte, len(destination.buffer))
		copy(msg, destination.buffer)

		destination.buffer = destination.buffer[:0]
		return msg
	}

	return []byte{}
}

func (destination *streamDestination) processMessage(message string) {
	start := 0
	for i, ch := range message {
		if ch == '\n' || ch == '\r' {
			destination.processString(message[start:i], true)
			start = i + 1
		}
	}

	if start < len(message) {
		destination.processString(message[start:], false)
	}
}

func (destination *streamDestination) processString(message string, terminates bool) {
	destination.processLock.Lock()
	defer destination.processLock.Unlock()

	for {
		message = destination.appendToBuffer(message)
		if len(message) == 0 {
			break
		}
		destination.flush()
	}

	if terminates {
		destination.flush()
	}
}

// Not thread safe.  should only be called when holding the processLock
func (destination *streamDestination) appendToBuffer(message string) string {
	if len(message)+len(destination.buffer) >= MAX_MESSAGE_SIZE {
		remainingSpaceInBuffer := MAX_MESSAGE_SIZE - len(destination.buffer)
		destination.buffer = append(destination.buffer, []byte(message[0:remainingSpaceInBuffer])...)

		r, _ := utf8.DecodeLastRune(destination.buffer[0:len(destination.buffer)])

		// if we error initially, go back to preserve utf8 boundaries
		bytesToCut := 0
		for r == utf8.RuneError && bytesToCut < 3 {
			bytesToCut++
			r, _ = utf8.DecodeLastRune(destination.buffer[0 : len(destination.buffer)-bytesToCut])
		}

		index := remainingSpaceInBuffer - bytesToCut
		if index < 0 {
			index = 0
			destination.buffer = destination.buffer[0 : len(destination.buffer)-remainingSpaceInBuffer]
		} else {
			destination.buffer = destination.buffer[0 : len(destination.buffer)-bytesToCut]
		}

		return message[index:]
	}

	destination.buffer = append(destination.buffer, []byte(message)...)
	return ""
}

func (d *streamDestination) withSource(ctx context.Context, sourceName string) *streamDestination {
	return &streamDestination{
		ctx:            ctx,
		sourceName:     sourceName,
		tags:           d.tags,
		messageType:    d.messageType,
		buffer:         make([]byte, 0, MAX_MESSAGE_SIZE),
		metronClient:   d.metronClient,
		logRateLimiter: d.logRateLimiter,
	}
}
