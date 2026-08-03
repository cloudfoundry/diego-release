package steps

import (
	"os"

	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/executor"
	"github.com/tedsuo/ifrit"
)

type emitCheckFailureMetricStep struct {
	checkStep     ifrit.Runner
	checkProtocol executor.CheckProtocol
	checkType     executor.HealthcheckType
	metronClient  loggingclient.IngressClient
}

const (
	CheckFailedCount = "ChecksFailedCount"
)

func NewEmitCheckFailureMetricStep(
	checkStep ifrit.Runner,
	checkProtocol executor.CheckProtocol,
	checkType executor.HealthcheckType,
	metronClient loggingclient.IngressClient) ifrit.Runner {
	return &emitCheckFailureMetricStep{
		checkStep:     checkStep,
		checkProtocol: checkProtocol,
		checkType:     checkType,
		metronClient:  metronClient,
	}
}

func (step *emitCheckFailureMetricStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	if step.checkStep == nil {
		return nil
	}

	checkProccess := ifrit.Background(step.checkStep)

	done := make(chan struct{})
	defer close(done)
	go waitForReadiness(checkProccess, ready, done)

	select {
	case err := <-checkProccess.Wait():
		if err != nil {
			step.emitFailureMetric()
		}
		return err
	case s := <-signals:
		checkProccess.Signal(s)
		return <-checkProccess.Wait()
	}
}

func (step *emitCheckFailureMetricStep) emitFailureMetric() {
	if step.checkType != executor.IsLivenessCheck {
		return
	}

	if step.checkProtocol == executor.TCPCheck {
		httpMetricName := "TCPLivenessChecksFailedCount"
		go step.metronClient.IncrementCounter(httpMetricName)

	} else if step.checkProtocol == executor.HTTPCheck {
		tcpMetricName := "HTTPLivenessChecksFailedCount"
		go step.metronClient.IncrementCounter(tcpMetricName)
	}
}

func waitForReadiness(p ifrit.Process, ready chan<- struct{}, done <-chan struct{}) {
	select {
	case <-p.Ready():
		close(ready)
	case <-done:
	}
}
