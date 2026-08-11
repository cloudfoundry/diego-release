package initializer

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/archiver/compressor"
	"code.cloudfoundry.org/cacheddownloader"
	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/containermetrics"
	"code.cloudfoundry.org/executor/depot"
	"code.cloudfoundry.org/executor/depot/containerstore"
	"code.cloudfoundry.org/executor/depot/event"
	"code.cloudfoundry.org/executor/depot/metrics"
	"code.cloudfoundry.org/executor/depot/transformer"
	"code.cloudfoundry.org/executor/depot/uploader"
	"code.cloudfoundry.org/executor/gardenhealth"
	"code.cloudfoundry.org/executor/guidgen"
	"code.cloudfoundry.org/executor/initializer/configuration"
	"code.cloudfoundry.org/garden"
	GardenClient "code.cloudfoundry.org/garden/client"
	GardenConnection "code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/volman/vollocal"
	"code.cloudfoundry.org/workpool"
	"github.com/google/shlex"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
)

const (
	PingGardenInterval                = time.Second
	StalledMetricHeartbeatInterval    = 5 * time.Second
	StalledGardenDuration             = "StalledGardenDuration"
	maxConcurrentUploads              = 5
	metricsReportInterval             = 1 * time.Minute
	megabytesToBytes                  = 1024 * 1024
	defaultMinCachePartitionFreeBytes = 5 * 1024 * 1024 * 1024
)

type executorContainers struct {
	gardenClient garden.Client
	owner        string
}

func (containers *executorContainers) Containers() ([]garden.Container, error) {
	return containers.gardenClient.Containers(garden.Properties{
		executor.ContainerOwnerProperty: containers.owner,
		executor.ContainerStateProperty: "all",
	})
}

var (
	creationWorkPool, deletionWorkPool *workpool.WorkPool
	metricsWorkPool, readWorkPool      *workpool.WorkPool
)

func Initialize(
	logger lager.Logger,
	config ExecutorConfig,
	cellID string,
	zone string,
	rootFSes map[string]string,
	sidecarRootFSPath string,
	metronClient loggingclient.IngressClient,
	clock clock.Clock,
) (
	executor.Client,
	*containermetrics.StatsReporter,
	grouper.Members,
	error,
) {
	logger.Info("garden-healthcheck-rootfs", lager.Data{"rootfs": sidecarRootFSPath})

	postSetupHook, err := shlex.Split(config.PostSetupHook)
	if err != nil {
		logger.Error("failed-to-parse-post-setup-hook", err)
		return nil, nil, grouper.Members{}, err
	}

	var gardenClient GardenClient.Client
	var gardenClientFactory containerstore.GardenClientFactory
	var k8sRootFSSizer configuration.RootFSSizer // non-nil only on the kubernetes path

	if config.UseKubernetesGardenClient {
		logger.Info("using-kubernetes-garden-client")
		gardenClient, gardenClientFactory, k8sRootFSSizer, err = newKubernetesGardenClient(logger, config, sidecarRootFSPath)
		if err != nil {
			return nil, nil, nil, err
		}
	} else {
		logger.Info("using-cf-garden-client")
		gardenClient = GardenClient.New(GardenConnection.New(config.GardenNetwork, config.GardenAddr))
		gardenClientFactory = containerstore.NewGardenClientFactory(config.GardenNetwork, config.GardenAddr)
	}

	err = waitForGarden(logger, gardenClient, metronClient, clock)
	if err != nil {
		return nil, nil, nil, err
	}

	containersFetcher := &executorContainers{
		gardenClient: gardenClient,
		owner:        config.ContainerOwnerName,
	}

	creationWorkPool, err = workpool.NewWorkPool(config.CreateWorkPoolSize)
	if err != nil {
		return nil, nil, nil, err
	}
	deletionWorkPool, err = workpool.NewWorkPool(config.DeleteWorkPoolSize)
	if err != nil {
		return nil, nil, nil, err
	}
	readWorkPool, err = workpool.NewWorkPool(config.ReadWorkPoolSize)
	if err != nil {
		return nil, nil, nil, err
	}
	metricsWorkPool, err = workpool.NewWorkPool(config.MetricsWorkPoolSize)
	if err != nil {
		return nil, nil, nil, err
	}

	err = destroyContainers(gardenClient, containersFetcher, logger)
	if err != nil {
		return nil, nil, nil, err
	}

	healthCheckWorkPool, err := workpool.NewWorkPool(config.HealthCheckWorkPoolSize)
	if err != nil {
		return nil, nil, grouper.Members{}, err
	}

	certsRetriever := systemcertsRetriever{}
	assetTLSConfig, err := TLSConfigFromConfig(logger, certsRetriever, config)
	if err != nil {
		return nil, nil, grouper.Members{}, err
	}

	downloader := cacheddownloader.NewDownloader(10*time.Minute, math.MaxInt8, assetTLSConfig)
	uploader := uploader.New(logger, 10*time.Minute, assetTLSConfig)

	cache := cacheddownloader.NewCache(config.CachePath, int64(config.MaxCacheSizeInBytes), int64(config.MinCachePartitionFreeBytes))
	cachedDownloader, err := cacheddownloader.New(
		downloader,
		cache,
		cacheddownloader.TarTransform,
	)
	if err != nil {
		return nil, nil, grouper.Members{}, err
	}

	err = cachedDownloader.RecoverState(logger.Session("downloader"))
	if err != nil {
		return nil, nil, grouper.Members{}, err
	}

	downloadRateLimiter := make(chan struct{}, uint(config.MaxConcurrentDownloads))

	transformer := initializeTransformer(
		cachedDownloader,
		setupWorkDir(logger, config.TempDir),
		downloadRateLimiter,
		maxConcurrentUploads,
		uploader,
		time.Duration(config.HealthyMonitoringInterval),
		time.Duration(config.UnhealthyMonitoringInterval),
		time.Duration(config.GracefulShutdownInterval),
		healthCheckWorkPool,
		clock,
		postSetupHook,
		config.PostSetupUser,
		config.EnableHealtcheckMetrics,
		sidecarRootFSPath,
		config.EnableContainerProxy,
		time.Duration(config.EnvoyDrainTimeout),
		config.EnableContainerProxyHealthChecks,
		time.Duration(config.ProxyHealthCheckInterval),
		time.Duration(config.DeclarativeHealthCheckDefaultTimeout),
	)

	hub := event.NewHub()

	totalCapacity, err := fetchCapacity(logger, gardenClient, config)
	if err != nil {
		return nil, nil, grouper.Members{}, err
	}
	var rootFSSizer configuration.RootFSSizer
	if k8sRootFSSizer != nil {
		rootFSSizer = k8sRootFSSizer
	} else {
		rootFSSizer, err = configuration.GetRootFSSizes(logger, gardenClient, guidgen.DefaultGenerator, config.ContainerOwnerName, rootFSes)
		if err != nil {
			return nil, nil, grouper.Members{}, err
		}
	}

	containerConfig := containerstore.ContainerConfig{
		OwnerName:              config.ContainerOwnerName,
		INodeLimit:             config.ContainerInodeLimit,
		MaxCPUShares:           config.ContainerMaxCpuShares,
		SetCPUWeight:           config.SetCPUWeight,
		ReservedExpirationTime: time.Duration(config.ReservedExpirationTime),
		ReapInterval:           time.Duration(config.ContainerReapInterval),
		MaxLogLinesPerSecond:   config.MaxLogLinesPerSecond,
		MetricReportInterval:   time.Duration(config.ContainerMetricsReportInterval),
	}

	driverConfig := vollocal.NewDriverConfig()
	driverConfig.DriverPaths = filepath.SplitList(config.VolmanDriverPaths)
	volmanClient, volmanDriverSyncer := vollocal.NewServer(logger, metronClient, driverConfig)

	var proxyConfigHandler containerstore.ProxyManager
	if config.EnableContainerProxy {
		proxyConfigHandler = containerstore.NewProxyConfigHandler(
			logger,
			config.ContainerProxyPath,
			config.ContainerProxyConfigPath,
			config.ContainerProxyTrustedCACerts,
			config.ContainerProxyVerifySubjectAltName,
			config.ContainerProxyRequireClientCerts,
			time.Duration(config.EnvoyConfigReloadDuration),
			clock,
			config.ContainerProxyADSServers,
			config.ProxyEnableHttp2,
		)
	} else {
		proxyConfigHandler = containerstore.NewNoopProxyConfigHandler()
	}

	instanceIdentityHandler := containerstore.NewInstanceIdentityHandler(
		config.InstanceIdentityCredDir,
		"/etc/cf-instance-credentials",
	)

	credManager, err := CredManagerFromConfig(logger, metronClient, config, clock, proxyConfigHandler, instanceIdentityHandler)
	if err != nil {
		return nil, nil, grouper.Members{}, err
	}

	volumeMountedFilesHandler := containerstore.NewVolumeMountedFilesHandler(
		containerstore.NewFSOperations(),
		config.VolumeMountedFiles,
		"/etc/cf-service-bindings",
	)

	logManager := containerstore.NewLogManager()

	containerStore := containerstore.New(
		containerConfig,
		&totalCapacity,
		gardenClientFactory,
		containerstore.NewDependencyManager(cachedDownloader, downloadRateLimiter),
		volmanClient,
		credManager,
		logManager,
		clock,
		hub,
		transformer,
		config.TrustedSystemCertificatesPath,
		metronClient,
		rootFSSizer,
		config.DeclarativeHealthcheckPath,
		proxyConfigHandler,
		cellID,
		config.EnableUnproxiedPortMappings,
		config.AdvertisePreferenceForInstanceAddress,
		volumeMountedFilesHandler,
		json.Marshal,
	)

	depotClient := depot.NewClient(
		totalCapacity,
		config.CachePath,
		containerStore,
		gardenClient,
		volmanClient,
		hub,
		creationWorkPool,
		deletionWorkPool,
		readWorkPool,
		metricsWorkPool,
	)

	metricsCache := &atomic.Value{}
	containerStatsReporter := containermetrics.NewStatsReporter(
		metronClient,
		config.EnableContainerProxy,
		float64(config.ProxyMemoryAllocationMB*megabytesToBytes),
		metricsCache,
	)
	cpuSpikeReporter := containermetrics.NewCPUSpikeReporter(metronClient)

	reportersRunner := containermetrics.NewReportersRunner(
		logger,
		time.Duration(config.ContainerMetricsReportInterval),
		clock,
		depotClient,
		containerStatsReporter,
		cpuSpikeReporter,
	)

	members := grouper.Members{
		{Name: "volman-driver-syncer", Runner: volmanDriverSyncer},
		{Name: "metrics-reporter", Runner: &metrics.Reporter{
			ExecutorSource: depotClient,
			Interval:       metricsReportInterval,
			Clock:          clock,
			Logger:         logger,
			MetronClient:   metronClient,
			Tags:           map[string]string{"zone": zone},
		}},
		{Name: "hub-closer", Runner: closeHub(logger, hub)},
		{Name: "container-metrics-reporter", Runner: reportersRunner},
	}

	// The kubernetes-backed garden client does not need the classic garden
	// healthcheck (which creates and runs a real healthcheck container), so the
	// healthcheck runner is only wired on the classic path.
	if !config.UseKubernetesGardenClient {
		healthcheckSpec := garden.ProcessSpec{
			Path: config.GardenHealthcheckProcessPath,
			Args: config.GardenHealthcheckProcessArgs,
			User: config.GardenHealthcheckProcessUser,
			Env:  config.GardenHealthcheckProcessEnv,
			Dir:  config.GardenHealthcheckProcessDir,
		}

		gardenHealthcheck := gardenhealth.NewChecker(
			sidecarRootFSPath,
			config.HealthCheckContainerOwnerName,
			time.Duration(config.GardenHealthcheckCommandRetryPause),
			healthcheckSpec,
			gardenClient,
			guidgen.DefaultGenerator,
		)

		members = append(members, grouper.Member{Name: "garden_health_checker", Runner: gardenhealth.NewRunner(
			time.Duration(config.GardenHealthcheckInterval),
			time.Duration(config.GardenHealthcheckEmissionInterval),
			time.Duration(config.GardenHealthcheckTimeout),
			logger,
			gardenHealthcheck,
			depotClient,
			metronClient,
			clock,
		)})
	}

	members = append(members,
		grouper.Member{Name: "registry-pruner", Runner: containerStore.NewRegistryPruner(logger)},
		grouper.Member{Name: "container-reaper", Runner: containerStore.NewContainerReaper(logger)},
	)

	return depotClient, containerStatsReporter, members, nil
}

func waitForGarden(logger lager.Logger, gardenClient garden.Client, metronClient loggingclient.IngressClient, clock clock.Clock) error {
	pingStart := clock.Now()
	logger = logger.Session("wait-for-garden", lager.Data{"initialTime:": pingStart})
	pingRequest := clock.NewTimer(0)
	pingResponse := make(chan error)
	heartbeatTimer := clock.NewTimer(StalledMetricHeartbeatInterval)

	for {
		select {
		case <-pingRequest.C():
			go func() {
				logger.Info("ping-garden", lager.Data{"wait-time-ns:": clock.Since(pingStart)})
				pingResponse <- gardenClient.Ping()
			}()

		case err := <-pingResponse:
			switch err.(type) {
			case nil:
				logger.Info("ping-garden-success", lager.Data{"wait-time-ns:": clock.Since(pingStart)})
				sendError := metronClient.SendDuration(StalledGardenDuration, 0)
				if sendError != nil {
					logger.Error("failed-to-send-stalled-duration-metric", sendError)
				}
				return nil
			case garden.UnrecoverableError:
				logger.Error("failed-to-ping-garden-with-unrecoverable-error", err)
				return err
			default:
				logger.Error("failed-to-ping-garden", err)
				pingRequest.Reset(PingGardenInterval)
			}

		case <-heartbeatTimer.C():
			logger.Info("emitting-stalled-garden-heartbeat", lager.Data{"wait-time-ns:": clock.Since(pingStart)})
			sendError := metronClient.SendDuration(StalledGardenDuration, clock.Since(pingStart))
			if sendError != nil {
				logger.Error("failed-to-send-stalled-duration-heartbeat-metric", sendError)
			}
			heartbeatTimer.Reset(StalledMetricHeartbeatInterval)
		}
	}
}

func fetchCapacity(logger lager.Logger, gardenClient GardenClient.Client, config ExecutorConfig) (executor.ExecutorResources, error) {
	capacity, err := configuration.ConfigureCapacity(gardenClient, config.MemoryMB, config.DiskMB, config.MaxCacheSizeInBytes, config.AutoDiskOverheadMB, config.UseSchedulableDiskSize)
	if err != nil {
		logger.Error("failed-to-configure-capacity", err)
		return executor.ExecutorResources{}, err
	}
	logger.Info("initial-capacity", lager.Data{"capacity": capacity})
	return capacity, nil
}

func destroyContainers(gardenClient garden.Client, containersFetcher *executorContainers, logger lager.Logger) error {
	logger.Info("executor-fetching-containers-to-destroy")
	containers, err := containersFetcher.Containers()
	if err != nil {
		logger.Error("executor-failed-to-get-containers", err)
		return err
	}
	logger.Info("executor-fetched-containers-to-destroy", lager.Data{"num-containers": len(containers)})

	type containerDeletionResult struct {
		handle string
		err    error
	}
	errInfoChannel := make(chan containerDeletionResult, len(containers))
	for _, container := range containers {
		go func(c garden.Container) {
			deletionWorkPool.Submit(func() {
				err := gardenClient.Destroy(c.Handle())
				errInfoChannel <- containerDeletionResult{handle: c.Handle(), err: err}
			})
		}(container)
	}
	for range containers {
		result := <-errInfoChannel
		if result.err != nil {
			logger.Error("executor-failed-to-destroy-container", result.err, lager.Data{"handle": result.handle})
			return result.err
		}
		logger.Info("executor-destroyed-stray-container", lager.Data{"handle": result.handle})
	}
	return nil
}

func setupWorkDir(logger lager.Logger, tempDir string) string {
	workDir := filepath.Join(tempDir, "executor-work")
	err := os.RemoveAll(workDir)
	if err != nil {
		logger.Error("working-dir.cleanup-failed", err)
		os.Exit(1)
	}
	err = os.MkdirAll(workDir, 0755)
	if err != nil {
		logger.Error("working-dir.create-failed", err)
		os.Exit(1)
	}
	return workDir
}

func initializeTransformer(
	cache cacheddownloader.CachedDownloader,
	workDir string,
	downloadRateLimiter chan struct{},
	maxConcurrentUploads uint,
	uploader uploader.Uploader,
	healthyMonitoringInterval time.Duration,
	unhealthyMonitoringInterval time.Duration,
	gracefulShutdownInterval time.Duration,
	healthCheckWorkPool *workpool.WorkPool,
	clock clock.Clock,
	postSetupHook []string,
	postSetupUser string,
	emitHealthCheckMetrics bool,
	declarativeHealthcheckRootFS string,
	enableContainerProxy bool,
	drainWait time.Duration,
	enableProxyHealthChecks bool,
	proxyHealthCheckInterval time.Duration,
	declarativeHealthCheckDefaultTimeout time.Duration,
) transformer.Transformer {
	var options []transformer.Option
	compressor := compressor.NewTgz()
	options = append(options, transformer.WithSidecarRootfs(declarativeHealthcheckRootFS))
	options = append(options, transformer.WithDeclarativeHealthChecks(declarativeHealthCheckDefaultTimeout))
	if emitHealthCheckMetrics {
		options = append(options, transformer.WithDeclarativeHealthcheckFailureMetrics())
	}
	if enableContainerProxy {
		options = append(options, transformer.WithContainerProxy(drainWait))
		if enableProxyHealthChecks {
			options = append(options, transformer.WithProxyLivenessChecks(proxyHealthCheckInterval))
		}
	}
	options = append(options, transformer.WithPostSetupHook(postSetupUser, postSetupHook))
	return transformer.NewTransformer(
		clock,
		cache,
		uploader,
		compressor,
		downloadRateLimiter,
		make(chan struct{}, maxConcurrentUploads),
		workDir,
		healthyMonitoringInterval,
		unhealthyMonitoringInterval,
		gracefulShutdownInterval,
		healthCheckWorkPool,
		options...,
	)
}

func closeHub(logger lager.Logger, hub event.Hub) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		close(ready)
		signal := <-signals
		err := hub.Close()
		hubLogger := logger.Session("close-hub")
		if err != nil {
			logger.Error("failed-to-close-hub", err)
		}
		hubLogger.Info("signalled", lager.Data{"signal": signal.String()})
		return nil
	})
}
