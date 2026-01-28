package cell_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	archive_helper "code.cloudfoundry.org/archiver/extractor/test_helper"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/inigo/fixtures"
	"code.cloudfoundry.org/inigo/helpers"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/routing-info/cfroutes"
	. "code.cloudfoundry.org/vizzini/matchers"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/tedsuo/ifrit/grouper"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LRP", func() {
	var (
		processGuid         string
		archiveFiles        []archive_helper.ArchiveFile
		fileServerStaticDir string

		ifritRuntime    ifrit.Process
		archiveFilePath string

		lock        *sync.Mutex
		eventSource events.EventSource
		events      []models.Event
	)

	getEvents := func() []models.Event {
		lock.Lock()
		defer lock.Unlock()
		return events
	}

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip(" not yet working on windows")
		}
		processGuid = helpers.GenerateGuid()

		var fileServer ifrit.Runner
		fileServer, fileServerStaticDir = componentMaker.FileServer()
		ifritRuntime = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{Name: "router", Runner: componentMaker.Router()},
			{Name: "file-server", Runner: fileServer},
			{Name: "rep", Runner: componentMaker.Rep()},
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
		helpers.StopProcesses(ifritRuntime)
	})

	Describe("desiring", func() {
		var lrp *models.DesiredLRP

		BeforeEach(func() {
			lrp = helpers.DefaultLRPCreateRequest(componentMaker.Addresses(), processGuid, "log-guid", 1)
			lrp.Setup = nil
			lrp.CachedDependencies = []*models.CachedDependency{{
				From:      fmt.Sprintf("http://%s/v1/static/%s", componentMaker.Addresses().FileServer, "lrp.zip"),
				To:        "/tmp/diego",
				Name:      "lrp bits",
				CacheKey:  "lrp-cache-key",
				LogSource: "APP",
			}}
			lrp.Privileged = true
		})

		JustBeforeEach(func() {
			err := bbsClient.DesireLRP(lgr, "", lrp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("eventually runs", func() {
			Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
			Eventually(helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)).Should(ConsistOf([]string{"0"}))
		})

		It("should send events as the LRP goes through its lifecycle ", func() {
			Eventually(getEvents).Should(ContainElement(MatchDesiredLRPCreatedEvent(processGuid)))
			Eventually(getEvents).Should(ContainElement(MatchActualLRPInstanceCreatedEvent(processGuid, 0)))
			Eventually(getEvents).Should(ContainElement(MatchActualLRPInstanceChangedEvent(processGuid, 0, models.ActualLRPStateClaimed)))
			Eventually(getEvents).Should(ContainElement(MatchActualLRPInstanceChangedEvent(processGuid, 0, models.ActualLRPStateRunning)))
		})

		Context("when using a private image", func() {
			BeforeEach(func() {
				lrp.RootFs = os.Getenv("INIGO_PRIVATE_DOCKER_IMAGE_URI")
				lrp.ImageUsername = os.Getenv("INIGO_PRIVATE_DOCKER_IMAGE_USERNAME")
				lrp.ImagePassword = os.Getenv("INIGO_PRIVATE_DOCKER_IMAGE_PASSWORD")
				lrp.Monitor = nil
				if os.Getenv("INIGO_PRIVATE_DOCKER_IMAGE_URI") == "" {
					Skip("no private docker image specified")
				}
			})

			It("eventually runs", func() {
				Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
				Eventually(helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)).Should(ConsistOf([]string{"0"}))
			})
		})

		Context("when using an ECR image", func() {
			BeforeEach(func() {
				lrp.RootFs = os.Getenv("INIGO_ECR_IMAGE_URI")
				lrp.ImageUsername = os.Getenv("INIGO_ECR_AWS_ACCESS_KEY_ID")
				lrp.ImagePassword = os.Getenv("INIGO_ECR_AWS_SECRET_ACCESS_KEY")
				lrp.Monitor = nil
				if os.Getenv("INIGO_ECR_IMAGE_URI") == "" {
					Skip("no ECR image specified")
				}
			})

			It("eventually runs", func() {
				Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
				Eventually(helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)).Should(ConsistOf([]string{"0"}))
			})
		})

		Context("when correct checksum information is provided", func() {
			var checksumValue string

			createChecksum := func(algorithm string) {
				archiveFilePath = filepath.Join(fileServerStaticDir, "lrp.zip")
				archive_helper.CreateZipArchive(
					archiveFilePath,
					archiveFiles,
				)
				content, err := os.ReadFile(archiveFilePath)
				Expect(err).NotTo(HaveOccurred())
				checksumValue, err = helpers.HexValueForByteArray(algorithm, content)
				Expect(err).NotTo(HaveOccurred())

				lrp.CachedDependencies[0].ChecksumAlgorithm = algorithm
				lrp.CachedDependencies[0].ChecksumValue = checksumValue
			}

			validateLRPDesired := func() {
				Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
				Eventually(helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)).Should(ConsistOf([]string{"0"}))
			}

			Context("for CachedDependency", func() {
				Context("for md5", func() {
					BeforeEach(func() {
						createChecksum("md5")
					})

					It("eventually desires the lrp", func() {
						validateLRPDesired()
					})
				})

				Context("for sha1", func() {
					BeforeEach(func() {
						createChecksum("sha1")
					})

					It("eventually desires the lrp", func() {
						validateLRPDesired()
					})
				})

				Context("for sha256", func() {
					BeforeEach(func() {
						createChecksum("sha256")
					})

					It("eventually desires the lrp", func() {
						validateLRPDesired()
					})
				})
			})

			Context("when validating checksum for download action", func() {
				createDownloadActionChecksum := func(algorithm string) {
					createChecksum(algorithm)
					lrp.CachedDependencies = []*models.CachedDependency{}
					lrp.Setup = models.WrapAction(&models.DownloadAction{
						From:              fmt.Sprintf("http://%s/v1/static/%s", componentMaker.Addresses().FileServer, "lrp.zip"),
						To:                "/tmp/diego",
						User:              "vcap",
						ChecksumAlgorithm: algorithm,
						ChecksumValue:     checksumValue,
					})
				}

				Context("with md5", func() {
					BeforeEach(func() {
						createDownloadActionChecksum("md5")
					})

					It("eventually desires the lrp", func() {
						validateLRPDesired()
					})
				})

				Context("with sha1", func() {
					BeforeEach(func() {
						createDownloadActionChecksum("sha1")
					})

					It("eventually desires the lrp", func() {
						validateLRPDesired()
					})
				})

				Context("with sha256", func() {
					BeforeEach(func() {
						createDownloadActionChecksum("sha256")
					})

					It("eventually desires the lrp", func() {
						validateLRPDesired()
					})
				})
			})

		})

		Context("when properties are present on the desired LRP", func() {
			BeforeEach(func() {
				lrp.Network = &models.Network{
					Properties: map[string]string{
						"my-key": "my-value",
					},
				}
			})

			It("passes them to garden", func() {
				Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))

				lrps, err := bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(lrps)).To(BeNumerically(">", 0))

				containerHandle := lrps[0].ActualLrpInstanceKey.InstanceGuid

				container, err := gardenClient.Lookup(containerHandle)
				Expect(err).NotTo(HaveOccurred())

				props, err := container.Properties()
				Expect(err).NotTo(HaveOccurred())

				Expect(props).To(HaveKeyWithValue("network.my-key", "my-value"))
			})
		})

		Context("when it's unhealthy for longer than its start timeout", func() {
			BeforeEach(func() {
				lrp.StartTimeoutMs = 5000

				lrp.Monitor = models.WrapAction(&models.RunAction{
					User: "vcap",
					Path: "false",
				})
			})

			It("eventually marks the LRP as crashed", func() {
				Eventually(
					helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil),
				).Should(Equal(models.ActualLRPStateCrashed))
			})
		})

		Describe("updating routes", func() {
			BeforeEach(func() {
				lrp.Ports = []uint32{8080, 9080}
				routes := cfroutes.CFRoutes{{Hostnames: []string{"lrp-route-8080"}, Port: 8080}}.RoutingInfo()
				lrp.Routes = &routes

				lrp.Action = models.WrapAction(&models.RunAction{
					User: "vcap",
					Path: "/tmp/diego/go-server",
					Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080 9080"}},
				})
			})

			It("can not access container ports without routes", func() {
				Eventually(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses().Router, "lrp-route-8080")).Should(Equal(http.StatusOK))
				Consistently(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses().Router, "lrp-route-8080")).Should(Equal(http.StatusOK))
				Consistently(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses().Router, "lrp-route-9080")).Should(Equal(http.StatusNotFound))
			})

			Context("when adding a route", func() {
				logger := lagertest.NewTestLogger("test")

				JustBeforeEach(func() {
					routes := cfroutes.CFRoutes{
						{Hostnames: []string{"lrp-route-8080"}, Port: 8080},
						{Hostnames: []string{"lrp-route-9080"}, Port: 9080},
					}.RoutingInfo()

					desiredUpdate := models.DesiredLRPUpdate{
						Routes: &routes,
					}

					logger.Info("just-before-each", lager.Data{
						"processGuid":   processGuid,
						"updateRequest": desiredUpdate})

					err := bbsClient.UpdateDesiredLRP(logger, "", processGuid, &desiredUpdate)

					logger.Info("after-update-desired", lager.Data{
						"error": err,
					})
					Expect(err).NotTo(HaveOccurred())
				})

				It("can immediately access the container port with the associated routes", func() {
					Eventually(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses().Router, "lrp-route-8080")).Should(Equal(http.StatusOK))
					Consistently(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses().Router, "lrp-route-8080")).Should(Equal(http.StatusOK))

					Eventually(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses().Router, "lrp-route-9080")).Should(Equal(http.StatusOK))
					Consistently(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses().Router, "lrp-route-9080")).Should(Equal(http.StatusOK))
				})
			})
		})

		Describe("when started with 2 instances", func() {
			BeforeEach(func() {
				lrp.Instances = 2
			})

			JustBeforeEach(func() {
				Eventually(func() []*models.ActualLRP {
					lrps, err := bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: processGuid})
					Expect(err).NotTo(HaveOccurred())

					return lrps
				}).Should(HaveLen(2))
				Eventually(helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)).Should(ConsistOf([]string{"0", "1"}))
			})

			Describe("changing the instances", func() {
				var newInstances int32

				BeforeEach(func() {
					newInstances = 0 // base value; overridden below
				})

				JustBeforeEach(func() {
					dlu := &models.DesiredLRPUpdate{}
					dlu.SetInstances(&newInstances)
					err := bbsClient.UpdateDesiredLRP(lgr, "", processGuid, dlu)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("scaling it up to 3", func() {
					BeforeEach(func() {
						newInstances = 3
					})

					It("scales up to the correct number of instances", func() {
						Eventually(func() []*models.ActualLRP {
							lrps, err := bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: processGuid})
							Expect(err).NotTo(HaveOccurred())

							return lrps
						}).Should(HaveLen(3))

						Eventually(helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)).Should(ConsistOf([]string{"0", "1", "2"}))
					})
				})

				Context("scaling it down to 1", func() {
					BeforeEach(func() {
						newInstances = 1
					})

					It("scales down to the correct number of instances", func() {
						Eventually(func() []*models.ActualLRP {
							lrps, err := bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: processGuid})
							Expect(err).NotTo(HaveOccurred())

							return lrps
						}).Should(HaveLen(1))

						Eventually(helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)).Should(ConsistOf([]string{"0"}))
					})
				})

				Context("scaling it down to 0", func() {
					BeforeEach(func() {
						newInstances = 0
					})

					It("scales down to the correct number of instances", func() {
						Eventually(func() []*models.ActualLRP {
							lrps, err := bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: processGuid})
							Expect(err).NotTo(HaveOccurred())

							return lrps
						}).Should(BeEmpty())

						Eventually(helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)).Should(BeEmpty())
					})

					It("can be scaled back up", func() {
						newInstances := int32(1)
						dlu := &models.DesiredLRPUpdate{}
						dlu.SetInstances(&newInstances)
						err := bbsClient.UpdateDesiredLRP(lgr, "", processGuid, dlu)
						Expect(err).NotTo(HaveOccurred())

						Eventually(func() []*models.ActualLRP {
							lrps, err := bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: processGuid})
							Expect(err).NotTo(HaveOccurred())

							return lrps
						}).Should(HaveLen(1))

						Eventually(helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)).Should(ConsistOf([]string{"0"}))
					})
				})
			})

			Describe("deleting it", func() {
				JustBeforeEach(func() {
					err := bbsClient.RemoveDesiredLRP(lgr, "", processGuid)
					Expect(err).NotTo(HaveOccurred())
				})

				It("stops all instances", func() {
					Eventually(func() []*models.ActualLRP {
						lrps, err := bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: processGuid})
						Expect(err).NotTo(HaveOccurred())

						return lrps
					}).Should(BeEmpty())

					Eventually(helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)).Should(BeEmpty())
				})
			})
		})

		Context("Egress Rules", func() {
			Context("default networking", func() {
				It("rejects outbound tcp traffic", func() {
					Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))

					var bytes []byte
					Eventually(func() int {
						var statusCode int
						var err error
						bytes, statusCode, err = helpers.ResponseBodyAndStatusCodeFromHost(
							componentMaker.Addresses().Router,
							helpers.DefaultHost,
							"curl",
						)
						Expect(err).NotTo(HaveOccurred())
						return statusCode
					}).Should(Equal(http.StatusOK))
					Expect(string(bytes)).To(Equal("7"))
				})
			})

			Context("with appropriate security group setting", func() {
				BeforeEach(func() {
					lrp.EgressRules = []*models.SecurityGroupRule{
						{
							Protocol:     models.TCPProtocol,
							Destinations: []string{"9.0.0.0-89.255.255.255", "90.0.0.0-94.0.0.0"},
							Ports:        []uint32{80},
						},
						{
							Protocol:     models.UDPProtocol,
							Destinations: []string{"0.0.0.0/0"},
							PortRange: &models.PortRange{
								Start: 53,
								End:   53,
							},
						},
					}
				})

				It("allows outbound tcp traffic", func() {
					Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
					var bytes []byte
					Eventually(func() int {
						var statusCode int
						var err error
						bytes, statusCode, err = helpers.ResponseBodyAndStatusCodeFromHost(
							componentMaker.Addresses().Router,
							helpers.DefaultHost,
							"curl",
						)
						Expect(err).NotTo(HaveOccurred())
						return statusCode
					}).Should(Equal(http.StatusOK))
					Expect(string(bytes)).To(Equal("0"))
				})
			})
		})

		Context("Extra preloaded rootfs is requested on linux", func() {
			BeforeEach(func() {
				if runtime.GOOS == "windows" {
					Skip("Skipping extra preloaded fs on windows")
				}
				lrp = helpers.LRPCreateRequestWithRootFS(componentMaker.Addresses(), processGuid, helpers.ExtraPreloadedRootFs)
			})

			It("starts the LRP", func() {
				Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
			})
		})

		Context("Unsupported preloaded rootfs is requested", func() {
			BeforeEach(func() {
				lrp = helpers.LRPCreateRequestWithRootFS(componentMaker.Addresses(), processGuid, helpers.BogusPreloadedRootFS)
			})

			It("fails and sets a placement error", func() {
				lrpFunc := func() string {
					lrps, err := bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: processGuid})
					Expect(err).NotTo(HaveOccurred())
					if len(lrps) == 0 {
						return ""
					}
					return lrps[0].PlacementError
				}

				Eventually(lrpFunc).Should(ContainSubstring("found no compatible cell"))
			})
		})

		Context("Unsupported arbitrary rootfs is requested", func() {
			BeforeEach(func() {
				lrp = helpers.LRPCreateRequestWithRootFS(componentMaker.Addresses(), processGuid, "socker://hello")
			})

			It("fails and sets a placement error", func() {
				lrpFunc := func() string {
					lrps, err := bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: processGuid})
					Expect(err).NotTo(HaveOccurred())
					if len(lrps) == 0 {
						return ""
					}
					return lrps[0].PlacementError
				}

				Eventually(lrpFunc).Should(ContainSubstring("found no compatible cell"))
			})
		})

		Context("Supported arbitrary rootfs scheme (viz., docker) is requested", func() {
			BeforeEach(func() {
				// docker is supported
				lrp = helpers.DockerLRPCreateRequest(componentMaker.Addresses(), processGuid)
				lrp.Setup = nil
				lrp.CachedDependencies = []*models.CachedDependency{}
			})

			It("runs", func() {
				Eventually(func() []*models.ActualLRP {
					lrps, err := bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: processGuid})
					Expect(err).NotTo(HaveOccurred())
					return lrps
				}).Should(HaveLen(1))

				Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
				poller := helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)
				Eventually(poller).Should(ConsistOf([]string{"0"}))
			})
		})
	})

	Context("Crashing LRPs", func() {
		crashCount := func(guid string, index int) func() int32 {
			return func() int32 {
				i := int32(index)
				lrps, err := bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: guid, Index: &i})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(lrps)).To(Equal(1))
				return lrps[0].CrashCount
			}
		}

		BeforeEach(func() {
			By("restarting the bbs with smaller convergeRepeatInterval")
			ginkgomon.Interrupt(bbsProcess)
			bbsProcess = ginkgomon.Invoke(componentMaker.BBS(
				overrideConvergenceRepeatInterval,
			))
		})

		Describe("crashing apps", func() {
			Context("when an app flaps", func() {
				var lrp *models.DesiredLRP

				BeforeEach(func() {
					lrp = helpers.CrashingLRPCreateRequest(componentMaker.Addresses(), processGuid)
				})

				JustBeforeEach(func() {
					err := bbsClient.DesireLRP(lgr, "", lrp)
					Expect(err).NotTo(HaveOccurred())
				})

				testAppRecovery := func(index int) {
					It("imediately restarts the app 3 times", func() {
						// the bbs immediately starts it 3 times
						Eventually(crashCount(processGuid, index)).Should(BeEquivalentTo(3))
						// then exponential backoff kicks in
						Consistently(crashCount(processGuid, index), 15*time.Second).Should(BeEquivalentTo(3))
						// eventually we cross the first backoff threshold (30 seconds)
						Eventually(crashCount(processGuid, index), 30*time.Second).Should(BeEquivalentTo(4))
					})
				}

				testAppRecovery(0)

				Context("when the app has multiple indices", func() {
					BeforeEach(func() {
						lrp.Instances = 2
					})

					testAppRecovery(1)
				})
			})
		})

		Describe("disappearing containrs", func() {
			Context("when a container is deleted unexpectedly", func() {
				var (
					lrps []*models.ActualLRP
				)

				BeforeEach(func() {
					lrp := helpers.DefaultLRPCreateRequest(componentMaker.Addresses(), processGuid, "log-guid", 1)

					err := bbsClient.DesireLRP(lgr, "", lrp)
					Expect(err).NotTo(HaveOccurred())

					Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
				})

				JustBeforeEach(func() {
					index := int32(0)
					var err error
					lrps, err = bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: processGuid, Index: &index})
					Expect(err).NotTo(HaveOccurred())
					Expect(len(lrps)).To(Equal(1))

					err = gardenClient.Destroy(lrps[0].ActualLrpInstanceKey.InstanceGuid)
					Expect(err).NotTo(HaveOccurred())
				})

				It("crashes the instance and restarts it", func() {
					Eventually(crashCount(processGuid, 0)).Should(BeEquivalentTo(1))
					Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
				})

				It("contains the instance guid and cell id", func() {
					Eventually(getEvents).Should(ContainElement(helpers.MatchActualLRPCrashedEvent(
						processGuid,
						lrps[0].ActualLrpInstanceKey.InstanceGuid,
						lrps[0].ActualLrpInstanceKey.CellId,
						0,
					)))
				})
			})
		})

		Describe("failed checksum", func() {
			var lrp *models.DesiredLRP

			desireLRPWithChecksum := func(algorithm string) {
				lrp = helpers.DefaultLRPCreateRequest(componentMaker.Addresses(), processGuid, "log-guid", 1)
				lrp.Setup = nil
				lrp.CachedDependencies = []*models.CachedDependency{{
					From:              fmt.Sprintf("http://%s/v1/static/%s", componentMaker.Addresses().FileServer, "lrp.zip"),
					To:                "/tmp/diego",
					Name:              "lrp bits",
					CacheKey:          "lrp-cache-key",
					LogSource:         "APP",
					ChecksumAlgorithm: algorithm,
					ChecksumValue:     "incorrect_checksum",
				}}
				lrp.Privileged = true
			}

			Context("for CachedDependencies", func() {
				Context("with invalid algorithm", func() {
					It("eventually crashes", func() {
						desireLRPWithChecksum("invalid_algorithm")
						err := bbsClient.DesireLRP(lgr, "", lrp)
						Expect(err).To(HaveOccurred())
					})
				})

				Context("when incorrect checksum value is provided", func() {
					Context("with md5", func() {
						BeforeEach(func() {
							desireLRPWithChecksum("md5")

							err := bbsClient.DesireLRP(lgr, "", lrp)
							Expect(err).NotTo(HaveOccurred())
						})

						It("eventually crashes", func() {
							Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateCrashed))
						})
					})

					Context("with sha1", func() {
						BeforeEach(func() {
							desireLRPWithChecksum("sha1")

							err := bbsClient.DesireLRP(lgr, "", lrp)
							Expect(err).NotTo(HaveOccurred())
						})

						It("eventually crashes", func() {
							Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateCrashed))
						})
					})

					Context("with sha256", func() {
						BeforeEach(func() {
							desireLRPWithChecksum("sha256")

							err := bbsClient.DesireLRP(lgr, "", lrp)
							Expect(err).NotTo(HaveOccurred())
						})

						It("eventually crashes", func() {
							Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateCrashed))
						})
					})
				})
			})

			Context("for DownloadAction", func() {
				createDownloadActionChecksum := func(algorithm string) {
					desireLRPWithChecksum(algorithm)
					lrp.Setup = models.WrapAction(&models.DownloadAction{
						From:              fmt.Sprintf("http://%s/v1/static/%s", componentMaker.Addresses().FileServer, "lrp.zip"),
						To:                "/tmp/diego",
						User:              "vcap",
						ChecksumAlgorithm: algorithm,
						ChecksumValue:     "incorrect_checksum",
					})
				}

				Context("with md5", func() {
					BeforeEach(func() {
						createDownloadActionChecksum("md5")
						err := bbsClient.DesireLRP(lgr, "", lrp)
						Expect(err).NotTo(HaveOccurred())
					})

					It("eventually desires the lrp", func() {
						Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateCrashed))
					})
				})

				Context("with sha1", func() {
					BeforeEach(func() {
						createDownloadActionChecksum("sha1")
						err := bbsClient.DesireLRP(lgr, "", lrp)
						Expect(err).NotTo(HaveOccurred())
					})

					It("eventually desires the lrp", func() {
						Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateCrashed))
					})
				})

				Context("with sha256", func() {
					BeforeEach(func() {
						createDownloadActionChecksum("sha256")
						err := bbsClient.DesireLRP(lgr, "", lrp)
						Expect(err).NotTo(HaveOccurred())
					})

					It("eventually desires the lrp", func() {
						Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateCrashed))
					})
				})
			})
		})
	})
})
