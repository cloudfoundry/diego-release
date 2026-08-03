package scheduler

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/route-emitter/cmd/route-emitter/config"
	"code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"github.com/nats-io/nats.go"
	uuid "github.com/nu7hatch/gouuid"
)

type RouteBroadcastScheduler struct {
	natsClient           diegonats.NATSClient
	externalServiceName  string
	clock                clock.Clock
	emitCh               chan struct{}
	externalServiceStart chan time.Duration
	cfg                  *config.RouteEmitterConfig
	logger               lager.Logger
	MaxGreetAttempts     int
}

func NewRouteBroadcastScheduler(
	clock clock.Clock,
	natsClient diegonats.NATSClient,
	logger lager.Logger,
	externalServiceName string,
	cfg *config.RouteEmitterConfig,
	emitCh chan struct{},
) *RouteBroadcastScheduler {
	return &RouteBroadcastScheduler{
		natsClient:          natsClient,
		externalServiceName: externalServiceName,

		clock:  clock,
		emitCh: emitCh,

		externalServiceStart: make(chan time.Duration),
		cfg:                  cfg,

		logger:           logger.Session("route-broadcast-scheduler", lager.Data{"name": externalServiceName}),
		MaxGreetAttempts: 30,
	}
}

func (s *RouteBroadcastScheduler) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	s.logger.Info("starting")
	replyUuid, err := uuid.NewV4()
	if err != nil {
		return err
	}

	err = s.listenForExternalService(replyUuid.String())
	if err != nil {
		return err
	}

	var registerInterval time.Duration
	retryGreetingTicker := s.clock.NewTicker(time.Second)

	//keep trying to greet until we hear from the external service
	greetAttempt := 0
GREET_LOOP:
	for {
		greetAttempt += 1
		s.logger.Info("greeting-external-service")
		err := s.greetExternalService(replyUuid.String())

		if err != nil {
			s.logger.Error("failed-to-greet-external-service", err)
			return err
		}

		if greetAttempt == s.MaxGreetAttempts {
			err = fmt.Errorf("attempted to greet '%v' %d times without success", s.externalServiceName, s.MaxGreetAttempts)
			s.logger.Error("greeting-external-service.giving-up", err)
			return err
		}

		select {
		case registerInterval = <-s.externalServiceStart:
			s.logger.Info("received-external-service-registry-interval", lager.Data{"interval": registerInterval.String()})
			break GREET_LOOP
		case <-retryGreetingTicker.C():
			s.logger.Info("retrying")
		case <-signals:
			s.logger.Info("stopping")
			return nil
		}
	}
	retryGreetingTicker.Stop()

	close(ready)
	s.logger.Info("started")

	// now keep emitting at the desired interval
	emitTicker := s.clock.NewTicker(registerInterval)

	randSource := rand.New(rand.NewSource(time.Now().UnixNano()))
	s.logger.Info("for loop")
	for {
		select {
		case registerInterval = <-s.externalServiceStart:
			s.logger.Info("received-new-external-service-prune-interval", lager.Data{"interval": registerInterval.String()})
			jitterInterval := randSource.Int63n(int64(s.cfg.JitterFactor * float64(registerInterval)))
			s.clock.Sleep(time.Duration(jitterInterval))
			emitTicker.Stop()
			emitTicker = s.clock.NewTicker(registerInterval)
			s.emit()
		case <-emitTicker.C():
			s.logger.Info("emitting-routes")
			s.emit()
		case <-signals:
			s.logger.Info("stopping")
			emitTicker.Stop()
			return nil
		}
	}
}

func (s *RouteBroadcastScheduler) emit() {
	select {
	case s.emitCh <- struct{}{}:
	default:
		s.logger.Debug("emit-already-in-progress")
	}
}

func (s *RouteBroadcastScheduler) listenForExternalService(replyUUID string) error {
	_, err := s.natsClient.Subscribe(fmt.Sprintf("%s.start", s.externalServiceName), s.handleExternalServiceStart)
	if err != nil {
		return err
	}

	sub, err := s.natsClient.Subscribe(replyUUID, s.handleExternalServiceStart)
	if err != nil {
		return err
	}
	err = sub.AutoUnsubscribe(1)
	if err != nil {
		s.logger.Debug("error-auto-unsubscribing", lager.Data{"error": err})
	}
	return nil
}

func (s *RouteBroadcastScheduler) greetExternalService(replyUUID string) error {
	err := s.natsClient.PublishRequest(fmt.Sprintf("%s.greet", s.externalServiceName), replyUUID, []byte{})
	if err != nil {
		return err
	}

	return nil
}

func (s *RouteBroadcastScheduler) handleExternalServiceStart(msg *nats.Msg) {
	var response routingtable.ExternalServiceGreetingMessage

	err := json.Unmarshal(msg.Data, &response)
	if err != nil {
		s.logger.Error("received-invalid-external-service-start", err, lager.Data{
			"payload": msg.Data,
		})
		return
	}

	greetInterval := response.MinimumRegisterInterval
	s.externalServiceStart <- time.Duration(greetInterval) * time.Second
}

func (s *RouteBroadcastScheduler) EmitCh() chan struct{} {
	return s.emitCh
}
