package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/debugserver"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/go-loggregator/v9/runtimeemitter"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/locket/jointlock"
	"code.cloudfoundry.org/locket/lock"
	locketmodels "code.cloudfoundry.org/locket/models"
	"code.cloudfoundry.org/route-emitter/cmd/route-emitter/config"
	"code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/route-emitter/emitter"
	"code.cloudfoundry.org/route-emitter/routehandlers"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/scheduler"
	"code.cloudfoundry.org/route-emitter/syncer"
	"code.cloudfoundry.org/route-emitter/unregistration"
	"code.cloudfoundry.org/route-emitter/watcher"
	routing_api "code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-api/uaaclient"
	"code.cloudfoundry.org/tlsconfig"
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
	routeEmitterLockKey = "route_emitter"
)

func main() {
	flag.Parse()

	cfg, err := config.NewRouteEmitterConfig(*configFilePath)
	if err != nil {
		logger, _ := lagerflags.NewFromConfig("route-emitter", lagerflags.DefaultLagerConfig())
		logger.Fatal("failed-to-parse-config", err)
	}

	logger, reconfigurableSink := lagerflags.NewFromConfig(cfg.LocketSessionName, cfg.LagerConfig)

	natsClient, err := initializeNATSClient(logger, cfg.NATSTLSEnabled, cfg.NATSCACertFile, cfg.NATSClientCertFile, cfg.NATSClientKeyFile)
	if err != nil {
		logger.Error("failed-to-initialize-nats-client", err)
		os.Exit(1)
	}

	clock := clock.NewClock()

	externalChan := make(chan struct{}, 1)
	internalChan := make(chan struct{}, 1)
	syncer := syncer.NewSyncer(clock, time.Duration(cfg.SyncInterval), logger)
	externalScheduler := scheduler.NewRouteBroadcastScheduler(clock, natsClient, logger, "router", &cfg, externalChan)
	internalScheduler := scheduler.NewRouteBroadcastScheduler(clock, natsClient, logger, "service-discovery", &cfg, internalChan)

	metronClient, err := initializeMetron(logger, cfg)
	if err != nil {
		logger.Error("failed-to-initialize-metron-client", err)
		os.Exit(1)
	}

	natsClientRunner := diegonats.NewClientRunner(cfg.NATSAddresses, cfg.NATSUsername, cfg.NATSPassword, logger, natsClient)

	bbsClient := initializeBBSClient(logger, cfg)

	localMode := cfg.CellID != ""
	table := routingtable.NewRoutingTable(cfg.RegisterDirectInstanceRoutes, cfg.TCPEnableTLS, metronClient)
	natsEmitter := initializeNatsEmitter(logger, natsClient, cfg.RouteEmittingWorkers, metronClient, cfg.EnableInternalEmitter)

	routeTTL := time.Duration(cfg.TCPRouteTTL)
	if routeTTL.Seconds() > 65535 {
		logger.Fatal("invalid-route-ttl", errors.New("route TTL value too large"), lager.Data{"ttl": routeTTL.Seconds()})
	}

	var routingAPIEmitter emitter.RoutingAPIEmitter
	if cfg.EnableTCPEmitter {
		tcpLogger := logger.Session("tcp")
		uaaTokenFetcher := newUaaTokenFetcher(tcpLogger, &cfg, clock)

		routingAPIAddress := fmt.Sprintf("%s:%d", cfg.RoutingAPI.URL, cfg.RoutingAPI.Port)
		logger.Debug("creating-routing-api-client", lager.Data{"api-location": routingAPIAddress})

		var routingAPIClient routing_api.Client
		if cfg.RoutingAPI.ClientCertFile != "" && cfg.RoutingAPI.ClientKeyFile != "" && cfg.RoutingAPI.CACertFile != "" {
			tlsConfig, err := tlsconfig.Build(
				tlsconfig.WithInternalServiceDefaults(),
				tlsconfig.WithIdentityFromFile(cfg.RoutingAPI.ClientCertFile, cfg.RoutingAPI.ClientKeyFile),
			).Client(
				tlsconfig.WithAuthorityFromFile(cfg.RoutingAPI.CACertFile),
			)
			if err != nil {
				logger.Fatal("failed-to-create-routing-api-tls-config", err)
			}
			routingAPIClient = routing_api.NewClientWithTLSConfig(routingAPIAddress, tlsConfig)
		} else {
			routingAPIClient = routing_api.NewClient(routingAPIAddress, false)
		}

		routingAPIEmitter = emitter.NewRoutingAPIEmitter(tcpLogger, routingAPIClient, uaaTokenFetcher, int(routeTTL.Seconds()))
	}

	unregistrationCache := unregistration.NewCache(logger)

	handler := routehandlers.NewHandler(table, natsEmitter, routingAPIEmitter, localMode, cfg.TCPEnableTLS, metronClient, unregistrationCache)

	watcher := watcher.NewWatcher(
		cfg.CellID,
		bbsClient,
		clock,
		handler,
		syncer.SyncCh(),
		externalScheduler.EmitCh(),
		internalScheduler.EmitCh(),
		logger,
		metronClient,
	)

	healthHandler := func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(http.StatusOK)
	}
	healthCheckServer := http_server.New(cfg.HealthCheckAddress, http.HandlerFunc(healthHandler))
	unregistrationSender := unregistration.NewSender(logger, clock, unregistrationCache, natsEmitter, time.Duration(cfg.UnregistrationInterval), cfg.UnregistrationSendCount)
	members := grouper.Members{
		{Name: "nats-client", Runner: natsClientRunner},
		{Name: "healthcheck", Runner: healthCheckServer},
		{Name: "unregistration", Runner: unregistrationSender},
	}

	if cfg.CellID == "" && cfg.LocketEnabled {
		locketClient, err := locket.NewClient(logger, cfg.ClientLocketConfig)
		if err != nil {
			logger.Fatal("failed-to-create-locket-client", err)
		}

		if cfg.UUID == "" {
			logger.Fatal("invalid-uuid", errors.New("invalid-uuid-from-config"))
		}

		lockIdentifier := &locketmodels.Resource{
			Key:      routeEmitterLockKey,
			Owner:    cfg.UUID,
			TypeCode: locketmodels.LOCK,
			Type:     locketmodels.LockType,
		}

		lockMembers := []grouper.Member{}
		lockMembers = append(lockMembers, grouper.Member{Name: "sql-lock", Runner: lock.NewLockRunner(
			logger,
			locketClient,
			lockIdentifier,
			locket.DefaultSessionTTLInSeconds,
			clock,
			locket.SQLRetryInterval,
		)})

		members = append(members,
			grouper.Member{Name: "lock", Runner: lockRunner(logger, clock, lockMembers)},
		)
	}

	members = append(members,
		grouper.Member{Name: "watcher", Runner: watcher},
		grouper.Member{Name: "external-scheduler", Runner: externalScheduler},
		grouper.Member{Name: "syncer", Runner: syncer},
	)

	if cfg.EnableInternalEmitter {
		members = append(members, grouper.Member{Name: "internal-scheduler", Runner: internalScheduler})
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
		logger.Error("finished-with-failure", err)
	} else {
		logger.Info("finished")
	}
	logger.Info("exited")
}

func lockRunner(logger lager.Logger, clk clock.Clock, locks []grouper.Member) ifrit.Runner {
	switch len(locks) {
	case 0:
		logger.Fatal("no-locks-configured", errors.New("Lock configuration must be provided"))
		return nil
	case 1:
		return locks[0]
	default:
		return jointlock.NewJointLock(clk, locket.DefaultSessionTTL, locks...)
	}
}

func newUaaTokenFetcher(logger lager.Logger, c *config.RouteEmitterConfig, klok clock.Clock) uaaclient.TokenFetcher {
	cfg := uaaclient.Config{}
	if c.RoutingAPI.AuthEnabled {
		u, err := url.Parse(c.OAuth.UaaURL)
		if err != nil {
			logger.Fatal("failed-parsing-uaa-url", err)
		}
		host, port, err := net.SplitHostPort(u.Host)
		if err != nil {
			logger.Fatal("failed-parsing-uaa-host", err, lager.Data{"url": c.OAuth.UaaURL, "host": u.Host})
		}
		portI, err := strconv.ParseUint(port, 10, 16)
		if err != nil {
			logger.Fatal("failed-parsing-uaa-port", err)
		}
		cfg.Port = uint16(portI)
		cfg.SkipSSLValidation = c.OAuth.SkipCertVerify
		cfg.ClientName = c.OAuth.ClientName
		cfg.ClientSecret = c.OAuth.ClientSecret
		cfg.CACerts = c.OAuth.CACerts
		cfg.TokenEndpoint = host
		cfg.RequestTimeout = time.Duration(c.OAuth.UaaRequestTimeout)
	}
	uaaTokenFetcher, err := uaaclient.NewTokenFetcher(!c.RoutingAPI.AuthEnabled, cfg, klok, 0, 0, 0, logger)
	if err != nil {
		logger.Fatal("failed-initializing-uaa-token-fetcher", err)
	}

	_, err = uaaTokenFetcher.FetchKey()
	if err != nil {
		logger.Fatal("failed-fetching-uaa-key", err)
	}

	return uaaTokenFetcher
}

func initializeMetron(logger lager.Logger, locketConfig config.RouteEmitterConfig) (loggingclient.IngressClient, error) {
	client, err := loggingclient.NewIngressClient(locketConfig.LoggregatorConfig)
	if err != nil {
		return nil, err
	}

	if locketConfig.LoggregatorConfig.UseV2API {
		emitter := runtimeemitter.NewV1(client)
		go emitter.Run()
	}

	return client, nil
}

func initializeNatsEmitter(
	logger lager.Logger,
	natsClient diegonats.NATSClient,
	routeEmittingWorkers int,
	metronClient loggingclient.IngressClient,
	emitInternalRoutes bool,
) emitter.NATSEmitter {
	workPool, err := workpool.NewWorkPool(routeEmittingWorkers)
	if err != nil {
		logger.Fatal("failed-to-construct-nats-emitter-workpool", err, lager.Data{"num-workers": routeEmittingWorkers}) // should never happen
	}

	return emitter.NewNATSEmitter(natsClient, workPool, logger, metronClient, emitInternalRoutes)
}

func initializeBBSClient(
	logger lager.Logger,
	cfg config.RouteEmitterConfig,
) bbs.Client {
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

func initializeNATSClient(logger lager.Logger, tlsEnabled bool, caFile, certFile, keyFile string) (diegonats.NATSClient, error) {
	var natsClient diegonats.NATSClient
	if tlsEnabled {
		tlsConfig, err := tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(certFile, keyFile),
		).Client(
			tlsconfig.WithAuthorityFromFile(caFile),
		)
		if err != nil {
			return nil, err
		}
		natsClient = diegonats.NewClientWithTLSConfig(tlsConfig)
	} else {
		natsClient = diegonats.NewClient()
	}

	natsPingDuration := 20 * time.Second
	logger.Info("setting-nats-ping-interval", lager.Data{"duration-in-seconds": natsPingDuration.Seconds()})
	natsClient.SetPingInterval(natsPingDuration)

	return natsClient, nil
}
