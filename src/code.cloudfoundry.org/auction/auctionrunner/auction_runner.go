package auctionrunner

import (
	"os"
	"time"

	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/v3"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/workpool"
)

type auctionRunner struct {
	logger lager.Logger

	delegate                      auctiontypes.AuctionRunnerDelegate
	metricEmitter                 auctiontypes.AuctionMetricEmitterDelegate
	batch                         *Batch
	clock                         clock.Clock
	workPool                      *workpool.WorkPool
	binPackFirstFitWeight         float64
	startingContainerWeight       float64
	startingContainerCountMaximum int
}

func New(
	logger lager.Logger,
	delegate auctiontypes.AuctionRunnerDelegate,
	metricEmitter auctiontypes.AuctionMetricEmitterDelegate,
	clock clock.Clock,
	workPool *workpool.WorkPool,
	binPackFirstFitWeight float64,
	startingContainerWeight float64,
	startingContainerCountMaximum int,
) *auctionRunner {
	return &auctionRunner{
		logger:                        logger,
		delegate:                      delegate,
		metricEmitter:                 metricEmitter,
		batch:                         NewBatch(clock),
		clock:                         clock,
		workPool:                      workPool,
		binPackFirstFitWeight:         binPackFirstFitWeight,
		startingContainerWeight:       startingContainerWeight,
		startingContainerCountMaximum: startingContainerCountMaximum,
	}
}

func (a *auctionRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	var hasWork chan Work
	hasWork = a.batch.HasWork

	for {
		select {
		case work := <-hasWork:
			logger := trace.LoggerWithTraceInfo(a.logger, work.TraceID).Session("auction")

			logger.Info("fetching-cell-reps")
			clients, err := a.delegate.FetchCellReps(logger, work.TraceID)
			if err != nil {
				logger.Error("failed-to-fetch-reps", err)
				time.Sleep(time.Second)
				hasWork = make(chan Work, 1)
				hasWork <- work
				break
			}
			logger.Info("fetched-cell-reps", lager.Data{"cell-reps-count": len(clients)})

			hasWork = a.batch.HasWork

			logger.Info("fetching-zone-state")
			fetchStatesStartTime := time.Now()
			zones := FetchStateAndBuildZones(logger, a.workPool, clients, a.metricEmitter, a.binPackFirstFitWeight)
			fetchStateDuration := time.Since(fetchStatesStartTime)
			err = a.metricEmitter.FetchStatesCompleted(fetchStateDuration)
			if err != nil {
				logger.Error("failed-sending-fetch-states-completed-metric", err)
			}

			cellCount := 0
			for zone, cells := range zones {
				logger.Info("zone-state", lager.Data{"zone": zone, "cell-count": len(cells)})
				cellCount += len(cells)
			}
			logger.Info("fetched-zone-state", lager.Data{
				"cell-state-count":    cellCount,
				"num-failed-requests": len(clients) - cellCount,
				"duration":            fetchStateDuration.String(),
			})

			logger.Info("fetching-auctions")
			lrpAuctions, taskAuctions := a.batch.DedupeAndDrain()
			logger.Info("fetched-auctions", lager.Data{
				"lrp-start-auctions": len(lrpAuctions),
				"task-auctions":      len(taskAuctions),
			})
			if len(lrpAuctions) == 0 && len(taskAuctions) == 0 {
				logger.Info("nothing-to-auction")
				break
			}

			logger.Info("scheduling")
			auctionRequest := auctiontypes.AuctionRequest{
				LRPs:  lrpAuctions,
				Tasks: taskAuctions,
			}

			scheduler := NewScheduler(a.workPool, zones, a.clock, logger, a.binPackFirstFitWeight, a.startingContainerWeight, a.startingContainerCountMaximum)
			auctionResults := scheduler.Schedule(auctionRequest)
			logger.Info("scheduled", lager.Data{
				"successful-lrp-start-auctions": len(auctionResults.SuccessfulLRPs),
				"successful-task-auctions":      len(auctionResults.SuccessfulTasks),
				"failed-lrp-start-auctions":     len(auctionResults.FailedLRPs),
				"failed-task-auctions":          len(auctionResults.FailedTasks),
			})

			err = a.metricEmitter.AuctionCompleted(auctionResults)
			if err != nil {
				logger.Debug("failed-emitting-auction-complete-metrics", lager.Data{"error": err})
			}
			a.delegate.AuctionCompleted(logger, work.TraceID, auctionResults)
		case <-signals:
			return nil
		}
	}
}

func (a *auctionRunner) ScheduleLRPsForAuctions(lrpStarts []auctioneer.LRPStartRequest, traceID string) {
	a.batch.AddLRPStarts(lrpStarts, traceID)
}

func (a *auctionRunner) ScheduleTasksForAuctions(tasks []auctioneer.TaskStartRequest, traceID string) {
	a.batch.AddTasks(tasks, traceID)
}
