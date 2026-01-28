package cell_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	archive_helper "code.cloudfoundry.org/archiver/extractor/test_helper"
	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/inigo/fixtures"
	"code.cloudfoundry.org/inigo/helpers"
	"code.cloudfoundry.org/lager/v3"
	repconfig "code.cloudfoundry.org/rep/cmd/rep/config"
	routeemitterconfig "code.cloudfoundry.org/route-emitter/cmd/route-emitter/config"
	routingapihelpers "code.cloudfoundry.org/route-emitter/cmd/route-emitter/runners"
	routing_api "code.cloudfoundry.org/routing-api"
	"code.cloudfoundry.org/routing-info/cfroutes"
	"code.cloudfoundry.org/routing-info/tcp_routes"
	"code.cloudfoundry.org/tlsconfig"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/tedsuo/ifrit/grouper"
)

var _ = Describe("LocalRouteEmitter", func() {
	var (
		processGuid                                  string
		ifritRuntime, cellAProcess, cellBProcess     ifrit.Process
		archiveFiles                                 []archive_helper.ArchiveFile
		fileServerStaticDir                          string
		cellAID, cellBID, cellARepAddr, cellBRepAddr string
		routeEmitterConfigs                          []func(*routeemitterconfig.RouteEmitterConfig)
		cellAPort, cellBPort                         uint16
	)

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip(" not yet working on windows")
		}
		processGuid = helpers.GenerateGuid()

		var fileServer ifrit.Runner
		fileServer, fileServerStaticDir = componentMaker.FileServer()

		cellAID = "cell-a"
		cellBID = "cell-b"

		cellRepPort, err := componentMaker.PortAllocator().ClaimPorts(2)
		Expect(err).NotTo(HaveOccurred())

		cellAPort = cellRepPort
		cellBPort = cellRepPort + 1

		cellARepAddr = fmt.Sprintf("0.0.0.0:%d", cellAPort)
		cellBRepAddr = fmt.Sprintf("0.0.0.0:%d", cellBPort)

		ifritRuntime = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{Name: "router", Runner: componentMaker.Router()},
			{Name: "file-server", Runner: fileServer},
			{Name: "auctioneer", Runner: componentMaker.Auctioneer()},
		}))

		archiveFiles = fixtures.GoServerApp()
	})

	AfterEach(func() {
		helpers.StopProcesses(ifritRuntime, cellAProcess, cellBProcess)
	})

	JustBeforeEach(func() {
		repA := componentMaker.RepN(1, func(config *repconfig.RepConfig) {
			config.CellID = cellAID
			config.ListenAddr = cellARepAddr
			config.EvacuationTimeout = durationjson.Duration(30 * time.Second)
		})

		repB := componentMaker.RepN(2, func(config *repconfig.RepConfig) {
			config.CellID = cellBID
			config.ListenAddr = cellBRepAddr
			config.EvacuationTimeout = durationjson.Duration(30 * time.Second)
		})

		routeEmitterAConfigs := append(routeEmitterConfigs, func(config *routeemitterconfig.RouteEmitterConfig) {
			config.SyncInterval = durationjson.Duration(time.Hour)
			config.CellID = cellAID
		})
		cellAProcess = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{Name: "rep-a", Runner: repA},
			{Name: "route-emitter-a", Runner: componentMaker.RouteEmitterN(1, routeEmitterAConfigs...)},
		}))
		routeEmitterBConfigs := append(routeEmitterConfigs, func(config *routeemitterconfig.RouteEmitterConfig) {
			config.SyncInterval = durationjson.Duration(time.Hour)
			config.CellID = cellBID
		})
		cellBProcess = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{Name: "rep-b", Runner: repB},
			{Name: "route-emitter-b", Runner: componentMaker.RouteEmitterN(1, routeEmitterBConfigs...)},
		}))

		archive_helper.CreateZipArchive(
			filepath.Join(fileServerStaticDir, "lrp.zip"),
			archiveFiles,
		)
	})

	Describe("desiring", func() {
		var (
			lrp       *models.DesiredLRP
			instances int32
		)

		BeforeEach(func() {
			instances = 1
			lrp = createDesiredLRP(processGuid)
		})

		JustBeforeEach(func() {
			lrp.Instances = instances
			err := bbsClient.DesireLRP(lgr, "", lrp)
			Expect(err).NotTo(HaveOccurred())
			Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
		})

		It("eventually is accessible through the router within a second", func() {
			Eventually(
				helpers.ResponseCodeFromHostPoller(componentMaker.Addresses().Router, helpers.DefaultHost),
				time.Second,
				10*time.Millisecond,
			).Should(Equal(http.StatusOK))
		})

		Context("when tcp route emitting is enabled", func() {
			var (
				routingAPI        *routingapihelpers.RoutingAPIRunner
				routingAPIProcess ifrit.Process
				sqlProcess        ifrit.Process
			)

			BeforeEach(func() {
				sqlRunner := test_helpers.NewSQLRunner(fmt.Sprintf("routingapi_%d", GinkgoParallelProcess()))
				sqlProcess = ginkgomon.Invoke(sqlRunner)
				routingAPI = componentMaker.RoutingAPI()
				routeEmitterConfigs = append(routeEmitterConfigs, func(cfg *routeemitterconfig.RouteEmitterConfig) {
					cfg.EnableTCPEmitter = true
					cfg.RoutingAPI = routeemitterconfig.RoutingAPIConfig{
						URL:         "http://localhost",
						Port:        routingAPI.Config.API.ListenPort,
						AuthEnabled: false,
					}
				})
				routingAPIProcess = ginkgomon.Invoke(routingAPI)
			})

			AfterEach(func() {
				ginkgomon.Interrupt(routingAPIProcess)
				ginkgomon.Interrupt(sqlProcess)
			})

			Context("and the lrp has a tcp route", func() {
				var routingAPIClient routing_api.Client

				BeforeEach(func() {
					var err error
					routingAPIClient, err = routingapihelpers.NewRoutingAPIClient(
						routingapihelpers.RoutingAPIClientConfig{
							Port: routingAPI.Config.API.ListenPort,
						},
					)
					Expect(err).NotTo(HaveOccurred())
					routerGroups, err := routingAPIClient.RouterGroups()
					Expect(err).NotTo(HaveOccurred())
					tcpRoute := tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: routerGroups[0].Guid,
							ExternalPort:    1234,
							ContainerPort:   8080,
						},
					}
					lrp.Routes = tcpRoute.RoutingInfo()
				})

				It("emits the tcp route of the lrp", func() {
					Eventually(func() error {
						routes, err := routingAPIClient.TcpRouteMappings()
						if err != nil {
							return err
						}
						if len(routes) != 1 {
							return fmt.Errorf("routes %#v does not have length 1", routes)
						}
						return nil
					}, 2*time.Second).Should(Succeed())
				})
			})
		})

		Context("when there are 3 instances", func() {
			BeforeEach(func() {
				instances = 3
			})

			Context("and a rep start evacuating", func() {
				JustBeforeEach(func() {
					evacuateARep(
						processGuid,
						lgr,
						bbsClient,
						cellAID, cellARepAddr,
						cellBID, cellBRepAddr,
						cellAPort, cellBPort,
					)
				})

				It("eventually should make the new lrp routable within a second", func() {
					Eventually(
						helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost),
						time.Second,
						10*time.Millisecond,
					).Should(ConsistOf([]string{"0", "1", "2"}))
				})
			})

			Context("and the app is deleted", func() {
				JustBeforeEach(func() {
					err := bbsClient.RemoveDesiredLRP(lgr, "", processGuid)
					Expect(err).NotTo(HaveOccurred())
				})

				It("eventually is not accessible through the router within a second", func() {
					Eventually(
						helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost),
						time.Second,
						10*time.Millisecond,
					).Should(BeEmpty())
				})
			})

			Context("and the app is updated", func() {
				var (
					desiredLRPUdate *models.DesiredLRPUpdate
				)

				BeforeEach(func() {
					desiredLRPUdate = &models.DesiredLRPUpdate{}
				})

				JustBeforeEach(func() {
					err := bbsClient.UpdateDesiredLRP(lgr, "", processGuid, desiredLRPUdate)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("to scale the app down", func() {
					BeforeEach(func() {
						newInstances := int32(1)
						desiredLRPUdate.SetInstances(&newInstances)
					})

					It("eventually extra routes are removed within a second", func() {
						Eventually(
							helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost),
							time.Second,
							10*time.Millisecond,
						).Should(ConsistOf([]string{"0"}))
					})
				})

				Context("to add new route", func() {
					BeforeEach(func() {
						routes := cfroutes.CFRoutes{{Hostnames: []string{helpers.DefaultHost, "some-other-route"}, Port: 8080}}.RoutingInfo()
						desiredLRPUdate.Routes = &routes
					})

					It("eventually is accessible using the new route within a second", func() {
						Eventually(
							helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, "some-other-route"),
							time.Second,
							10*time.Millisecond,
						).Should(ConsistOf([]string{"0", "1", "2"}))
					})
				})

				Context("and all routes are deleted", func() {
					BeforeEach(func() {
						routes := cfroutes.CFRoutes{}.RoutingInfo()
						desiredLRPUdate.Routes = &routes
					})

					It("eventually not accessible using its route within a second", func() {
						Eventually(
							helpers.ResponseCodeFromHostPoller(componentMaker.Addresses().Router, helpers.DefaultHost),
							time.Second,
							10*time.Millisecond,
						).Should(Equal(404))
					})
				})
			})
		})

		Context("when the instances count change", func() {
			var (
				newInstances int32
			)

			JustBeforeEach(func() {
				dlu := &models.DesiredLRPUpdate{}
				dlu.SetInstances(&newInstances)
				err := bbsClient.UpdateDesiredLRP(lgr, "", processGuid, dlu)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("to 3 instances", func() {
				BeforeEach(func() {
					newInstances = 3
				})

				JustBeforeEach(func() {
					for i := 1; i < int(newInstances); i++ {
						Eventually(helpers.LRPInstanceStatePoller(lgr, bbsClient, processGuid, i, nil)).Should(Equal(models.ActualLRPStateRunning))
					}
				})

				It("eventually is accessible through the router within a second", func() {
					Eventually(
						helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost),
						time.Second,
						10*time.Millisecond,
					).Should(ConsistOf([]string{"0", "1", "2"}))
				})
			})

			Context("to 0 instances", func() {
				BeforeEach(func() {
					newInstances = 0
				})

				It("eventually is not accessible through the router within a second", func() {
					Eventually(
						helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost),
						time.Second,
						10*time.Millisecond,
					).Should(BeEmpty())
				})
			})
		})
	})
})

func createDesiredLRP(processGuid string) *models.DesiredLRP {
	lrp := helpers.DefaultLRPCreateRequest(componentMaker.Addresses(), processGuid, "log-guid", 1)
	lrp.Setup = nil
	lrp.CachedDependencies = []*models.CachedDependency{{
		From:      fmt.Sprintf("http://%s/v1/static/%s", componentMaker.Addresses().FileServer, "lrp.zip"),
		To:        "/tmp/diego",
		Name:      "lrp bits",
		CacheKey:  "lrp-cache-key",
		LogSource: "APP",
	}}
	lrp.Privileged = true
	lrp.Action = models.WrapAction(&models.RunAction{
		User: "vcap",
		Path: "/tmp/diego/go-server",
		Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
	})
	routes := cfroutes.CFRoutes{{Hostnames: []string{helpers.DefaultHost}, Port: 8080}}.RoutingInfo()
	lrp.Routes = &routes
	return lrp
}

func evacuateARep(
	processGuid string,
	logger lager.Logger,
	bbsClient bbs.InternalClient,
	cellAID, cellARepAddr string,
	cellBID, cellBRepAddr string,
	cellAPort, cellBPort uint16,
) {
	By("finding rep with one instance running")
	lrps, err := bbsClient.ActualLRPs(logger, "", models.ActualLRPFilter{ProcessGuid: processGuid})
	Expect(err).NotTo(HaveOccurred())
	Expect(lrps).To(HaveLen(3))
	instancePerRepCount := map[string]int{}
	for _, lrp := range lrps {
		cellID := lrp.ActualLrpInstanceKey.CellId
		instancePerRepCount[cellID]++
	}
	repWithOneInstance := ""
	for cellID, count := range instancePerRepCount {
		if count == 1 {
			repWithOneInstance = cellID
			break
		}
	}
	Expect(repWithOneInstance).NotTo(BeEmpty())

	tlscfg, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(componentMaker.RepSSLConfig().ServerCert, componentMaker.RepSSLConfig().ServerKey),
	).Client(
		tlsconfig.WithAuthorityFromFile(componentMaker.RepSSLConfig().CACert),
	)
	Expect(err).NotTo(HaveOccurred())

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlscfg,
		},
	}

	otherRepID := ""
	var evacuatingRepPort uint16
	if repWithOneInstance == cellAID {
		evacuatingRepPort = cellAPort
		otherRepID = cellBID
	} else if repWithOneInstance == cellBID {
		evacuatingRepPort = cellBPort
		otherRepID = cellAID
	} else {
		Fail(fmt.Sprintf("cell id %s doesn't match either cell-a or cell-b", repWithOneInstance))
	}

	By(fmt.Sprintf("sending evacuate request to %s", repWithOneInstance))
	resp, err := httpClient.Post(fmt.Sprintf("https://127.0.0.1:%d/evacuate", evacuatingRepPort), "text/html", nil)
	Expect(err).NotTo(HaveOccurred())
	resp.Body.Close()
	Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

	By("waiting for the lrp to run on the new cell")
	Eventually(func() map[string]int {
		lrps := helpers.RunningActualLRPs(logger, bbsClient, processGuid)
		cellIDs := map[string]int{}
		for _, lrp := range lrps {
			cellIDs[lrp.ActualLrpInstanceKey.CellId]++
		}
		return cellIDs
	}).Should(Equal(map[string]int{otherRepID: 3}))
}
