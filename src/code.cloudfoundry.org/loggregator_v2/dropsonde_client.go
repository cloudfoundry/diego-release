package loggregator_v2

import (
	"time"

	"github.com/cloudfoundry/dropsonde/logs"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
)

type dropsondeClient struct{}

func (c *dropsondeClient) Batcher() Batcher {
	return c
}

func (c *dropsondeClient) Send() error {
	return nil
}

func (c *dropsondeClient) IncrementCounter(name string) error {
	return metrics.IncrementCounter(name)
}
func (c *dropsondeClient) SendAppLog(appID, message, sourceType, sourceInstance string) error {
	return logs.SendAppLog(appID, message, sourceType, sourceInstance)
}

func (c *dropsondeClient) SendAppErrorLog(appID, message, sourceType, sourceInstance string) error {
	return logs.SendAppErrorLog(appID, message, sourceType, sourceInstance)
}

func (c *dropsondeClient) SendAppMetrics(m *events.ContainerMetric) error {
	return metrics.Send(m)
}

func (c *dropsondeClient) SendDuration(name string, duration time.Duration) error {
	return c.sendComponentMetric(name, float64(duration), "nanos")
}

func (c *dropsondeClient) SendMebiBytes(name string, mebibytes int) error {
	return c.sendComponentMetric(name, float64(mebibytes), "MiB")
}

func (c *dropsondeClient) SendMetric(name string, value int) error {
	return c.sendComponentMetric(name, float64(value), "Metric")
}

func (c *dropsondeClient) SendBytesPerSecond(name string, value float64) error {
	return c.sendComponentMetric(name, value, "B/s")
}

func (c *dropsondeClient) SendRequestsPerSecond(name string, value float64) error {
	return c.sendComponentMetric(name, value, "Req/s")
}

func (c *dropsondeClient) sendComponentMetric(name string, value float64, unit string) error {
	return metrics.SendValue(name, value, unit)
}
