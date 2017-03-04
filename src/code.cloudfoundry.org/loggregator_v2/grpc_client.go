package loggregator_v2

import (
	"context"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry/sonde-go/events"
)

//go:generate counterfeiter -o fakes/fake_ingress_server.go . IngressServer
//go:generate counterfeiter -o fakes/fake_ingress_sender_server.go . Ingress_SenderServer

type envelopeWithResponseChannel struct {
	envelope *Envelope
	errCh    chan error
}

type Connector func() (IngressClient, error)

type grpcClient struct {
	logger        lager.Logger
	ingressClient IngressClient
	sender        Ingress_SenderClient
	envelopes     chan *envelopeWithResponseChannel
	connector     Connector
}

func NewGrpcClient(logger lager.Logger, ingressClient IngressClient) *grpcClient {
	client := &grpcClient{
		logger: logger.Session("grpc-client"),
		// connector: connector,
		ingressClient: ingressClient,
		envelopes:     make(chan *envelopeWithResponseChannel),
	}
	go client.start()
	return client
}

func (c *grpcClient) start() {
	for {
		envelopeWithResponseChannel := <-c.envelopes
		envelope := envelopeWithResponseChannel.envelope
		errCh := envelopeWithResponseChannel.errCh
		if c.sender == nil {
			var err error
			c.sender, err = c.ingressClient.Sender(context.Background())
			if err != nil {
				c.logger.Error("failed-to-create-grpc-sender", err)
				errCh <- err
				continue
			}
		}
		err := c.sender.Send(envelope)
		if err != nil {
			c.sender = nil
		}
		errCh <- err
	}
}

func createLogEnvelope(appID, message, sourceType, sourceInstance string, logType Log_Type) *Envelope {
	return &Envelope{
		Timestamp: int64(time.Now().UnixNano()),
		SourceId:  appID,
		Message: &Envelope_Log{
			Log: &Log{
				Payload: []byte(message),
				Type:    logType,
			},
		},
		Tags: map[string]*Value{
			"source_type": &Value{
				Data: &Value_Text{
					Text: sourceType,
				},
			},
			"source_instance": &Value{
				Data: &Value_Text{
					Text: sourceInstance,
				},
			},
		},
	}
}

func (c *grpcClient) send(envelope *Envelope) error {
	e := &envelopeWithResponseChannel{
		envelope: envelope,
		errCh:    make(chan error),
	}
	defer close(e.errCh)

	c.envelopes <- e
	err := <-e.errCh
	return err
}

func (c *grpcClient) SendAppLog(appID, message, sourceType, sourceInstance string) error {
	return c.send(createLogEnvelope(appID, message, sourceType, sourceInstance, Log_OUT))
}

func (c *grpcClient) SendAppErrorLog(appID, message, sourceType, sourceInstance string) error {
	return c.send(createLogEnvelope(appID, message, sourceType, sourceInstance, Log_ERR))
}

func (c *grpcClient) SendAppMetrics(m *events.ContainerMetric) error {
	c.logger.Info("grpc-logger-send-metric", lager.Data{"app-id": m.GetApplicationId()})
	return c.send(&Envelope{
		Timestamp: int64(time.Now().UnixNano()),
		SourceId:  m.GetApplicationId(),
		Message: &Envelope_Gauge{
			Gauge: &Gauge{
				Metrics: map[string]*GaugeValue{
					"instance_index": &GaugeValue{
						Value: float64(m.GetInstanceIndex()),
					},
					"cpu": &GaugeValue{
						Value: float64(m.GetCpuPercentage()),
					},
					"memory": &GaugeValue{
						Value: float64(m.GetMemoryBytes()),
					},
					"disk": &GaugeValue{
						Value: float64(m.GetDiskBytes()),
					},
					"memory_quota": &GaugeValue{
						Value: float64(m.GetMemoryBytesQuota()),
					},
					"disk_quota": &GaugeValue{
						Value: float64(m.GetDiskBytesQuota()),
					},
				},
			},
		},
	})
}
