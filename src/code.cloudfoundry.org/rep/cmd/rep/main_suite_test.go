package main_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"code.cloudfoundry.org/bbs"
	bbsconfig "code.cloudfoundry.org/bbs/cmd/bbs/config"
	bbstestrunner "code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/bbs/test_helpers/sqlrunner"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"code.cloudfoundry.org/inigo/helpers/portauthority"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/locket"
	locketconfig "code.cloudfoundry.org/locket/cmd/locket/config"
	locketrunner "code.cloudfoundry.org/locket/cmd/locket/testrunner"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"google.golang.org/grpc/grpclog"
)

const (
	requestIdHeader = "f256f938-9e14-4abd-974f-63c6138f1cca"
)

var (
	cellID              string
	representativePath  string
	serverPort          uint16
	serverPortSecurable uint16

	bbsConfig        bbsconfig.BBSConfig
	bbsBinPath       string
	bbsURL           *url.URL
	bbsRunner        *ginkgomon.Runner
	bbsProcess       ifrit.Process
	bbsClient        bbs.InternalClient
	auctioneerServer *ghttp.Server
	node             int

	locketAddress string
	locketBinPath string
	locketProcess ifrit.Process
	locketRunner  *ginkgomon.Runner

	testIngressServer *testhelpers.TestIngressServer

	testMetricsChan   chan *loggregator_v2.Envelope
	signalMetricsChan chan struct{}

	sqlProcess                                              ifrit.Process
	sqlRunner                                               sqlrunner.SQLRunner
	portAllocator                                           portauthority.PortAllocator
	metronCAFile, metronServerCertFile, metronServerKeyFile string

	fixturesPath = "fixtures"
)

func TestRep(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rep Integration Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	bbsConfig, err := gexec.Build("code.cloudfoundry.org/bbs/cmd/bbs", "-race")
	Expect(err).NotTo(HaveOccurred())

	locketPath, err := gexec.Build("code.cloudfoundry.org/locket/cmd/locket", "-race")
	Expect(err).NotTo(HaveOccurred())

	representative, err := gexec.Build("code.cloudfoundry.org/rep/cmd/rep", "-race")
	Expect(err).NotTo(HaveOccurred())

	return []byte(strings.Join([]string{representative, locketPath, bbsConfig}, ","))
}, func(pathsByte []byte) {

	node = GinkgoParallelProcess()
	startPort := 1060*node + 10
	portRange := 1000
	endPort := startPort + portRange

	var err error
	portAllocator, err = portauthority.New(startPort, endPort)
	Expect(err).NotTo(HaveOccurred())

	grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, io.Discard, io.Discard))

	// tests here are fairly Eventually driven which tends to flake out under
	// load (for insignificant reasons); bump the default a bit higher than the
	// default (1 second)
	SetDefaultEventuallyTimeout(5 * time.Second)

	strPath := string(pathsByte)
	representativePath = strings.Split(strPath, ",")[0]
	locketBinPath = strings.Split(strPath, ",")[1]
	bbsBinPath = strings.Split(strPath, ",")[2]

	cellID = "the_rep_id-" + strconv.Itoa(GinkgoParallelProcess())

	serverPort, err = portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())
	serverPortSecurable, err = portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())

	dbName := fmt.Sprintf("diego_rep_%d", GinkgoParallelProcess())

	sqlRunner = test_helpers.NewSQLRunner(dbName)
	sqlProcess = ginkgomon.Invoke(sqlRunner)

	locketPort, err := portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())
	locketAddress = fmt.Sprintf("127.0.0.1:%d", locketPort)

	bbsPort, err := portAllocator.ClaimPorts(2)
	Expect(err).NotTo(HaveOccurred())
	healthPort := bbsPort + 1
	bbsAddress := fmt.Sprintf("127.0.0.1:%d", bbsPort)
	healthAddress := fmt.Sprintf("127.0.0.1:%d", healthPort)

	bbsURL = &url.URL{
		Scheme: "https",
		Host:   bbsAddress,
	}

	auctioneerServer = ghttp.NewServer()
	auctioneerServer.UnhandledRequestStatusCode = http.StatusAccepted
	auctioneerServer.AllowUnhandledRequests = true

	bbsConfig = bbsconfig.BBSConfig{
		ListenAddress:            bbsAddress,
		AdvertiseURL:             bbsURL.String(),
		AuctioneerAddress:        auctioneerServer.URL(),
		DatabaseDriver:           sqlRunner.DriverName(),
		DatabaseConnectionString: sqlRunner.ConnectionString(),
		HealthAddress:            healthAddress,
		EncryptionConfig: encryption.EncryptionConfig{
			EncryptionKeys: map[string]string{"label": "key"},
			ActiveKeyLabel: "label",
		},

		CaFile:   path.Join(fixturesPath, "green-certs", "server-ca.crt"),
		CertFile: path.Join(fixturesPath, "green-certs", "server.crt"),
		KeyFile:  path.Join(fixturesPath, "green-certs", "server.key"),

		UUID:                        "bbs",
		SessionName:                 "bbs",
		CommunicationTimeout:        durationjson.Duration(10 * time.Second),
		RequireSSL:                  true,
		DesiredLRPCreationTimeout:   durationjson.Duration(1 * time.Minute),
		ExpireCompletedTaskDuration: durationjson.Duration(2 * time.Minute),
		ExpirePendingTaskDuration:   durationjson.Duration(30 * time.Minute),
		ConvergeRepeatInterval:      durationjson.Duration(30 * time.Second),
		KickTaskDuration:            durationjson.Duration(30 * time.Second),
		DBConnectionTimeout:         durationjson.Duration(30 * time.Second),
		LockTTL:                     durationjson.Duration(locket.DefaultSessionTTL),
		LockRetryInterval:           durationjson.Duration(locket.RetryInterval),
		ReportInterval:              durationjson.Duration(1 * time.Minute),
		ConvergenceWorkers:          20,
		UpdateWorkers:               1000,
		TaskCallbackWorkers:         1000,
		MaxOpenDatabaseConnections:  200,
		MaxIdleDatabaseConnections:  200,
		AuctioneerRequireTLS:        false,
		RepClientSessionCacheSize:   0,
		LagerConfig: lagerflags.LagerConfig{
			LogLevel: "info",
		},
		LoggregatorConfig: loggingclient.Config{
			CACertPath: path.Join(fixturesPath, "metron", "CA.crt"),
			CertPath:   path.Join(fixturesPath, "metron", "client.crt"),
			KeyPath:    path.Join(fixturesPath, "metron", "client.key"),
		},
		ClientLocketConfig: locketrunner.ClientLocketConfig(),
	}
	bbsConfig.ClientLocketConfig.LocketAddress = locketAddress
})

var _ = BeforeEach(func() {

	var err error
	metronCAFile = path.Join(fixturesPath, "metron", "CA.crt")
	metronServerCertFile = path.Join(fixturesPath, "metron", "metron.crt")
	metronServerKeyFile = path.Join(fixturesPath, "metron", "metron.key")
	testIngressServer, err = testhelpers.NewTestIngressServer(metronServerCertFile, metronServerKeyFile, metronCAFile)
	Expect(err).NotTo(HaveOccurred())
	receiversChan := testIngressServer.Receivers()
	testIngressServer.Start()

	testMetricsChan, signalMetricsChan = testhelpers.TestMetricChan(receiversChan)

	locketRunner = locketrunner.NewLocketRunner(locketBinPath, func(cfg *locketconfig.LocketConfig) {
		cfg.DatabaseConnectionString = sqlRunner.ConnectionString()
		cfg.DatabaseDriver = sqlRunner.DriverName()
		cfg.ListenAddress = locketAddress
		cfg.LoggregatorConfig.APIPort, _ = testIngressServer.Port()
		cfg.LoggregatorConfig.CACertPath = metronCAFile
		cfg.LoggregatorConfig.CertPath = metronServerCertFile
		cfg.LoggregatorConfig.KeyPath = metronServerKeyFile

	})
	bbsConfig.LoggregatorConfig.APIPort, _ = testIngressServer.Port()
	logger := lagertest.NewTestLogger("test")
	logger.Debug(fmt.Sprintf("bbs locket address: %s", locketAddress))
	locketProcess = ginkgomon.Invoke(locketRunner)

	bbsRunner = bbstestrunner.New(bbsBinPath, bbsConfig)
	bbsProcess = ginkgomon.Invoke(bbsRunner)
})

var _ = AfterEach(func() {
	ginkgomon.Kill(locketProcess)
	Eventually(locketProcess.Wait()).Should(Receive())

	ginkgomon.Kill(bbsProcess)
	Eventually(bbsProcess.Wait()).Should(Receive())

	Eventually(func() error {
		l, err := net.Listen("tcp", bbsConfig.HealthAddress)
		if err != nil {
			return err
		}
		return l.Close()
	}, 10*time.Second, 100*time.Millisecond).Should(Succeed())

	testIngressServer.Stop()
	close(signalMetricsChan)
	sqlRunner.ResetTables([]string{"domains", "configurations", "tasks", "desired_lrps", "actual_lrps", "locks"})
})

var _ = SynchronizedAfterSuite(func() {
	ginkgomon.Kill(sqlProcess, "10s")
	if runner != nil {
		runner.KillWithFire()
	}
	if auctioneerServer != nil {
		auctioneerServer.Close()
	}
}, func() {
	gexec.CleanupBuildArtifacts()
})
