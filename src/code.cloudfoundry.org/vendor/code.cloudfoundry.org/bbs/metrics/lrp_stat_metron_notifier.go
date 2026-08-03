package metrics

import (
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	logging "code.cloudfoundry.org/diego-logging-client"
	loggregator "code.cloudfoundry.org/go-loggregator/v9"
	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/ifrit"
)

const (
	DefaultEmitMetricsFrequency = 15 * time.Second

	domainMetricPrefix = "Domain."

	ConvergenceLRPRunsMetric     = "ConvergenceLRPRuns"
	ConvergenceLRPDurationMetric = "ConvergenceLRPDuration"

	LRPsUnclaimedMetric     = "LRPsUnclaimed"
	LRPsClaimedMetric       = "LRPsClaimed"
	LRPsRunningMetric       = "LRPsRunning"
	CrashedActualLRPsMetric = "CrashedActualLRPs"
	LRPsMissingMetric       = "LRPsMissing"
	LRPsExtraMetric         = "LRPsExtra"

	SuspectRunningLRPsMetric = "SuspectRunningActualLRPs"
	SuspectClaimedLRPsMetric = "SuspectClaimedActualLRPs"

	LRPsDesiredMetric         = "LRPsDesired"
	CrashingDesiredLRPsMetric = "CrashingDesiredLRPs"

	PresentCellsMetric = "PresentCells"
	SuspectCellsMetric = "SuspectCells"
)

//counterfeiter:generate -o fakes/fake_lrp_stat_metron_notifier.go . LRPStatMetronNotifier
type LRPStatMetronNotifier interface {
	ifrit.Runner

	RecordFreshDomains(domains []string)
	RecordConvergenceDuration(duration time.Duration)
	RecordLRPCounts(
		unclaimed, claimed, running, crashed, missing, extra,
		suspectRunning, suspectClaimed, desired, crashingDesired int,
	)
	RecordCellCounts(present, suspect int)
}

type lrpStatMetronNotifier struct {
	clock        clock.Clock
	mutex        sync.Mutex
	metricSender loggingMetricSender

	metrics lrpMetrics
}

type lrpMetrics struct {
	domainsMetric []string

	convergenceLRPRuns     uint64
	convergenceLRPDuration time.Duration

	lrpsUnclaimed     int
	lrpsClaimed       int
	lrpsRunning       int
	crashedActualLRPs int
	lrpsMissing       int
	lrpsExtra         int

	suspectRunningLRPs int
	suspectClaimedLRPs int

	lrpsDesired         int
	crashingDesiredLRPs int

	presentCells int
	suspectCells int
}

func NewLRPStatMetronNotifier(logger lager.Logger, clock clock.Clock, metronClient logging.IngressClient) LRPStatMetronNotifier {
	return &lrpStatMetronNotifier{
		clock: clock,
		metricSender: loggingMetricSender{
			logger:       logger,
			metronClient: metronClient,
		},
	}
}

func (t *lrpStatMetronNotifier) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	ticker := t.clock.NewTicker(DefaultEmitMetricsFrequency)
	close(ready)
	for {
		select {
		case <-ticker.C():
			t.emitMetrics()
		case <-signals:
			return nil
		}
	}
}

func (lrp *lrpStatMetronNotifier) RecordFreshDomains(domains []string) {
	lrp.mutex.Lock()
	defer lrp.mutex.Unlock()

	lrp.metrics.domainsMetric = domains
}

func (lrp *lrpStatMetronNotifier) RecordConvergenceDuration(duration time.Duration) {
	lrp.mutex.Lock()
	defer lrp.mutex.Unlock()

	lrp.metrics.convergenceLRPRuns++
	lrp.metrics.convergenceLRPDuration = duration
}

func (lrp *lrpStatMetronNotifier) RecordLRPCounts(
	unclaimed, claimed, running, crashed, missing, extra,
	suspectRunning, suspectClaimed, desired, crashingDesired int,
) {
	lrp.mutex.Lock()
	defer lrp.mutex.Unlock()

	lrp.metrics.lrpsUnclaimed = unclaimed
	lrp.metrics.lrpsClaimed = claimed
	lrp.metrics.lrpsRunning = running
	lrp.metrics.crashedActualLRPs = crashed
	lrp.metrics.lrpsMissing = missing
	lrp.metrics.lrpsExtra = extra

	lrp.metrics.suspectRunningLRPs = suspectRunning
	lrp.metrics.suspectClaimedLRPs = suspectClaimed

	lrp.metrics.lrpsDesired = desired
	lrp.metrics.crashingDesiredLRPs = crashingDesired
}

func (lrp *lrpStatMetronNotifier) RecordCellCounts(present int, suspect int) {
	lrp.mutex.Lock()
	defer lrp.mutex.Unlock()

	lrp.metrics.presentCells = present
	lrp.metrics.suspectCells = suspect
}

func (lrp *lrpStatMetronNotifier) emitMetrics() {
	lrp.mutex.Lock()
	defer lrp.mutex.Unlock()

	if lrp.metrics.convergenceLRPRuns > 0 {
		lrp.metricSender.IncrementCounterWithDelta(ConvergenceLRPRunsMetric, lrp.metrics.convergenceLRPRuns)
		lrp.metrics.convergenceLRPRuns = 0
	}
	lrp.metricSender.SendDuration(ConvergenceLRPDurationMetric, lrp.metrics.convergenceLRPDuration)

	for _, domain := range lrp.metrics.domainsMetric {
		lrp.metricSender.SendMetric(domainMetricPrefix+domain, 1)
	}

	lrp.metricSender.SendMetric(LRPsUnclaimedMetric, lrp.metrics.lrpsUnclaimed)
	lrp.metricSender.SendMetric(LRPsClaimedMetric, lrp.metrics.lrpsClaimed)
	lrp.metricSender.SendMetric(LRPsRunningMetric, lrp.metrics.lrpsRunning)
	lrp.metricSender.SendMetric(CrashedActualLRPsMetric, lrp.metrics.crashedActualLRPs)
	lrp.metricSender.SendMetric(LRPsMissingMetric, lrp.metrics.lrpsMissing)
	lrp.metricSender.SendMetric(LRPsExtraMetric, lrp.metrics.lrpsExtra)
	lrp.metricSender.SendMetric(SuspectRunningLRPsMetric, lrp.metrics.suspectRunningLRPs)
	lrp.metricSender.SendMetric(SuspectClaimedLRPsMetric, lrp.metrics.suspectClaimedLRPs)
	lrp.metricSender.SendMetric(LRPsDesiredMetric, lrp.metrics.lrpsDesired)
	lrp.metricSender.SendMetric(CrashingDesiredLRPsMetric, lrp.metrics.crashingDesiredLRPs)

	lrp.metricSender.SendMetric(PresentCellsMetric, lrp.metrics.presentCells)
	lrp.metricSender.SendMetric(SuspectCellsMetric, lrp.metrics.suspectCells)
}

type loggingMetricSender struct {
	logger       lager.Logger
	metronClient logging.IngressClient
}

func (l loggingMetricSender) logMetricErr(metricName string, err error) {
	if err != nil {
		l.logger.Error("failed-sending-metric", err, lager.Data{"metric-name": metricName})
	}
}

func (l loggingMetricSender) SendMetric(name string, value int, opts ...loggregator.EmitGaugeOption) {
	f := l.metronClient.SendMetric
	l.logMetricErr(name, f(name, value, opts...))
}

func (l loggingMetricSender) SendDuration(name string, value time.Duration, opts ...loggregator.EmitGaugeOption) {
	f := l.metronClient.SendDuration
	l.logMetricErr(name, f(name, value, opts...))
}

func (l loggingMetricSender) IncrementCounterWithDelta(name string, value uint64) {
	f := l.metronClient.IncrementCounterWithDelta
	l.logMetricErr(name, f(name, value))
}
