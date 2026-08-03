package cell_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	auctioneerconfig "code.cloudfoundry.org/auctioneer/cmd/auctioneer/config"
	"code.cloudfoundry.org/durationjson"
	fileserverconfig "code.cloudfoundry.org/fileserver/cmd/file-server/config"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/localip"
	repconfig "code.cloudfoundry.org/rep/cmd/rep/config"
	routeemitterconfig "code.cloudfoundry.org/route-emitter/cmd/route-emitter/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/tedsuo/ifrit/grouper"

	"code.cloudfoundry.org/bbs"
	bbsconfig "code.cloudfoundry.org/bbs/cmd/bbs/config"
	"code.cloudfoundry.org/bbs/serviceclient"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"code.cloudfoundry.org/inigo/helpers"
	"code.cloudfoundry.org/inigo/helpers/certauthority"
	"code.cloudfoundry.org/inigo/helpers/portauthority"
	"code.cloudfoundry.org/inigo/inigo_announcement_server"
	"code.cloudfoundry.org/inigo/world"
	locketconfig "code.cloudfoundry.org/locket/cmd/locket/config"
)

const cellSuiteEventuallyTestTimeout = 30 * time.Second
const cellSuiteEventuallyPollingInterval = 1 * time.Second

var (
	componentMaker world.ComponentMaker

	plumbing, bbsProcess, gardenProcess ifrit.Process
	gardenClient                        garden.Client
	bbsClient                           bbs.InternalClient
	bbsServiceClient                    serviceclient.ServiceClient
	bbsRunner                           *ginkgomon.Runner
	gardenRunner                        *runner.GardenRunner
	lgr                                 lager.Logger
	suiteTempDir                        string

	testMetricsChan   chan *loggregator_v2.Envelope
	signalMetricsChan chan struct{}
	testIngressServer *testhelpers.TestIngressServer

	modifyFunAuctioneerLoggregatorConfig                    func(cfg *auctioneerconfig.AuctioneerConfig)
	modifyFunRouteEmitterLoggregatorConfig                  func(cfg *routeemitterconfig.RouteEmitterConfig)
	modifyFunRepLoggregatorConfig                           func(cfg *repconfig.RepConfig)
	modifyFunFileServerLoggregatorConfig                    func(cfg *fileserverconfig.FileServerConfig)
	modifyFuncBBSLoggregatorConfig                          func(cfg *bbsconfig.BBSConfig)
	metronCAFile, metronServerCertFile, metronServerKeyFile string
)

func overrideConvergenceRepeatInterval(conf *bbsconfig.BBSConfig) {
	conf.ConvergeRepeatInterval = durationjson.Duration(time.Second)
}

var _ = SynchronizedBeforeSuite(func() []byte {
	suiteTempDir = world.TempDir("before-suite")
	artifacts := world.BuiltArtifacts{
		Lifecycles: world.BuiltLifecycles{},
	}

	artifacts.Lifecycles.BuildLifecycles("dockerapplifecycle", suiteTempDir)
	artifacts.Executables = CompileTestedExecutables()
	artifacts.Healthcheck = CompileHealthcheckExecutable(suiteTempDir)

	payload, err := json.Marshal(artifacts)
	Expect(err).NotTo(HaveOccurred())

	return payload
}, func(encodedBuiltArtifacts []byte) {
	var builtArtifacts world.BuiltArtifacts

	err := json.Unmarshal(encodedBuiltArtifacts, &builtArtifacts)
	Expect(err).NotTo(HaveOccurred())

	_, dbBaseConnectionString := world.DBInfo()

	localIP, err := localip.LocalIP()
	Expect(err).NotTo(HaveOccurred())

	addresses := world.ComponentAddresses{
		Garden:              fmt.Sprintf("127.0.0.1:%d", 10000+GinkgoParallelProcess()),
		NATS:                fmt.Sprintf("127.0.0.1:%d", 11000+GinkgoParallelProcess()),
		Rep:                 fmt.Sprintf("127.0.0.1:%d", 14000+GinkgoParallelProcess()),
		FileServer:          fmt.Sprintf("%s:%d", localIP, 17000+GinkgoParallelProcess()),
		Router:              fmt.Sprintf("127.0.0.1:%d", 18000+GinkgoParallelProcess()),
		RouterStatus:        fmt.Sprintf("127.0.0.1:%d", 18100+GinkgoParallelProcess()),
		RouterRoutes:        fmt.Sprintf("127.0.0.1:%d", 18200+GinkgoParallelProcess()),
		RouterRouteServices: fmt.Sprintf("127.0.0.1:%d", 18300+GinkgoParallelProcess()),
		BBS:                 fmt.Sprintf("127.0.0.1:%d", 20500+GinkgoParallelProcess()*2),
		Health:              fmt.Sprintf("127.0.0.1:%d", 20500+GinkgoParallelProcess()*2+1),
		Auctioneer:          fmt.Sprintf("127.0.0.1:%d", 23000+GinkgoParallelProcess()),
		SSHProxy:            fmt.Sprintf("127.0.0.1:%d", 23500+GinkgoParallelProcess()),
		SSHProxyHealthCheck: fmt.Sprintf("127.0.0.1:%d", 24500+GinkgoParallelProcess()),
		FakeVolmanDriver:    fmt.Sprintf("127.0.0.1:%d", 25500+GinkgoParallelProcess()),
		Locket:              fmt.Sprintf("127.0.0.1:%d", 26500+GinkgoParallelProcess()),
		SQL:                 fmt.Sprintf("%sdiego_%d", dbBaseConnectionString, GinkgoParallelProcess()),
	}

	node := GinkgoParallelProcess()
	startPort := 1000 * node
	portRange := 950
	endPort := startPort + portRange

	allocator, err := portauthority.New(startPort, endPort)
	Expect(err).NotTo(HaveOccurred())

	certDepot := world.TempDirWithParent(suiteTempDir, "cert-depot")

	certAuthority, err := certauthority.NewCertAuthority(certDepot, "ca")
	Expect(err).NotTo(HaveOccurred())

	componentMaker = world.MakeComponentMaker(builtArtifacts, addresses, allocator, certAuthority)
	componentMaker.Setup()
})

var _ = SynchronizedAfterSuite(func() {
	if componentMaker != nil {
		componentMaker.Teardown()
	}
}, func() {
	gexec.CleanupBuildArtifacts()
	os.RemoveAll(suiteTempDir)
})

var _ = BeforeEach(func() {

	fixturesPath := "../fixtures/certs"
	var err error
	metronCAFile = filepath.Join(fixturesPath, "metron", "CA.crt")
	metronServerCertFile = filepath.Join(fixturesPath, "metron", "metron.crt")
	metronServerKeyFile = filepath.Join(fixturesPath, "metron", "metron.key")
	testIngressServer, err = testhelpers.NewTestIngressServer(metronServerCertFile, metronServerKeyFile, metronCAFile)
	Expect(err).NotTo(HaveOccurred())
	receiversChan := testIngressServer.Receivers()
	testIngressServer.Start()

	testMetricsChan, signalMetricsChan = testhelpers.TestMetricChan(receiversChan)

	modifyFuncLocketLoggregatorConfig := func(cfg *locketconfig.LocketConfig) {
		cfg.LoggregatorConfig = setupMetronConfig(cfg.LoggregatorConfig)
	}

	modifyFuncBBSLoggregatorConfig = func(cfg *bbsconfig.BBSConfig) {
		cfg.LoggregatorConfig = setupMetronConfig(cfg.LoggregatorConfig)
	}

	modifyFunAuctioneerLoggregatorConfig = func(cfg *auctioneerconfig.AuctioneerConfig) {
		cfg.LoggregatorConfig = setupMetronConfig(cfg.LoggregatorConfig)
	}

	modifyFunRouteEmitterLoggregatorConfig = func(cfg *routeemitterconfig.RouteEmitterConfig) {
		cfg.LoggregatorConfig = setupMetronConfig(cfg.LoggregatorConfig)
	}

	modifyFunRepLoggregatorConfig = func(cfg *repconfig.RepConfig) {
		cfg.LoggregatorConfig = setupMetronConfig(cfg.LoggregatorConfig)
	}

	modifyFunFileServerLoggregatorConfig = func(cfg *fileserverconfig.FileServerConfig) {
		cfg.LoggregatorConfig = setupMetronConfig(cfg.LoggregatorConfig)
	}

	plumbing = ginkgomon.Invoke(grouper.NewOrdered(os.Kill, grouper.Members{
		{Name: "initial-services", Runner: grouper.NewParallel(os.Kill, grouper.Members{
			{Name: "sql", Runner: componentMaker.SQL()},
			{Name: "nats", Runner: componentMaker.NATS()},
		})},
		{Name: "locket", Runner: componentMaker.Locket(modifyFuncLocketLoggregatorConfig)},
	}))
	gardenRunner = componentMaker.Garden()
	gardenProcess = ginkgomon.Invoke(gardenRunner)
	bbsRunner = componentMaker.BBS(modifyFuncBBSLoggregatorConfig)
	bbsProcess = ginkgomon.Invoke(bbsRunner)

	lgr = lager.NewLogger("test")
	lgr.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))

	gardenClient = componentMaker.GardenClient()
	bbsClient = componentMaker.BBSClient()
	bbsServiceClient = componentMaker.BBSServiceClient(lgr)

	inigo_announcement_server.Start(os.Getenv("EXTERNAL_ADDRESS"))
})

var _ = AfterEach(func() {
	inigo_announcement_server.Stop()

	destroyContainerErrors := helpers.CleanupGarden(gardenClient)

	helpers.StopProcesses(bbsProcess)
	helpers.StopProcesses(gardenProcess)
	helpers.StopProcesses(plumbing)

	testIngressServer.Stop()
	close(signalMetricsChan)

	Expect(destroyContainerErrors).To(
		BeEmpty(),
		"%d containers failed to be destroyed!",
		len(destroyContainerErrors),
	)
})

func TestCell(t *testing.T) {
	helpers.RegisterDefaultTimeouts()

	RegisterFailHandler(Fail)

	RunSpecs(t, "Cell Integration Suite")
}

func CompileHealthcheckExecutable(tmpDir string) string {
	healthcheckDir := world.TempDirWithParent(tmpDir, "healthcheck")
	healthcheckPath, err := gexec.Build("code.cloudfoundry.org/healthcheck/cmd/healthcheck", "-race")
	Expect(err).NotTo(HaveOccurred())

	if runtime.GOOS == "windows" {
		err = os.Rename(healthcheckPath, filepath.Join(healthcheckDir, "healthcheck.exe"))
		Expect(err).NotTo(HaveOccurred())
	} else {
		err = os.Rename(healthcheckPath, filepath.Join(healthcheckDir, "healthcheck"))
		Expect(err).NotTo(HaveOccurred())
	}

	return healthcheckDir
}

func CompileTestedExecutables() world.BuiltExecutables {
	var err error

	builtExecutables := world.BuiltExecutables{}

	cwd, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Chdir(os.Getenv("GARDEN_GOPATH"))).To(Succeed())
	builtExecutables["garden"], err = gexec.Build("./cmd/gdn", "-race", "-a", "-tags", "daemon")
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Chdir(cwd)).To(Succeed())

	builtExecutables["auctioneer"], err = gexec.Build("code.cloudfoundry.org/auctioneer/cmd/auctioneer", "-race")
	Expect(err).NotTo(HaveOccurred())

	builtExecutables["rep"], err = gexec.Build("code.cloudfoundry.org/rep/cmd/rep", "-race")
	Expect(err).NotTo(HaveOccurred())

	builtExecutables["bbs"], err = gexec.Build("code.cloudfoundry.org/bbs/cmd/bbs", "-race")
	Expect(err).NotTo(HaveOccurred())

	builtExecutables["locket"], err = gexec.Build("code.cloudfoundry.org/locket/cmd/locket", "-race")
	Expect(err).NotTo(HaveOccurred())

	builtExecutables["file-server"], err = gexec.Build("code.cloudfoundry.org/fileserver/cmd/file-server", "-race")
	Expect(err).NotTo(HaveOccurred())

	builtExecutables["route-emitter"], err = gexec.Build("code.cloudfoundry.org/route-emitter/cmd/route-emitter", "-race")
	Expect(err).NotTo(HaveOccurred())

	if runtime.GOOS != "windows" {
		Expect(os.Chdir(os.Getenv("ROUTER_GOPATH"))).To(Succeed())
		builtExecutables["router"], err = gexec.Build("code.cloudfoundry.org/gorouter/cmd/gorouter", "-race")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chdir(cwd)).To(Succeed())
	}

	builtExecutables["routing-api"], err = gexec.Build("code.cloudfoundry.org/routing-api/cmd/routing-api", "-race")
	Expect(err).NotTo(HaveOccurred())

	builtExecutables["ssh-proxy"], err = gexec.Build("code.cloudfoundry.org/diego-ssh/cmd/ssh-proxy", "-race")
	Expect(err).NotTo(HaveOccurred())

	os.Setenv("CGO_ENABLED", "0")
	builtExecutables["sshd"], err = gexec.Build("code.cloudfoundry.org/diego-ssh/cmd/sshd", "-a", "-installsuffix", "static")
	os.Unsetenv("CGO_ENABLED")
	Expect(err).NotTo(HaveOccurred())

	return builtExecutables
}

func setupMetronConfig(cfg loggingclient.Config) loggingclient.Config {
	cfg.APIPort, _ = testIngressServer.Port()
	cfg.CACertPath = metronCAFile
	cfg.CertPath = metronServerCertFile
	cfg.KeyPath = metronServerKeyFile
	return cfg
}
