package vizzini_test

import (
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager/v3"
	. "code.cloudfoundry.org/vizzini/matchers"

	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func MakeGraceExit(baseURL string, status int) {
	//make sure Grace is up first
	Eventually(EndpointCurler(baseURL + "/env")).Should(Equal(http.StatusOK))

	//make Grace exit
	for i := 0; i < 10; i++ {
		url := fmt.Sprintf("%s/exit/%d", baseURL, status)
		resp, err := http.Post(url, "application/octet-stream", nil)
		Expect(err).NotTo(HaveOccurred())
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return
		}
		logger.Info(
			"grace-exit-request-failed",
			lager.Data{"response-status-code": resp.StatusCode, "attempt": i, "desired-exit-status": status},
		)
		time.Sleep(10 * time.Millisecond)
	}
	Fail("failed to make grace exit")
}

func TellGraceToDeleteFile(baseURL string, filename string) {
	url := fmt.Sprintf("%s/file/%s", baseURL, filename)
	req, err := http.NewRequest("DELETE", url, nil)
	Expect(err).NotTo(HaveOccurred())
	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	resp.Body.Close()
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
}

var _ = Describe("Crashes", func() {
	var lrp *models.DesiredLRP
	var url string

	BeforeEach(func() {
		url = fmt.Sprintf("http://%s", RouteForGuid(guid))
		lrp = DesiredLRPWithGuid(guid)
	})

	Describe("Annotating the Crash Reason", func() {
		BeforeEach(func() {
			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
		})

		It("adds the crash reason to the application", func() {
			MakeGraceExit(url, 17)
			Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 1))
			actualLRP, err := ActualLRPByProcessGuidAndIndex(logger, guid, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLRP.CrashReason).To(ContainSubstring("Exited with status 17"))
		})
	})

	Describe("backoff behavior", func() {
		BeforeEach(func() {
			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
		})

		It("{SLOW} restarts the application immediately twice, and then starts backing it off, and updates the modification tag as it goes", func() {
			actualLRP, err := ActualLRPByProcessGuidAndIndex(logger, guid, 0)
			Expect(err).NotTo(HaveOccurred())
			tag := actualLRP.ModificationTag

			By("immediately restarting #1")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 1))

			restartedActualLRP, err := ActualLRPByProcessGuidAndIndex(logger, guid, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(restartedActualLRP.InstanceGuid).NotTo(Equal(actualLRP.InstanceGuid))
			Expect(restartedActualLRP.ModificationTag.Epoch).To(Equal(tag.Epoch))
			Expect(restartedActualLRP.ModificationTag.Index).To(BeNumerically(">", tag.Index))

			By("immediately restarting #2")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 2))

			By("eventually restarting #3 (slow)")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0), ConvergerInterval).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateCrashed, 3))
			Consistently(ActualGetter(logger, guid, 0), CrashRestartTimeout-5*time.Second).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateCrashed, 3))
			Eventually(ActualGetter(logger, guid, 0), ConvergerInterval*2).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 3))
			Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
		})

		It("deletes the crashed ActualLRP when scaling down", func() {
			By("immediately restarting #1")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 1))

			By("immediately restarting #2")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 2))

			By("eventually restarting #3")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0), ConvergerInterval).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateCrashed, 3))

			By("deleting the DesiredLRP")
			Expect(bbsClient.RemoveDesiredLRP(logger, traceID, guid)).To(Succeed())
			Eventually(ActualByProcessGuidGetter(logger, guid)).Should(BeEmpty())
		})
	})

	Describe("killing crashed applications", func() {
		BeforeEach(func() {
			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
		})

		It("should delete the Crashed ActualLRP succesfully", func() {
			By("immediately restarting #1")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 1))

			By("immediately restarting #2")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 2))

			By("eventually restarting #3")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(logger, guid, 0), ConvergerInterval).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateCrashed, 3))

			actualLRPKey := models.NewActualLRPKey(guid, 0, domain)
			Expect(bbsClient.RetireActualLRP(logger, traceID, &actualLRPKey)).To(Succeed())
			Eventually(ActualByProcessGuidGetter(logger, guid)).Should(BeEmpty())
		})
	})

	Context("with no monitor action", func() {
		BeforeEach(func() {
			lrp.Monitor = nil
		})

		Context("when running a single action", func() {
			BeforeEach(func() {
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
			})

			It("comes up as soon as the process starts", func() {
				Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
			})

			Context("when the process dies with exit code 0", func() {
				BeforeEach(func() {
					MakeGraceExit(url, 0)
				})

				It("gets restarted immediately", func() {
					Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 1))
					Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
				})
			})

			Context("when the process dies with exit code 1", func() {
				BeforeEach(func() {
					MakeGraceExit(url, 1)
				})

				It("gets restarted immediately", func() {
					Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 1))
					Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
				})
			})
		})

		Context("when running several actions", func() {
			Context("codependently", func() {
				BeforeEach(func() {
					lrp.Action = models.WrapAction(models.Codependent(
						&models.RunAction{
							Path: "bash",
							Args: []string{"-c", "while true; do sleep 1; done"},
							User: "vcap",
						},
						&models.RunAction{
							Path: "/tmp/grace/grace",
							Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
							User: "vcap",
						},
					))
				})

				JustBeforeEach(func() {
					Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				})

				Context("when one of the actions finishes", func() {
					JustBeforeEach(func() {
						Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
						MakeGraceExit(url, 0)
					})

					It("gets restarted immediately", func() {
						Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 1))
						Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
					})
				})
			})

			Context("with a failing check definition", func() {
				JustBeforeEach(func() {
					Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				})

				Context("when failing readiness", func() {
					BeforeEach(func() {
						lrp.Action = models.WrapAction(
							models.Parallel(
								&models.RunAction{
									Path: "/tmp/grace/grace",
									Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
									User: "vcap",
								},
							),
						)
						lrp.StartTimeoutMs = 3000
					})

					Context("with both http and tcp healthcheck", func() {
						BeforeEach(func() {
							lrp.CheckDefinition = &models.CheckDefinition{
								Checks: []*models.Check{
									{
										TcpCheck: &models.TCPCheck{
											Port: 9090,
										},
									},
									{
										HttpCheck: &models.HTTPCheck{
											Port: 9090,
											Path: "/ping",
										},
									},
								},
							}
						})

						It("shows the monitor crash reasons", func() {

							Eventually(ActualGetter(logger, guid, 0), HealthyCheckInterval+5*time.Second).Should(BeActualLRPWithCrashCount(guid, 0, 1))

							actualLRP, err := ActualGetter(logger, guid, 0)()
							Expect(err).NotTo(HaveOccurred())

							Expect(actualLRP.CrashReason).To(ContainSubstring("Instance never healthy after"))
							Expect(actualLRP.CrashReason).To(MatchRegexp("failed to make TCP connection to .*:9090: dial tcp .*:9090: connect: connection refused"))
							Expect(actualLRP.CrashReason).To(ContainSubstring("failed to make HTTP request to '/ping' on port 9090: connection refused"))
						})
					})
				})

				Context("when failing liveness", func() {
					BeforeEach(func() {
						lrp.Action = models.WrapAction(
							models.Parallel(
								&models.RunAction{
									Path: "bash",
									Args: []string{"-c", "while true; do sleep 1; done"},
									User: "vcap",
								},
								&models.RunAction{
									Path: "/tmp/grace/grace",
									Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
									User: "vcap",
								},
							),
						)
					})

					Context("with http healthcheck", func() {
						BeforeEach(func() {
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
						})

						It("shows the monitor crash reasons", func() {

							Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
							MakeGraceExit(url, 0)
							Eventually(ActualGetter(logger, guid, 0), HealthyCheckInterval+10*time.Second).Should(BeActualLRPWithCrashCount(guid, 0, 1))

							actualLRP, err := ActualGetter(logger, guid, 0)()
							Expect(err).NotTo(HaveOccurred())

							Expect(actualLRP.CrashReason).To(ContainSubstring("Instance became unhealthy: Liveness check unsuccessful: failed to make HTTP request to '/ping' on port 8080: connection refused"))
						})
					})

					Context("with tcp healthcheck", func() {
						BeforeEach(func() {
							lrp.CheckDefinition = &models.CheckDefinition{
								Checks: []*models.Check{
									{
										TcpCheck: &models.TCPCheck{
											Port: 8080,
										},
									},
								},
							}
						})

						It("shows the monitor crash reasons", func() {

							Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
							MakeGraceExit(url, 0)
							Eventually(ActualGetter(logger, guid, 0), HealthyCheckInterval+10*time.Second).Should(BeActualLRPWithCrashCount(guid, 0, 1))

							actualLRP, err := ActualGetter(logger, guid, 0)()
							Expect(err).NotTo(HaveOccurred())

							Expect(actualLRP.CrashReason).To(MatchRegexp("Instance became unhealthy: Liveness check unsuccessful: failed to make TCP connection to .*:8080: dial tcp .*:8080: connect: connection refused"))
						})
					})

					Context("with both http and tcp healthcheck", func() {
						BeforeEach(func() {
							lrp.CheckDefinition = &models.CheckDefinition{
								Checks: []*models.Check{
									{
										TcpCheck: &models.TCPCheck{
											Port: 8080,
										},
									},
									{
										HttpCheck: &models.HTTPCheck{
											Port: 8080,
											Path: "/ping",
										},
									},
								},
							}
						})

						It("shows the monitor crash reasons", func() {

							Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
							MakeGraceExit(url, 0)
							Eventually(ActualGetter(logger, guid, 0), HealthyCheckInterval+10*time.Second).Should(BeActualLRPWithCrashCount(guid, 0, 1))

							actualLRP, err := ActualGetter(logger, guid, 0)()
							Expect(err).NotTo(HaveOccurred())

							Expect(actualLRP.CrashReason).To(ContainSubstring("Instance became unhealthy:"))
							Expect(actualLRP.CrashReason).To(SatisfyAny(
								MatchRegexp("failed to make TCP connection to .*:8080: dial tcp .*:8080: connect: connection refused"),
								ContainSubstring("failed to make HTTP request to '/ping' on port 8080: connection refused"),
							))
						})
					})
				})
			})

			Context("in parallel", func() {
				BeforeEach(func() {
					lrp.Action = models.WrapAction(models.Parallel(
						&models.RunAction{
							Path: "bash",
							Args: []string{"-c", "while true; do sleep 1; done"},
							User: "vcap",
						},
						&models.RunAction{
							Path: "/tmp/grace/grace",
							Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
							User: "vcap",
						},
					))
					Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
					Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
				})

				Context("when one of the actions finishes", func() {
					BeforeEach(func() {
						MakeGraceExit(url, 2)
					})

					It("does not crash", func() {
						Consistently(ActualGetter(logger, guid, 0), 5).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
					})
				})
			})
		})
	})

	Context("with a monitor action", func() {
		Context("when the monitor eventually succeeds", func() {
			var directURL string
			var indirectURL string
			BeforeEach(func() {
				lrp.Action = models.WrapAction(&models.RunAction{
					Path: "/tmp/grace/grace",
					Args: []string{"-upFile=up"},
					User: "vcap",
					Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
				})

				lrp.Monitor = models.WrapAction(&models.RunAction{
					Path: "cat",
					Args: []string{"/tmp/up"},
					User: "vcap",
				})

				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
				Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
				directURL = "https://" + TLSDirectAddressFor(guid, 0, 8080)
				indirectURL = "http://" + RouteForGuid(guid)
			})

			It("enters the running state", func() {
				Expect(ActualGetter(logger, guid, 0)()).To(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
			})

			Context("when the process dies with exit code 0", func() {
				BeforeEach(func() {
					MakeGraceExit(indirectURL, 0)
				})

				It("does not get marked as crashed (may have daemonized)", func() {
					Consistently(ActualGetter(logger, guid, 0), 3).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateRunning, 0))
				})
			})

			Context("when the process dies with exit code 0 and the monitor subsequently fails", func() {
				BeforeEach(func() {
					//tell grace to delete the file then exit, it's highly unlikely that the health check will run
					//between these two lines so the test should actually be covering the edge case in question
					TellGraceToDeleteFile(url, "up")
					MakeGraceExit(indirectURL, 0)
				})

				It("{SLOW} is marked as crashed", func() {
					Consistently(ActualGetter(logger, guid, 0), 2).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning), "Banking on the fact that the health check runs every thirty seconds and is unlikely to run immediately")
					Eventually(ActualGetter(logger, guid, 0), HealthyCheckInterval+5*time.Second).Should(BeActualLRPWithCrashCount(guid, 0, 1))
				})
			})

			Context("when the process dies with exit code 1", func() {
				BeforeEach(func() {
					MakeGraceExit(indirectURL, 1)
				})

				It("is marked as crashed (immediately)", func() {
					Eventually(ActualGetter(logger, guid, 0), HealthyCheckInterval/3).Should(BeActualLRPWithCrashCount(guid, 0, 1))
				})
			})

			Context("when the monitor subsequently fails", func() {
				BeforeEach(func() {
					TellGraceToDeleteFile(indirectURL, "up")
				})

				It("{SLOW} is marked as crashed (and reaped)", func() {
					actualLRP, err := ActualLRPByProcessGuidAndIndex(logger, guid, 0)
					Expect(err).ToNot(HaveOccurred())

					tlsConfig, err := containerProxyTLSConfig(actualLRP.InstanceGuid)
					Expect(err).NotTo(HaveOccurred())

					httpClient := &http.Client{
						Timeout: time.Second,
						Transport: &http.Transport{
							TLSClientConfig: tlsConfig,
						},
					}

					By("first validate that we can connect to the container directly using " + directURL)
					_, err = httpClient.Get(directURL + "/env")
					Expect(err).NotTo(HaveOccurred())

					By("being marked as crashed")
					Eventually(ActualGetter(logger, guid, 0), HealthyCheckInterval+10*time.Second).Should(BeActualLRPWithCrashCount(guid, 0, 1))

					By("tearing down the process -- this reaches out to the container's direct address " + directURL + " and ensures we can't reach it")
					_, err = httpClient.Get(directURL + "/env")
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when the monitor never succeeds", func() {
			JustBeforeEach(func() {
				lrp.Monitor = models.WrapAction(&models.RunAction{
					Path: "false",
					User: "vcap",
				})

				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateClaimed))
			})

			Context("when the process dies with exit code 0", func() {
				BeforeEach(func() {
					lrp.Action = models.WrapAction(&models.RunAction{
						Path: "/tmp/grace/grace",
						Args: []string{"-exitAfter=2s", "-exitAfterCode=0"},
						User: "vcap",
						Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
					})
				})

				It("does not get marked as crash, as it has presumably daemonized and we are waiting on the health check", func() {
					Consistently(ActualGetter(logger, guid, 0), 3).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateClaimed, 0))
				})
			})

			Context("when the process dies with exit code 1", func() {
				BeforeEach(func() {
					lrp.Action = models.WrapAction(&models.RunAction{
						Path: "/tmp/grace/grace",
						Args: []string{"-exitAfter=2s", "-exitAfterCode=1"},
						User: "vcap",
						Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
					})
				})

				It("gets marked as crashed (immediately)", func() {
					Eventually(ActualGetter(logger, guid, 0), 30*time.Second).Should(BeActualLRPWithCrashCount(guid, 0, 1))
				})
			})

			Context("and there is a StartTimeout", func() {
				BeforeEach(func() {
					lrp.StartTimeoutMs = 3000
				})

				It("never enters the running state and is marked as crashed after the StartTimeout", func() {
					Consistently(ActualGetter(logger, guid, 0), 3).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateClaimed))
					Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithCrashCount(guid, 0, 1))
				})
			})

			Context("and there is no start timeout", func() {
				BeforeEach(func() {
					lrp.StartTimeoutMs = 0
				})

				It("never enters the running state, and never crashes", func() {
					Consistently(ActualGetter(logger, guid, 0), 5).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateClaimed, 0))
				})
			})
		})
	})
})
