package transformer

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"code.cloudfoundry.org/lager/v3"

	"code.cloudfoundry.org/archiver/compressor"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cacheddownloader"
	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/log_streamer"
	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/executor/depot/uploader"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/workpool"
	"github.com/tedsuo/ifrit"
)

const (
	healthCheckNofiles                          uint64 = 1024
	DefaultDeclarativeHealthcheckRequestTimeout        = int(1 * time.Second / time.Millisecond) // this is just 1000, transformed eventually into a str: "1000ms" or equivalently "1s"
	HealthLogSource                                    = "HEALTH"
)

var ErrNoCheck = errors.New("no check configured")
var HealthCheckDstPath string = filepath.Join(string(os.PathSeparator), "etc", "cf-assets", "healthcheck")

//go:generate counterfeiter -o faketransformer/fake_transformer.go . Transformer

type Transformer interface {
	StepsRunner(lager.Logger, executor.Container, garden.Container, log_streamer.LogStreamer, Config) (ifrit.Runner, chan steps.ReadinessState, error)
}

type Config struct {
	ProxyTLSPorts     []uint16
	BindMounts        []garden.BindMount
	CreationStartTime time.Time
	MetronClient      loggingclient.IngressClient
}

type transformer struct {
	cachedDownloader cacheddownloader.CachedDownloader
	uploader         uploader.Uploader
	compressor       compressor.Compressor
	downloadLimiter  chan struct{}
	uploadLimiter    chan struct{}
	tempDir          string
	clock            clock.Clock

	sidecarRootFS                        string
	declarativeHealthCheckDefaultTimeout time.Duration
	emitHealthCheckMetrics               bool
	healthyMonitoringInterval            time.Duration
	unhealthyMonitoringInterval          time.Duration
	gracefulShutdownInterval             time.Duration
	healthCheckWorkPool                  *workpool.WorkPool

	useContainerProxy                bool
	drainWait                        time.Duration
	enableContainerProxyHealthChecks bool
	proxyHealthCheckInterval         time.Duration

	postSetupHook []string
	postSetupUser string

	enableDropletCaching bool
}

type Option func(*transformer)

func WithSidecarRootfs(
	sidecarRootFS string,
) Option {
	return func(t *transformer) {
		t.sidecarRootFS = sidecarRootFS
	}
}

func WithDeclarativeHealthChecks(timeout time.Duration) Option {
	return func(t *transformer) {

		if timeout <= 0 {
			timeout = time.Duration(DefaultDeclarativeHealthcheckRequestTimeout) * time.Millisecond
		}
		t.declarativeHealthCheckDefaultTimeout = timeout
	}
}

func WithContainerProxy(drainWait time.Duration) Option {
	return func(t *transformer) {
		t.useContainerProxy = true
		t.drainWait = drainWait
	}
}

func WithProxyLivenessChecks(interval time.Duration) Option {
	return func(t *transformer) {
		t.enableContainerProxyHealthChecks = true
		t.proxyHealthCheckInterval = interval
	}
}

func WithPostSetupHook(user string, hook []string) Option {
	return func(t *transformer) {
		t.postSetupUser = user
		t.postSetupHook = hook
	}
}

func WithDeclarativeHealthcheckFailureMetrics() Option {
	return func(t *transformer) {
		t.emitHealthCheckMetrics = true
	}
}

func WithEnableDropletCaching(enableDropletCaching bool) Option {
	return func(t *transformer) {
		t.enableDropletCaching = enableDropletCaching
	}
}

func NewTransformer(
	clock clock.Clock,
	cachedDownloader cacheddownloader.CachedDownloader,
	uploader uploader.Uploader,
	compressor compressor.Compressor,
	downloadLimiter chan struct{},
	uploadLimiter chan struct{},
	tempDir string,
	healthyMonitoringInterval time.Duration,
	unhealthyMonitoringInterval time.Duration,
	gracefulShutdownInterval time.Duration,
	healthCheckWorkPool *workpool.WorkPool,
	opts ...Option,
) *transformer {
	t := &transformer{
		cachedDownloader:            cachedDownloader,
		uploader:                    uploader,
		compressor:                  compressor,
		downloadLimiter:             downloadLimiter,
		uploadLimiter:               uploadLimiter,
		tempDir:                     tempDir,
		healthyMonitoringInterval:   healthyMonitoringInterval,
		unhealthyMonitoringInterval: unhealthyMonitoringInterval,
		gracefulShutdownInterval:    gracefulShutdownInterval,
		healthCheckWorkPool:         healthCheckWorkPool,
		clock:                       clock,
	}

	for _, o := range opts {
		o(t)
	}

	return t
}

func (t *transformer) stepFor(
	logStreamer log_streamer.LogStreamer,
	action *models.Action,
	container garden.Container,
	externalIP string,
	internalIP string,
	internalIPv6 string,
	ports []executor.PortMapping,
	suppressExitStatusCode bool,
	monitorOutputWrapper bool,
	logger lager.Logger,
) ifrit.Runner {
	a := action.GetValue()
	switch actionModel := a.(type) {
	case *models.RunAction:
		return steps.NewRun(
			container,
			*actionModel,
			logStreamer.WithSource(actionModel.LogSource),
			logger,
			externalIP,
			internalIP,
			internalIPv6,
			ports,
			t.clock,
			t.gracefulShutdownInterval,
			suppressExitStatusCode,
		)

	case *models.DownloadAction:
		return steps.NewDownload(
			container,
			*actionModel,
			t.cachedDownloader,
			t.downloadLimiter,
			logStreamer.WithSource(actionModel.LogSource),
			logger,
			t.enableDropletCaching,
		)

	case *models.UploadAction:
		return steps.NewUpload(
			container,
			*actionModel,
			t.uploader,
			t.compressor,
			t.tempDir,
			logStreamer.WithSource(actionModel.LogSource),
			t.uploadLimiter,
			logger,
		)
	case *models.EmitProgressAction:
		return steps.NewEmitProgress(
			t.stepFor(
				logStreamer,
				actionModel.Action,
				container,
				externalIP,
				internalIP,
				internalIPv6,
				ports,
				suppressExitStatusCode,
				monitorOutputWrapper,
				logger,
			),
			actionModel.StartMessage,
			actionModel.SuccessMessage,
			actionModel.FailureMessagePrefix,
			logStreamer.WithSource(actionModel.LogSource),
			logger,
		)

	case *models.TimeoutAction:
		return steps.NewTimeout(
			t.stepFor(
				logStreamer.WithSource(actionModel.LogSource),
				actionModel.Action,
				container,
				externalIP,
				internalIP,
				internalIPv6,
				ports,
				suppressExitStatusCode,
				monitorOutputWrapper,
				logger,
			),
			time.Duration(actionModel.TimeoutMs)*time.Millisecond,
			t.clock,
			logger,
		)

	case *models.TryAction:
		return steps.NewTry(
			t.stepFor(
				logStreamer.WithSource(actionModel.LogSource),
				actionModel.Action,
				container,
				externalIP,
				internalIP,
				internalIPv6,
				ports,
				suppressExitStatusCode,
				monitorOutputWrapper,
				logger,
			),
			logger,
		)

	case *models.ParallelAction:
		subSteps := make([]ifrit.Runner, len(actionModel.Actions))
		for i, action := range actionModel.Actions {
			var subStep ifrit.Runner
			if monitorOutputWrapper {
				buffer := log_streamer.NewConcurrentBuffer(bytes.NewBuffer(nil))
				bufferedLogStreamer := log_streamer.NewBufferStreamer(buffer, buffer)
				subStep = steps.NewOutputWrapper(t.stepFor(
					bufferedLogStreamer,
					action,
					container,
					externalIP,
					internalIP,
					internalIPv6,
					ports,
					suppressExitStatusCode,
					monitorOutputWrapper,
					logger,
				),
					buffer,
				)
			} else {
				subStep = t.stepFor(
					logStreamer.WithSource(actionModel.LogSource),
					action,
					container,
					externalIP,
					internalIP,
					internalIPv6,
					ports,
					suppressExitStatusCode,
					monitorOutputWrapper,
					logger,
				)
			}
			subSteps[i] = subStep
		}
		return steps.NewParallel(subSteps)

	case *models.CodependentAction:
		subSteps := make([]ifrit.Runner, len(actionModel.Actions))
		for i, action := range actionModel.Actions {
			var subStep ifrit.Runner
			if monitorOutputWrapper {
				buffer := log_streamer.NewConcurrentBuffer(bytes.NewBuffer(nil))
				bufferedLogStreamer := log_streamer.NewBufferStreamer(buffer, buffer)
				subStep = steps.NewOutputWrapper(t.stepFor(
					bufferedLogStreamer,
					action,
					container,
					externalIP,
					internalIP,
					internalIPv6,
					ports,
					suppressExitStatusCode,
					monitorOutputWrapper,
					logger,
				),
					buffer,
				)
			} else {
				subStep = t.stepFor(
					logStreamer.WithSource(actionModel.LogSource),
					action,
					container,
					externalIP,
					internalIP,
					internalIPv6,
					ports,
					suppressExitStatusCode,
					monitorOutputWrapper,
					logger,
				)
			}
			subSteps[i] = subStep
		}
		errorOnExit := true
		return steps.NewCodependent(subSteps, errorOnExit, false)

	case *models.SerialAction:
		subSteps := make([]ifrit.Runner, len(actionModel.Actions))
		for i, action := range actionModel.Actions {
			subSteps[i] = t.stepFor(
				logStreamer,
				action,
				container,
				externalIP,
				internalIP,
				internalIPv6,
				ports,
				suppressExitStatusCode,
				monitorOutputWrapper,
				logger,
			)
		}
		return steps.NewSerial(subSteps)
	}

	panic(fmt.Sprintf("unknown action: %T", action))
}

func overrideSuppressLogOutput(monitorAction *models.Action) {
	if monitorAction.RunAction != nil {
		monitorAction.RunAction.SuppressLogOutput = false
	} else if monitorAction.TryAction != nil {
		overrideSuppressLogOutput(monitorAction.TryAction.Action)
	} else if monitorAction.ParallelAction != nil {
		for _, action := range monitorAction.ParallelAction.Actions {
			overrideSuppressLogOutput(action)
		}
	} else if monitorAction.SerialAction != nil {
		for _, action := range monitorAction.SerialAction.Actions {
			overrideSuppressLogOutput(action)
		}
	} else if monitorAction.CodependentAction != nil {
		for _, action := range monitorAction.CodependentAction.Actions {
			overrideSuppressLogOutput(action)
		}
	} else if monitorAction.EmitProgressAction != nil {
		overrideSuppressLogOutput(monitorAction.EmitProgressAction.Action)
	} else if monitorAction.TimeoutAction != nil {
		overrideSuppressLogOutput(monitorAction.TimeoutAction.Action)
	}
}

func (t *transformer) StepsRunner(
	logger lager.Logger,
	container executor.Container,
	gardenContainer garden.Container,
	logStreamer log_streamer.LogStreamer,
	config Config,
) (ifrit.Runner, chan steps.ReadinessState, error) {
	var setup, action, postSetup, monitor, readinessMonitor, longLivedAction ifrit.Runner
	var substeps []ifrit.Runner

	if container.Setup != nil {
		setup = t.stepFor(
			logStreamer,
			container.Setup,
			gardenContainer,
			container.ExternalIP,
			container.InternalIP,
			container.InternalIPv6,
			container.Ports,
			false,
			false,
			logger.Session("setup"),
		)
	}
	setup = steps.NewTimedStep(logger, setup, config.MetronClient, t.clock, config.CreationStartTime)

	if len(t.postSetupHook) > 0 {
		actionModel := models.RunAction{
			Path: t.postSetupHook[0],
			Args: t.postSetupHook[1:],
			User: t.postSetupUser,
		}
		suppressExitStatusCode := false
		postSetup = steps.NewRun(
			gardenContainer,
			actionModel,
			log_streamer.NewNoopStreamer(),
			logger.Session("post-setup"),
			container.ExternalIP,
			container.InternalIP,
			container.InternalIPv6,
			container.Ports,
			t.clock,
			t.gracefulShutdownInterval,
			suppressExitStatusCode,
		)
	}

	if container.Action == nil {
		err := errors.New("container cannot have empty action")
		logger.Error("steps-runner-empty-action", err)
		return nil, nil, err
	}

	action = t.stepFor(
		logStreamer,
		container.Action,
		gardenContainer,
		container.ExternalIP,
		container.InternalIP,
		container.InternalIPv6,
		container.Ports,
		false,
		false,
		logger.Session("action"),
	)

	substeps = append(substeps, action)

	for _, sidecar := range container.Sidecars {
		substeps = append(substeps, t.stepFor(logStreamer,
			sidecar.Action,
			gardenContainer,
			container.ExternalIP,
			container.InternalIP,
			container.InternalIPv6,
			container.Ports,
			false,
			false,
			logger.Session("sidecar"),
		))
	}

	var proxyStartupChecks []ifrit.Runner
	var proxyLivenessChecks []ifrit.Runner

	if t.useContainerProxy {
		envoyStartupLogger := logger.Session("envoy-startup-check")
		envoyLivenessLogger := logger.Session("envoy-liveness-check")

		for idx, port := range config.ProxyTLSPorts {
			// add envoy startup checks
			startupSidecarName := fmt.Sprintf("%s-envoy-startup-healthcheck-%d", gardenContainer.Handle(), idx)

			step := t.createCheck(
				&container,
				gardenContainer,
				config.BindMounts,
				"",
				startupSidecarName,
				int(port),
				DefaultDeclarativeHealthcheckRequestTimeout,
				executor.TCPCheck,
				executor.IsStartupCheck,
				t.unhealthyMonitoringInterval,
				envoyStartupLogger,
				"instance proxy failed to start",
				config.MetronClient,
				false,
			)

			if t.enableContainerProxyHealthChecks {
				livenessSidecarName := fmt.Sprintf("%s-envoy-liveness-healthcheck-%d", gardenContainer.Handle(), idx)

				livenessStep := t.createCheck(
					&container,
					gardenContainer,
					config.BindMounts,
					"",
					livenessSidecarName,
					int(port),
					DefaultDeclarativeHealthcheckRequestTimeout,
					executor.TCPCheck,
					executor.IsLivenessCheck,
					t.proxyHealthCheckInterval,
					envoyLivenessLogger,
					"instance proxy health check failed",
					config.MetronClient,
					t.emitHealthCheckMetrics,
				)

				proxyLivenessChecks = append(proxyLivenessChecks, livenessStep)
			}

			proxyStartupChecks = append(proxyStartupChecks, step)
		}
	}
	var readinessChan chan steps.ReadinessState
	if container.CheckDefinition != nil {
		if container.CheckDefinition.Checks != nil {
			monitor = t.transformCheckDefinition(logger,
				&container,
				gardenContainer,
				logStreamer,
				config.BindMounts,
				proxyStartupChecks,
				proxyLivenessChecks,
				config.MetronClient,
			)

			substeps = append(substeps, monitor)
		}

		if container.CheckDefinition.ReadinessChecks != nil {
			var err error
			readinessMonitor, readinessChan, err = t.transformReadinessCheckDefinition(logger,
				&container,
				gardenContainer,
				logStreamer,
				config.BindMounts,
				proxyStartupChecks,
				config.MetronClient,
			)
			if err == nil {
				substeps = append(substeps, readinessMonitor)
			}
		}
	} else if container.Monitor != nil {
		overrideSuppressLogOutput(container.Monitor)
		monitor = steps.NewMonitor(
			func() ifrit.Runner {
				return t.stepFor(
					logStreamer,
					container.Monitor,
					gardenContainer,
					container.ExternalIP,
					container.InternalIP,
					container.InternalIPv6,
					container.Ports,
					true,
					true,
					logger.Session("monitor-run"),
				)
			},
			logger.Session("monitor"),
			t.clock,
			logStreamer,
			time.Duration(container.StartTimeoutMs)*time.Millisecond,
			t.healthyMonitoringInterval,
			t.unhealthyMonitoringInterval,
			t.healthCheckWorkPool,
			proxyStartupChecks...,
		)
		substeps = append(substeps, monitor)
	}

	if len(substeps) > 1 {
		longLivedAction = steps.NewCodependent(substeps, false, len(container.Sidecars) > 0)
	} else {
		longLivedAction = action
	}

	if t.useContainerProxy && container.EnableContainerProxy {
		containerProxyStep := t.transformContainerProxyStep(
			gardenContainer,
			container,
			logger,
			logStreamer,
			config.BindMounts,
		)
		longLivedAction = steps.NewCodependent([]ifrit.Runner{longLivedAction, containerProxyStep}, false, true)
	}

	var cumulativeStep ifrit.Runner
	if postSetup == nil {
		cumulativeStep = steps.NewSerial([]ifrit.Runner{setup, longLivedAction})
	} else {
		cumulativeStep = steps.NewSerial([]ifrit.Runner{setup, postSetup, longLivedAction})
	}

	return cumulativeStep, readinessChan, nil
}

func (t *transformer) createCheck(
	container *executor.Container,
	gardenContainer garden.Container,
	bindMounts []garden.BindMount,
	path,
	sidecarName string,
	port,
	timeout int,
	checkProtocol executor.CheckProtocol,
	checkType executor.HealthcheckType,
	interval time.Duration,
	logger lager.Logger,
	prefix string,
	metronClient loggingclient.IngressClient,
	emitMetric bool,
) ifrit.Runner {

	nofiles := healthCheckNofiles

	sourceName := HealthLogSource
	if container.CheckDefinition != nil && container.CheckDefinition.LogSource != "" {
		sourceName = container.CheckDefinition.LogSource
	}

	args := []string{
		fmt.Sprintf("-port=%d", port),
		fmt.Sprintf("-timeout=%dms", timeout),
	}

	if checkProtocol == executor.HTTPCheck {
		args = append(args, fmt.Sprintf("-uri=%s", path))
	}

	if checkType == executor.IsStartupCheck {
		args = append(args, fmt.Sprintf("-startup-interval=%s", interval))
		args = append(args, fmt.Sprintf("-startup-timeout=%s", time.Duration(container.StartTimeoutMs)*time.Millisecond))
	}

	if checkType == executor.IsLivenessCheck {
		args = append(args, fmt.Sprintf("-liveness-interval=%s", interval))
	}

	if checkType == executor.IsUntilSuccessReadinessCheck {
		args = append(args, fmt.Sprintf("-until-ready-interval=%s", interval))
	}

	if checkType == executor.IsUntilFailureReadinessCheck {
		args = append(args, fmt.Sprintf("-readiness-interval=%s", interval))
	}

	rl := models.ResourceLimits{}
	rl.SetNofile(nofiles)
	runAction := models.RunAction{
		LogSource:      sourceName,
		ResourceLimits: &rl,
		Path:           filepath.Join(HealthCheckDstPath, "healthcheck"),
		Args:           args,
	}

	buffer := log_streamer.NewConcurrentBuffer(bytes.NewBuffer(nil))
	bufferedLogStreamer := log_streamer.NewBufferStreamer(buffer, buffer)
	sidecar := steps.Sidecar{
		Name:                    sidecarName,
		Image:                   garden.ImageRef{URI: t.sidecarRootFS},
		BindMounts:              bindMounts,
		OverrideContainerLimits: &garden.ProcessLimits{},
	}
	runStep := steps.NewRunWithSidecar(gardenContainer,
		runAction,
		bufferedLogStreamer,
		logger,
		container.ExternalIP,
		container.InternalIP,
		container.InternalIPv6,
		container.Ports,
		t.clock,
		t.gracefulShutdownInterval,
		true,
		sidecar,
		container.Privileged,
	)

	var wrapperStep ifrit.Runner
	if prefix != "" {
		wrapperStep = steps.NewOutputWrapperWithPrefix(runStep, buffer, prefix)
	} else {
		wrapperStep = steps.NewOutputWrapper(runStep, buffer)
	}

	if emitMetric {
		wrapperStep = steps.NewEmitCheckFailureMetricStep(wrapperStep, checkProtocol, checkType, metronClient)
	}

	return wrapperStep
}

func (t *transformer) transformReadinessCheckDefinition(
	logger lager.Logger,
	container *executor.Container,
	gardenContainer garden.Container,
	logstreamer log_streamer.LogStreamer,
	bindMounts []garden.BindMount,
	proxyStartupChecks []ifrit.Runner,
	metronClient loggingclient.IngressClient,
) (ifrit.Runner, chan steps.ReadinessState, error) {
	var untilSuccessReadinessChecks []ifrit.Runner
	var untilFailureReadinessChecks []ifrit.Runner

	sourceName := HealthLogSource
	if container.CheckDefinition.LogSource != "" {
		sourceName = container.CheckDefinition.LogSource
	}

	logger.Info("transform-readiness-check-definitions-starting")
	defer func() {
		logger.Info("transform-readiness-check-definitions-finished")
	}()

	readinessLogger := logger.Session("readiness-check")

	for index, check := range container.CheckDefinition.ReadinessChecks {
		untilReadySidecarName := fmt.Sprintf("%s-until-ready-healthcheck-%d", gardenContainer.Handle(), index)
		readinessSidecarName := fmt.Sprintf("%s-readiness-healthcheck-%d", gardenContainer.Handle(), index)

		if err := check.Validate(); err != nil {
			logger.Error("invalid-readiness-check", err, lager.Data{"check": check})
			continue
		}

		if check.HttpCheck != nil {
			timeout, interval, path := t.applyCheckDefaults(
				int(check.HttpCheck.RequestTimeoutMs),
				time.Duration(check.HttpCheck.IntervalMs)*time.Millisecond,
				check.HttpCheck.Path,
			)

			untilSuccessReadinessChecks = append(untilSuccessReadinessChecks, t.createCheck(
				container,
				gardenContainer,
				bindMounts,
				path,
				untilReadySidecarName,
				int(check.HttpCheck.Port),
				timeout,
				executor.HTTPCheck,
				executor.IsUntilSuccessReadinessCheck,
				t.unhealthyMonitoringInterval,
				readinessLogger,
				"",
				metronClient,
				false,
			))

			untilFailureReadinessChecks = append(untilFailureReadinessChecks, t.createCheck(
				container,
				gardenContainer,
				bindMounts,
				path,
				readinessSidecarName,
				int(check.HttpCheck.Port),
				timeout,
				executor.HTTPCheck,
				executor.IsUntilFailureReadinessCheck,
				interval,
				readinessLogger,
				"",
				metronClient,
				false,
			))
		} else { // is tcp check
			timeout, interval, _ := t.applyCheckDefaults(
				int(check.TcpCheck.ConnectTimeoutMs),
				time.Duration(check.TcpCheck.IntervalMs)*time.Millisecond,
				"",
			)

			untilSuccessReadinessChecks = append(untilSuccessReadinessChecks, t.createCheck(
				container,
				gardenContainer,
				bindMounts,
				"",
				readinessSidecarName,
				int(check.TcpCheck.Port),
				timeout,
				executor.TCPCheck,
				executor.IsUntilSuccessReadinessCheck,
				t.unhealthyMonitoringInterval,
				readinessLogger,
				"",
				metronClient,
				false,
			))

			untilFailureReadinessChecks = append(untilFailureReadinessChecks, t.createCheck(
				container,
				gardenContainer,
				bindMounts,
				"",
				readinessSidecarName,
				int(check.TcpCheck.Port),
				timeout,
				executor.TCPCheck,
				executor.IsUntilFailureReadinessCheck,
				interval,
				readinessLogger,
				"",
				metronClient,
				false,
			))
		}
	}
	if len(untilSuccessReadinessChecks) == 0 {
		return nil, nil, errors.New("no-valid-readiness-checks")
	}
	readinessChan := make(chan steps.ReadinessState)
	untilSuccessReadinessCheck := steps.NewParallel(untilSuccessReadinessChecks)
	untilFailureReadinessCheck := steps.NewCodependent(untilFailureReadinessChecks, true, true)
	return steps.NewReadinessHealthCheckStep(
		untilSuccessReadinessCheck,
		untilFailureReadinessCheck,
		logstreamer.WithSource(sourceName),
		readinessChan,
		logger,
	), readinessChan, nil
}

func (t *transformer) applyCheckDefaults(timeout int, interval time.Duration, path string) (int, time.Duration, string) {
	if timeout <= 0 {
		timeout = int(t.declarativeHealthCheckDefaultTimeout / time.Millisecond)
	}

	if path == "" {
		path = "/"
	}
	// we can use the fact that time.Duration is an int64 to simplify creating the proper time.Duration object from desired number of Milliseconds
	if interval == 0 {
		interval = t.healthyMonitoringInterval
	}

	return timeout, interval, path
}

func (t *transformer) transformCheckDefinition(
	logger lager.Logger,
	container *executor.Container,
	gardenContainer garden.Container,
	logstreamer log_streamer.LogStreamer,
	bindMounts []garden.BindMount,
	proxyStartupChecks []ifrit.Runner,
	proxyLivenessChecks []ifrit.Runner,
	metronClient loggingclient.IngressClient,
) ifrit.Runner {
	var startupChecks []ifrit.Runner
	var livenessChecks []ifrit.Runner

	sourceName := HealthLogSource
	if container.CheckDefinition.LogSource != "" {
		sourceName = container.CheckDefinition.LogSource
	}

	logger.Info("transform-check-definitions-starting")
	defer func() {
		logger.Info("transform-check-definitions-finished")
	}()

	startupLogger := logger.Session("startup-check")
	livenessLogger := logger.Session("liveness-check")

	for index, check := range container.CheckDefinition.Checks {

		startupSidecarName := fmt.Sprintf("%s-startup-healthcheck-%d", gardenContainer.Handle(), index)
		livenessSidecarName := fmt.Sprintf("%s-liveness-healthcheck-%d", gardenContainer.Handle(), index)

		if err := check.Validate(); err != nil {
			logger.Error("invalid-check", err, lager.Data{"check": check})
		} else if check.HttpCheck != nil {
			timeout, interval, path := t.applyCheckDefaults(
				int(check.HttpCheck.RequestTimeoutMs),
				time.Duration(check.HttpCheck.IntervalMs)*time.Millisecond,
				check.HttpCheck.Path,
			)

			startupChecks = append(startupChecks, t.createCheck(
				container,
				gardenContainer,
				bindMounts,
				path,
				startupSidecarName,
				int(check.HttpCheck.Port),
				timeout,
				executor.HTTPCheck,
				executor.IsStartupCheck,
				t.unhealthyMonitoringInterval,
				startupLogger,
				"",
				metronClient,
				false,
			))
			livenessChecks = append(livenessChecks, t.createCheck(
				container,
				gardenContainer,
				bindMounts,
				path,
				livenessSidecarName,
				int(check.HttpCheck.Port),
				timeout,
				executor.HTTPCheck,
				executor.IsLivenessCheck,
				interval,
				livenessLogger,
				"",
				metronClient,
				t.emitHealthCheckMetrics,
			))

		} else if check.TcpCheck != nil {

			timeout, interval, _ := t.applyCheckDefaults(
				int(check.TcpCheck.ConnectTimeoutMs),
				time.Duration(check.TcpCheck.IntervalMs)*time.Millisecond,
				"", // only needed for http checks
			)

			startupChecks = append(startupChecks, t.createCheck(
				container,
				gardenContainer,
				bindMounts,
				"",
				startupSidecarName,
				int(check.TcpCheck.Port),
				timeout,
				executor.TCPCheck,
				executor.IsStartupCheck,
				t.unhealthyMonitoringInterval,
				startupLogger,
				"",
				metronClient,
				false,
			))
			livenessChecks = append(livenessChecks, t.createCheck(
				container,
				gardenContainer,
				bindMounts,
				"",
				livenessSidecarName,
				int(check.TcpCheck.Port),
				timeout,
				executor.TCPCheck,
				executor.IsLivenessCheck,
				interval,
				livenessLogger,
				"",
				metronClient,
				t.emitHealthCheckMetrics,
			))
		}
	}

	startupCheck := steps.NewParallel(append(proxyStartupChecks, startupChecks...))
	livenessChecks = append(livenessChecks, proxyLivenessChecks...)
	livenessCheck := steps.NewCodependent(livenessChecks, false, false)

	return steps.NewHealthCheckStep(
		startupCheck,
		livenessCheck,
		logger,
		t.clock,
		logstreamer,
		logstreamer.WithSource(sourceName),
		time.Duration(container.StartTimeoutMs)*time.Millisecond,
	)
}

func (t *transformer) transformContainerProxyStep(
	container garden.Container,
	execContainer executor.Container,
	logger lager.Logger,
	streamer log_streamer.LogStreamer,
	bindMounts []garden.BindMount,
) ifrit.Runner {

	envoyArgs := []string{
		"-c", "/etc/cf-assets/envoy_config/envoy.yaml",
		"--drain-time-s", strconv.Itoa(int(t.drainWait.Seconds())),
		"--log-level", "critical",
	}

	runAction := envoyRunAction(envoyArgs)

	sidecar := steps.Sidecar{
		Image:      garden.ImageRef{URI: t.sidecarRootFS},
		BindMounts: bindMounts,
		Name:       fmt.Sprintf("%s-envoy", container.Handle()),
	}

	proxyLogger := logger.Session("proxy")

	return steps.NewBackground(steps.NewRunWithSidecar(
		container,
		runAction,
		streamer.WithSource("PROXY"),
		proxyLogger,
		execContainer.ExternalIP,
		execContainer.InternalIP,
		execContainer.InternalIPv6,
		execContainer.Ports,
		t.clock,
		t.gracefulShutdownInterval,
		false,
		sidecar,
		execContainer.Privileged,
	), proxyLogger)
}
