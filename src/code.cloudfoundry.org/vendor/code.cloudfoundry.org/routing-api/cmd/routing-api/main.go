package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/locket/lock"
	locketmodels "code.cloudfoundry.org/locket/models"
	routing_api "code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-api/admin"
	"code.cloudfoundry.org/routing-api/config"
	"code.cloudfoundry.org/routing-api/db"
	"code.cloudfoundry.org/routing-api/handlers"
	"code.cloudfoundry.org/routing-api/helpers"
	"code.cloudfoundry.org/routing-api/metrics"
	"code.cloudfoundry.org/routing-api/migration"
	"code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/routing-api/uaaclient"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/cactus/go-statsd-client/statsd"
	"github.com/cloudfoundry/dropsonde"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
	"github.com/tedsuo/rata"
)

const (
	sessionName     = "routing_api"
	pruningInterval = 10 * time.Second
)

var configPath = flag.String("config", "", "Configuration for routing-api")
var devMode = flag.Bool("devMode", false, "Disable authentication for easier development iteration")
var ip = flag.String("ip", "", "The public ip of the routing api")

func route(f func(w http.ResponseWriter, r *http.Request)) http.Handler {
	return http.HandlerFunc(f)
}

func main() {
	lagerflags.AddFlags(flag.CommandLine)
	flag.Parse()

	logger, reconfigurableSink := lagerflags.New("routing-api")

	err := checkFlags()
	if err != nil {
		logger.Error("failed-check-flags", err)
		os.Exit(1)
	}

	cfg, err := config.NewConfigFromFile(*configPath, *devMode)
	if err != nil {
		logger.Error("failed-load-config", err)
		os.Exit(1)
	}

	err = dropsonde.Initialize(cfg.MetronConfig.Address+":"+cfg.MetronConfig.Port, cfg.LogGuid)
	if err != nil {
		logger.Error("failed-initialize-dropsonde", err)
		os.Exit(1)
	}

	if cfg.DebugAddress != "" {
		_, err := debugserver.Run(cfg.DebugAddress, reconfigurableSink)
		if err != nil {
			logger.Error("failed-debug-server", err, lager.Data{"debug_address": cfg.DebugAddress})
		}
	}

	database, err := buildDatabase(logger, cfg)
	if err != nil {
		os.Exit(1)
	}

	prefix := "routing_api"
	statsdClient, err := statsd.NewBufferedClient(cfg.StatsdEndpoint, prefix, cfg.StatsdClientFlushInterval, 512)
	if err != nil {
		logger.Error("failed-to-create-statsd-client", err)
		os.Exit(1)
	}
	defer func() {
		err := statsdClient.Close()
		if err != nil {
			logger.Error("failed-to-close-statsd-client", err)
		}
	}()

	adminServer, err := admin.NewServer(cfg.AdminPort, database, logger.Session("admin-server"))
	if err != nil {
		logger.Error("failed-to-create-admin-server", err)
		os.Exit(1)
	}

	stopper := constructStopper(database)

	clock := clock.NewClock()

	releaseLock := make(chan os.Signal)
	lockErrChan := make(chan error)
	metricsTicker := time.NewTicker(cfg.MetricsReportingInterval)
	metricsReporter := metrics.NewMetricsReporter(database, statsdClient, metricsTicker, logger.Session("metrics"))
	migrationProcess := runMigration(database, logger.Session("migration"))
	routerGroupSeeder := seedRouterGroups(cfg, database, logger.Session("seeding"))

	locks := grouper.Members{}

	var locketClient locketmodels.LocketClient
	locketClient, err = locket.NewClient(logger, cfg.Locket)
	if err != nil {
		logger.Fatal("failed-to-create-locket-client", err)
	}

	lockIdentifier := &locketmodels.Resource{
		Key:      cfg.LockResouceKey,
		Owner:    cfg.UUID,
		TypeCode: locketmodels.LOCK,
	}

	locks = append(locks, grouper.Member{Name: "sql-lock", Runner: lock.NewLockRunner(
		logger,
		locketClient,
		lockIdentifier,
		locket.DefaultSessionTTLInSeconds,
		clock,
		locket.SQLRetryInterval,
	)})

	if len(locks) == 0 {
		logger.Fatal("no-locks-configured", errors.New("lock configuration must be provided"))
	}

	lockGroup := grouper.NewOrdered(os.Interrupt, locks)
	lockAcquirer := initializeLockAcquirer(lockGroup, releaseLock, lockErrChan)
	lockReleaser := initializeLockReleaser(releaseLock, lockErrChan, cfg.RetryInterval)

	uaaConfig := uaaclient.Config{
		Port:              cfg.OAuth.Port,
		SkipSSLValidation: cfg.OAuth.SkipSSLValidation,
		ClientName:        cfg.OAuth.ClientName,
		ClientSecret:      cfg.OAuth.ClientSecret,
		CACerts:           cfg.OAuth.CACerts,
		TokenEndpoint:     cfg.OAuth.TokenEndpoint,
	}

	uaaClient, err := uaaclient.NewTokenValidator(*devMode, uaaConfig, logger)
	if err != nil {
		logger.Error("creating-uaa-client", err)
		os.Exit(1)
	}

	members := grouper.Members{
		grouper.Member{Name: "lock-acquirer", Runner: lockAcquirer},
		grouper.Member{Name: "migration", Runner: migrationProcess},
		grouper.Member{Name: "seed-router-groups", Runner: routerGroupSeeder},
	}

	if cfg.API.HTTPEnabled {
		httpAPIHandler := apiHandler(cfg, uaaClient, database, statsdClient, logger.Session("api-http-server"))
		httpAPIServer := http_server.New(fmt.Sprintf(":%d", cfg.API.ListenPort), httpAPIHandler)

		// As of Dec 2022, the tls route to routing-api is added to
		// gorouter via route_registrar. This routerRegister is not needed
		// unless customers need to keep routing-api as http, for example
		// because tiles connect via http. It can be removed once we
		// sunset http for routing-api.
		// See https://github.com/cloudfoundry/cf-deployment/commit/0796dc168ed032b845be81bebc9c311a0317eadc
		routerRegister := constructRouteRegister(
			cfg.API.ListenPort,
			cfg.LogGuid,
			cfg.SystemDomain,
			cfg.MaxTTL,
			database,
			logger.Session("route-register"),
		)
		members = append(members,
			grouper.Member{Name: "api-server", Runner: httpAPIServer},
			grouper.Member{Name: "route-register", Runner: routerRegister},
		)
	}

	config, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(cfg.API.MTLSServerCertPath, cfg.API.MTLSServerKeyPath),
	).Server(
		tlsconfig.WithClientAuthenticationFromFile(cfg.API.MTLSClientCAPath),
	)
	if err != nil {
		logger.Fatal("mtls-mis-configured", err)
	}

	mtlsAPIHandler := apiHandler(cfg, uaaClient, database, statsdClient, logger.Session("api-mtls-server"))
	mtlsAPIServer := http_server.NewTLSServer(fmt.Sprintf(":%d", cfg.API.MTLSListenPort), mtlsAPIHandler, config)
	members = append(members, grouper.Member{
		Name: "api-mtls-server", Runner: mtlsAPIServer},
	)

	members = append(members,
		grouper.Member{Name: "admin-server", Runner: adminServer},
		grouper.Member{Name: "conn-stopper", Runner: stopper},
		grouper.Member{Name: "metrics", Runner: metricsReporter},
	)

	if isSql(cfg.SqlDB) {
		routePruner := runCleanupRoutes(database, logger)
		members = append(members, grouper.Member{Name: "sql-route-pruner", Runner: routePruner})
	}
	members = append(members, grouper.Member{Name: "lock-releaser", Runner: lockReleaser})

	group := grouper.NewOrdered(os.Interrupt, members)
	process := ifrit.Invoke(sigmon.New(group))

	// This is used by testrunner to signal ready for tests.
	logger.Info("started", lager.Data{"port": cfg.API.ListenPort})

	errChan := process.Wait()
	err = <-errChan
	if err != nil {
		logger.Error("shutdown-error", err)
		os.Exit(1)
	}
	logger.Info("exited")
}

func isSql(sqlDB config.SqlDB) bool {
	return (sqlDB.Host != "" && sqlDB.Port > 0 && sqlDB.Schema != "")
}

func constructStopper(database db.DB) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		close(ready)
		<-signals
		database.CancelWatches()

		return nil
	})
}

func seedRouterGroups(cfg config.Config, database db.DB, logger lager.Logger) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		if len(cfg.RouterGroups) > 0 {
			routerGroups, _ := database.ReadRouterGroups()
			// if config not empty and db is empty, seed
			if len(routerGroups) == 0 {
				for _, rg := range cfg.RouterGroups {
					guid, err := uuid.NewV4()
					if err != nil {
						logger.Error("failed to generate a guid for router group", err)
						return err
					}
					rg.Guid = guid.String()
					logger.Info("seeding", lager.Data{"router-group": rg})
					err = database.SaveRouterGroup(rg)
					if err != nil {
						logger.Error("failed to save router group from config", err)
						return err
					}
				}
			}
		}
		close(ready)
		sig := <-signals
		logger.Info("received-signal", lager.Data{"signal": sig})
		return nil
	})
}

func runCleanupRoutes(sqlDatabase db.DB, logger lager.Logger) ifrit.Runner {
	pruneLogger := logger.Session("prune-routes")
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		sqlDB, ok := sqlDatabase.(*db.SqlDB)
		if !ok {
			return nil
		}
		close(ready)

		sqlDB.CleanupRoutes(pruneLogger, pruningInterval, signals)
		return nil
	})
}

func runMigration(database db.DB, logger lager.Logger) ifrit.Runner {
	if sqlDB, ok := database.(*db.SqlDB); ok {
		return migration.NewRunner(sqlDB, logger)
	}
	return migration.NewRunner(nil, logger)
}

func constructRouteRegister(
	port uint16,
	logGuid string,
	systemDomain string,
	maxTTL time.Duration,
	database db.DB,
	logger lager.Logger,
) ifrit.Runner {
	host := fmt.Sprintf("api.%s/routing", systemDomain)
	route := models.NewRoute(host, port, *ip, logGuid, "", int(maxTTL.Seconds()))

	registerInterval := int(maxTTL.Seconds()) / 2
	ticker := time.NewTicker(time.Duration(registerInterval) * time.Second)

	return helpers.NewRouteRegister(database, route, ticker, logger)
}

func apiHandler(cfg config.Config, uaaClient uaaclient.TokenValidator, database db.DB, statsdClient statsd.Statter, logger lager.Logger) http.Handler {
	validator := handlers.NewValidator()
	routesHandler := handlers.NewRoutesHandler(uaaClient, int(cfg.MaxTTL.Seconds()), validator, database, logger)
	eventStreamHandler := handlers.NewEventStreamHandler(uaaClient, database, logger, statsdClient)
	routerGroupsHandler := handlers.NewRouteGroupsHandler(uaaClient, logger, database)
	tcpMappingsHandler := handlers.NewTcpRouteMappingsHandler(uaaClient, validator, database, int(cfg.MaxTTL.Seconds()), logger)

	actions := rata.Handlers{
		routing_api.UpsertRoute:           route(routesHandler.Upsert),
		routing_api.DeleteRoute:           route(routesHandler.Delete),
		routing_api.ListRoute:             route(routesHandler.List),
		routing_api.EventStreamRoute:      route(eventStreamHandler.EventStream),
		routing_api.ListRouterGroups:      route(routerGroupsHandler.ListRouterGroups),
		routing_api.CreateRouterGroup:     route(routerGroupsHandler.CreateRouterGroup),
		routing_api.UpdateRouterGroup:     route(routerGroupsHandler.UpdateRouterGroup),
		routing_api.DeleteRouterGroup:     route(routerGroupsHandler.DeleteRouterGroup),
		routing_api.UpsertTcpRouteMapping: route(tcpMappingsHandler.Upsert),
		routing_api.DeleteTcpRouteMapping: route(tcpMappingsHandler.Delete),
		routing_api.ListTcpRouteMapping:   route(tcpMappingsHandler.List),
		routing_api.EventStreamTcpRoute:   route(eventStreamHandler.TcpEventStream),
	}

	handler, err := rata.NewRouter(routing_api.Routes(), actions)
	if err != nil {
		logger.Error("failed to create router", err)
		os.Exit(1)
	}
	return handlers.LogWrap(handler, logger)
}

func checkFlags() error {
	if *configPath == "" {
		return errors.New("no configuration file provided")
	}

	if *ip == "" {
		return errors.New("no ip address provided")
	}

	return nil
}

func initializeLockAcquirer(lockRunner ifrit.Runner, releaseLock chan os.Signal, lockErrChan chan error) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {

		go func() {
			err := lockRunner.Run(releaseLock, ready)
			lockErrChan <- err
		}()

		<-signals
		return nil
	})
}

func initializeLockReleaser(releaseLock chan os.Signal, lockErrChan chan error, retryInterval time.Duration) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		close(ready)
		var err error
		for {
			select {
			case signal := <-signals:
				releaseLock <- signal

			case err = <-lockErrChan:
				lockRetryInterval := 5 * time.Second
				//Give other routing-api enough time to grab the lock
				time.Sleep(retryInterval + lockRetryInterval)

				return err
			}
		}
	})
}

func buildDatabase(
	logger lager.Logger,
	cfg config.Config,
) (db.DB, error) {
	var err error
	var database db.DB

	data := lager.Data{"host": cfg.SqlDB.Host, "port": cfg.SqlDB.Port}
	logger.Info("database", data)
	database, err = db.NewSqlDB(&cfg.SqlDB)
	if err != nil {
		logger.Error("failed-initialize-sql-connection", err, data)
		return nil, err
	}
	return database, nil
}
