package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/locket/db"
)

type DBHealthCheckRunner struct {
	logger                      lager.Logger
	sqlDB                       db.LocketHealthCheckDB
	clock                       clock.Clock
	lock                        sync.Mutex
	isRunning                   bool
	HealthCheckFailureThreshold int
	HealthCheckTimeout          time.Duration
	HealthCheckInterval         time.Duration
}

func NewDBHealthCheckRunner(logger lager.Logger, sqlDB db.LocketHealthCheckDB, clock clock.Clock, failureCount int, timeout, interval time.Duration) *DBHealthCheckRunner {
	if failureCount == 0 {
		failureCount = 3
	}
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	if interval == 0 {
		interval = 10 * time.Second
	}
	return &DBHealthCheckRunner{
		logger:                      logger.Session("db-health-check-runner"),
		sqlDB:                       sqlDB,
		clock:                       clock,
		HealthCheckFailureThreshold: failureCount,
		HealthCheckTimeout:          timeout,
		HealthCheckInterval:         interval,
		lock:                        sync.Mutex{},
	}
}

func (runner *DBHealthCheckRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)
	runner.logger.Info("starting")
	defer runner.logger.Info("exiting")

	ticker := runner.clock.NewTicker(runner.HealthCheckInterval)
	healthCheckResults := make(chan error)
	for {
		runner.logger.Debug("reentering-run-loop")
		select {
		case err := <-healthCheckResults:
			runner.lock.Lock()
			runner.isRunning = false
			runner.lock.Unlock()
			if err != nil {
				runner.logger.Error("database-failure-detected-restarting-locket", err)
				return err
			}
			runner.logger.Info("health-check-succeeded")
		case <-signals:
			runner.logger.Info("exiting-due-to-signal")
			return nil
		case <-ticker.C():
			runner.lock.Lock()
			if !runner.isRunning {
				runner.isRunning = true
				runner.logger.Debug("executing-health-check")
				go runner.ExecuteTimedHealthCheckWithRetries(healthCheckResults)
			}
			runner.lock.Unlock()
		}
	}
}

func (runner *DBHealthCheckRunner) ExecuteTimedHealthCheckWithRetries(resultChan chan error) {
	var errs []error
	for i := 1; i <= runner.HealthCheckFailureThreshold; i++ {
		logger := runner.logger.WithData(lager.Data{"attempt": i})
		logger.Debug("executing-timed-health-check")
		err := runner.ExecuteTimedHealthCheck()
		if err != nil {
			logger.Error("failed-health-check", err)
			errs = append(errs, err)
		} else {
			resultChan <- nil
			return
		}
	}
	finalErr := errors.Join(errs...)
	runner.logger.Error("health-check-attempts-exceeded", finalErr, lager.Data{"max-attempts": runner.HealthCheckFailureThreshold})
	resultChan <- finalErr
}

func (runner *DBHealthCheckRunner) ExecuteTimedHealthCheck() error {
	timer := runner.clock.NewTimer(runner.HealthCheckTimeout)
	errChan := make(chan error)
	go runner.runDBHealthCheck(errChan)

	select {
	case err := <-errChan:
		if err == nil {
			return nil
		} else {
			return err
		}
	case <-timer.C():
		err := fmt.Errorf("timed out after %s while executing DB health check", runner.HealthCheckTimeout)
		runner.logger.Error("health-check-timed-out", err)
		return err
	}
}

func (runner *DBHealthCheckRunner) runDBHealthCheck(errChan chan error) {
	err := runner.sqlDB.PerformLocketHealthCheck(context.Background(), runner.logger, time.Now())
	errChan <- err
}
