package steps

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/log_streamer"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
)

const ExitTimeout = 1 * time.Second

type runStep struct {
	container                garden.Container
	model                    models.RunAction
	streamer                 log_streamer.LogStreamer
	logger                   lager.Logger
	externalIP               string
	internalIP               string
	internalIPv6             string
	portMappings             []executor.PortMapping
	clock                    clock.Clock
	gracefulShutdownInterval time.Duration
	suppressExitStatusCode   bool
	sidecar                  Sidecar
}

type Sidecar struct {
	Image                   garden.ImageRef
	Name                    string
	BindMounts              []garden.BindMount
	OverrideContainerLimits *garden.ProcessLimits
}

func NewRun(
	container garden.Container,
	model models.RunAction,
	streamer log_streamer.LogStreamer,
	logger lager.Logger,
	externalIP string,
	internalIP string,
	internalIPv6 string,
	portMappings []executor.PortMapping,
	clock clock.Clock,
	gracefulShutdownInterval time.Duration,
	suppressExitStatusCode bool,
) *runStep {
	return NewRunWithSidecar(
		container,
		model,
		streamer,
		logger,
		externalIP,
		internalIP,
		internalIPv6,
		portMappings,
		clock,
		gracefulShutdownInterval,
		suppressExitStatusCode,
		Sidecar{},
		false,
	)
}

func NewRunWithSidecar(
	container garden.Container,
	model models.RunAction,
	streamer log_streamer.LogStreamer,
	logger lager.Logger,
	externalIP string,
	internalIP string,
	internalIPv6 string,
	portMappings []executor.PortMapping,
	clock clock.Clock,
	gracefulShutdownInterval time.Duration,
	suppressExitStatusCode bool,
	sidecar Sidecar,
	privileged bool,
) *runStep {
	logger = logger.Session("run-step")
	return &runStep{
		container:                container,
		model:                    model,
		streamer:                 streamer,
		logger:                   logger,
		externalIP:               externalIP,
		internalIP:               internalIP,
		internalIPv6:             internalIPv6,
		portMappings:             portMappings,
		clock:                    clock,
		gracefulShutdownInterval: gracefulShutdownInterval,
		suppressExitStatusCode:   suppressExitStatusCode,
		sidecar:                  sidecar,
	}
}

func (step *runStep) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	step.logger.Info("running")

	envVars := convertEnvironmentVariables(step.model.Env)

	envVars = append(envVars, step.networkingEnvVars()...)

	select {
	case <-signals:
		step.logger.Info("cancelled-before-creating-process")
		return new(CancelledError)
	default:
	}

	exitStatusChan := make(chan int, 1)
	errChan := make(chan error, 1)

	step.logger.Debug("creating-process")

	var nofile *uint64
	if step.model.ResourceLimits != nil {
		nofile = step.model.ResourceLimits.GetNofilePtr()
	}

	var processIO garden.ProcessIO
	if step.model.SuppressLogOutput {
		processIO = garden.ProcessIO{
			Stdout: io.Discard,
			Stderr: io.Discard,
		}
	} else {
		processIO = garden.ProcessIO{
			Stdout: step.streamer.Stdout(),
			Stderr: step.streamer.Stderr(),
		}
	}

	processChan := make(chan garden.Process, 1)
	runStartTime := step.clock.Now()
	go func() {
		process, err := step.container.Run(garden.ProcessSpec{
			ID:   step.sidecar.Name,
			Path: step.model.Path,
			Args: step.model.Args,
			Dir:  step.model.Dir,
			Env:  envVars,
			User: step.model.User,

			Limits: garden.ResourceLimits{
				Nofile: nofile,
			},

			Image:                   step.sidecar.Image,
			BindMounts:              step.sidecar.BindMounts,
			OverrideContainerLimits: step.sidecar.OverrideContainerLimits,
		}, processIO)
		if err != nil {
			errChan <- err
		} else {
			processChan <- process
		}
	}()

	var process garden.Process
	select {
	case err := <-errChan:
		if err != nil {
			processErrorMessage := "failed-creating-process"
			step.logger.Error(processErrorMessage, err, lager.Data{"duration": step.clock.Now().Sub(runStartTime)})
			step.writeToStdOut(fmt.Sprintf("%s: %s", processErrorMessage, err))

			return err
		}
	case process = <-processChan:
	case <-signals:
		step.logger.Info("cancelled-before-process-creation-completed")
		return new(CancelledError)
	}

	logger := step.logger.WithData(lager.Data{"process": process.ID()})
	logger.Debug("successful-process-create", lager.Data{"duration": step.clock.Now().Sub(runStartTime)})

	close(ready)

	go func() {
		exitStatus, err := process.Wait()
		if err != nil {
			errChan <- err
		} else {
			exitStatusChan <- exitStatus
		}
	}()

	var killSwitch <-chan time.Time
	var exitTimeout <-chan time.Time

	for {
		select {
		case exitStatus := <-exitStatusChan:
			cancelled := signals == nil
			killed := cancelled && killSwitch == nil

			logger.Info("process-exit", lager.Data{
				"exitStatus": exitStatus,
				"cancelled":  cancelled,
			})

			var exitErrorMessage, emittableExitErrorMessage string

			if !step.suppressExitStatusCode {
				exitErrorMessage = fmt.Sprintf("Exit status %d", exitStatus)
				emittableExitErrorMessage = fmt.Sprintf("%s: Exited with status %d", step.streamer.SourceName(), exitStatus)
			}

			if killed {
				exitErrorMessage = fmt.Sprintf("%s (exceeded %s graceful shutdown interval)", exitErrorMessage, step.gracefulShutdownInterval)
			}

			if exitStatus != 0 {
				info, err := step.container.Info()
				if err != nil {
					logger.Error("failed-to-get-info", err)
				} else {
					for _, ev := range info.Events {
						if ev == "out of memory" || ev == "Out of memory" {
							exitErrorMessage = fmt.Sprintf("%s (out of memory)", exitErrorMessage)
							emittableExitErrorMessage = fmt.Sprintf("%s (out of memory)", emittableExitErrorMessage)
							break
						}
					}
				}
			}

			step.writeToStdOut(exitErrorMessage)

			if killed {
				return new(ExceededGracefulShutdownIntervalError)
			}

			if cancelled {
				return new(CancelledError)
			}

			if exitStatus != 0 {
				logger.Error("run-step-failed-with-nonzero-status-code", errors.New(exitErrorMessage), lager.Data{"status-code": exitStatus})
				return NewEmittableError(nil, "%s", emittableExitErrorMessage)
			}

			return nil

		case err := <-errChan:
			logger.Error("running-error", err)
			return err

		case <-signals:
			logger.Debug("signalling-terminate")
			err := process.Signal(garden.SignalTerminate)
			if err != nil {
				logger.Error("signalling-terminate-failed", err)
			}

			logger.Debug("signalling-terminate-success")
			signals = nil

			killTimer := step.clock.NewTimer(step.gracefulShutdownInterval)
			defer killTimer.Stop()

			killSwitch = killTimer.C()

		case <-killSwitch:
			killLogger := logger.Session("graceful-shutdown-timeout-exceeded")

			killLogger.Info("signalling-kill")
			err := process.Signal(garden.SignalKill)
			if err != nil {
				killLogger.Error("signalling-kill-failed", err)
			}

			killLogger.Info("signalling-kill-success")
			killSwitch = nil

			exitTimer := step.clock.NewTimer(ExitTimeout)
			defer exitTimer.Stop()

			exitTimeout = exitTimer.C()

		case <-exitTimeout:
			logger.Error("process-did-not-exit", nil, lager.Data{
				"timeout": ExitTimeout,
			})

			return new(ExitTimeoutError)
		}
	}
}

func convertEnvironmentVariables(environmentVariables []*models.EnvironmentVariable) []string {
	converted := []string{}

	for _, env := range environmentVariables {
		converted = append(converted, env.Name+"="+env.Value)
	}

	return converted
}

func (step *runStep) networkingEnvVars() []string {
	var envVars []string

	envVars = append(envVars, "CF_INSTANCE_IP="+step.externalIP)
	envVars = append(envVars, "CF_INSTANCE_INTERNAL_IP="+step.internalIP)

	if len(step.internalIPv6) != 0 {
		envVars = append(envVars, "CF_INSTANCE_INTERNAL_IPV6="+step.internalIPv6)
	}

	if len(step.portMappings) > 0 {
		if step.portMappings[0].HostPort > 0 {
			envVars = append(envVars, fmt.Sprintf("CF_INSTANCE_PORT=%d", step.portMappings[0].HostPort))
			envVars = append(envVars, fmt.Sprintf("CF_INSTANCE_ADDR=%s:%d", step.externalIP, step.portMappings[0].HostPort))
		}

		type cfPortMapping struct {
			External         uint16 `json:"external,omitempty"`
			Internal         uint16 `json:"internal"`
			ExternalTLSProxy uint16 `json:"external_tls_proxy,omitempty"`
			InternalTLSProxy uint16 `json:"internal_tls_proxy,omitempty"`
		}

		cfPortMappings := []cfPortMapping{}

		for _, portMap := range step.portMappings {
			cfPortMappings = append(cfPortMappings,
				cfPortMapping{
					Internal:         portMap.ContainerPort,
					External:         portMap.HostPort,
					InternalTLSProxy: portMap.ContainerTLSProxyPort,
					ExternalTLSProxy: portMap.HostTLSProxyPort,
				})
		}

		mappingsValue, err := json.Marshal(cfPortMappings)
		if err != nil {
			step.logger.Error("marshal-networking-env-vars-failed", err)
			mappingsValue = []byte("[]")
		}

		envVars = append(envVars, fmt.Sprintf("CF_INSTANCE_PORTS=%s", mappingsValue))
	} else {
		envVars = append(envVars, "CF_INSTANCE_PORT=")
		envVars = append(envVars, "CF_INSTANCE_ADDR=")
		envVars = append(envVars, "CF_INSTANCE_PORTS=[]")
	}

	return envVars
}

func (step *runStep) writeToStdOut(message string) {
	if !step.model.SuppressLogOutput {
		_, err := step.streamer.Stdout().Write([]byte(message))
		if err != nil {
			step.logger.Debug("failed-writing-to-stdout", lager.Data{"error": err})
		}
		step.streamer.Flush()
	}
}
