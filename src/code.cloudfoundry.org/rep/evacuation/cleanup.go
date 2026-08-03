package evacuation

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/v3"
)

const (
	exitTimeoutOffset = 5 * time.Second
)

var strandedEvacuatingActualLRPsMetric = "StrandedEvacuatingActualLRPs"

type EvacuationCleanup struct {
	clock          clock.Clock
	logger         lager.Logger
	cellID         string
	exitTimeout    time.Duration
	bbsClient      bbs.InternalClient
	executorClient executor.Client
	metronClient   loggingclient.IngressClient
}

func NewEvacuationCleanup(
	logger lager.Logger,
	cellID string,
	gracefulShutdownInterval time.Duration,
	proxyReloadDuration time.Duration,
	bbsClient bbs.InternalClient,
	executorClient executor.Client,
	clock clock.Clock,
	metronClient loggingclient.IngressClient,
) *EvacuationCleanup {
	return &EvacuationCleanup{
		logger:         logger,
		cellID:         cellID,
		exitTimeout:    gracefulShutdownInterval + proxyReloadDuration + exitTimeoutOffset,
		bbsClient:      bbsClient,
		executorClient: executorClient,
		clock:          clock,
		metronClient:   metronClient,
	}
}

func (e *EvacuationCleanup) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := e.logger.Session("evacuation-cleanup")

	logger.Info("started")
	defer logger.Info("complete")

	close(ready)

	signal := <-signals
	logger.Info("signalled", lager.Data{"signal": signal})

	traceID := "" // evacutaion cleanup is not originated through API

	actualLRPs, err := e.bbsClient.ActualLRPs(logger, traceID, models.ActualLRPFilter{CellID: e.cellID})
	if err != nil {
		logger.Error("failed-fetching-actual-lrp-groups", err)
		return err
	}

	strandedEvacuationCount := 0
	for _, actualLRP := range actualLRPs {
		if actualLRP.GetPresence() != models.ActualLRP_Evacuating {
			continue
		}

		strandedEvacuationCount++
	}

	err = e.metronClient.SendMetric(strandedEvacuatingActualLRPsMetric, strandedEvacuationCount)
	if err != nil {
		logger.Error("failed-sending-stranded-evacuating-lrp-metric", err, lager.Data{"count": strandedEvacuationCount})
	}

	logger.Info("finished-evacuating", lager.Data{"stranded-evacuating-actual-lrps": strandedEvacuationCount})

	logger.Info("deleting-all-containers")

	exitTimer := e.clock.NewTimer(e.exitTimeout)

	checkRunningContainersTimer := e.clock.NewTicker(1 * time.Second)
	containersSignalled := make(chan struct{})
	containersDeleted := make(chan struct{})
	go e.deleteRunningContainers(logger, traceID, containersSignalled)
	go e.checkRunningContainers(logger, checkRunningContainersTimer.C(), containersSignalled, containersDeleted)

	select {
	case <-exitTimer.C():
		logger.Info("failed-to-cleanup-all-containers")
		return errors.New("failed-to-cleanup-all-containers")
	case <-containersDeleted:
		logger.Info("deleted-containers-successfully")
		return nil
	}
}

func (e *EvacuationCleanup) checkRunningContainers(
	logger lager.Logger,
	ticker <-chan time.Time,
	containersSignalled <-chan struct{},
	containersDeleted chan<- struct{},
) {
	hasRunningContainers := func() bool {
		containers, err := e.executorClient.ListContainers(logger)
		if err != nil {
			logger.Error("failed-listing-containers", err)
			// assume no container is running if we can't list them
			return false
		}

		if len(containers) > 0 {
			return true
		} else {
			return false
		}
	}

	defer close(containersDeleted)

	// wait for all containers to be signalled, this only makes the tests easier
	// to write since they depend on the signalling and checking to happen
	// sequentially, but isn't necessary for the operation of the cleanup
	<-containersSignalled
	for hasRunningContainers() {
		logger.Info("waiting-for-containers-to-delete")
		<-ticker
	}
}

func (e *EvacuationCleanup) deleteRunningContainers(logger lager.Logger, traceID string, containersSignalled chan<- struct{}) {
	defer close(containersSignalled)

	containers, err := e.executorClient.ListContainers(logger)
	if err != nil {
		logger.Error("failed-listing-containers", err)
		return
	}

	logger.Info("sending-signal-to-containers")

	var wg sync.WaitGroup
	for _, container := range containers {
		sourceName, tags := container.RunInfo.LogConfig.GetSourceNameAndTagsForLogging()

		metricError := e.metronClient.SendAppLog(fmt.Sprintf("Cell %s reached evacuation timeout for instance %s", e.cellID, container.Guid), sourceName, tags)
		if metricError != nil {
			logger.Debug("failed-sending-app-log-for-instance-evacuation-timeout", lager.Data{"error": metricError})
		}
		wg.Add(1)
		go func(logger lager.Logger, traceID string, containerGuid string) {
			defer wg.Done()
			err := e.executorClient.DeleteContainer(logger, traceID, containerGuid)
			if err != nil {
				logger.Error("failed-to-delete-container", err, lager.Data{"container-guid": containerGuid})
			}
		}(logger, traceID, container.Guid)
	}

	logger.Info("sent-signal-to-containers")
	wg.Wait()
}
