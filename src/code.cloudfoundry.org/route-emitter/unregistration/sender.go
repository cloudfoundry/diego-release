package unregistration

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/route-emitter/emitter"
	"code.cloudfoundry.org/route-emitter/routingtable"
)

type Sender struct {
	logger      lager.Logger
	clock       clock.Clock
	cache       Cache
	natsEmitter emitter.NATSEmitter
	interval    time.Duration
	sendCount   int
}

func NewSender(
	logger lager.Logger,
	clock clock.Clock,
	cache Cache,
	natsEmitter emitter.NATSEmitter,
	interval time.Duration,
	sendCount int,
) Sender {
	return Sender{
		logger:      logger.Session("unregistration-sender"),
		clock:       clock,
		cache:       cache,
		natsEmitter: natsEmitter,
		interval:    interval,
		sendCount:   sendCount,
	}
}

func (s Sender) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	s.logger.Info("starting")
	close(ready)
	defer s.logger.Info("exiting")

	sendTicker := s.clock.NewTicker(s.interval)
	defer sendTicker.Stop()

	for {
		select {
		case <-signals:
			s.logger.Info("stopping")
			return nil

		case <-sendTicker.C():
			messages := s.cache.List()
			if len(messages) > 0 {
				s.logger.Debug("messages", lager.Data{"cache": messages})
			}
			for _, message := range messages {
				err := s.natsEmitter.Emit(routingtable.MessagesToEmit{
					UnregistrationMessages: []routingtable.RegistryMessage{message.RegistryMessage},
				})
				if err != nil {
					s.logger.Error("error-emitting-route-unregistration-messages", err)
				}
				message.SentCount++
				if message.SentCount == s.sendCount {
					err := s.cache.Remove([]routingtable.RegistryMessage{message.RegistryMessage})
					if err != nil {
						s.logger.Error("error-removing-unregister-messages-from-cache", err)
					}
				}
			}
		}
	}
}
