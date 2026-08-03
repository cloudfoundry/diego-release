package vizzini_test

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/routing-info/cfroutes"

	. "code.cloudfoundry.org/vizzini/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Routing Related Tests", func() {
	var lrp *models.DesiredLRP

	Describe("sticky sessions", func() {
		var httpClient *http.Client

		BeforeEach(func() {
			jar, err := cookiejar.New(nil)
			Expect(err).NotTo(HaveOccurred())

			httpClient = &http.Client{
				Jar: jar,
			}

			lrp = DesiredLRPWithGuid(guid)
			lrp.Instances = 3

			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
			Eventually(IndexCounter(guid, httpClient)).Should(Equal(3))
		})

		It("should only route to the stuck instance", func() {
			resp, err := httpClient.Get("http://" + RouteForGuid(guid) + "/stick")
			Expect(err).NotTo(HaveOccurred())
			resp.Body.Close()

			//for some reason this isn't always 1!  it's sometimes 2....
			Expect(IndexCounter(guid, httpClient)()).To(BeNumerically("<", 3))

			resp, err = httpClient.Get("http://" + RouteForGuid(guid) + "/unstick")
			Expect(err).NotTo(HaveOccurred())
			resp.Body.Close()

			Expect(IndexCounter(guid, httpClient)()).To(Equal(3))
		})
	})

	Describe("supporting multiple ports", func() {
		var primaryURL string
		BeforeEach(func() {
			lrp = DesiredLRPWithGuid(guid)
			lrp.Ports = []uint32{8080, 9999}
			primaryURL = "http://" + RouteForGuid(guid) + "/env"

			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
			Eventually(EndpointCurler(primaryURL)).Should(Equal(http.StatusOK))
		})

		It("should be able to route to multiple ports", func() {
			By("updating the LRP with a new route to a port 9999")
			newRoute := RouteForGuid(NewGuid())
			routes, err := cfroutes.CFRoutesFromRoutingInfo(*lrp.Routes)
			Expect(err).NotTo(HaveOccurred())
			routes = append(routes, cfroutes.CFRoute{
				Hostnames: []string{newRoute},
				Port:      9999,
			})
			routingInfo := routes.RoutingInfo()
			Expect(bbsClient.UpdateDesiredLRP(logger, traceID, guid, &models.DesiredLRPUpdate{
				Routes: &routingInfo,
			})).To(

				Succeed())

			By("verifying that the new route is hooked up to the port")
			Eventually(EndpointContentCurler("http://" + newRoute)).Should(Equal("grace side-channel"))

			By("verifying that the original route is fine")
			Expect(EndpointContentCurler(primaryURL)()).To(ContainSubstring("DAQUIRI"), "something on the original endpoint that's not in the new one")

			By("adding a new route to the new port")
			veryNewRoute := RouteForGuid(NewGuid())
			routes[1].Hostnames = append(routes[1].Hostnames, veryNewRoute)
			routingInfo = routes.RoutingInfo()
			Expect(bbsClient.UpdateDesiredLRP(logger, traceID, guid, &models.DesiredLRPUpdate{
				Routes: &routingInfo,
			})).To(

				Succeed())

			Eventually(EndpointContentCurler("http://" + veryNewRoute)).Should(Equal("grace side-channel"))
			Expect(EndpointContentCurler("http://" + newRoute)()).To(Equal("grace side-channel"))
			Expect(EndpointContentCurler(primaryURL)()).To(ContainSubstring("DAQUIRI"), "something on the original endpoint that's not in the new one")

			By("tearing down the new port")
			Expect(bbsClient.UpdateDesiredLRP(logger, traceID, guid, &models.DesiredLRPUpdate{
				Routes: lrp.Routes,
			})).To(

				Succeed())

			Eventually(EndpointCurler("http://" + newRoute)).ShouldNot(Equal(http.StatusOK))
		})
	})

	Context("as containers come and go", func() {
		var url string
		var lrp *models.DesiredLRP

		BeforeEach(func() {
			url = "http://" + RouteForGuid(guid) + "/env"
			lrp = DesiredLRPWithGuid(guid)
			lrp.Instances = 3
			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
			Eventually(ActualByProcessGuidGetter(logger, guid)).Should(ConsistOf(
				BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning),
				BeActualLRPWithState(guid, 1, models.ActualLRPStateRunning),
				BeActualLRPWithState(guid, 2, models.ActualLRPStateRunning),
			))
		})

		It("{SLOW} should only route to running containers", func() {
			done := make(chan struct{})
			badCodes := []int{}
			attempts := 0

			go func() {
				defer GinkgoRecover()
				t := time.NewTicker(10 * time.Millisecond)
				for {
					select {
					case <-done:
						t.Stop()
					case <-t.C:
						attempts += 1
						code := EndpointCurler(url)()
						if code != http.StatusOK {
							badCodes = append(badCodes, code)
						}
					}
				}
			}()

			updateToThree := models.DesiredLRPUpdate{}
			updateToThree.SetInstances(3)

			updateToOne := models.DesiredLRPUpdate{}
			updateToOne.SetInstances(1)

			for i := 0; i < 4; i++ {
				By(fmt.Sprintf("Scaling down then back up #%d", i+1))
				Eventually(func() error {
					_, err := bbsClient.DesiredLRPByProcessGuid(logger, traceID, guid)
					return err
				}, "60s", "500ms").Should(Succeed())
				Expect(bbsClient.UpdateDesiredLRP(logger, traceID, guid, &updateToOne)).To(Succeed())
				Eventually(ActualByProcessGuidGetter(logger, guid)).Should(ConsistOf(
					BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning),
				))

				time.Sleep(200 * time.Millisecond)

				Expect(bbsClient.UpdateDesiredLRP(logger, traceID, guid, &updateToThree)).To(Succeed())
				Eventually(ActualByProcessGuidGetter(logger, guid)).Should(ConsistOf(
					BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning),
					BeActualLRPWithState(guid, 1, models.ActualLRPStateRunning),
					BeActualLRPWithState(guid, 2, models.ActualLRPStateRunning),
				))
			}

			close(done)

			fmt.Fprintf(GinkgoWriter, "%d bad codes out of %d attempts (%.3f%%)", len(badCodes), attempts, float64(len(badCodes))/float64(attempts)*100)
			Expect(len(badCodes)).To(BeNumerically("<", float64(attempts)*0.01))
		})
	})

	Describe("scaling down an LRP", func() {

		BeforeEach(func() {
			lrp = DesiredLRPWithGuid(guid)
			lrp.Action = models.WrapAction(&models.RunAction{
				Path: "/tmp/grace/grace",
				Args: []string{"-catchTerminate"},
				User: "vcap",
				Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
			})

			lrp.Instances = 5
		})

		JustBeforeEach(func() {
			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
			Eventually(IndexCounter(guid), 60).Should(Equal(int(lrp.Instances)))
		})

		AfterEach(func() {
			Expect(bbsClient.RemoveDesiredLRP(logger, traceID, lrp.ProcessGuid)).To(Succeed())
			Eventually(func() []*models.ActualLRP {
				lrps, err := bbsClient.ActualLRPs(logger, traceID, models.ActualLRPFilter{ProcessGuid: lrp.ProcessGuid})
				Expect(err).NotTo(HaveOccurred())
				return lrps
			}, 20*time.Second).Should(BeEmpty())
		})

		Context("and the LRP has a setup step", func() {
			BeforeEach(func() {
				// simulate the same LRP structure found in CF. It turns out we broke
				// this in https://www.pivotaltracker.com/story/show/156776776, and
				// only discovered it later in
				// https://www.pivotaltracker.com/story/show/156522178.
				lrp.Setup = models.WrapAction(&models.RunAction{
					Path: "bash",
					Args: []string{"-c", "echo hi"},
					User: "vcap",
				})
			})

			It("finish outstanding requests", func() {
				errCh := make(chan error, 1)
				go func() {
					resp, err := http.Get("http://" + RouteForGuid(guid) + "/sleep/8s")
					resp.Body.Close()
					errCh <- err
				}()

				dlu := &models.DesiredLRPUpdate{Routes: lrp.Routes}
				dlu.SetInstances(0)
				dlu.SetAnnotation(lrp.Annotation)

				Expect(bbsClient.UpdateDesiredLRP(logger, traceID, lrp.ProcessGuid, dlu)).To(Succeed())

				Consistently(errCh, 5*time.Second).ShouldNot(Receive())
			})
		})

		It("quickly stops routing to the removed indices", func() {
			dlu := &models.DesiredLRPUpdate{Routes: lrp.Routes}
			dlu.SetInstances(1)
			dlu.SetAnnotation(lrp.Annotation)

			Expect(bbsClient.UpdateDesiredLRP(logger, traceID, lrp.ProcessGuid, dlu)).To(Succeed())

			Eventually(IndexCounterWithAttempts(guid, 10), 2*time.Second).Should(Equal(1))
			Consistently(IndexCounterWithAttempts(guid, 10), 5*time.Second).Should(Equal(1))
		})
	})
})
