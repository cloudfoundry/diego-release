package testhelpers

import (
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

type MetricAndValue struct {
	Name  string
	Value int32
}

func MatchV2Metric(target MetricAndValue) types.GomegaMatcher {
	return gomega.SatisfyAll(
		gomega.WithTransform(func(source *loggregator_v2.Envelope) *loggregator_v2.Gauge {
			return source.GetGauge()
		}, gomega.Not(gomega.BeNil())),
		gomega.WithTransform(func(source *loggregator_v2.Envelope) map[string]*loggregator_v2.GaugeValue {
			return source.GetGauge().GetMetrics()
		}, gomega.HaveKey(target.Name)),
	)
}

func MatchV2MetricAndValue(target MetricAndValue) types.GomegaMatcher {
	return gomega.SatisfyAll(
		gomega.WithTransform(func(source *loggregator_v2.Envelope) *loggregator_v2.Gauge {
			return source.GetGauge()
		}, gomega.Not(gomega.BeNil())),
		gomega.WithTransform(func(source *loggregator_v2.Envelope) map[string]*loggregator_v2.GaugeValue {
			return source.GetGauge().GetMetrics()
		}, gomega.HaveKey(target.Name)),
		gomega.WithTransform(func(source *loggregator_v2.Envelope) int32 {
			return int32(source.GetGauge().GetMetrics()[target.Name].Value)
		}, gomega.Equal(target.Value)),
	)
}

func TestMetricChan(receiversChan chan loggregator_v2.Ingress_BatchSenderServer) (chan *loggregator_v2.Envelope, chan struct{}) {
	signalMetricsChan := make(chan struct{})
	testMetricsChan := make(chan *loggregator_v2.Envelope)
	go func() {
		for {
			select {
			case receiver := <-receiversChan:
				go func() {
					for {
						batch, err := receiver.Recv()
						if err != nil {
							return
						}
						for _, envelope := range batch.Batch {
							select {
							case testMetricsChan <- envelope:
							case <-signalMetricsChan:
								return
							}
						}
					}
				}()
			case <-signalMetricsChan:
				return
			}
		}
	}()
	return testMetricsChan, signalMetricsChan
}
