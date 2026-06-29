package main

import (
	"flag"
	"os"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/spiffe-agent/bbsresolver"
	"code.cloudfoundry.org/spiffe-agent/cfattestor"
	"code.cloudfoundry.org/spiffe-agent/config"
	"code.cloudfoundry.org/spiffe-agent/signer"
	"code.cloudfoundry.org/spiffe-agent/workloadapi"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

var configFilePath = flag.String(
	"config",
	"",
	"Path to JSON configuration file",
)

func main() {
	flag.Parse()

	cfg, err := config.NewSpiffeAgentConfig(*configFilePath)
	if err != nil {
		panic(err)
	}

	logger, _ := lagerflags.NewFromConfig("spiffe-agent", cfg.LagerConfig)

	bbsClient := initializeBBSClient(logger, cfg)
	resolver := bbsresolver.New(bbsClient, cfg.CellID, logger)
	attestor := cfattestor.New(resolver)

	httpClient, err := cfg.SignerHTTPClient()
	if err != nil {
		logger.Fatal("failed-to-build-signer-http-client", err)
	}
	sgnr := signer.New(httpClient, cfg.SignerURL, cfg.SignerClientID, cfg.SignerClientSecret, cfg.TrustDomain)

	srv := workloadapi.NewServer(attestor, sgnr)

	members := grouper.Members{
		{Name: "workload-api", Runner: workloadapi.NewRunner(cfg.SocketPath, srv, logger)},
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

func initializeBBSClient(logger lager.Logger, cfg config.Config) bbs.InternalClient {
	bbsClient, err := bbs.NewClientWithConfig(bbs.ClientConfig{
		URL:      cfg.BBSAddress,
		IsTLS:    true,
		CAFile:   cfg.BBSCACertFile,
		CertFile: cfg.BBSClientCertFile,
		KeyFile:  cfg.BBSClientKeyFile,
	})
	if err != nil {
		logger.Fatal("failed-to-configure-secure-bbs-client", err)
	}
	return bbsClient
}
