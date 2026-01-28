package emitter

import (
	"encoding/json"
	"sync"

	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/workpool"
)

const (
	httpRouteNATSMessagesEmittedCounter     = "HTTPRouteNATSMessagesEmitted"
	internalRouteNATSMessagesEmittedCounter = "InternalRouteNATSMessagesEmitted"
)

//go:generate counterfeiter -o fakes/fake_nats_emitter.go . NATSEmitter
type NATSEmitter interface {
	Emit(messagesToEmit routingtable.MessagesToEmit) error
}

type natsEmitter struct {
	natsClient         diegonats.NATSClient
	workPool           *workpool.WorkPool
	logger             lager.Logger
	metronClient       loggingclient.IngressClient
	emitInternalRoutes bool
}

func NewNATSEmitter(natsClient diegonats.NATSClient, workPool *workpool.WorkPool, logger lager.Logger, metronClient loggingclient.IngressClient, emitInternalRoutes bool) NATSEmitter {
	return &natsEmitter{
		natsClient:         natsClient,
		workPool:           workPool,
		logger:             logger.Session("nats-emitter"),
		metronClient:       metronClient,
		emitInternalRoutes: emitInternalRoutes,
	}
}

func (n *natsEmitter) Emit(messagesToEmit routingtable.MessagesToEmit) error {
	errors := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(len(messagesToEmit.RegistrationMessages))
	for _, message := range messagesToEmit.RegistrationMessages {
		n.emit("router.register", message, &wg, errors)
	}

	wg.Add(len(messagesToEmit.UnregistrationMessages))
	for _, message := range messagesToEmit.UnregistrationMessages {
		n.emit("router.unregister", message, &wg, errors)
	}

	var numberOfInternalMessages uint64
	numberOfMessages := uint64(len(messagesToEmit.RegistrationMessages) + len(messagesToEmit.UnregistrationMessages))
	numberOfHTTPMessages := uint64(len(messagesToEmit.RegistrationMessages) + len(messagesToEmit.UnregistrationMessages))
	if n.emitInternalRoutes {
		wg.Add(len(messagesToEmit.InternalRegistrationMessages))
		for _, message := range messagesToEmit.InternalRegistrationMessages {
			n.emit("service-discovery.register", message, &wg, errors)
		}

		wg.Add(len(messagesToEmit.InternalUnregistrationMessages))
		for _, message := range messagesToEmit.InternalUnregistrationMessages {
			n.emit("service-discovery.unregister", message, &wg, errors)
		}

		numberOfInternalMessages = uint64(len(messagesToEmit.InternalRegistrationMessages) + len(messagesToEmit.InternalUnregistrationMessages))
		numberOfMessages += numberOfInternalMessages
	}

	wg.Wait()

	select {
	case finalError := <-errors:
		return finalError
	default:
	}

	err := n.metronClient.IncrementCounterWithDelta(httpRouteNATSMessagesEmittedCounter, numberOfHTTPMessages)
	if err != nil {
		n.logger.Error("cannot-emit-number-of-http-messages", err)
	}

	if n.emitInternalRoutes {
		err := n.metronClient.IncrementCounterWithDelta(internalRouteNATSMessagesEmittedCounter, numberOfInternalMessages)
		if err != nil {
			n.logger.Error("cannot-emit-number-of-internal-messages", err)
		}
	}

	return nil
}

func (n *natsEmitter) emit(subject string, message routingtable.RegistryMessage, wg *sync.WaitGroup, errors chan error) {
	n.workPool.Submit(func() {
		var err error
		defer func() {
			if err != nil {
				select {
				case errors <- err:
				default:
				}
			}
			wg.Done()
		}()

		n.logger.Debug("emit", lager.Data{
			"subject": subject,
			"message": message,
		})

		payload, err := json.Marshal(message)
		if err != nil {
			n.logger.Error("failed-to-marshal", err, lager.Data{
				"message": message,
				"subject": subject,
			})
		}

		err = n.natsClient.Publish(subject, payload)
		if err != nil {
			n.logger.Error("failed-to-publish", err, lager.Data{
				"message": message,
				"subject": subject,
			})
		}
	})
}
