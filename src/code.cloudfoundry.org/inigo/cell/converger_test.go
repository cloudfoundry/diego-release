package cell_test

import (
	"os"
	"path/filepath"
	"runtime"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/inigo/fixtures"
	"code.cloudfoundry.org/inigo/helpers"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/tedsuo/ifrit/grouper"

	archive_helper "code.cloudfoundry.org/archiver/extractor/test_helper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Convergence to desired state", func() {
	var (
		ifritRuntime ifrit.Process
		auctioneer   ifrit.Process
		rep          ifrit.Process

		appId       string
		processGuid string

		runningLRPsPoller         func() []models.ActualLRP
		helloWorldInstancePoller  func() []string
		runningLRPsPresencePoller func(models.ActualLRP_Presence) func() []models.ActualLRP
	)

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip(" not yet working on windows")
		}

		fileServer, fileServerStaticDir := componentMaker.FileServer(modifyFunFileServerLoggregatorConfig)

		ifritRuntime = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{Name: "file-server", Runner: fileServer},
			{Name: "route-emitter", Runner: componentMaker.RouteEmitter(modifyFunRouteEmitterLoggregatorConfig)},
			{Name: "router", Runner: componentMaker.Router()},
		}))

		archive_helper.CreateZipArchive(
			filepath.Join(fileServerStaticDir, "lrp.zip"),
			fixtures.GoServerApp(),
		)

		appId = helpers.GenerateGuid()

		processGuid = helpers.GenerateGuid()

		runningLRPsPoller = func() []models.ActualLRP {
			return helpers.ActiveActualLRPs(lgr, bbsClient, processGuid)
		}

		runningLRPsPresencePoller = func(presence models.ActualLRP_Presence) func() []models.ActualLRP {
			return func() []models.ActualLRP {
				lrps := helpers.ActiveActualLRPs(lgr, bbsClient, processGuid)
				filteredLRPs := []models.ActualLRP{}
				for _, lrp := range lrps {
					if lrp.Presence == presence {
						filteredLRPs = append(filteredLRPs, lrp)
					}
				}
				return filteredLRPs
			}
		}

		helloWorldInstancePoller = helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)
	})

	AfterEach(func() {
		By("Stopping all the processes")
		helpers.StopProcesses(auctioneer, rep, ifritRuntime)
	})

	Describe("Executor fault tolerance", func() {
		BeforeEach(func() {
			auctioneer = ginkgomon.Invoke(componentMaker.Auctioneer(modifyFunAuctioneerLoggregatorConfig))
		})

		Context("when an rep and converger are running", func() {
			var initialInstanceGuids []string

			BeforeEach(func() {
				rep = ginkgomon.Invoke(componentMaker.Rep(modifyFunRepLoggregatorConfig))

				By("restarting the bbs with smaller convergeRepeatInterval")
				ginkgomon.Interrupt(bbsProcess)
				bbsProcess = ginkgomon.Invoke(componentMaker.BBS(
					overrideConvergenceRepeatInterval, modifyFuncBBSLoggregatorConfig,
				))

				By("creating and ActualLRP")
				err := bbsClient.DesireLRP(lgr, "", helpers.DefaultLRPCreateRequest(componentMaker.Addresses(), processGuid, appId, 2))
				Expect(err).NotTo(HaveOccurred())
				Eventually(runningLRPsPoller).Should(HaveLen(2))
				Eventually(helloWorldInstancePoller).Should(Equal([]string{"0", "1"}))

				By("collecting the ActualLRP instance guids")
				initialActuals := runningLRPsPoller()
				initialInstanceGuids = []string{initialActuals[0].InstanceGuid, initialActuals[1].InstanceGuid}
			})

			It("marks the LRPs as Suspect until the rep comes back, and then marks the LRPs as Ordinary", func() {
				By("killing the lone rep")
				ginkgomon.Interrupt(rep)

				By("Asserting that the LRPs are marked as Suspect")
				Eventually(runningLRPsPresencePoller(models.ActualLRP_Suspect)).Should(HaveLen(2))

				By("bringing back the original rep")
				rep = ginkgomon.Invoke(componentMaker.Rep(modifyFunRepLoggregatorConfig))

				Eventually(runningLRPsPoller).Should(HaveLen(2))
				Eventually(helloWorldInstancePoller).Should(Equal([]string{"0", "1"}))

				By("Asserting that the LRPs marked as Ordinary")
				currentActuals := runningLRPsPoller()
				instanceGuids := []string{currentActuals[0].InstanceGuid, currentActuals[1].InstanceGuid}
				Expect(instanceGuids).NotTo(ContainElement(initialInstanceGuids[0]))
				Expect(instanceGuids).NotTo(ContainElement(initialInstanceGuids[1]))
				Eventually(runningLRPsPresencePoller(models.ActualLRP_Ordinary)).Should(HaveLen(2))
			})

			Context("and a second rep is running", func() {
				var firstActualLRPs []models.ActualLRP
				var rep2 ifrit.Process

				BeforeEach(func() {
					firstActualLRPs = runningLRPsPoller()
					rep2 = ginkgomon.Invoke(componentMaker.RepN(1, modifyFunRepLoggregatorConfig))
				})

				AfterEach(func() {
					helpers.StopProcesses(rep2)
				})

				It("marks the LRPs as Suspect until they get started on the other rep", func() {
					By("killing the original rep")
					ginkgomon.Interrupt(rep)

					By("Asserting that the LRPs are marked as Suspect")
					Eventually(runningLRPsPresencePoller(models.ActualLRP_Suspect)).Should(HaveLen(2))

					By("Asserting that the LRPs are started on the second rep")
					Eventually(func() bool {
						secondActualLRPs := runningLRPsPoller()
						if len(secondActualLRPs) != 2 {
							return false
						}
						return secondActualLRPs[0].CellId != firstActualLRPs[0].CellId &&
							secondActualLRPs[1].CellId != firstActualLRPs[1].CellId
					}).Should(BeTrue())
				})
			})
		})

		Context("when a converger is running without a rep", func() {
			BeforeEach(func() {
				By("restarting the bbs with smaller convergeRepeatInterval")
				ginkgomon.Interrupt(bbsProcess)
				bbsProcess = ginkgomon.Invoke(componentMaker.BBS(
					overrideConvergenceRepeatInterval, modifyFuncBBSLoggregatorConfig,
				))
			})

			Context("and an LRP is desired", func() {
				BeforeEach(func() {
					err := bbsClient.DesireLRP(lgr, "", helpers.DefaultLRPCreateRequest(componentMaker.Addresses(), processGuid, appId, 1))
					Expect(err).NotTo(HaveOccurred())

					Consistently(runningLRPsPoller).Should(BeEmpty())
					Consistently(helloWorldInstancePoller).Should(BeEmpty())
				})

				Context("and then a rep come up", func() {
					BeforeEach(func() {
						rep = ginkgomon.Invoke(componentMaker.Rep(modifyFunRepLoggregatorConfig))
					})

					It("eventually brings the LRP up", func() {
						Eventually(runningLRPsPoller).Should(HaveLen(1))
						Eventually(helloWorldInstancePoller).Should(Equal([]string{"0"}))
					})
				})
			})
		})
	})

	Describe("Auctioneer Fault Tolerance", func() {
		BeforeEach(func() {
			By("restarting the bbs with smaller convergeRepeatInterval")
			ginkgomon.Interrupt(bbsProcess)
			bbsProcess = ginkgomon.Invoke(componentMaker.BBS(
				overrideConvergenceRepeatInterval, modifyFuncBBSLoggregatorConfig,
			))
		})

		Context("when a rep is running with no auctioneer", func() {
			BeforeEach(func() {
				rep = ginkgomon.Invoke(componentMaker.Rep(modifyFunRepLoggregatorConfig))
			})

			Context("and an LRP is desired", func() {
				BeforeEach(func() {
					err := bbsClient.DesireLRP(lgr, "", helpers.DefaultLRPCreateRequest(componentMaker.Addresses(), processGuid, appId, 1))
					Expect(err).NotTo(HaveOccurred())

					Consistently(runningLRPsPoller).Should(BeEmpty())
					Consistently(helloWorldInstancePoller).Should(BeEmpty())
				})

				Context("and then an auctioneer comes up", func() {
					BeforeEach(func() {
						auctioneer = ginkgomon.Invoke(componentMaker.Auctioneer(modifyFunAuctioneerLoggregatorConfig))
					})

					It("eventually brings it up", func() {
						Eventually(runningLRPsPoller).Should(HaveLen(1))
						Eventually(helloWorldInstancePoller, cellSuiteEventuallyTestTimeout, cellSuiteEventuallyPollingInterval).Should(Equal([]string{"0"}))
					})
				})
			})
		})

		Context("when an auctioneer is running with no rep", func() {
			BeforeEach(func() {
				auctioneer = ginkgomon.Invoke(componentMaker.Auctioneer(modifyFunAuctioneerLoggregatorConfig))
			})

			Context("and an LRP is desired", func() {
				BeforeEach(func() {
					err := bbsClient.DesireLRP(lgr, "", helpers.DefaultLRPCreateRequest(componentMaker.Addresses(), processGuid, appId, 1))
					Expect(err).NotTo(HaveOccurred())

					Consistently(runningLRPsPoller).Should(BeEmpty())
					Consistently(helloWorldInstancePoller).Should(BeEmpty())
				})

				Context("and the rep come up", func() {
					BeforeEach(func() {
						rep = ginkgomon.Invoke(componentMaker.Rep(modifyFunRepLoggregatorConfig))
					})

					It("eventually brings it up", func() {
						Eventually(runningLRPsPoller).Should(HaveLen(1))
						Eventually(helloWorldInstancePoller).Should(Equal([]string{"0"}))
					})
				})
			})
		})
	})
})
