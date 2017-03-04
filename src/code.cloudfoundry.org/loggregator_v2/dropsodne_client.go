package loggregator_v2

import (
	"github.com/cloudfoundry/dropsonde/logs"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
)

type dropsondeClient struct{}

func (c *dropsondeClient) SendAppLog(appID, message, sourceType, sourceInstance string) error {
	return logs.SendAppLog(appID, message, sourceType, sourceInstance)
}

func (c *dropsondeClient) SendAppErrorLog(appID, message, sourceType, sourceInstance string) error {
	return logs.SendAppErrorLog(appID, message, sourceType, sourceInstance)
}

func (c *dropsondeClient) SendAppMetrics(m *events.ContainerMetric) error {
	return metrics.Send(m)
}
