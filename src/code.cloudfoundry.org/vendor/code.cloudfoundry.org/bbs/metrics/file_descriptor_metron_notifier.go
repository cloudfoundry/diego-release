package metrics

import (
	"os"

	"github.com/tedsuo/ifrit"

	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3"
)

const (
	OpenFileDescriptorsMetric = "OpenFileDescriptors"
	FileDescriptorUnits       = "descriptors"
)

type FileDescriptorMetronNotifier struct {
	Logger       lager.Logger
	metronClient loggingclient.IngressClient
	ticker       clock.Ticker
	procFSPath   string
}

func NewFileDescriptorMetronNotifier(logger lager.Logger, newTicker clock.Ticker, metronClient loggingclient.IngressClient, procPath string) ifrit.Runner {
	return &FileDescriptorMetronNotifier{
		Logger:       logger,
		metronClient: metronClient,
		ticker:       newTicker,
		procFSPath:   procPath,
	}
}

func (notifier FileDescriptorMetronNotifier) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := notifier.Logger.Session("file-descriptor-notifier")
	logger.Info("starting")

	close(ready)

	logger.Info("started")
	defer logger.Info("finished")

	for {
		select {
		case <-notifier.ticker.C():
			nDescriptors, err := notifier.descriptorCount()

			if err != nil {
				logger.Error("failed-to-read-proc-filesystem", err)
				continue
			}

			err = notifier.metronClient.SendMetric(OpenFileDescriptorsMetric, nDescriptors)
			if err != nil {
				logger.Error("error-sending-metric", err)
			}
		case <-signals:
			return nil
		}
	}
}

func (notifier FileDescriptorMetronNotifier) descriptorCount() (int, error) {
	descriptorInfos, err := os.ReadDir(notifier.procFSPath)

	if err != nil {
		return 0, err
	}

	count := 0
	for range descriptorInfos {
		count++
	}

	return count, nil
}
