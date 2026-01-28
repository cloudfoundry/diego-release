package vizzini_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/routing-info/cfroutes"
	. "code.cloudfoundry.org/vizzini/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LRPs", func() {
	var lrp *models.DesiredLRP
	var url string

	BeforeEach(func() {
		url = "http://" + RouteForGuid(guid) + "/env?json=true"
		lrp = DesiredLRPWithGuid(guid)
	})

	Describe("Desiring LRPs", func() {
		Context("when the LRP is well-formed", func() {
			BeforeEach(func() {
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
			})

			It("desires the LRP", func() {
				Eventually(LRPGetter(logger, guid)).ShouldNot(BeZero())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
				Eventually(ActualByDomainGetter(logger, domain)).Should(ContainElement(BeActualLRP(guid, 0)))

				fetchedLRP, err := bbsClient.DesiredLRPByProcessGuid(logger, traceID, guid)
				Expect(err).NotTo(HaveOccurred())
				Expect(fetchedLRP.Annotation).To(Equal("arbitrary-data"))
			})
		})

		Context("when the LRP's process guid contains invalid characters", func() {
			It("should fail to create", func() {
				lrp.ProcessGuid = "abc def"
				err := bbsClient.DesireLRP(logger, traceID, lrp)
				Expect(models.ConvertError(err).Type).To(Equal(models.Error_InvalidRequest))

				lrp.ProcessGuid = "abc/def"
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).NotTo(Succeed())

				lrp.ProcessGuid = "abc,def"
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).NotTo(Succeed())

				lrp.ProcessGuid = "abc.def"
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).NotTo(Succeed())

				lrp.ProcessGuid = "abc∆def"
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).NotTo(Succeed())
			})
		})

		Context("when the LRP's # of instances is == 0", func() {
			It("should create the LRP and allow the user to subsequently scale up", func() {
				lrp.Instances = 0
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())

				dlu := &models.DesiredLRPUpdate{}
				instances := int32(2)
				dlu.SetInstances(&instances)
				Expect(bbsClient.UpdateDesiredLRP(logger, traceID, lrp.ProcessGuid, dlu)).To(Succeed())

				Eventually(IndexCounter(guid)).Should(Equal(2))
			})
		})

		Context("when required fields are missing", func() {
			It("should fail to create", func() {
				lrpCopy := new(models.DesiredLRP)

				By("not having ProcessGuid")
				*lrpCopy = *lrp
				lrpCopy.ProcessGuid = ""
				Expect(bbsClient.DesireLRP(logger, traceID, lrpCopy)).NotTo(Succeed())

				By("not having a domain")
				*lrpCopy = *lrp
				lrpCopy.Domain = ""
				Expect(bbsClient.DesireLRP(logger, traceID, lrpCopy)).NotTo(Succeed())

				By("not having an action")
				*lrpCopy = *lrp
				lrpCopy.Action = nil
				Expect(bbsClient.DesireLRP(logger, traceID, lrpCopy)).NotTo(Succeed())

				By("not having a rootfs")
				*lrpCopy = *lrp
				lrpCopy.RootFs = ""
				Expect(bbsClient.DesireLRP(logger, traceID, lrpCopy)).NotTo(Succeed())

				By("having a malformed rootfs")
				*lrpCopy = *lrp
				lrpCopy.RootFs = "ploop"
				Expect(bbsClient.DesireLRP(logger, traceID, lrpCopy)).NotTo(Succeed())
			})
		})

		Context("when the annotation is too large", func() {
			It("should fail", func() {
				lrp.Annotation = strings.Repeat("7", 1024*10+1)
				err := bbsClient.DesireLRP(logger, traceID, lrp)
				Expect(models.ConvertError(err).Type).To(Equal(models.Error_InvalidRequest))
			})
		})

		Context("when the DesiredLRP already exists", func() {
			BeforeEach(func() {
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			})

			It("should fail", func() {
				err := bbsClient.DesireLRP(logger, traceID, lrp)
				Expect(models.ConvertError(err).Type).To(Equal(models.Error_ResourceExists))
			})
		})
	})

	Describe("Specifying environment variables", func() {
		BeforeEach(func() {
			lrp.EnvironmentVariables = []*models.EnvironmentVariable{
				{Name: "CONTAINER_LEVEL", Value: "AARDVARK"},
				{Name: "OVERRIDE", Value: "BANANA"},
			}

			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
			Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
		})

		It("should be possible to specify environment variables on both the DesiredLRP and the RunAction", func() {
			resp, err := http.Get(url)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())

			env := [][]string{}
			err = json.Unmarshal(bodyBytes, &env)
			Expect(err).NotTo(HaveOccurred())

			Expect(env).To(ContainElement([]string{"CONTAINER_LEVEL", "AARDVARK"}))
			Expect(env).To(ContainElement([]string{"ACTION_LEVEL", "COYOTE"}))
			Expect(env).To(ContainElement([]string{"OVERRIDE", "DAQUIRI"}))
			Expect(env).NotTo(ContainElement([]string{"OVERRIDE", "BANANA"}))
		})
	})

	Describe("Specifying declarative health check", func() {
		BeforeEach(func() {
			lrp.Setup = models.WrapAction(models.Serial(
				&models.DownloadAction{
					From:     config.GraceTarballURL,
					To:       ".",
					CacheKey: "grace",
					User:     "vcap",
				},
				&models.DownloadAction{
					From:     config.FileServerAddress + "/v1/static/buildpack_app_lifecycle/buildpack_app_lifecycle.tgz",
					To:       "/tmp/lifecycle",
					CacheKey: "buildpack-app-lifecycle",
					User:     "vcap",
				},
			))
			// check the wrong port to ensure the check definition is actually being used
			lrp.Monitor = models.WrapAction(&models.RunAction{
				Path: "/tmp/lifecycle/healthcheck",
				Args: []string{"-port=8090", "-uri=/ping"},
				User: "vcap",
			})

			lrp.CheckDefinition = &models.CheckDefinition{
				Checks: []*models.Check{
					{
						HttpCheck: &models.HTTPCheck{
							Port: 8080,
							Path: "/ping",
						},
					},
				},
			}

			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
		})

		It("should run", func() {
			if !config.EnableDeclarativeHealthcheck {
				Skip("declarative are not enabled")
			}

			Eventually(ActualByDomainGetter(logger, domain)).Should(ContainElement(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning)))
		})
	})

	Describe("Specifying HTTP-based health check (move to inigo or DATs once CC can specify an HTTP-based health-check)", func() {
		BeforeEach(func() {
			lrp.Setup = models.WrapAction(models.Serial(
				&models.DownloadAction{
					From:     config.GraceTarballURL,
					To:       ".",
					CacheKey: "grace",
					User:     "vcap",
				},
				&models.DownloadAction{
					From:     config.FileServerAddress + "/v1/static/buildpack_app_lifecycle/buildpack_app_lifecycle.tgz",
					To:       "/tmp/lifecycle",
					CacheKey: "buildpack-app-lifecycle",
					User:     "vcap",
				},
			))

			lrp.Monitor = models.WrapAction(&models.RunAction{
				Path: "/tmp/lifecycle/healthcheck",
				Args: []string{"-port=8080", "-uri=/ping"},
				User: "vcap",
			})

			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
		})

		It("should run", func() {
			Eventually(ActualByDomainGetter(logger, domain)).Should(ContainElement(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning)))
		})
	})

	Describe("specifying sidecars on an LRP", func() {
		var (
			sidecar1Route string
			sidecar2Route string
		)

		BeforeEach(func() {
			sidecar1Route = RouteForGuid("sidecar-1-" + guid)
			sidecar2Route = RouteForGuid("sidecar-2-" + guid)
			routingInfo := cfroutes.CFRoutes{
				{Port: 8080, Hostnames: []string{RouteForGuid(guid)}},
				{Port: 8090, Hostnames: []string{sidecar1Route}},
				{Port: 8100, Hostnames: []string{sidecar2Route}},
			}.RoutingInfo()
			lrp.Routes = &routingInfo
			lrp.Ports = []uint32{8080, 8090, 8100}
			lrp.Sidecars = []*models.Sidecar{
				{
					Action: models.WrapAction(&models.RunAction{
						Path: "/tmp/grace/grace",
						User: "vcap",
						Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8090"}, {Name: "SECONDARY_PORT", Value: "9090"}},
					}),
				},
				{
					Action: models.WrapAction(&models.RunAction{
						Path: "/tmp/grace/grace",
						User: "vcap",
						Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8100"}, {Name: "SECONDARY_PORT", Value: "9100"}},
					}),
				},
			}

			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
		})

		It("should run", func() {
			Eventually(ActualByDomainGetter(logger, domain)).Should(ContainElement(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning)))
			Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			Eventually(EndpointCurler("http://" + sidecar1Route + "/env")).Should(Equal(http.StatusOK))
			Eventually(EndpointCurler("http://" + sidecar2Route + "/env")).Should(Equal(http.StatusOK))
		})

	})

	Describe("when a sidecar crashes", func() {
		BeforeEach(func() {
			lrp.Sidecars = append(lrp.Sidecars, &models.Sidecar{
				Action: models.WrapAction(&models.RunAction{
					User: "vcap",
					Path: "/bin/bash",
					Args: []string{"-c", "exit 1"},
				}),
			})

			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
		})

		It("gets marked as crashed (immediately)", func() {
			Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithCrashCount(guid, 0, 1))
		})
	})

	Describe("{DOCKER} Creating a Docker-based LRP", func() {
		BeforeEach(func() {
			lrp.RootFs = config.GraceBusyboxImageURL
			lrp.Setup = models.WrapAction(&models.DownloadAction{
				From:     config.FileServerAddress + "/v1/static/docker_app_lifecycle/docker_app_lifecycle.tgz",
				To:       "/tmp/lifecycle",
				CacheKey: "docker-app-lifecycle",
				User:     "root",
			})
			lrp.Action = models.WrapAction(&models.RunAction{
				Path: "/grace",
				User: "root",
				Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
			})
			lrp.Monitor = models.WrapAction(&models.RunAction{
				Path: "/tmp/lifecycle/healthcheck",
				Args: []string{"-port=8080"},
				User: "root",
			})
		})
		JustBeforeEach(func() {
			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
		})

		It("should succeed", func() {
			Eventually(EndpointCurler(url), 120).Should(Equal(http.StatusOK), "Docker can be quite slow to spin up...")
			Eventually(ActualByDomainGetter(logger, domain)).Should(ContainElement(BeActualLRP(guid, 0)))
		})

		Context("with an OCI image", func() {
			BeforeEach(func() {
				lrp.RootFs = config.DiegoDockerOCIImageURL
				lrp.Action = models.WrapAction(&models.RunAction{
					Path: "dockerapp",
					Args: []string{"-name", "with_oci_image_index"},
					User: "root",
					Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
				})
			})
			It("should succeed", func() {
				Eventually(EndpointCurler(url), 120).Should(Equal(http.StatusOK), "Docker can be quite slow to spin up...")
				Eventually(ActualByDomainGetter(logger, domain)).Should(ContainElement(BeActualLRP(guid, 0)))
			})
		})
	})

	Describe("Updating an existing DesiredLRP", func() {
		var tag *models.ModificationTag

		BeforeEach(func() {
			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
			Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			fetchedLRP, err := bbsClient.DesiredLRPByProcessGuid(logger, traceID, lrp.ProcessGuid)
			Expect(err).NotTo(HaveOccurred())
			tag = fetchedLRP.ModificationTag
		})

		Context("By explicitly updating it", func() {
			Context("when the LRP exists", func() {
				It("allows updating instances", func() {
					dlu := &models.DesiredLRPUpdate{}
					instances := int32(2)
					dlu.SetInstances(&instances)
					Expect(bbsClient.UpdateDesiredLRP(logger, traceID, guid, dlu)).To(Succeed())

					Eventually(IndexCounter(guid)).Should(Equal(2))
				})

				It("allows scaling down to 0", func() {
					dlu := &models.DesiredLRPUpdate{}
					instances := int32(0)
					dlu.SetInstances(&instances)
					Expect(bbsClient.UpdateDesiredLRP(logger, traceID, guid, dlu)).To(Succeed())

					Eventually(IndexCounter(guid)).Should(Equal(0))
				})

				It("allows updating routes", func() {
					newRoute := RouteForGuid(NewGuid())
					routes, err := cfroutes.CFRoutesFromRoutingInfo(*lrp.Routes)
					Expect(err).NotTo(HaveOccurred())

					routes[0].Hostnames = append(routes[0].Hostnames, newRoute)
					routingInfo := routes.RoutingInfo()

					Expect(bbsClient.UpdateDesiredLRP(logger, traceID, guid, &models.DesiredLRPUpdate{
						Routes: &routingInfo,
					})).To(Succeed())

					Eventually(EndpointCurler("http://" + newRoute + "/env")).Should(Equal(http.StatusOK))
					Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
				})

				It("allows updating annotations", func() {
					annotation := "my new annotation"
					dlu := &models.DesiredLRPUpdate{}
					dlu.SetAnnotation(&annotation)
					Expect(bbsClient.UpdateDesiredLRP(logger, traceID, guid, dlu)).To(Succeed())

					lrp, err := bbsClient.DesiredLRPByProcessGuid(logger, traceID, guid)
					Expect(err).NotTo(HaveOccurred())
					Expect(lrp.Annotation).To(Equal("my new annotation"))
				})

				It("allows multiple simultaneous updates", func() {

					newRoute := RouteForGuid(NewGuid())
					routes, err := cfroutes.CFRoutesFromRoutingInfo(*lrp.Routes)
					Expect(err).NotTo(HaveOccurred())

					routes[0].Hostnames = append(routes[0].Hostnames, newRoute)
					routingInfo := routes.RoutingInfo()

					dlu := &models.DesiredLRPUpdate{Routes: &routingInfo}
					instances := int32(2)
					dlu.SetInstances(&instances)
					annotation := "my new annotation"
					dlu.SetAnnotation(&annotation)
					Expect(bbsClient.UpdateDesiredLRP(logger, traceID, guid, dlu)).To(Succeed())

					Eventually(IndexCounter(guid)).Should(Equal(2))

					Eventually(EndpointCurler("http://" + newRoute + "/env")).Should(Equal(http.StatusOK))
					Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))

					lrp, err := bbsClient.DesiredLRPByProcessGuid(logger, traceID, guid)
					Expect(err).NotTo(HaveOccurred())
					Expect(lrp.Annotation).To(Equal("my new annotation"))
				})

				It("updates the modification index when a change occurs", func() {
					By("not modifying if no change has been made")
					fetchedLRP, err := bbsClient.DesiredLRPByProcessGuid(logger, traceID, lrp.ProcessGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(fetchedLRP.ModificationTag).To(Equal(tag))

					By("modifying when a change is made")
					dlu := &models.DesiredLRPUpdate{}
					instances := int32(2)
					dlu.SetInstances(&instances)
					Expect(bbsClient.UpdateDesiredLRP(logger, traceID, guid, dlu)).To(Succeed())

					Eventually(IndexCounter(guid)).Should(Equal(2))

					fetchedLRP, err = bbsClient.DesiredLRPByProcessGuid(logger, traceID, lrp.ProcessGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(fetchedLRP.ModificationTag.Epoch).To(Equal(tag.Epoch))
					Expect(fetchedLRP.ModificationTag.Index).To(BeNumerically(">", tag.Index))
				})
			})

			Context("when the LRP does not exist", func() {
				It("errors", func() {
					dlu := &models.DesiredLRPUpdate{}
					instances := int32(2)
					dlu.SetInstances(&instances)
					err := bbsClient.UpdateDesiredLRP(logger, traceID, "flooberdoobey", dlu)
					Expect(models.ConvertError(err).Type).To(Equal(models.Error_ResourceNotFound))
				})
			})
		})
	})

	Describe("Getting a DesiredLRP", func() {
		Context("when the DesiredLRP exists", func() {
			BeforeEach(func() {
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			})

			It("should succeed", func() {
				lrp, err := bbsClient.DesiredLRPByProcessGuid(logger, traceID, guid)
				Expect(err).NotTo(HaveOccurred())
				Expect(lrp.ProcessGuid).To(Equal(guid))
			})
		})

		Context("when the DesiredLRP does not exist", func() {
			It("should error", func() {
				lrp, err := bbsClient.DesiredLRPByProcessGuid(logger, traceID, "floobeedoo")
				Expect(lrp).To(BeZero())
				Expect(models.ConvertError(err).Type).To(Equal(models.Error_ResourceNotFound))
			})
		})
	})

	Describe("Getting All DesiredLRPs and Getting DesiredLRPs by Domain", func() {
		var otherGuids []string

		BeforeEach(func() {
			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
			Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))

			otherGuids = []string{NewGuid(), NewGuid()}
			for _, otherGuid := range otherGuids {
				otherLRP := DesiredLRPWithGuid(otherGuid)
				otherLRP.Domain = otherDomain
				Expect(bbsClient.DesireLRP(logger, traceID, otherLRP)).To(Succeed())
				url := "http://" + RouteForGuid(otherGuid) + "/env"
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK), "LRP '"+otherGuid+"' failed to become routable within the timeout")
			}
		})

		It("should fetch desired lrps in the given domain", func() {
			lrpsInDomain, err := bbsClient.DesiredLRPs(logger, traceID, models.DesiredLRPFilter{Domain: domain})
			Expect(err).NotTo(HaveOccurred())

			lrpsInOtherDomain, err := bbsClient.DesiredLRPs(logger, traceID, models.DesiredLRPFilter{Domain: otherDomain})
			Expect(err).NotTo(HaveOccurred())

			Expect(lrpsInDomain).To(HaveLen(1))
			Expect(lrpsInOtherDomain).To(HaveLen(2))
			Expect([]string{lrpsInOtherDomain[0].ProcessGuid, lrpsInOtherDomain[1].ProcessGuid}).To(ConsistOf(otherGuids))
		})

		It("should not error if a domain is empty", func() {
			lrpsInDomain, err := bbsClient.DesiredLRPs(logger, traceID, models.DesiredLRPFilter{Domain: "farfignoogan"})
			Expect(err).NotTo(HaveOccurred())
			Expect(lrpsInDomain).To(BeEmpty())
		})

		It("should fetch all desired lrps", func() {
			allDesiredLRPs, err := bbsClient.DesiredLRPs(logger, traceID, models.DesiredLRPFilter{})
			Expect(err).NotTo(HaveOccurred())

			//if we're running in parallel there may be more than 3 things here!
			Expect(len(allDesiredLRPs)).To(BeNumerically(">=", 3))
			lrpGuids := []string{}
			for _, lrp := range allDesiredLRPs {
				lrpGuids = append(lrpGuids, lrp.ProcessGuid)
			}
			Expect(lrpGuids).To(ContainElement(guid))
			Expect(lrpGuids).To(ContainElement(otherGuids[0]))
			Expect(lrpGuids).To(ContainElement(otherGuids[1]))
		})
	})

	Describe("Deleting DesiredLRPs", func() {
		Context("when the DesiredLRP exists", func() {
			It("should be deleted", func() {
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))

				Expect(bbsClient.RemoveDesiredLRP(logger, traceID, guid)).To(Succeed())
				_, err := bbsClient.DesiredLRPByProcessGuid(logger, traceID, guid)
				Expect(err).To(HaveOccurred())
				Eventually(EndpointCurler(url)).ShouldNot(Equal(http.StatusOK))
			})
		})

		Context("when the DesiredLRP is deleted after it is claimed but before it is running #86668966", func() {
			It("should succesfully remove any ActualLRP", func() {
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(ActualByProcessGuidGetter(logger, lrp.ProcessGuid)).Should(ContainElement(BeActualLRPWithState(lrp.ProcessGuid, 0, models.ActualLRPStateClaimed)))
				//note: we don't wait for the ActualLRP to start running
				Expect(bbsClient.RemoveDesiredLRP(logger, traceID, lrp.ProcessGuid)).To(Succeed())
				Eventually(ActualByProcessGuidGetter(logger, lrp.ProcessGuid)).Should(BeEmpty())
			})
		})

		Context("when the DesiredLRP does not exist", func() {
			It("should not be deleted, and should error", func() {
				err := bbsClient.RemoveDesiredLRP(logger, traceID, "floobeedoobee")
				Expect(models.ConvertError(err).Type).To(Equal(models.Error_ResourceNotFound))
			})
		})
	})

	Describe("Getting all ActualLRPs", func() {
		BeforeEach(func() {
			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
			Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
		})

		It("should fetch all Actual LRPs", func() {
			actualLRPs, err := ActualsByDomain(logger, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(actualLRPs)).To(BeNumerically(">=", 1))
			Expect(actualLRPs).To(ContainElement(BeActualLRP(guid, 0)))
		})
	})

	Describe("Getting ActualLRPs by Domain", func() {
		Context("when the domain is empty", func() {
			It("returns an empty list", func() {
				Expect(ActualsByDomain(logger, "floobidoo")).To(BeEmpty())
			})
		})

		Context("when the domain contains instances", func() {
			var otherDomainLRP1 *models.DesiredLRP
			var otherDomainLRP2 *models.DesiredLRP

			BeforeEach(func() {
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())

				otherGuid1 := NewGuid()
				otherDomainLRP1 = DesiredLRPWithGuid(otherGuid1)
				otherDomainLRP1.Instances = 2
				otherDomainLRP1.Domain = otherDomain
				Expect(bbsClient.DesireLRP(logger, traceID, otherDomainLRP1)).To(Succeed())

				otherGuid2 := NewGuid()
				otherDomainLRP2 = DesiredLRPWithGuid(otherGuid2)
				otherDomainLRP2.Domain = otherDomain
				Expect(bbsClient.DesireLRP(logger, traceID, otherDomainLRP2)).To(Succeed())

				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
				Eventually(IndexCounter(otherDomainLRP1.ProcessGuid)).Should(Equal(2), "LRP '"+otherGuid1+"' instances failed to come up")
				Eventually(IndexCounter(otherDomainLRP2.ProcessGuid)).Should(Equal(1), "LRP '"+otherGuid2+"' instances failed to come up")
			})

			It("returns said instances", func() {
				actualLRPs, err := ActualsByDomain(logger, domain)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualLRPs).To(HaveLen(1))
				Expect(actualLRPs).To(ConsistOf(BeActualLRP(guid, 0)))

				actualLRPs, err = ActualsByDomain(logger, otherDomain)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualLRPs).To(HaveLen(3))

				Expect(actualLRPs).To(ConsistOf(
					BeActualLRP(otherDomainLRP1.ProcessGuid, 0),
					BeActualLRP(otherDomainLRP1.ProcessGuid, 1),
					BeActualLRP(otherDomainLRP2.ProcessGuid, 0),
				))
			})
		})
	})

	Describe("Getting ActualLRPs by ProcessGuid", func() {
		Context("when there are none", func() {
			It("returns an empty list", func() {
				Expect(ActualsByProcessGuid(logger, "floobeedoo")).To(BeEmpty())
			})
		})

		Context("when there are ActualLRPs for a given ProcessGuid", func() {
			BeforeEach(func() {
				lrp.Instances = 2
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(IndexCounter(guid)).Should(Equal(2))
			})

			It("returns the ActualLRPs", func() {
				Expect(ActualsByProcessGuid(logger, guid)).To(ConsistOf(
					BeActualLRP(guid, 0),
					BeActualLRP(guid, 1),
				))

			})
		})
	})

	Describe("Getting the ActualLRP at a given index for a ProcessGuid", func() {
		Context("when there is no matching ProcessGuid", func() {
			It("should return a missing ActualLRP error", func() {
				actualLRP, err := ActualLRPByProcessGuidAndIndex(logger, "floobeedoo", 0)
				Expect(actualLRP).To(BeZero())
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when there are no ActualLRPs at the given index", func() {
			BeforeEach(func() {
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(IndexCounter(guid)).Should(Equal(1))
			})

			It("should return a missing ActualLRP error", func() {
				actualLRP, err := ActualLRPByProcessGuidAndIndex(logger, guid, 1)
				Expect(actualLRP).To(BeZero())
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when there is an ActualLRP at the given index", func() {
			BeforeEach(func() {
				lrp.Instances = 2
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(IndexCounter(guid)).Should(Equal(2))
			})

			It("returns them", func() {
				Expect(ActualLRPByProcessGuidAndIndex(logger, guid, 0)).To(BeActualLRP(guid, 0))
				Expect(ActualLRPByProcessGuidAndIndex(logger, guid, 1)).To(BeActualLRP(guid, 1))
			})
		})
	})

	Describe("Restarting an ActualLRP", func() {
		Context("when there is no matching ProcessGuid", func() {
			It("returns an error", func() {
				lrpKey := models.NewActualLRPKey(guid, 0, domain)
				Expect(bbsClient.RetireActualLRP(logger, traceID, &lrpKey)).NotTo(Succeed())
			})
		})

		Context("when there is no ActualLRP at the given index", func() {
			BeforeEach(func() {
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			})

			It("returns an error", func() {
				lrpKey := models.NewActualLRPKey(guid, 1, domain)
				Expect(bbsClient.RetireActualLRP(logger, traceID, &lrpKey)).NotTo(Succeed())
			})
		})

		Context("{SLOW} when an ActualLRP exists at the given ProcessGuid and index", func() {
			BeforeEach(func() {
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			})

			It("restarts the actual lrp", func() {
				initialTime, err := StartedAtGetter(guid)()
				Expect(err).NotTo(HaveOccurred())
				lrpKey := models.NewActualLRPKey(guid, 0, domain)
				Expect(bbsClient.RetireActualLRP(logger, traceID, &lrpKey)).To(Succeed())
				//This needs a large timeout as the converger needs to run for it to return
				Eventually(StartedAtGetter(guid), ConvergerInterval*2).Should(BeNumerically(">", initialTime))
			})
		})
	})

	Describe("when an ActualLRP cannot be allocated", func() {
		Context("because it's too large", func() {
			BeforeEach(func() {
				lrp.MemoryMb = 1024 * 1024
			})

			It("should report this fact on the UNCLAIMED ActualLRP", func() {
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(ActualGetter(logger, guid, 0)).Should(BeUnclaimedActualLRPWithPlacementError(guid, 0))

				actualLRP, err := ActualLRPByProcessGuidAndIndex(logger, guid, 0)
				Expect(err).NotTo(HaveOccurred())

				Expect(actualLRP.State).To(Equal(models.ActualLRPStateUnclaimed))
				Expect(actualLRP.PlacementError).To(ContainSubstring("insufficient resources"))
			})
		})

		Context("because of a rootfs mismatch", func() {
			BeforeEach(func() {
				lrp.RootFs = models.PreloadedRootFS("fruitfs")
			})

			It("should allow creation of the task but should (fairly quickly) mark the task as failed", func() {
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(ActualGetter(logger, guid, 0)).Should(BeUnclaimedActualLRPWithPlacementError(guid, 0))

				actualLRP, err := ActualLRPByProcessGuidAndIndex(logger, guid, 0)
				Expect(err).NotTo(HaveOccurred())

				Expect(actualLRP.State).To(Equal(models.ActualLRPStateUnclaimed))
				Expect(actualLRP.PlacementError).To(ContainSubstring("found no compatible cell"))
			})
		})
	})
})
