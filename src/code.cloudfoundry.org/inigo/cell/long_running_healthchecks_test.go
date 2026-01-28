package cell_test

import (
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

		var fileServer ifrit.Runner
		fileServer, fileServerStaticDir = componentMaker.FileServer()

		turnOnLongRunningHealthchecks := func(cfg *config.RepConfig) {
			cfg.EnableDeclarativeHealthcheck = true
			cfg.DeclarativeHealthCheckDefaultTimeout = durationjson.Duration(1 * time.Second)
			cfg.DeclarativeHealthcheckPath = componentMaker.Artifacts().Healthcheck
			cfg.HealthCheckWorkPoolSize = 1
		}

		fixturesPath := path.Join("..", "fixtures", "certs")
		metronCAFile := path.Join(fixturesPath, "metron", "CA.crt")
		metronClientCertFile := path.Join(fixturesPath, "metron", "client.crt")
		metronClientKeyFile := path.Join(fixturesPath, "metron", "client.key")
		metronServerCertFile := path.Join(fixturesPath, "metron", "metron.crt")
		metronServerKeyFile := path.Join(fixturesPath, "metron", "metron.key")
		var err error
		testIngressServer, err = testhelpers.NewTestIngressServer(metronServerCertFile, metronServerKeyFile, metronCAFile)
		Expect(err).NotTo(HaveOccurred())

		Expect(testIngressServer.Start()).To(Succeed())

		metricsPort, err := testIngressServer.Port()
		Expect(err).NotTo(HaveOccurred())

		loggregatorConfig := func(cfg *config.RepConfig) {
			cfg.LoggregatorConfig = logging.Config{
				BatchFlushInterval: 10 * time.Millisecond,
				BatchMaxSize:       1,
				UseV2API:           true,
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
			{Name: "auctioneer", Runner: componentMaker.Auctioneer()},
			{Name: "route-emitter", Runner: componentMaker.RouteEmitter()},
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
				instances := int32(2)
				dlu.SetInstances(&instances)
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
	})
})
