package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"

	cf_debug_server "code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/tlsconfig"

	"code.cloudfoundry.org/dockerdriver"
	"code.cloudfoundry.org/dockerdriver/driverhttp"
	"code.cloudfoundry.org/goshims/filepathshim"
	"code.cloudfoundry.org/goshims/osshim"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/localdriver"
	"code.cloudfoundry.org/localdriver/oshelper"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

var atAddress = flag.String(
	"listenAddr",
	"0.0.0.0:9750",
	"host:port to serve volume management functions",
)

var driversPath = flag.String(
	"driversPath",
	"",
	"Path to directory where drivers are installed",
)

var transport = flag.String(
	"transport",
	"tcp",
	"Transport protocol to transmit HTTP over",
)

var mountDir = flag.String(
	"mountDir",
	"/tmp/volumes",
	"Path to directory where local volumes are created",
)

var requireSSL = flag.Bool(
	"requireSSL",
	false,
	"whether the local driver should require ssl-secured communication",
)

var caFile = flag.String(
	"caFile",
	"",
	"the certificate authority public key file to use with ssl authentication",
)

var certFile = flag.String(
	"certFile",
	"",
	"the public key file to use with ssl authentication",
)

var keyFile = flag.String(
	"keyFile",
	"",
	"the private key file to use with ssl authentication",
)
var clientCertFile = flag.String(
	"clientCertFile",
	"",
	"the public key file to use with client ssl authentication",
)

var clientKeyFile = flag.String(
	"clientKeyFile",
	"",
	"the private key file to use with client ssl authentication",
)

var insecureSkipVerify = flag.Bool(
	"insecureSkipVerify",
	false,
	"whether SSL communication should skip verification of server IP addresses in the certificate",
)

var uniqueVolumeIds = flag.Bool(
	"uniqueVolumeIds",
	false,
	"whether the local driver should opt-in to unique volumes",
)

func main() {
	parseCommandLine()

	var logger lager.Logger
	var logTap *lager.ReconfigurableSink

	var localDriverServer ifrit.Runner

	if *transport == "tcp" {
		logger, logTap = newLogger()
		defer logger.Info("ends")
		localDriverServer = createLocalDriverServer(logger, *atAddress, *driversPath, *mountDir, false, false)
	} else if *transport == "tcp-json" {
		logger, logTap = newLogger()
		defer logger.Info("ends")
		localDriverServer = createLocalDriverServer(logger, *atAddress, *driversPath, *mountDir, true, *uniqueVolumeIds)
	} else {
		logger, logTap = newUnixLogger()
		defer logger.Info("ends")

		localDriverServer = createLocalDriverUnixServer(logger, *atAddress, *driversPath, *mountDir)
	}

	servers := grouper.Members{
		{Name: "localdriver-server", Runner: localDriverServer},
	}
	if dbgAddr := cf_debug_server.DebugAddress(flag.CommandLine); dbgAddr != "" {
		servers = append(grouper.Members{
			{Name: "debug-server", Runner: cf_debug_server.Runner(dbgAddr, logTap)},
		}, servers...)
	}
	process := ifrit.Invoke(processRunnerFor(servers))

	logger.Info("started")

	untilTerminated(logger, process)
}

func exitOnFailure(logger lager.Logger, err error) {
	if err != nil {
		logger.Fatal("fatal-err-aborting", err)
	}
}

func untilTerminated(logger lager.Logger, process ifrit.Process) {
	err := <-process.Wait()
	exitOnFailure(logger, err)
}

func processRunnerFor(servers grouper.Members) ifrit.Runner {
	return sigmon.New(grouper.NewOrdered(os.Interrupt, servers))
}

func createLocalDriverServer(logger lager.Logger, atAddress, driversPath, mountDir string, jsonSpec bool, uniqueVolumeIds bool) ifrit.Runner {
	advertisedUrl := "http://" + atAddress
	logger.Info("writing-spec-file", lager.Data{"location": driversPath, "name": "localdriver", "address": advertisedUrl})
	if jsonSpec {
		driverJsonSpec := dockerdriver.DriverSpec{Name: "localdriver", Address: advertisedUrl, UniqueVolumeIds: uniqueVolumeIds}

		if *requireSSL {
			absCaFile, err := filepath.Abs(*caFile)
			exitOnFailure(logger, err)
			absClientCertFile, err := filepath.Abs(*clientCertFile)
			exitOnFailure(logger, err)
			absClientKeyFile, err := filepath.Abs(*clientKeyFile)
			exitOnFailure(logger, err)
			driverJsonSpec.TLSConfig = &dockerdriver.TLSConfig{InsecureSkipVerify: *insecureSkipVerify, CAFile: absCaFile, CertFile: absClientCertFile, KeyFile: absClientKeyFile}
			driverJsonSpec.Address = "https://" + atAddress
		}

		jsonBytes, err := json.Marshal(driverJsonSpec)

		exitOnFailure(logger, err)
		err = dockerdriver.WriteDriverSpec(logger, driversPath, "localdriver", "json", jsonBytes)
		exitOnFailure(logger, err)
	} else {
		err := dockerdriver.WriteDriverSpec(logger, driversPath, "localdriver", "spec", []byte(advertisedUrl))
		exitOnFailure(logger, err)
	}

	client := localdriver.NewLocalDriver(&osshim.OsShim{}, &filepathshim.FilepathShim{}, mountDir, oshelper.NewOsHelper(), uniqueVolumeIds)
	handler, err := driverhttp.NewHandler(logger, client)
	exitOnFailure(logger, err)

	var server ifrit.Runner
	if *requireSSL {
		tlsConfig, err := tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(*certFile, *keyFile),
		).Server(tlsconfig.WithClientAuthenticationFromFile(*caFile))
		if err != nil {
			logger.Fatal("tls-configuration-failed", err)
		}
		server = http_server.NewTLSServer(atAddress, handler, tlsConfig)
	} else {
		server = http_server.New(atAddress, handler)
	}

	return server
}

func createLocalDriverUnixServer(logger lager.Logger, atAddress, driversPath, mountDir string) ifrit.Runner {
	client := localdriver.NewLocalDriver(&osshim.OsShim{}, &filepathshim.FilepathShim{}, mountDir, oshelper.NewOsHelper(), false)
	handler, err := driverhttp.NewHandler(logger, client)
	exitOnFailure(logger, err)
	return http_server.NewUnixServer(atAddress, handler)
}

func newLogger() (lager.Logger, *lager.ReconfigurableSink) {
	return lagerflags.NewFromConfig("localdriver-server", lagerflags.ConfigFromFlags())
}

func newUnixLogger() (lager.Logger, *lager.ReconfigurableSink) {
	logger, reconfigurableSink := lagerflags.New("localdriver-server")
	return logger, reconfigurableSink
}

func parseCommandLine() {
	lagerflags.AddFlags(flag.CommandLine)
	cf_debug_server.AddFlags(flag.CommandLine)
	flag.Parse()
}
