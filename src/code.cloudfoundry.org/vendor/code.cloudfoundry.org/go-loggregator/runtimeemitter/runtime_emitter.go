package runtimeemitter

import (
	"runtime"
	"time"

	"code.cloudfoundry.org/go-loggregator"
)

// Emitter will emit a gauge with runtime stats via the sender on the given
// interval. default interval is 15 seconds.
type Emitter struct {
	interval time.Duration
	sender   valueSender
	// sender   Sender
}

type valueSender interface {
	send(heap, stack, gc, goroutines float64)
}

// Sender is the interface of the client that can be used to emit gauge
// metrics.
type Sender interface {
	EmitGauge(opts ...loggregator.EmitGaugeOption)
}

// RuntimeEmitterOption is the option provides configuration for an Emitter.
type RuntimeEmitterOption func(e *Emitter)

// WithInterval returns a RuntimeEmitterOption to configure the interval at
// which the runtime emitter emits gauges.
func WithInterval(d time.Duration) RuntimeEmitterOption {
	return func(e *Emitter) {
		e.interval = d
	}
}

// New returns an Emitter that is configured with the given sender and
// RuntimeEmitterOptions.
func New(sender Sender, opts ...RuntimeEmitterOption) *Emitter {
	e := &Emitter{
		sender:   v2Sender{sender: sender},
		interval: 10 * time.Second,
	}

	for _, o := range opts {
		o(e)
	}

	return e
}

// V1Sender is the interface of the v1 client that can be used to emit value
// metrics.
type V1Sender interface {
	SendComponentMetric(name string, value float64, unit string) error
}

// NewV1 returns an Emitter that is configured with the given v1 sender and
// RuntimeEmitterOptions.
func NewV1(sender V1Sender, opts ...RuntimeEmitterOption) *Emitter {
	e := &Emitter{
		sender:   v1Sender{sender: sender},
		interval: 10 * time.Second,
	}

	for _, o := range opts {
		o(e)
	}

	return e
}

// Run starts the ticker with the configured interval and emits a gauge on
// that interval. This method will block but the user may run in a go routine.
func (e *Emitter) Run() {
	for range time.Tick(e.interval) {
		memstats := &runtime.MemStats{}
		runtime.ReadMemStats(memstats)
		e.sender.send(
			float64(memstats.HeapAlloc),
			float64(memstats.StackInuse),
			float64(memstats.PauseNs[(memstats.NumGC+255)%256]),
			float64(runtime.NumGoroutine()),
		)
	}
}

type v2Sender struct {
	sender Sender
}

func (s v2Sender) send(heap, stack, gc, goroutines float64) {
	s.sender.EmitGauge(
		loggregator.WithGaugeValue("memoryStats.numBytesAllocatedHeap", heap, "Bytes"),
		loggregator.WithGaugeValue("memoryStats.numBytesAllocatedStack", stack, "Bytes"),
		loggregator.WithGaugeValue("memoryStats.lastGCPauseTimeNS", gc, "ns"),
		loggregator.WithGaugeValue("numGoRoutines", goroutines, "Count"),
	)
}

type v1Sender struct {
	sender V1Sender
}

func (s v1Sender) send(heap, stack, gc, goroutines float64) {
	s.sender.SendComponentMetric("memoryStats.numBytesAllocatedHeap", heap, "Bytes")
	s.sender.SendComponentMetric("memoryStats.numBytesAllocatedStack", stack, "Bytes")
	s.sender.SendComponentMetric("memoryStats.lastGCPauseTimeNS", gc, "ns")
	s.sender.SendComponentMetric("numGoRoutines", goroutines, "Count")
}
