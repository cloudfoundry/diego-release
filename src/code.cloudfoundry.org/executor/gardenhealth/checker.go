package gardenhealth

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/guidgen"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/lager/v3"
)

const (
	HealthcheckPrefix          = "check-"
	HealthcheckTag             = "tag:healthcheck-tag"
	HealthcheckTagValue        = "healthcheck"
	HealthcheckNetworkProperty = "network.healthcheck"
)

type UnrecoverableError string

func (e UnrecoverableError) Error() string {
	return string(e)
}

type HealthcheckFailedError int

func (e HealthcheckFailedError) Error() string {
	return fmt.Sprintf("Healthcheck exited with %d", e)
}

//go:generate counterfeiter -o fakegardenhealth/fake_checker.go . Checker

type Checker interface {
	Healthcheck(lager.Logger) error
	Cancel(lager.Logger)
}

type checker struct {
	rootFSPath         string
	containerOwnerName string
	retryInterval      time.Duration
	healthcheckSpec    garden.ProcessSpec
	gardenClient       garden.Client
	guidGenerator      guidgen.Generator
}

// NewChecker constructs a checker.
//
// healthcheckSpec describes the process to run in the healthcheck container and
// retryInterval describes the amount of time to wait to sleep when retrying a
// failed garden command.
func NewChecker(
	rootFSPath string,
	containerOwnerName string,
	retryInterval time.Duration,
	healthcheckSpec garden.ProcessSpec,
	gardenClient garden.Client,
	guidGenerator guidgen.Generator,
) Checker {
	return &checker{
		rootFSPath:         rootFSPath,
		containerOwnerName: containerOwnerName,
		retryInterval:      retryInterval,
		healthcheckSpec:    healthcheckSpec,
		gardenClient:       gardenClient,
		guidGenerator:      guidGenerator,
	}
}

func (c *checker) Cancel(logger lager.Logger) {
	logger = logger.Session("cancel")

	containers, err := c.list(logger)
	if err != nil {
		logger.Error("failed-to-list-containers", err)
		return
	}

	err = c.destroyContainers(logger, containers)
	if err != nil {
		logger.Error("failed-to-destroy-containers", err)
		return
	}
}

func (c *checker) list(logger lager.Logger) ([]garden.Container, error) {
	logger = logger.Session("list")
	logger.Debug("starting")
	defer logger.Debug("complete")

	var containers []garden.Container
	err := retryOnFail(c.retryInterval, func(attempt uint) (listErr error) {
		containers, listErr = c.gardenClient.Containers(garden.Properties{
			HealthcheckTag: HealthcheckTagValue,
		})
		if listErr != nil {
			logger.Error("failed", listErr, lager.Data{"attempt": attempt})
			return listErr
		}

		logger.Debug("succeeded", lager.Data{"attempt": attempt})
		return nil
	})

	return containers, err
}

func (c *checker) destroyContainers(logger lager.Logger, containers []garden.Container) error {
	logger = logger.Session("destroy-containers")
	logger.Debug("starting", lager.Data{"numContainers": len(containers)})
	defer logger.Debug("complete")

	for i := range containers {
		err := retryOnFail(c.retryInterval, func(attempt uint) (destroyErr error) {
			handle := containers[i].Handle()
			destroyErr = c.gardenClient.Destroy(handle)
			if destroyErr != nil {
				if destroyErr.Error() == server.ErrConcurrentDestroy.Error() {
					// Log but don't fail if container is already being destroyed
					logger.Debug("already-being-destroyed", lager.Data{"handle": handle})
					return nil
				}

				logger.Error("failed", destroyErr, lager.Data{"handle": handle, "attempt": attempt})
				return destroyErr
			}

			logger.Debug("succeeded", lager.Data{"handle": handle, "attempt": attempt})
			return nil
		})

		if err != nil {
			return err
		}
	}

	logger.Debug("succeeded")
	return nil
}

func (c *checker) create(logger lager.Logger) (string, garden.Container, error) {
	logger = logger.Session("create")
	logger.Debug("starting")
	defer logger.Debug("complete")

	guid := HealthcheckPrefix + c.guidGenerator.Guid(logger)
	var container garden.Container
	err := retryOnFail(c.retryInterval, func(attempt uint) (createErr error) {
		container, createErr = c.gardenClient.Create(garden.ContainerSpec{
			Handle:     guid,
			RootFSPath: c.rootFSPath,
			Properties: garden.Properties{
				executor.ContainerOwnerProperty: c.containerOwnerName,
				HealthcheckTag:                  HealthcheckTagValue,
				HealthcheckNetworkProperty:      "true",
			},
		})
		if createErr != nil {
			logger.Error("failed", createErr, lager.Data{"attempt": attempt})
			return createErr
		}

		logger.Debug("succeeded", lager.Data{"attempt": attempt})
		return nil
	})

	return guid, container, err
}

func (c *checker) cleanupDestroy(logger lager.Logger, guid string) error {
	logger = logger.Session("cleanup-destroy")
	logger.Debug("starting")
	defer logger.Debug("complete")

	err := retryOnFail(c.retryInterval, func(attempt uint) (destroyErr error) {
		destroyErr = c.destroyContainer(guid)
		if destroyErr != nil {
			if destroyErr.Error() == server.ErrConcurrentDestroy.Error() {
				// Log but don't fail if container is already being destroyed
				logger.Debug("already-being-destroyed", lager.Data{"handle": guid})
				return nil
			}

			logger.Error("failed", destroyErr, lager.Data{"handle": guid, "attempt": attempt})
			return destroyErr
		}

		logger.Debug("succeeded", lager.Data{"attempt": attempt})
		return nil
	})

	return err
}

func (c *checker) run(logger lager.Logger, container garden.Container) (garden.Process, error) {
	logger = logger.Session("run", lager.Data{
		"processPath": c.healthcheckSpec.Path,
		"processArgs": c.healthcheckSpec.Args,
		"processUser": c.healthcheckSpec.User,
		"processEnv":  c.healthcheckSpec.Env,
		"processDir":  c.healthcheckSpec.Dir,
	})
	logger.Debug("starting")
	defer logger.Debug("complete")

	var proc garden.Process
	err := retryOnFail(c.retryInterval, func(attempt uint) (runErr error) {
		proc, runErr = container.Run(c.healthcheckSpec, garden.ProcessIO{})
		if runErr != nil {
			logger.Error("failed", runErr, lager.Data{"attempt": attempt})
			return runErr
		}

		logger.Debug("succeeded", lager.Data{"attempt": attempt})
		return nil
	})

	return proc, err
}

func (c *checker) wait(logger lager.Logger, proc garden.Process) (int, error) {
	logger = logger.Session("wait")
	logger.Debug("starting")
	defer logger.Debug("complete")

	var exitCode int
	err := retryOnFail(c.retryInterval, func(attempt uint) (waitErr error) {
		exitCode, waitErr = proc.Wait()
		if waitErr != nil {
			logger.Error("failed", waitErr, lager.Data{"attempt": attempt})
			return waitErr
		}

		logger.Debug("succeeded", lager.Data{"attempt": attempt})
		return nil
	})

	return exitCode, err
}

// Healthcheck destroys any existing healthcheck containers, creates a new container,
// runs a process in the new container, waits for the process to exit, then destroys
// the created container.
//
// If any of these steps fail, the failed step will be retried
// up to gardenhealth.MaxRetries times. If the command continues to fail after the
// retries, an error will be returned, indicating the healthcheck failed.
func (c *checker) Healthcheck(logger lager.Logger) (healthcheckResult error) {
	logger = logger.Session("healthcheck")
	logger.Info("starting")
	defer logger.Info("complete")

	defer func() {
		if healthcheckResult != nil {
			logger.Error("failed-health-check", healthcheckResult)
		} else {
			logger.Info("passed-health-check")
		}
	}()

	containers, err := c.list(logger)
	if err != nil {
		return err
	}

	err = c.destroyContainers(logger, containers)
	if err != nil {
		return err
	}

	guid, container, err := c.create(logger)
	if err != nil {
		return err
	}

	defer func() {
		err := c.cleanupDestroy(logger, guid)
		if err != nil {
			healthcheckResult = err
		}
	}()

	proc, err := c.run(logger, container)
	if err != nil {
		return err
	}

	exitCode, err := c.wait(logger, proc)
	if err != nil {
		return err
	}

	if exitCode != 0 {
		return HealthcheckFailedError(exitCode)
	}

	return nil
}

func (c *checker) destroyContainer(guid string) error {
	err := c.gardenClient.Destroy(guid)
	switch err.(type) {
	case nil:
		return nil
	case garden.ContainerNotFoundError:
		return err
	default:
		return UnrecoverableError(err.Error())
	}
}

const (
	maxRetries = 3
)

func retryOnFail(retryInterval time.Duration, cmd func(attempt uint) error) error {
	var err error

	for i := uint(0); i < maxRetries; i++ {
		err = cmd(i)
		if err == nil {
			return nil
		}

		time.Sleep(retryInterval)
	}

	return err
}
