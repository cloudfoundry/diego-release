package harmonizer

import (
	"os"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/operationq"
	"code.cloudfoundry.org/rep/generator"
)

type EventConsumer struct {
	logger    lager.Logger
	generator generator.Generator
	queue     operationq.Queue
}

func NewEventConsumer(
	logger lager.Logger,
	generator generator.Generator,
	queue operationq.Queue,
) *EventConsumer {
	return &EventConsumer{
		logger:    logger,
		generator: generator,
		queue:     queue,
	}
}

func (consumer *EventConsumer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := consumer.logger.Session("event-consumer")
	logger.Info("starting")
	defer logger.Info("finished")

	stream, err := consumer.generator.OperationStream(consumer.logger)
	if err != nil {
		logger.Error("failed-subscribing-to-operation-stream", err)
		return err
	}

	close(ready)
	logger.Info("started")

	for {
		select {
		case op, ok := <-stream:
			if !ok {
				logger.Info("event-stream-closed")
				return nil
			}

			consumer.queue.Push(op)

		case signal := <-signals:
			logger.Info("received-signal", lager.Data{"signal": signal.String()})
			return nil
		}
	}
}
