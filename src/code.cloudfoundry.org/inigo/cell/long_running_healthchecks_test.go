package cell_test

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	archive_helper "code.cloudfoundry.org/archiver/extractor/test_helper"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"
	logging "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/inigo/fixtures"
	"code.cloudfoundry.org/inigo/helpers"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/cmd/rep/config"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/tedsuo/ifrit/grouper"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Context("when declarative healthchecks is turned on", func() {
	var (
		processGuid         string
		archiveFiles        []archive_helper.ArchiveFile
		fileServerStaticDir string
		cellPortsStart      uint16
		cellAddr            string
		cellSecureAddr      string

		ifritRuntime ifrit.Process

		lock              *sync.Mutex
		eventSource       events.EventSource
		events            []models.Event
		testIngressServer *testhelpers.TestIngressServer
	)

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip(" not yet working on windows")
		}

		processGuid = helpers.GenerateGuid()

		var err error
		cellPortsStart, err = componentMaker.PortAllocator().ClaimPorts(2)
		Expect(err).NotTo(HaveOccurred())
		cellAddr = fmt.Sprintf("0.0.0.0:%d", cellPortsStart)
		cellSecureAddr = fmt.Sprintf("0.0.0.0:%d", cellPortsStart+1)

		var fileServer ifrit.Runner
		fileServer, fileServerStaticDir = componentMaker.FileServer(modifyFunFileServerLoggregatorConfig)

		turnOnLongRunningHealthchecks := func(cfg *config.RepConfig) {
			cfg.DeclarativeHealthCheckDefaultTimeout = durationjson.Duration(1 * time.Second)
			cfg.DeclarativeHealthcheckPath = componentMaker.Artifacts().Healthcheck
			cfg.HealthCheckWorkPoolSize = 1
			cfg.ListenAddr = cellAddr
			cfg.ListenAddrSecurable = cellSecureAddr
		}

		fixturesPath := path.Join("..", "fixtures", "certs")
		metronCAFile := path.Join(fixturesPath, "metron", "CA.crt")
		metronClientCertFile := path.Join(fixturesPath, "metron", "client.crt")
		metronClientKeyFile := path.Join(fixturesPath, "metron", "client.key")
		metronServerCertFile := path.Join(fixturesPath, "metron", "metron.crt")
		metronServerKeyFile := path.Join(fixturesPath, "metron", "metron.key")
		testIngressServer, err = testhelpers.NewTestIngressServer(metronServerCertFile, metronServerKeyFile, metronCAFile)
		Expect(err).NotTo(HaveOccurred())

		Expect(testIngressServer.Start()).To(Succeed())

		metricsPort, err := testIngressServer.Port()
		Expect(err).NotTo(HaveOccurred())

		loggregatorConfig := func(cfg *config.RepConfig) {
			cfg.LoggregatorConfig = logging.Config{
				BatchFlushInterval: 10 * time.Millisecond,
				BatchMaxSize:       1,
				APIPort:            metricsPort,
				CACertPath:         metronCAFile,
				KeyPath:            metronClientKeyFile,
				CertPath:           metronClientCertFile,
			}
			cfg.ContainerMetricsReportInterval = durationjson.Duration(5 * time.Second)
		}

		logger := lagertest.NewTestLogger("metron-agent")
		metronAgent := ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)
			testMetricsChan, signalMetricsChan := testhelpers.TestMetricChan(testIngressServer.Receivers())
			defer close(signalMetricsChan)
			for {
				select {
				case envelope := <-testMetricsChan:
					if log := envelope.GetLog(); log != nil {
						logger.Info("received-data", lager.Data{"message": string(log.GetPayload())})
					}
				case <-signals:
					return nil
				}
			}
		})

		ifritRuntime = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{Name: "router", Runner: componentMaker.Router()},
			{Name: "file-server", Runner: fileServer},
			{Name: "metron-agent", Runner: metronAgent},
			{Name: "rep", Runner: componentMaker.Rep(turnOnLongRunningHealthchecks, loggregatorConfig)},
			{Name: "auctioneer", Runner: componentMaker.Auctioneer(modifyFunAuctioneerLoggregatorConfig)},
			{Name: "route-emitter", Runner: componentMaker.RouteEmitter(modifyFunRouteEmitterLoggregatorConfig)},
		}))

		archiveFiles = fixtures.GoServerApp()
		archive_helper.CreateZipArchive(
			filepath.Join(fileServerStaticDir, "lrp.zip"),
			archiveFiles,
		)

		lock = &sync.Mutex{}
	})

	JustBeforeEach(func() {
		var err error
		eventSource, err = bbsClient.SubscribeToInstanceEvents(lgr)
		Expect(err).NotTo(HaveOccurred())
		go func() {
			defer GinkgoRecover()

			for {
				event, err := eventSource.Next()
				if err != nil {
					return
				}
				lock.Lock()
				events = append(events, event)
				lock.Unlock()
			}
		}()
	})

	AfterEach(func() {
		testIngressServer.Stop()
		helpers.StopProcesses(ifritRuntime)
	})

	Describe("desiring", func() {
		var lrp *models.DesiredLRP

		BeforeEach(func() {
			lrp = helpers.DefaultDeclaritiveHealthcheckLRPCreateRequest(componentMaker.Addresses(), processGuid, "log-guid", 1)
		})

		JustBeforeEach(func() {
			err := bbsClient.DesireLRP(lgr, "", lrp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("eventually runs", func() {
			Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
			Eventually(helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)).Should(ConsistOf([]string{"0"}))
		})

		Context("the container is privileged", func() {
			BeforeEach(func() {
				lrp.Privileged = true
			})

			It("eventually runs", func() {
				Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
				Eventually(helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)).Should(ConsistOf([]string{"0"}))
			})
		})

		Context("when the lrp is scaled up", func() {
			JustBeforeEach(func() {
				Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
				dlu := &models.DesiredLRPUpdate{}
				dlu.SetInstances(2)
				bbsClient.UpdateDesiredLRP(lgr, "", processGuid, dlu)
			})

			It("eventually runs", func() {
				Eventually(helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)).Should(ConsistOf([]string{"0", "1"}))
			})
		})

		Context("when the lrp does not have a start timeout", func() {
			BeforeEach(func() {
				lrp.StartTimeoutMs = 0
			})

			It("eventually runs", func() {
				Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
				Eventually(helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)).Should(ConsistOf([]string{"0"}))
			})
		})

		Context("when startup check times out but readiness check succeeds immediately", func() {
			var repClient rep.Client
			BeforeEach(func() {
				// Create an LRP with:
				// - Startup check on port 9999 (never becomes available, will timeout)
				// - Readiness check on port 8080 (app listens here, will succeed immediately)
				// - Very short startup timeout to trigger the timeout quickly
				lrp = helpers.DefaultDeclaritiveHealthcheckLRPCreateRequest(componentMaker.Addresses(), processGuid, "log-guid", 1)
				lrp.StartTimeoutMs = int64(2 * time.Second / time.Millisecond) // 2 second timeout

				// Startup check: TCP check on port 9999 (never available)
				// Readiness check: TCP check on port 8080 (available immediately)
				lrp.CheckDefinition = &models.CheckDefinition{
					Checks: []*models.Check{
						{
							TcpCheck: &models.TCPCheck{
								Port:             9999, // Port that will never be available
								ConnectTimeoutMs: 100,
								IntervalMs:       100,
							},
						},
					},
					ReadinessChecks: []*models.Check{
						{
							TcpCheck: &models.TCPCheck{
								Port:             8080, // Port where app listens, available immediately
								ConnectTimeoutMs: 100,
								IntervalMs:       100,
							},
						},
					},
				}
				factory := componentMaker.RepClientFactory()
				var err error
				addr := fmt.Sprintf("https://cell.service.cf.internal:%d", cellPortsStart)
				secureAddr := fmt.Sprintf("https://cell.service.cf.internal:%d", cellPortsStart+1)
				repClient, err = factory.CreateClient(addr, secureAddr, "")
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				// Wait for the LRP to start
				Eventually(func() (int, error) {
					state, err := repClient.State(lgr)
					if err != nil {
						return -1, err
					}
					return len(state.LRPs), nil
				}).Should(Equal(1))
			})

			It("all processes exit when startup check times out", func() {
				// The startup check should timeout after 2 seconds
				// The readiness check should succeed immediately (port 8080 is available)
				// This creates the deadlock scenario: readiness check tries to send to channel
				// but storeNode.run() has already exited due to startup check failure
				// We verify that all processes exit (no deadlock)

				// Verify that the rep no longer has an lrp running
				Eventually(func() (int, error) {
					state, err := repClient.State(lgr)
					if err != nil {
						return -1, err
					}
					return len(state.LRPs), nil
				}).Should(Equal(0))
			})
		})
	})
})
