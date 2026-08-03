package harmonizer

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/operationq"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context"
	"code.cloudfoundry.org/rep/generator"
)

const repBulkSyncDuration = "RepBulkSyncDuration"

type Bulker struct {
	logger lager.Logger

	pollInterval           time.Duration
	evacuationPollInterval time.Duration
	evacuationNotifier     evacuation_context.EvacuationNotifier
	clock                  clock.Clock
	generator              generator.Generator
	queue                  operationq.Queue
	metronClient           loggingclient.IngressClient
}

func NewBulker(
	logger lager.Logger,
	pollInterval time.Duration,
	evacuationPollInterval time.Duration,
	evacuationNotifier evacuation_context.EvacuationNotifier,
	clock clock.Clock,
	generator generator.Generator,
	queue operationq.Queue,
	metronClient loggingclient.IngressClient,
) *Bulker {
	return &Bulker{
		logger: logger,

		pollInterval:           pollInterval,
		evacuationPollInterval: evacuationPollInterval,
		evacuationNotifier:     evacuationNotifier,
		clock:                  clock,
		generator:              generator,
		queue:                  queue,
		metronClient:           metronClient,
	}
}

func (b *Bulker) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	evacuateNotify := b.evacuationNotifier.EvacuateNotify()
	close(ready)

	logger := b.logger.Session("running-bulker")

	logger.Info("starting", lager.Data{
		"interval": b.pollInterval.String(),
	})
	defer logger.Info("finished")

	interval := b.pollInterval

	timer := b.clock.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-timer.C():

		case <-evacuateNotify:
			timer.Stop()
			evacuateNotify = nil

			logger.Info("notified-of-evacuation")
			interval = b.evacuationPollInterval

		case signal := <-signals:
			logger.Info("received-signal", lager.Data{"signal": signal.String()})
			return nil
		}

		b.sync(logger)
		timer.Reset(interval)
	}
}

func (b *Bulker) sync(logger lager.Logger) {
	logger = logger.Session("sync")

	logger.Info("starting")
	defer logger.Info("finished")

	startTime := b.clock.Now()

	ops, batchError := b.generator.BatchOperations(logger)

	endTime := b.clock.Now()

	sendError := b.metronClient.SendDuration(repBulkSyncDuration, endTime.Sub(startTime))
	if sendError != nil {
		logger.Error("failed-to-send-rep-bulk-sync-duration-metric", sendError)
	}

	if batchError != nil {
		logger.Error("failed-to-generate-operations", batchError)
		return
	}

	for _, operation := range ops {
		b.queue.Push(operation)
	}
}
