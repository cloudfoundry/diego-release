package evacuation_context

import "sync"

//go:generate counterfeiter -o fake_evacuation_context/fake_evacuatable.go . Evacuatable
type Evacuatable interface {
	Evacuate()
}

//go:generate counterfeiter -o fake_evacuation_context/fake_evacuation_reporter.go . EvacuationReporter
type EvacuationReporter interface {
	Evacuating() bool
}

//go:generate counterfeiter -o fake_evacuation_context/fake_evacuation_notifier.go . EvacuationNotifier
type EvacuationNotifier interface {
	EvacuateNotify() <-chan struct{}
}

type evacuationContext struct {
	evacuated chan struct{}
	mu        sync.Mutex
}

func New() (Evacuatable, EvacuationReporter, EvacuationNotifier) {
	evacuationContext := &evacuationContext{
		evacuated: make(chan struct{}),
	}

	return evacuationContext, evacuationContext, evacuationContext
}

func (e *evacuationContext) Evacuate() {
	e.mu.Lock()
	defer e.mu.Unlock()

	select {
	case <-e.evacuated:
	default:
		close(e.evacuated)
	}
}

func (e *evacuationContext) Evacuating() bool {
	select {
	case <-e.evacuated:
		return true
	default:
		return false
	}
}

func (e *evacuationContext) EvacuateNotify() <-chan struct{} {
	return e.evacuated
}
