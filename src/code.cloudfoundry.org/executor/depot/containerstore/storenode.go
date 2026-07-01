package containerstore

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/event"
	"code.cloudfoundry.org/executor/depot/log_streamer"
	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/executor/depot/transformer"
	"code.cloudfoundry.org/executor/initializer/configuration"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/volman"
	"github.com/hashicorp/errwrap"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
)

const DownloadCachedDependenciesFailed = "failed to download cached artifacts"
const ContainerCreationFailedMessage = "failed to create container"
const ContainerExpirationMessage = "expired container"
const ContainerMissingMessage = "missing garden container"
const VolmanMountFailed = "failed to mount volume"
const BindMountCleanupFailed = "failed to cleanup bindmount artifacts"
const CredDirFailed = "failed to create credentials directory"
const VolumeMountedFileFailed = "failed to create volume mounted files"

const ContainerCompletedCount = "ContainerCompletedCount"
const ContainerExitedOnTimeoutCount = "ContainerExitedOnTimeoutCount"

// To be deprecated
const (
	GardenContainerCreationSucceededDuration    = "GardenContainerCreationSucceededDuration"
	GardenContainerCreationFailedDuration       = "GardenContainerCreationFailedDuration"
	GardenContainerDestructionSucceededDuration = "GardenContainerDestructionSucceededDuration"
	GardenContainerDestructionFailedDuration    = "GardenContainerDestructionFailedDuration"
	ContainerSetupFailedDuration                = "ContainerSetupFailedDuration"
)

//go:generate counterfeiter -o containerstorefakes/fake_proxymanager.go . ProxyManager
type ProxyManager interface {
	CredentialHandler
	ProxyPorts(lager.Logger, *executor.Container) ([]executor.ProxyPortMapping, []uint16, error)
}

type storeNode struct {
	modifiedIndex               uint
	hostTrustedCertificatesPath string
	metronClient                loggingclient.IngressClient

	// infoLock protects modifying info and swapping gardenContainer pointers
	infoLock           *sync.Mutex
	info               executor.Container
	bindMountCacheKeys []BindMountCacheKey
	gardenContainer    garden.Container

	clock clock.Clock

	// opLock serializes public methods that involve garden interactions
	opLock                                *sync.Mutex
	gardenClientFactory                   GardenClientFactory
	dependencyManager                     DependencyManager
	volumeManager                         volman.Manager
	credManager                           CredManager
	logManager                            LogManager
	generateLogMetrics                    bool
	logStreamer                           log_streamer.LogStreamer
	eventEmitter                          event.Hub
	transformer                           transformer.Transformer
	process                               ifrit.Process
	config                                *ContainerConfig
	rootFSSizer                           configuration.RootFSSizer
	declarativeHealthcheckPath            string
	proxyConfigHandler                    ProxyManager
	bindMounts                            []garden.BindMount
	cellID                                string
	enableUnproxiedPortMappings           bool
	advertisePreferenceForInstanceAddress bool

	destroying, stopping int32

	startTime         time.Time
	regenerateCertsCh chan struct{}

	volumeMountedFiles VolumeMountedFilesImplementor

	jsonMarshaller func(any) ([]byte, error)
}

func newStoreNode(
	config *ContainerConfig,
	declarativeHealthcheckPath string,
	container executor.Container,
	gardenClientFactory GardenClientFactory,
	clock clock.Clock,
	dependencyManager DependencyManager,
	volumeManager volman.Manager,
	credManager CredManager,
	logManager LogManager,
	generateLogMetrics bool,
	eventEmitter event.Hub,
	transformer transformer.Transformer,
	hostTrustedCertificatesPath string,
	metronClient loggingclient.IngressClient,
	proxyConfigHandler ProxyManager,
	rootFSSizer configuration.RootFSSizer,
	cellID string,
	enableUnproxiedPortMappings bool,
	advertisePreferenceForInstanceAddress bool,
	volumeMountedFiles VolumeMountedFilesImplementor,
	jsonMarshaller func(any) ([]byte, error),
) *storeNode {
	return &storeNode{
		config:                                config,
		info:                                  container,
		infoLock:                              &sync.Mutex{},
		opLock:                                &sync.Mutex{},
		gardenClientFactory:                   gardenClientFactory,
		clock:                                 clock,
		dependencyManager:                     dependencyManager,
		volumeManager:                         volumeManager,
		credManager:                           credManager,
		logManager:                            logManager,
		generateLogMetrics:                    generateLogMetrics,
		eventEmitter:                          eventEmitter,
		transformer:                           transformer,
		modifiedIndex:                         0,
		hostTrustedCertificatesPath:           hostTrustedCertificatesPath,
		metronClient:                          metronClient,
		declarativeHealthcheckPath:            declarativeHealthcheckPath,
		proxyConfigHandler:                    proxyConfigHandler,
		rootFSSizer:                           rootFSSizer,
		cellID:                                cellID,
		enableUnproxiedPortMappings:           enableUnproxiedPortMappings,
		advertisePreferenceForInstanceAddress: advertisePreferenceForInstanceAddress,
		regenerateCertsCh:                     make(chan struct{}, 1),
		volumeMountedFiles:                    volumeMountedFiles,
		jsonMarshaller:                        jsonMarshaller,
	}
}

func (n *storeNode) acquireOpLock(logger lager.Logger) {
	startTime := time.Now()
	n.opLock.Lock()
	logger.Debug("ops-lock-aquired", lager.Data{"lock-wait-time": time.Since(startTime).String()})
}

func (n *storeNode) releaseOpLock(logger lager.Logger) {
	n.opLock.Unlock()
	logger.Debug("ops-lock-released")
}

func (n *storeNode) Info() executor.Container {
	n.infoLock.Lock()
	defer n.infoLock.Unlock()

	return n.info.Copy()
}

func (n *storeNode) GetFiles(logger lager.Logger, sourcePath string) (io.ReadCloser, error) {
	n.infoLock.Lock()
	gc := n.gardenContainer
	n.infoLock.Unlock()
	if gc == nil {
		return nil, executor.ErrContainerNotFound
	}
	return gc.StreamOut(garden.StreamOutSpec{Path: sourcePath, User: "root"})
}

func (n *storeNode) Initialize(logger lager.Logger, req *executor.RunRequest) error {
	logger = logger.Session("node-initialize")
	n.infoLock.Lock()
	defer n.infoLock.Unlock()

	err := n.info.TransitionToInitialize(req)
	if err != nil {
		logger.Error("failed-to-initialize", err)
		return err
	}
	return nil
}

func (n *storeNode) Create(logger lager.Logger, traceID string) error {
	logger = logger.Session("node-create")
	n.acquireOpLock(logger)
	defer n.releaseOpLock(logger)

	n.infoLock.Lock()
	info := n.info.Copy()
	n.infoLock.Unlock()

	if !info.ValidateTransitionTo(executor.StateCreated) {
		logger.Error("failed-to-create", executor.ErrInvalidTransition)
		return executor.ErrInvalidTransition
	}

	createContainer := func() error {
		mounts, err := n.dependencyManager.DownloadCachedDependencies(logger, info.CachedDependencies, info.LogConfig, n.metronClient)
		if err != nil {
			n.complete(logger, traceID, true, DownloadCachedDependenciesFailed, true)
			return err
		}

		n.bindMounts = mounts.GardenBindMounts

		if n.hostTrustedCertificatesPath != "" && info.TrustedSystemCertificatesPath != "" {
			mount := garden.BindMount{
				SrcPath: n.hostTrustedCertificatesPath,
				DstPath: info.TrustedSystemCertificatesPath,
				Mode:    garden.BindMountModeRO,
				Origin:  garden.BindMountOriginHost,
			}
			n.bindMounts = append(n.bindMounts, mount)

			info.Env = append(info.Env, executor.EnvironmentVariable{Name: "CF_SYSTEM_CERT_PATH", Value: info.TrustedSystemCertificatesPath})
		}

		volumeMounts, err := n.mountVolumes(logger, info)
		if err != nil {
			var failMsg string
			if safeError, ok := err.(volman.SafeError); ok {
				failMsg = fmt.Sprintf("%s, errors: %s", VolmanMountFailed, safeError.Error())
			} else {
				failMsg = VolmanMountFailed
			}
			logger.Error("failed-to-mount-volume", err)
			n.complete(logger, traceID, true, failMsg, true)
			return err
		}
		n.bindMounts = append(n.bindMounts, volumeMounts...)

		credMounts, envs, err := n.credManager.CreateCredDir(logger, n.info)
		if err != nil {
			n.complete(logger, traceID, true, CredDirFailed, true)
			return err
		}

		if len(info.VolumeMountedFiles) > 0 {
			volumeMountedFile, err := n.volumeMountedFiles.CreateDir(logger, info)
			if err != nil {
				n.complete(logger, traceID, true, VolumeMountedFileFailed, true)
				return err
			}

			n.bindMounts = append(n.bindMounts, volumeMountedFile...)
		}

		n.bindMounts = append(n.bindMounts, credMounts...)

		info.Env = append(info.Env, envs...)

		logger.Info("adding-healthcheck-bindmounts")
		n.bindMounts = append(n.bindMounts, garden.BindMount{
			Origin:  garden.BindMountOriginHost,
			SrcPath: n.declarativeHealthcheckPath,
			DstPath: "/etc/cf-assets/healthcheck",
		})

		sourceName, tags := n.info.LogConfig.GetSourceNameAndTagsForLogging()
		metricErr := n.metronClient.SendAppLog(fmt.Sprintf("Cell %s creating container for instance %s", n.cellID, n.Info().Guid), sourceName, tags)
		if metricErr != nil {
			logger.Debug("failed-to-emit-appLog", lager.Data{"error": metricErr})
		}
		gardenContainer, err := n.createGardenContainer(logger, traceID, &info)
		if err != nil {
			metricErr = n.metronClient.SendAppErrorLog(fmt.Sprintf("Cell %s failed to create container for instance %s: %s", n.cellID, n.Info().Guid, err.Error()), sourceName, tags)
			if metricErr != nil {
				logger.Debug("failed-to-emit-appErrorLog", lager.Data{"error": metricErr})
			}
			n.complete(logger, traceID, true, fmt.Sprintf("%s: %s", ContainerCreationFailedMessage, err.Error()), true)
			return err
		}
		metricErr = n.metronClient.SendAppLog(fmt.Sprintf("Cell %s successfully created container for instance %s", n.cellID, n.Info().Guid), sourceName, tags)
		if metricErr != nil {
			logger.Debug("failed-to-emit-appLog", lager.Data{"error": metricErr})
		}

		n.infoLock.Lock()
		n.gardenContainer = gardenContainer
		n.info = info
		err = n.info.TransitionToCreate()
		n.bindMountCacheKeys = mounts.CacheKeys
		n.infoLock.Unlock()

		return err
	}

	n.startTime = n.clock.Now()
	err := createContainer()
	if err != nil {
		duration := n.clock.Since(n.startTime)
		logger.Error("container-setup-failed", err, lager.Data{"duration": duration})
		go n.metronClient.SendDuration(ContainerSetupFailedDuration, duration)
	}

	return err
}

func (n *storeNode) mountVolumes(logger lager.Logger, info executor.Container) ([]garden.BindMount, error) {
	gardenMounts := []garden.BindMount{}
	for _, volume := range info.VolumeMounts {
		hostMount, err := n.volumeManager.Mount(logger, volume.Driver, volume.VolumeId, info.Guid, volume.Config)
		if err != nil {
			return nil, err
		}
		gardenMounts = append(gardenMounts,
			garden.BindMount{
				SrcPath: hostMount.Path,
				DstPath: volume.ContainerPath,
				Origin:  garden.BindMountOriginHost,
				Mode:    garden.BindMountMode(volume.Mode),
			})
	}
	return gardenMounts, nil
}

func (n *storeNode) gardenProperties(container *executor.Container) (garden.Properties, error) {
	properties := garden.Properties{}
	if container.Network != nil {
		for key, value := range container.Network.Properties {
			properties["network."+key] = value
		}
	}
	properties[executor.ContainerOwnerProperty] = n.config.OwnerName
	logConfig, err := n.jsonMarshaller(container.LogConfig)
	if err != nil {
		return nil, err
	}
	properties["log_config"] = string(logConfig)

	return properties, nil
}

func dedupPorts(ports []executor.PortMapping) []executor.PortMapping {
	seen := make(map[uint16]bool, len(ports))
	deduped := make([]executor.PortMapping, 0, len(ports))
	for _, port := range ports {
		if seen[port.ContainerPort] {
			continue
		}
		seen[port.ContainerPort] = true
		deduped = append(deduped, port)
	}
	return deduped
}

func (n *storeNode) createGardenContainer(logger lager.Logger, traceID string, info *executor.Container) (garden.Container, error) {
	netOutRules, err := convertEgressToNetOut(logger, info.EgressRules)
	if err != nil {
		return nil, err
	}

	info.Ports = dedupPorts(info.Ports)

	proxyPortMapping, extraPorts, err := n.proxyConfigHandler.ProxyPorts(logger, info)
	if err != nil {
		return nil, err
	}
	for _, port := range extraPorts {
		info.Ports = append(info.Ports, executor.PortMapping{
			ContainerPort: port,
		})
	}

	netInPorts := info.Ports
	if !n.enableUnproxiedPortMappings {
		netInPorts = []executor.PortMapping{}
		for _, port := range extraPorts {
			netInPorts = append(netInPorts, executor.PortMapping{
				ContainerPort: port,
			})
		}
	}

	netInRules := make([]garden.NetIn, 0, len(netInPorts))
	for _, portMapping := range netInPorts {
		if portMapping.ContainerPort != C2CTLSPort {
			netInRules = append(netInRules, garden.NetIn{
				HostPort:      uint32(portMapping.HostPort),
				ContainerPort: uint32(portMapping.ContainerPort),
			})
		}
	}

	diskLimitBytesHard := uint64(info.DiskMB) * 1024 * 1024
	if diskLimitBytesHard != 0 {
		diskLimitBytesHard += n.rootFSSizer.RootFSSizeFromPath(info.RootFSPath)
	}

	gardenProperties, err := n.gardenProperties(info)
	if err != nil {
		logger.Error("failed-computing-garden-properties", err)
		return nil, err
	}

	containerSpec := garden.ContainerSpec{
		Handle:     info.Guid,
		Privileged: info.Privileged,
		Image: garden.ImageRef{
			URI:      info.RootFSPath,
			Username: info.ImageUsername,
			Password: info.ImagePassword,
		},
		Env:        convertEnvVars(info.Env),
		BindMounts: n.bindMounts,
		Limits: garden.Limits{
			Memory: garden.MemoryLimits{
				LimitInBytes: uint64(info.MemoryMB * 1024 * 1024),
			},
			Disk: garden.DiskLimits{
				ByteHard:  diskLimitBytesHard,
				InodeHard: n.config.INodeLimit,
				Scope:     garden.DiskLimitScopeTotal,
			},
			Pid: garden.PidLimits{
				Max: uint64(info.MaxPids),
			},
			CPU: garden.CPULimits{
				LimitInShares: uint64(float64(n.config.MaxCPUShares) * float64(info.CPUWeight) / 100.0),
			},
		},
		Properties: gardenProperties,
		NetIn:      netInRules,
		NetOut:     netOutRules,
	}

	if n.config.SetCPUWeight {
		containerSpec.Limits.CPU.Weight = uint64(info.MemoryMB)
	}

	gardenContainer, err := createContainer(logger, containerSpec, n.gardenClientFactory.NewGardenClient(logger, traceID), n.metronClient)
	if err != nil {
		return nil, err
	}

	containerInfo, err := gardenContainer.Info()
	if err != nil {
		if err := n.destroyContainer(logger, traceID); err != nil {
			logger.Error("failed-to-destroy-container", err)
		}
		return nil, err
	}

	info.Ports = n.portMappingFromContainerInfo(containerInfo, info.Ports, proxyPortMapping)
	info.ExternalIP = containerInfo.ExternalIP
	info.InternalIP = containerInfo.ContainerIP
	info.InternalIPv6 = containerInfo.ContainerIPv6
	info.AdvertisePreferenceForInstanceAddress = n.advertisePreferenceForInstanceAddress

	logger.Debug("container-created", lager.Data{"info": info})

	info.MemoryLimit = containerSpec.Limits.Memory.LimitInBytes
	info.DiskLimit = containerSpec.Limits.Disk.ByteHard

	return gardenContainer, nil
}

func (n *storeNode) portMappingFromContainerInfo(
	containerInfo garden.ContainerInfo,
	appPorts []executor.PortMapping,
	proxyToAppPort []executor.ProxyPortMapping,
) []executor.PortMapping {
	proxyPorts := make(map[uint16]struct{})

	// construct a map from unproxied to proxied port
	appToProxyPortMappings := make(map[uint16][]uint16)
	for _, mapping := range proxyToAppPort {
		appToProxyPortMappings[mapping.AppPort] = append(appToProxyPortMappings[mapping.AppPort], mapping.ProxyPort)
		proxyPorts[mapping.ProxyPort] = struct{}{}
	}

	// construct a map from container to host port
	containerToHostPortMappings := make(map[uint16]uint16)
	for _, portMapping := range containerInfo.MappedPorts {
		containerToHostPortMappings[uint16(portMapping.ContainerPort)] = uint16(portMapping.HostPort)
	}

	// use the above two mappings to construct a list of PortMappings containing
	// the following information for each application port:
	//
	// - app-container-port
	// - app-host-port
	// - proxy-container-port
	// - proxy-host-port
	ports := []executor.PortMapping{}
	for _, portMapping := range appPorts {
		appPort := portMapping.ContainerPort

		// skip if this is a proxy port
		if _, ok := proxyPorts[appPort]; ok {
			continue
		}

		hostPort := containerToHostPortMappings[appPort]
		proxyContainerPorts := appToProxyPortMappings[appPort]
		if len(proxyContainerPorts) > 0 {
			for _, proxyContainerPort := range proxyContainerPorts {
				proxyHostPort := containerToHostPortMappings[proxyContainerPort]
				ports = append(ports, executor.PortMapping{
					HostPort:              hostPort,
					ContainerPort:         appPort,
					ContainerTLSProxyPort: proxyContainerPort,
					HostTLSProxyPort:      proxyHostPort,
				})
			}
		} else {
			ports = append(ports, executor.PortMapping{
				HostPort:      hostPort,
				ContainerPort: appPort,
			})
		}
	}

	return ports
}

func (n *storeNode) Run(logger lager.Logger, traceID string) error {
	logger = logger.Session("node-run")

	n.acquireOpLock(logger)
	defer n.releaseOpLock(logger)

	if n.info.State != executor.StateCreated {
		logger.Error("failed-to-run", executor.ErrInvalidTransition)
		return executor.ErrInvalidTransition
	}

	metricReportInverval := n.config.MetricReportInterval
	if !n.generateLogMetrics {
		metricReportInverval = 0
	}
	n.logStreamer = n.logManager.NewLogStreamer(n.info.LogConfig, n.metronClient, n.config.MaxLogLinesPerSecond, n.info.LogRateLimitBytesPerSecond, metricReportInverval)

	credManagerRunner := n.credManager.Runner(logger, n, n.regenerateCertsCh)

	proxyTLSPorts := make([]uint16, len(n.info.Ports))
	for i, p := range n.info.Ports {
		proxyTLSPorts[i] = p.ContainerTLSProxyPort
	}
	cfg := transformer.Config{
		BindMounts:        n.bindMounts,
		ProxyTLSPorts:     proxyTLSPorts,
		CreationStartTime: n.startTime,
		MetronClient:      n.metronClient,
	}
	runner, readinessChan, err := n.transformer.StepsRunner(logger, n.info, n.gardenContainer, n.logStreamer, cfg)
	if err != nil {
		return err
	}

	group := grouper.NewQueueOrdered(os.Interrupt, grouper.Members{
		{Name: "cred-manager-runner", Runner: credManagerRunner},
		{Name: "runner", Runner: runner},
	})
	n.process = ifrit.Background(group)
	go n.run(logger, readinessChan, n.logStreamer, traceID)
	return nil
}

func (n *storeNode) completeWithError(logger lager.Logger, traceID string, err error) {
	exitTrace, ok := err.(grouper.ErrorTrace)
	if ok {
		for _, event := range exitTrace {
			err := event.Err
			errorTrace, ok := err.(grouper.ErrorTrace)
			if ok {
				n.completeWithError(logger, traceID, errorTrace)
				return
			} else if err != nil {
				if event.Member.Name != "runner" {
					err = errors.New(event.Member.Name + " exited: " + err.Error())
				}
				n.completeWithError(logger, traceID, err)
			}
		}
		return
	}

	var errorStr string
	if err != nil {
		errorStr = err.Error()
		if errwrap.ContainsType(err, new(steps.ExceededGracefulShutdownIntervalError)) || errwrap.ContainsType(err, new(steps.ExitTimeoutError)) {
			metricErr := n.metronClient.IncrementCounter(ContainerExitedOnTimeoutCount)
			if metricErr != nil {
				logger.Debug("failed-to-emit-container-exited-on-timeout-metric", lager.Data{"error": metricErr})
			}
		}
	}

	if errorStr != "" {
		n.complete(logger, traceID, true, errorStr, false)
		return
	}
	n.complete(logger, traceID, false, "", false)
}

func (n *storeNode) run(logger lager.Logger, readinessCh chan steps.ReadinessState, logStreamer log_streamer.LogStreamer, traceID string) {
	defer logStreamer.Stop()

	// wait for container runner to start
	logger.Debug("execute-process")
	defer n.metronClient.IncrementCounter(ContainerCompletedCount)
	select {
	case err := <-n.process.Wait():
		n.completeWithError(logger, traceID, err)
		return
	case <-n.process.Ready():
		// fallthrough, healthcheck passed
	}
	logger.Debug("healthcheck-passed")

	n.infoLock.Lock()
	n.info.State = executor.StateRunning
	if readinessCh == nil {
		n.info.Routable = true
	} else {
		n.info.Routable = false
	}
	info := n.info.Copy()
	n.infoLock.Unlock()

	n.eventEmitter.Emit(executor.NewContainerRunningEvent(info, traceID))

	if readinessCh == nil {
		logger.Debug("readiness-healthcheck-is-not-defined")

		err := <-n.process.Wait()
		n.completeWithError(logger, traceID, err)
		return
	}

	logger.Debug("readiness-healthcheck-defined")
	for {
		select {
		case msg := <-readinessCh: // This goroutine will automatically exit when it's calling function (run) exits.
			switch msg {
			case steps.IsNotReady:
				logger.Debug("readiness-healthcheck-failed")
				n.infoLock.Lock()
				n.info.Routable = false
				n.infoLock.Unlock()

			case steps.IsReady:
				logger.Debug("readiness-healthcheck-passed")
				n.infoLock.Lock()
				n.info.Routable = true
				n.infoLock.Unlock()
			}

			n.infoLock.Lock()
			info := n.info.Copy()
			n.infoLock.Unlock()
			go n.eventEmitter.Emit(executor.NewContainerRunningEvent(info, traceID))

		case err := <-n.process.Wait():
			n.completeWithError(logger, traceID, err)
			return
		}
	}
}

func (n *storeNode) Update(logger lager.Logger, req *executor.UpdateRequest) error {
	logger = logger.Session("node-update")

	n.acquireOpLock(logger)
	defer n.releaseOpLock(logger)

	n.infoLock.Lock()

	if req.InternalRoutes != nil {
		n.info.InternalRoutes = req.InternalRoutes
	}

	var logConfigJSON []byte
	if req.MetricTags != nil {
		n.info.LogConfig.Tags = req.MetricTags
		n.info.MetricsConfig.Tags = req.MetricTags

		var err error
		logConfigJSON, err = n.jsonMarshaller(n.info.LogConfig)
		if err != nil {
			logger.Error("failed-to-serialize-log-config", err, lager.Data{"guid": req.Guid})
			n.infoLock.Unlock()
			return err
		}
	}

	n.infoLock.Unlock()

	if req.InternalRoutes != nil {
		n.regenerateCertsCh <- struct{}{}
	}

	if req.MetricTags != nil {
		n.logStreamer.UpdateTags(req.MetricTags)

		err := n.gardenContainer.SetProperty("log_config", string(logConfigJSON))
		if err != nil {
			logger.Error("failed-to-set-log-config-property", err, lager.Data{"guid": req.Guid})
			return err
		}
	}
	return nil
}

func (n *storeNode) Stop(logger lager.Logger, traceID string) {
	if !atomic.CompareAndSwapInt32(&n.stopping, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&n.stopping, 0)

	logger = logger.Session("node-stop")
	n.acquireOpLock(logger)
	defer n.releaseOpLock(logger)

	n.stop(logger, traceID)
}

func (n *storeNode) stop(logger lager.Logger, traceID string) {
	n.infoLock.Lock()
	stopped := n.info.RunResult.Stopped
	n.info.RunResult.Stopped = true
	n.infoLock.Unlock()
	if n.process != nil {
		if !stopped {
			sourceName, tags := n.info.LogConfig.GetSourceNameAndTagsForLogging()
			metricErr := n.metronClient.SendAppLog(fmt.Sprintf("Cell %s stopping instance %s", n.cellID, n.Info().Guid), sourceName, tags)
			if metricErr != nil {
				logger.Debug("failed-to-emit-app-log", lager.Data{"error": metricErr})
			}
		}

		n.process.Signal(os.Interrupt)
		logger.Debug("signalled-process")
	} else {
		if n.info.State != executor.StateCompleted {
			n.complete(logger, traceID, true, "stopped-before-running", false)
		}
	}
}

func (n *storeNode) Destroy(logger lager.Logger, traceID string) error {
	if !atomic.CompareAndSwapInt32(&n.destroying, 0, 1) {
		return nil
	}
	defer atomic.StoreInt32(&n.destroying, 0)

	logger = logger.Session("node-destroy")
	n.acquireOpLock(logger)
	defer n.releaseOpLock(logger)

	n.stop(logger, traceID)

	if n.process != nil {
		<-n.process.Wait()
	}

	n.infoLock.Lock()
	info := n.info.Copy()
	n.infoLock.Unlock()

	sourceName, tags := n.info.LogConfig.GetSourceNameAndTagsForLogging()

	metricErr := n.metronClient.SendAppLog(fmt.Sprintf("Cell %s destroying container for instance %s", n.cellID, info.Guid), sourceName, tags)
	if metricErr != nil {
		logger.Debug("failed-to-emit-app-log", lager.Data{"error": metricErr})
	}
	// ensure these directories are removed even if the container fails to destroy
	defer n.removeCredsDir(logger, info)
	defer n.umountVolumeMounts(logger, info)
	defer n.removeVolumeMountedFiles(logger, info)

	err := n.destroyContainer(logger, traceID)
	if err != nil {
		metricErr = n.metronClient.SendAppLog(fmt.Sprintf("Cell %s failed to destroy container for instance %s", n.cellID, info.Guid), sourceName, tags)
		if metricErr != nil {
			logger.Debug("failed-to-emit-app-log", lager.Data{"error": metricErr})
		}
		return err
	}
	metricErr = n.metronClient.SendAppLog(fmt.Sprintf("Cell %s successfully destroyed container for instance %s", n.cellID, info.Guid), sourceName, tags)
	if metricErr != nil {
		logger.Debug("failed-to-emit-app-log", lager.Data{"error": metricErr})
	}

	cacheKeys := n.bindMountCacheKeys

	var bindMountCleanupErr error
	err = n.dependencyManager.ReleaseCachedDependencies(logger, cacheKeys)
	if err != nil {
		logger.Error("failed-to-release-cached-deps", err)
		bindMountCleanupErr = errors.New(BindMountCleanupFailed)
	}

	return bindMountCleanupErr
}

func (n *storeNode) destroyContainer(logger lager.Logger, traceID string) error {
	logger.Debug("destroying-garden-container")

	startTime := time.Now()
	err := n.gardenClientFactory.NewGardenClient(logger, traceID).Destroy(n.info.Guid)
	destroyDuration := time.Since(startTime)

	if err != nil {
		if _, ok := err.(garden.ContainerNotFoundError); ok {
			logger.Error("container-not-found-in-garden", err)
		} else if err.Error() == server.ErrConcurrentDestroy.Error() {
			logger.Error("container-destroy-in-progress", err)
		} else {
			logger.Error("failed-to-destroy-container-in-garden", err)
			logger.Info("failed-to-destroy-container-in-garden", lager.Data{
				"destroy-took": destroyDuration.String(),
			})
			if err := n.metronClient.SendDuration(GardenContainerDestructionFailedDuration, destroyDuration); err != nil {
				logger.Error("failed-to-send-duration", err, lager.Data{"metric-name": GardenContainerDestructionFailedDuration})
			}
			return err
		}
	}

	logger.Info("destroyed-container-in-garden", lager.Data{
		"destroy-took": destroyDuration.String(),
	})
	if err := n.metronClient.SendDuration(GardenContainerDestructionSucceededDuration, destroyDuration); err != nil {
		logger.Error("failed-to-send-duration", err, lager.Data{"metric-name": GardenContainerDestructionSucceededDuration})
	}
	return nil
}

func (n *storeNode) Expire(logger lager.Logger, traceID string, now time.Time) bool {
	n.infoLock.Lock()
	defer n.infoLock.Unlock()

	if n.info.State != executor.StateReserved {
		return false
	}

	lifespan := now.Sub(time.Unix(0, n.info.AllocatedAt))
	if lifespan >= n.config.ReservedExpirationTime {
		n.info.TransitionToComplete(true, ContainerExpirationMessage, false)
		go n.eventEmitter.Emit(executor.NewContainerCompleteEvent(n.info, traceID))
		return true
	}

	return false
}

// returns true if the container was reaped (i.e. a container was previously
// created in garden but disappeared)
func (n *storeNode) Reap(logger lager.Logger, traceID string) bool {
	n.infoLock.Lock()
	defer n.infoLock.Unlock()

	if n.info.IsCreated() {
		// ensure these directories are removed even if the container fails to destroy
		n.removeCredsDir(logger, n.info.Copy())

		n.info.TransitionToComplete(true, ContainerMissingMessage, false)
		go n.eventEmitter.Emit(executor.NewContainerCompleteEvent(n.info, traceID))
		return true
	}

	return false
}

func (n *storeNode) complete(logger lager.Logger, traceID string, failed bool, failureReason string, retryable bool) {
	logger.Debug("node-complete", lager.Data{"failed": failed, "reason": failureReason})
	n.infoLock.Lock()
	defer n.infoLock.Unlock()
	n.info.TransitionToComplete(failed, failureReason, retryable)
	go n.eventEmitter.Emit(executor.NewContainerCompleteEvent(n.info, traceID))
}

func (n *storeNode) removeCredsDir(logger lager.Logger, info executor.Container) {
	err := n.credManager.RemoveCredDir(logger, info)
	if err != nil {
		logger.Error("failed-to-delete-container-proxy-config-dir", err)
	}
}

func (n *storeNode) removeVolumeMountedFiles(logger lager.Logger, info executor.Container) {
	err := n.volumeMountedFiles.RemoveDir(logger, info)
	if err != nil {
		logger.Error("failed-to-delete-volume-mounted-files-config-dir", err)
	}
}

func (n *storeNode) umountVolumeMounts(logger lager.Logger, info executor.Container) {
	for _, volume := range info.VolumeMounts {
		err := n.volumeManager.Unmount(logger, volume.Driver, volume.VolumeId, info.Guid)
		if err != nil {
			logger.Error("failed-to-unmount-volume", err)
		}
	}
}

func createContainer(logger lager.Logger, spec garden.ContainerSpec, client garden.Client, metronClient loggingclient.IngressClient) (garden.Container, error) {
	logger.Info("creating-container-in-garden")
	startTime := time.Now()
	container, err := client.Create(spec)
	createDuration := time.Since(startTime)
	if err != nil {
		logger.Error("failed-to-create-container-in-garden", err)
		logger.Info("failed-to-create-container-in-garden", lager.Data{
			"create-took": createDuration.String(),
		})
		if err := metronClient.SendDuration(GardenContainerCreationFailedDuration, createDuration); err != nil {
			logger.Error("failed-to-send-duration", err, lager.Data{"metric-name": GardenContainerCreationFailedDuration})
		}
		return nil, err
	}
	logger.Info("created-container-in-garden", lager.Data{"create-took": createDuration.String()})
	if err := metronClient.SendDuration(GardenContainerCreationSucceededDuration, createDuration); err != nil {
		logger.Error("failed-to-send-duration", err, lager.Data{"metric-name": GardenContainerCreationSucceededDuration})
	}
	return container, nil
}
