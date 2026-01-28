package main

import (
	"errors"
	"flag"
	"os"
	"time"

	"code.cloudfoundry.org/auctioneer/auctionmetricemitterdelegate"
	"code.cloudfoundry.org/auctioneer/auctionrunnerdelegate"
	"code.cloudfoundry.org/auctioneer/cmd/auctioneer/config"
	"code.cloudfoundry.org/auctioneer/handlers"
	"code.cloudfoundry.org/bbs"
	cfhttp "code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/debugserver"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/go-loggregator/v9/runtimeemitter"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/locket/jointlock"
	"code.cloudfoundry.org/locket/lock"
	"code.cloudfoundry.org/locket/lockheldmetrics"
	locketmodels "code.cloudfoundry.org/locket/models"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/tlsconfig"

	"code.cloudfoundry.org/auction/auctionrunner"
	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/workpool"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

var configFilePath = flag.String(
	"config",
	"",
	"Path to JSON configuration file",
)

const (
	serverProtocol    = "http"
	auctioneerLockKey = "auctioneer"
)

func main() {
	flag.Parse()

	cfg, err := config.NewAuctioneerConfig(*configFilePath)
	if err != nil {
		// TODO: Test me?
		panic(err)
	}

	logger, reconfigurableSink := lagerflags.NewFromConfig("auctioneer", cfg.LagerConfig)
	metronClient, err := initializeMetron(logger, cfg)
	if err != nil {
		logger.Fatal("failed-to-initialize-metron", err)
	}

	if err := validateBBSAddress(cfg.BBSAddress); err != nil {
		logger.Fatal("invalid-bbs-address", err)
	}

	locks := []grouper.Member{}
	if cfg.UUID == "" {
		logger.Fatal("invalid-uuid", errors.New("invalid-uuid-from-config"))
	}

	locketClient, err := locket.NewClient(logger, cfg.ClientLocketConfig)
	if err != nil {
		logger.Fatal("failed-to-connect-to-locket", err)
	}

	lockIdentifier := &locketmodels.Resource{
		Key:      auctioneerLockKey,
		Owner:    cfg.UUID,
		TypeCode: locketmodels.LOCK,
		Type:     locketmodels.LockType,
	}

	clock := clock.NewClock()
	locks = append(locks, grouper.Member{Name: "sql-lock", Runner: lock.NewLockRunner(
		logger,
		locketClient,
		lockIdentifier,
		locket.DefaultSessionTTLInSeconds,
		clock,
		locket.SQLRetryInterval,
	)})

	var lock ifrit.Runner
	switch len(locks) {
	case 0:
		logger.Fatal("no-locks-configured", errors.New("Lock configuration must be provided"))
	case 1:
		lock = locks[0]
	default:
		lock = jointlock.NewJointLock(clock, locket.DefaultSessionTTL, locks...)
	}

	auctionRunner := initializeAuctionRunner(logger, cfg, initializeBBSClient(logger, cfg), metronClient)

	var auctionServer ifrit.Runner
	if cfg.ServerCertFile != "" || cfg.ServerKeyFile != "" || cfg.CACertFile != "" {
		tlsConfig, err := tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(cfg.ServerCertFile, cfg.ServerKeyFile),
		).Server(tlsconfig.WithClientAuthenticationFromFile(cfg.CACertFile))
		if err != nil {
			logger.Fatal("invalid-tls-config", err)
		}
		auctionServer = http_server.NewTLSServer(cfg.ListenAddress, handlers.New(logger, auctionRunner, metronClient), tlsConfig)
	} else {
		auctionServer = http_server.New(cfg.ListenAddress, handlers.New(logger, auctionRunner, metronClient))
	}

	metricsTicker := clock.NewTicker(time.Duration(cfg.ReportInterval))
	lockHeldMetronNotifier := lockheldmetrics.NewLockHeldMetronNotifier(logger, metricsTicker, metronClient)

	members := grouper.Members{
		{Name: "lock-held-metrics", Runner: lockHeldMetronNotifier},
		{Name: "lock", Runner: lock},
		{Name: "set-lock-held-metrics", Runner: lockheldmetrics.SetLockHeldRunner(logger, *lockHeldMetronNotifier)},
		{Name: "auction-runner", Runner: auctionRunner},
		{Name: "auction-server", Runner: auctionServer},
	}

	if cfg.DebugAddress != "" {
		members = append(grouper.Members{
			{Name: "debug-server", Runner: debugserver.Runner(cfg.DebugAddress, reconfigurableSink)},
		}, members...)
	}

	group := grouper.NewOrdered(os.Interrupt, members)

	monitor := ifrit.Invoke(sigmon.New(group))

	logger.Info("started")

	err = <-monitor.Wait()
	if err != nil {
		logger.Error("exited-with-failure", err)
		os.Exit(1)
	}

	logger.Info("exited")
}

func initializeAuctionRunner(logger lager.Logger, cfg config.AuctioneerConfig, bbsClient bbs.InternalClient, metronClient loggingclient.IngressClient) auctiontypes.AuctionRunner {
	httpClient := cfhttp.NewClient(
		cfhttp.WithRequestTimeout(time.Duration(cfg.CommunicationTimeout)),
	)
	stateClient := cfhttp.NewClient(
		cfhttp.WithRequestTimeout(time.Duration(cfg.CellStateTimeout)),
	)
	repTLSConfig := &rep.TLSConfig{
		RequireTLS:      true,
		CaCertFile:      cfg.RepCACert,
		CertFile:        cfg.RepClientCert,
		KeyFile:         cfg.RepClientKey,
		ClientCacheSize: cfg.RepClientSessionCacheSize,
	}
	repClientFactory, err := rep.NewClientFactory(httpClient, stateClient, repTLSConfig)
	if err != nil {
		logger.Fatal("new-rep-client-factory-failed", err)
	}

	delegate := auctionrunnerdelegate.New(repClientFactory, bbsClient)
	metricEmitter := auctionmetricemitterdelegate.New(metronClient)
	workPool, err := workpool.NewWorkPool(cfg.AuctionRunnerWorkers)
	if err != nil {
		logger.Fatal("failed-to-construct-auction-runner-workpool", err, lager.Data{"num-workers": cfg.AuctionRunnerWorkers}) // should never happen
	}

	return auctionrunner.New(
		logger,
		delegate,
		metricEmitter,
		clock.NewClock(),
		workPool,
		cfg.BinPackFirstFitWeight,
		cfg.StartingContainerWeight,
		cfg.StartingContainerCountMaximum,
	)
}

func initializeMetron(logger lager.Logger, cfg config.AuctioneerConfig) (loggingclient.IngressClient, error) {
	client, err := loggingclient.NewIngressClient(cfg.LoggregatorConfig)
	if err != nil {
		return nil, err
	}

	if cfg.LoggregatorConfig.UseV2API {
		emitter := runtimeemitter.NewV1(client)
		go emitter.Run()
	}

	return client, nil
}

func validateBBSAddress(bbsAddress string) error {
	if bbsAddress == "" {
		return errors.New("bbsAddress is required")
	}
	return nil
}

func initializeBBSClient(logger lager.Logger, cfg config.AuctioneerConfig) bbs.InternalClient {
	bbsClient, err := bbs.NewClientWithConfig(bbs.ClientConfig{
		URL:                    cfg.BBSAddress,
		IsTLS:                  true,
		CAFile:                 cfg.BBSCACertFile,
		CertFile:               cfg.BBSClientCertFile,
		KeyFile:                cfg.BBSClientKeyFile,
		ClientSessionCacheSize: cfg.BBSClientSessionCacheSize,
		MaxIdleConnsPerHost:    cfg.BBSMaxIdleConnsPerHost,
		RequestTimeout:         time.Duration(cfg.CommunicationTimeout),
	})
	if err != nil {
		logger.Fatal("Failed to configure secure BBS client", err)
	}
	return bbsClient
}
