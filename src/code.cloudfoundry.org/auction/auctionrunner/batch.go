package auctionrunner

import (
	"sync"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/rep"

	"code.cloudfoundry.org/clock"
)

type Work struct {
	TraceID string
}

type Batch struct {
	lrpAuctions  []auctiontypes.LRPAuction
	taskAuctions []auctiontypes.TaskAuction
	lock         *sync.Mutex
	HasWork      chan Work
	clock        clock.Clock
}

func NewBatch(clock clock.Clock) *Batch {
	return &Batch{
		lrpAuctions: []auctiontypes.LRPAuction{},
		lock:        &sync.Mutex{},
		clock:       clock,
		HasWork:     make(chan Work, 1),
	}
}

func (b *Batch) AddLRPStarts(starts []auctioneer.LRPStartRequest, traceID string) {
	auctions := make([]auctiontypes.LRPAuction, 0, len(starts))
	now := b.clock.Now()
	for i := range starts {
		start := &starts[i]
		for _, index := range start.Indices {
			lrpKey := models.NewActualLRPKey(start.ProcessGuid, int32(index), start.Domain)
			auction := auctiontypes.NewLRPAuction(rep.NewLRP("", lrpKey, start.Resource, start.PlacementConstraint), now)
			auctions = append(auctions, auction)
		}
	}

	b.lock.Lock()
	b.lrpAuctions = append(b.lrpAuctions, auctions...)
	b.claimToHaveWork(traceID)
	b.lock.Unlock()
}

func (b *Batch) AddTasks(tasks []auctioneer.TaskStartRequest, traceID string) {
	auctions := make([]auctiontypes.TaskAuction, 0, len(tasks))
	now := b.clock.Now()
	for i := range tasks {
		auctions = append(auctions, auctiontypes.NewTaskAuction(tasks[i].Task, now))
	}

	b.lock.Lock()
	b.taskAuctions = append(b.taskAuctions, auctions...)
	b.claimToHaveWork(traceID)
	b.lock.Unlock()
}

func (b *Batch) DedupeAndDrain() ([]auctiontypes.LRPAuction, []auctiontypes.TaskAuction) {
	b.lock.Lock()
	lrpAuctions := b.lrpAuctions
	taskAuctions := b.taskAuctions
	b.lrpAuctions = []auctiontypes.LRPAuction{}
	b.taskAuctions = []auctiontypes.TaskAuction{}
	select {
	case <-b.HasWork:
	default:
	}
	b.lock.Unlock()

	dedupedLRPAuctions := []auctiontypes.LRPAuction{}
	presentLRPAuctions := map[string]bool{}
	for _, startAuction := range lrpAuctions {
		id := startAuction.Identifier()
		if presentLRPAuctions[id] {
			continue
		}
		presentLRPAuctions[id] = true
		dedupedLRPAuctions = append(dedupedLRPAuctions, startAuction)
	}

	dedupedTaskAuctions := []auctiontypes.TaskAuction{}
	presentTaskAuctions := map[string]bool{}
	for _, taskAuction := range taskAuctions {
		id := taskAuction.Identifier()
		if presentTaskAuctions[id] {
			continue
		}
		presentTaskAuctions[id] = true
		dedupedTaskAuctions = append(dedupedTaskAuctions, taskAuction)
	}

	return dedupedLRPAuctions, dedupedTaskAuctions
}

func (b *Batch) claimToHaveWork(traceID string) {
	select {
	case b.HasWork <- Work{TraceID: traceID}:
	default:
	}
}
