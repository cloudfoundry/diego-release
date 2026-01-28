package main_test

import (
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"testing"
	"time"

	"code.cloudfoundry.org/bbs"
	bbsconfig "code.cloudfoundry.org/bbs/cmd/bbs/config"
	bbstestrunner "code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/bbs/test_helpers/sqlrunner"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/inigo/helpers/portauthority"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/locket"
	locketconfig "code.cloudfoundry.org/locket/cmd/locket/config"
	locketrunner "code.cloudfoundry.org/locket/cmd/locket/testrunner"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"google.golang.org/grpc/grpclog"
)

var (
	auctioneerPath       string
	auctioneerServerPort uint16
	auctioneerLocation   string

	bbsConfig  bbsconfig.BBSConfig
	bbsBinPath string
	bbsURL     *url.URL
	bbsRunner  *ginkgomon.Runner
	bbsProcess ifrit.Process
	bbsClient  bbs.InternalClient

	locketAddress string
	locketBinPath string
	locketProcess ifrit.Process
	locketRunner  *ginkgomon.Runner

	sqlProcess ifrit.Process
	sqlRunner  sqlrunner.SQLRunner

	logger        lager.Logger
	portAllocator portauthority.PortAllocator
)

func TestAuctioneer(t *testing.T) {
	// these integration tests can take a bit, especially under load;
	// 1 second is too harsh
	SetDefaultEventuallyTimeout(10 * time.Second)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Auctioneer Cmd Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	compiledBBSPath, err := gexec.Build("code.cloudfoundry.org/bbs/cmd/bbs", "-race")
	Expect(err).NotTo(HaveOccurred())

	compiledAuctioneerPath, err := gexec.Build("code.cloudfoundry.org/auctioneer/cmd/auctioneer", "-race")
	Expect(err).NotTo(HaveOccurred())

	locketPath, err := gexec.Build("code.cloudfoundry.org/locket/cmd/locket", "-race")
	Expect(err).NotTo(HaveOccurred())

	return []byte(strings.Join([]string{compiledAuctioneerPath, compiledBBSPath, locketPath}, ","))
}, func(pathsByte []byte) {
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, io.Discard, io.Discard))
	node := GinkgoParallelProcess()
	startPort := 1050 * node
	portRange := 1000
	endPort := startPort + portRange

	var err error
	portAllocator, err = portauthority.New(startPort, endPort)
	Expect(err).NotTo(HaveOccurred())

	paths := string(pathsByte)
	auctioneerPath = strings.Split(paths, ",")[0]
	bbsBinPath = strings.Split(paths, ",")[1]
	locketBinPath = strings.Split(paths, ",")[2]

	dbName := fmt.Sprintf("diego_%d", GinkgoParallelProcess())
	sqlRunner = test_helpers.NewSQLRunner(dbName)
	sqlProcess = ginkgomon.Invoke(sqlRunner)

	locketPort, err := portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())
	locketAddress = fmt.Sprintf("localhost:%d", locketPort)

	auctioneerServerPort, err = portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())
	auctioneerLocation = fmt.Sprintf("127.0.0.1:%d", auctioneerServerPort)

	logger = lagertest.NewTestLogger("test")

	bbsPort, err := portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())
	healthPort, err := portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())
	bbsAddress := fmt.Sprintf("127.0.0.1:%d", bbsPort)
	healthAddress := fmt.Sprintf("127.0.0.1:%d", healthPort)

	bbsURL = &url.URL{
		Scheme: "https",
		Host:   bbsAddress,
	}

	fixturesPath := "fixtures"

	bbsConfig = bbsconfig.BBSConfig{
		UUID:                        "bbs",
		SessionName:                 "bbs",
		CommunicationTimeout:        durationjson.Duration(10 * time.Second),
		RequireSSL:                  true,
		DesiredLRPCreationTimeout:   durationjson.Duration(1 * time.Minute),
		ExpireCompletedTaskDuration: durationjson.Duration(2 * time.Minute),
		ExpirePendingTaskDuration:   durationjson.Duration(30 * time.Minute),
		ConvergeRepeatInterval:      durationjson.Duration(30 * time.Second),
		KickTaskDuration:            durationjson.Duration(30 * time.Second),
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
		LagerConfig:                 lagerflags.DefaultLagerConfig(),

		ListenAddress:     bbsAddress,
		AdvertiseURL:      bbsURL.String(),
		AuctioneerAddress: "http://" + auctioneerLocation,
		HealthAddress:     healthAddress,
		EncryptionConfig: encryption.EncryptionConfig{
			EncryptionKeys: map[string]string{"label": "key"},
			ActiveKeyLabel: "label",
		},

		DatabaseDriver:           sqlRunner.DriverName(),
		DatabaseConnectionString: sqlRunner.ConnectionString(),

		ClientLocketConfig: locketrunner.ClientLocketConfig(),

		CaFile:   path.Join(fixturesPath, "green-certs", "ca.crt"),
		CertFile: path.Join(fixturesPath, "green-certs", "server.crt"),
		KeyFile:  path.Join(fixturesPath, "green-certs", "server.key"),
	}
	bbsConfig.ClientLocketConfig.LocketAddress = locketAddress
})

var _ = BeforeEach(func() {
	locketRunner = locketrunner.NewLocketRunner(locketBinPath, func(cfg *locketconfig.LocketConfig) {
		cfg.DatabaseConnectionString = sqlRunner.ConnectionString()
		cfg.DatabaseDriver = sqlRunner.DriverName()
		cfg.ListenAddress = locketAddress
	})
	locketProcess = ginkgomon.Invoke(locketRunner)

	bbsRunner = bbstestrunner.New(bbsBinPath, bbsConfig)
	bbsProcess = ginkgomon.Invoke(bbsRunner)
})

var _ = AfterEach(func() {
	ginkgomon.Interrupt(locketProcess)
	ginkgomon.Kill(bbsProcess)

	sqlRunner.Reset()
})

var _ = SynchronizedAfterSuite(func() {
	ginkgomon.Kill(sqlProcess)
}, func() {
	gexec.CleanupBuildArtifacts()
})
