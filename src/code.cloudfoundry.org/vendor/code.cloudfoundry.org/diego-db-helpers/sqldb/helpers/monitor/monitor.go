package monitor

import (
	"database/sql"
	"sync"
	"sync/atomic"
	"time"
)

//go:generate counterfeiter -generate

//counterfeiter:generate . Monitor
type Monitor interface {
	Monitor(func() error) error

	Total() int64
	Succeeded() int64
	Failed() int64

	ReadAndResetDurationMax() time.Duration
	ReadAndResetInFlightMax() int64
}

type monitor struct {
	total     int64
	succeeded int64
	failed    int64

	durationLock *sync.RWMutex
	durationMax  time.Duration

	inFlightLock *sync.RWMutex
	inFlight     int64
	inFlightMax  int64
}

func New() Monitor {
	return &monitor{
		durationLock: new(sync.RWMutex),
		inFlightLock: new(sync.RWMutex),
	}
}

func (m *monitor) Monitor(f func() error) error {
	m.updateInFlight(1)
	defer m.updateInFlight(-1)

	start := time.Now()
	err := f()
	m.setDurationMax(time.Since(start))

	if err != nil && err != sql.ErrNoRows {
		if err != sql.ErrTxDone {
			atomic.AddInt64(&m.total, 1)
			atomic.AddInt64(&m.failed, 1)
		}
	} else {
		atomic.AddInt64(&m.total, 1)
		atomic.AddInt64(&m.succeeded, 1)
	}

	return err
}

func (m *monitor) Total() int64 {
	return atomic.LoadInt64(&m.total)
}

func (m *monitor) Succeeded() int64 {
	return atomic.LoadInt64(&m.succeeded)
}

func (m *monitor) Failed() int64 {
	return atomic.LoadInt64(&m.failed)
}

func (m *monitor) ReadAndResetInFlightMax() int64 {
	var oldMax int64
	m.inFlightLock.Lock()
	oldMax = m.inFlightMax
	m.inFlightMax = m.inFlight
	m.inFlightLock.Unlock()
	return oldMax
}

func (m *monitor) updateInFlight(delta int64) {
	m.inFlightLock.Lock()
	m.inFlight += delta
	if m.inFlight > m.inFlightMax {
		m.inFlightMax = m.inFlight
	}
	m.inFlightLock.Unlock()
}

func (m *monitor) ReadAndResetDurationMax() time.Duration {
	var oldDuration time.Duration
	m.durationLock.Lock()
	oldDuration = m.durationMax
	m.durationMax = 0
	m.durationLock.Unlock()
	return oldDuration
}

func (m *monitor) setDurationMax(d time.Duration) {
	m.durationLock.Lock()
	if d > m.durationMax {
		m.durationMax = d
	}
	m.durationLock.Unlock()
}
