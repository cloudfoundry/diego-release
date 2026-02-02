package diego_logging_client

import (
	"cmp"
	"fmt"
	"time"

	loggregator "code.cloudfoundry.org/go-loggregator/v9"
	"google.golang.org/grpc"
)

type Config struct {
	APIHost       string `json:"loggregator_api_host"`
	APIPort       int    `json:"loggregator_api_port"`
	CACertPath    string `json:"loggregator_ca_path"`
	CertPath      string `json:"loggregator_cert_path"`
	KeyPath       string `json:"loggregator_key_path"`
	JobDeployment string `json:"loggregator_job_deployment"`
	JobName       string `json:"loggregator_job_name"`
	JobIndex      string `json:"loggregator_job_index"`
	JobIP         string `json:"loggregator_job_ip"`
	JobOrigin     string `json:"loggregator_job_origin"`
	SourceID      string `json:"loggregator_source_id"`
	InstanceID    string `json:"loggregator_instance_id"`

	BatchMaxSize       uint `json:"loggregator_batch_max_size"`
	BatchFlushInterval time.Duration

	AppMetricExclusionFilter []string `json:"loggregator_app_metric_exclusion_filter"`
}

func (c Config) APIAddr() string {
	return fmt.Sprintf("%s:%d", cmp.Or(c.APIHost, "127.0.0.1"), c.APIPort)
}

// A ContainerMetric records resource usage of an app in a container.
type ContainerMetric struct {
	ApplicationId            string //deprecated
	InstanceIndex            int32  //deprecated
	CpuPercentage            float64
	CpuEntitlementPercentage float64
	MemoryBytes              uint64
	DiskBytes                uint64
	MemoryBytesQuota         uint64
	DiskBytesQuota           uint64
	AbsoluteCPUUsage         uint64
	AbsoluteCPUEntitlement   uint64
	ContainerAge             uint64
	RxBytes                  *uint64
	TxBytes                  *uint64
	Tags                     map[string]string
}

type SpikeMetric struct {
	Start time.Time
	End   time.Time
	Tags  map[string]string
}

// IngressClient is the shared contract between v1 and v2 clients.
//
//go:generate counterfeiter -o testhelpers/fake_ingress_client.go . IngressClient
type IngressClient interface {
	SendDuration(name string, value time.Duration, opts ...loggregator.EmitGaugeOption) error
	SendMebiBytes(name string, value int, opts ...loggregator.EmitGaugeOption) error
	SendMetric(name string, value int, opts ...loggregator.EmitGaugeOption) error
	SendBytesPerSecond(name string, value float64) error
	SendRequestsPerSecond(name string, value float64) error
	IncrementCounter(name string) error
	IncrementCounterWithDelta(name string, value uint64) error
	SendAppLog(message, sourceType string, tags map[string]string) error
	SendAppErrorLog(message, sourceType string, tags map[string]string) error
	SendAppMetrics(metrics ContainerMetric) error
	SendAppLogRate(rate, rateLimit float64, tags map[string]string) error
	SendSpikeMetrics(metrics SpikeMetric) error
	SendComponentMetric(name string, value float64, unit string) error
}

// NewIngressClient returns a v2 client if the config.UseV2API is true, or a no op client.
func NewIngressClient(config Config) (IngressClient, error) {
	return newV2IngressClient(config)
}

// NewV2IngressClient creates a V2 connection to the Loggregator API.
func newV2IngressClient(config Config) (IngressClient, error) {
	tlsConfig, err := loggregator.NewIngressTLSConfig(
		config.CACertPath,
		config.CertPath,
		config.KeyPath,
	)
	if err != nil {
		return nil, err
	}

	opts := []loggregator.IngressOption{
		// Whereas Metron will add tags for deployment, name, index, and ip,
		// it does not add job origin and so we must add it manually here.
		loggregator.WithTag("origin", config.JobOrigin),
	}

	if config.BatchMaxSize != 0 {
		opts = append(opts, loggregator.WithBatchMaxSize(config.BatchMaxSize))
	}

	if config.BatchFlushInterval != time.Duration(0) {
		opts = append(opts, loggregator.WithBatchFlushInterval(config.BatchFlushInterval))
	}

	if config.APIPort != 0 {
		opts = append(opts, loggregator.WithAddr(config.APIAddr()))
	}

	//lint:ignore SA1019 - we can't use grpc.WithContextDial until loggregator is updated for grpc.DialContext
	opts = append(opts, loggregator.WithDialOptions(grpc.WithBlock(), grpc.WithTimeout(time.Second)))

	c, err := loggregator.NewIngressClient(tlsConfig, opts...)
	if err != nil {
		return nil, err
	}

	return WrapClient(c, config.SourceID, config.InstanceID, config.AppMetricExclusionFilter), nil
}

func WrapClient(c logClient, s, i string, f []string) IngressClient {
	return client{client: c, sourceID: s, instanceID: i, appMetricExclusionFilter: f}
}

type logClient interface {
	EmitLog(msg string, opts ...loggregator.EmitLogOption)
	EmitGauge(opts ...loggregator.EmitGaugeOption)
	EmitTimer(name string, start, stop time.Time, opts ...loggregator.EmitTimerOption)
	EmitCounter(name string, opts ...loggregator.EmitCounterOption)
}

type client struct {
	client                   logClient
	sourceID                 string
	instanceID               string
	appMetricExclusionFilter []string
}

func (c client) SendDuration(name string, value time.Duration, opts ...loggregator.EmitGaugeOption) error {
	opts = append([]loggregator.EmitGaugeOption{
		loggregator.WithGaugeSourceInfo(c.sourceID, c.instanceID),
		loggregator.WithGaugeValue(name, float64(value), "nanos"),
	}, opts...)
	c.client.EmitGauge(opts...)

	return nil
}

func (c client) SendMebiBytes(name string, value int, opts ...loggregator.EmitGaugeOption) error {
	opts = append([]loggregator.EmitGaugeOption{
		loggregator.WithGaugeSourceInfo(c.sourceID, c.instanceID),
		loggregator.WithGaugeValue(name, float64(value), "MiB"),
	}, opts...)
	c.client.EmitGauge(opts...)
	return nil
}

func (c client) SendMetric(name string, value int, opts ...loggregator.EmitGaugeOption) error {
	opts = append([]loggregator.EmitGaugeOption{
		loggregator.WithGaugeSourceInfo(c.sourceID, c.instanceID),
		loggregator.WithGaugeValue(name, float64(value), "Metric"),
	}, opts...)
	c.client.EmitGauge(opts...)

	return nil
}

func (c client) SendBytesPerSecond(name string, value float64) error {
	c.client.EmitGauge(
		loggregator.WithGaugeSourceInfo(c.sourceID, c.instanceID),
		loggregator.WithGaugeValue(name, value, "B/s"),
	)
	return nil
}

func (c client) SendRequestsPerSecond(name string, value float64) error {
	c.client.EmitGauge(
		loggregator.WithGaugeSourceInfo(c.sourceID, c.instanceID),
		loggregator.WithGaugeValue(name, value, "Req/s"),
	)
	return nil
}

func (c client) IncrementCounter(name string) error {
	c.client.EmitCounter(
		name,
		loggregator.WithCounterSourceInfo(c.sourceID, c.instanceID),
	)

	return nil
}

func (c client) IncrementCounterWithDelta(name string, value uint64) error {
	c.client.EmitCounter(
		name,
		loggregator.WithCounterSourceInfo(c.sourceID, c.instanceID),
		loggregator.WithDelta(value),
	)

	return nil
}

func (c client) SendAppLog(message, sourceType string, tags map[string]string) error {
	c.client.EmitLog(
		message,
		loggregator.WithAppInfo(tags["source_id"], sourceType, tags["instance_id"]),
		loggregator.WithEnvelopeTags(tags),
		loggregator.WithStdout(),
	)
	return nil
}

func (c client) SendAppErrorLog(message, sourceType string, tags map[string]string) error {
	c.client.EmitLog(
		message,
		loggregator.WithAppInfo(tags["source_id"], sourceType, tags["instance_id"]),
		loggregator.WithEnvelopeTags(tags),
	)
	return nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func (c client) appendUnlessFiltered(opts []loggregator.EmitGaugeOption, gaugeName string, gaugeOption loggregator.EmitGaugeOption) []loggregator.EmitGaugeOption {
	if !contains(c.appMetricExclusionFilter, gaugeName) {
		opts = append(opts, gaugeOption)
	}
	return opts
}

func (c client) SendAppMetrics(m ContainerMetric) error {
	c.client.EmitGauge(
		loggregator.WithGaugeSourceInfo(m.Tags["source_id"], m.Tags["instance_id"]),
		loggregator.WithGaugeValue("cpu", m.CpuPercentage, "percentage"),
		loggregator.WithGaugeValue("memory", float64(m.MemoryBytes), "bytes"),
		loggregator.WithGaugeValue("disk", float64(m.DiskBytes), "bytes"),
		loggregator.WithGaugeValue("memory_quota", float64(m.MemoryBytesQuota), "bytes"),
		loggregator.WithGaugeValue("disk_quota", float64(m.DiskBytesQuota), "bytes"),
		loggregator.WithEnvelopeTags(m.Tags),
	)

	// Emit the new metrics in a separate envelope. Loggregator will convert a
	// gauge envelope with cpu, memory, disk, etc. to a container metric
	// envelope and ignore the rest of the fields. Emitting absolute_usage,
	// absolute_entitlement & container_age in a separate envelope allows v1
	// subscribers (cf nozzle) to be able to see those fields.  Note,
	// Loggregator will emit each value in a separate envelope for v1
	// subscribers.
	//
	// Above metrics are not filterable, as that would break this magic downstream.

	opts := []loggregator.EmitGaugeOption{
		loggregator.WithGaugeSourceInfo(m.Tags["source_id"], m.Tags["instance_id"]),
		loggregator.WithEnvelopeTags(m.Tags),
	}

	opts = c.appendUnlessFiltered(opts, "container_age", loggregator.WithGaugeValue("container_age", float64(m.ContainerAge), "nanoseconds"))
	opts = c.appendUnlessFiltered(opts, "cpu_entitlement", loggregator.WithGaugeValue("cpu_entitlement", float64(m.CpuEntitlementPercentage), "percentage"))
	opts = c.appendUnlessFiltered(opts, "absolute_usage", loggregator.WithGaugeValue("absolute_usage", float64(m.AbsoluteCPUUsage), "nanoseconds"))
	opts = c.appendUnlessFiltered(opts, "absolute_entitlement", loggregator.WithGaugeValue("absolute_entitlement", float64(m.AbsoluteCPUEntitlement), "nanoseconds"))

	c.client.EmitGauge(opts...)

	if m.RxBytes != nil && !contains(c.appMetricExclusionFilter, "rx_bytes") {
		c.client.EmitCounter("rx_bytes", loggregator.WithCounterSourceInfo(m.Tags["source_id"], m.Tags["instance_id"]), loggregator.WithTotal(*m.RxBytes))
	}

	if m.TxBytes != nil && !contains(c.appMetricExclusionFilter, "tx_bytes") {
		c.client.EmitCounter("tx_bytes", loggregator.WithCounterSourceInfo(m.Tags["source_id"], m.Tags["instance_id"]), loggregator.WithTotal(*m.TxBytes))
	}

	return nil
}

func (c client) SendAppLogRate(rate, rateLimit float64, tags map[string]string) error {
	c.client.EmitGauge(
		loggregator.WithGaugeSourceInfo(tags["source_id"], tags["instance_id"]),
		loggregator.WithGaugeValue("log_rate", rate, "B/s"),
		loggregator.WithGaugeValue("log_rate_limit", rateLimit, "B/s"),
		loggregator.WithEnvelopeTags(tags),
	)
	return nil
}

func (c client) SendSpikeMetrics(m SpikeMetric) error {
	c.client.EmitTimer("spike", m.Start, m.End,
		loggregator.WithTimerSourceInfo(m.Tags["source_id"], m.Tags["instance_id"]),
		loggregator.WithEnvelopeTags(m.Tags),
	)

	return nil
}

func (c client) SendComponentMetric(name string, value float64, unit string) error {
	c.client.EmitGauge(
		loggregator.WithGaugeSourceInfo(c.sourceID, c.instanceID),
		loggregator.WithGaugeValue(name, value, unit),
	)

	return nil
}
