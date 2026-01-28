package main_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/auctioneer/cmd/auctioneer/config"
	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/clock"
	diego_logging_client "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/locket"
	locketrunner "code.cloudfoundry.org/locket/cmd/locket/testrunner"
	"code.cloudfoundry.org/locket/lock"
	locketmodels "code.cloudfoundry.org/locket/models"
	"code.cloudfoundry.org/rep"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
)

const (
	defaultAuctioneerClientRequestTimeout = 5 * time.Second
)

var _ = Describe("Auctioneer", func() {
	var (
		auctioneerConfig config.AuctioneerConfig

		runner            *ginkgomon.Runner
		auctioneerProcess ifrit.Process

		auctioneerClient auctioneer.Client

		testIngressServer *testhelpers.TestIngressServer
		testMetricsChan   chan *loggregator_v2.Envelope
		signalMetricsChan chan struct{}
	)

	BeforeEach(func() {
		fixturesPath := "fixtures"

		caFile := path.Join(fixturesPath, "green-certs", "ca.crt")
		clientCertFile := path.Join(fixturesPath, "green-certs", "client.crt")
		clientKeyFile := path.Join(fixturesPath, "green-certs", "client.key")

		metronCAFile := path.Join(fixturesPath, "metron", "CA.crt")
		metronClientCertFile := path.Join(fixturesPath, "metron", "client.crt")
		metronClientKeyFile := path.Join(fixturesPath, "metron", "client.key")
		metronServerCertFile := path.Join(fixturesPath, "metron", "metron.crt")
		metronServerKeyFile := path.Join(fixturesPath, "metron", "metron.key")

		var err error
		testIngressServer, err = testhelpers.NewTestIngressServer(metronServerCertFile, metronServerKeyFile, metronCAFile)
		Expect(err).NotTo(HaveOccurred())
		receiversChan := testIngressServer.Receivers()
		testIngressServer.Start()
		metricsPort, err := testIngressServer.Port()
		Expect(err).NotTo(HaveOccurred())

		testMetricsChan, signalMetricsChan = testhelpers.TestMetricChan(receiversChan)

		bbsClient, err = bbs.NewClient(bbsURL.String(), caFile, clientCertFile, clientKeyFile, 0, 0)
		Expect(err).NotTo(HaveOccurred())

		auctioneerConfig = config.AuctioneerConfig{
			AuctionRunnerWorkers:          1000,
			BinPackFirstFitWeight:         0.0,
			CellStateTimeout:              durationjson.Duration(1 * time.Second),
			CommunicationTimeout:          durationjson.Duration(10 * time.Second),
			LagerConfig:                   lagerflags.DefaultLagerConfig(),
			LockTTL:                       durationjson.Duration(locket.DefaultSessionTTL),
			StartingContainerCountMaximum: 0,
			StartingContainerWeight:       .25,

			BBSAddress:        bbsURL.String(),
			BBSCACertFile:     caFile,
			BBSClientCertFile: clientCertFile,
			BBSClientKeyFile:  clientKeyFile,
			ListenAddress:     auctioneerLocation,
			LockRetryInterval: durationjson.Duration(time.Second),
			UUID:              "auctioneer-boshy-bosh",
			ReportInterval:    durationjson.Duration(10 * time.Millisecond),
			LoggregatorConfig: diego_logging_client.Config{
				BatchFlushInterval: 10 * time.Millisecond,
				BatchMaxSize:       1,
				UseV2API:           true,
				APIPort:            metricsPort,
				CACertPath:         metronCAFile,
				KeyPath:            metronClientKeyFile,
				CertPath:           metronClientCertFile,
			},
		}
		auctioneerConfig.ClientLocketConfig = locketrunner.ClientLocketConfig()
		auctioneerConfig.ClientLocketConfig.LocketAddress = locketAddress
		auctioneerClient = auctioneer.NewClient("http://"+auctioneerLocation, defaultAuctioneerClientRequestTimeout)
	})

	JustBeforeEach(func() {
		configFile, err := os.CreateTemp("", "auctioneer-config")
		Expect(err).NotTo(HaveOccurred())

		encoder := json.NewEncoder(configFile)
		err = encoder.Encode(&auctioneerConfig)
		Expect(err).NotTo(HaveOccurred())

		runner = ginkgomon.New(ginkgomon.Config{
			Name: "auctioneer",
			Command: exec.Command(
				auctioneerPath,
				"-config", configFile.Name(),
			),
			StartCheck: "auctioneer.started",
			Cleanup: func() {
				os.RemoveAll(configFile.Name())
			},
		})
	})

	AfterEach(func() {
		ginkgomon.Interrupt(auctioneerProcess)
		ginkgomon.Interrupt(locketProcess)
		testIngressServer.Stop()
		close(signalMetricsChan)
	})

	Context("when the metron agent isn't up", func() {
		BeforeEach(func() {
			testIngressServer.Stop()
		})

		It("exits with non-zero status code", func() {
			auctioneerProcess = ifrit.Background(runner)
			Eventually(auctioneerProcess.Wait()).Should(Receive(HaveOccurred()))
		})
	})

	Context("when the bbs is down", func() {
		BeforeEach(func() {
			ginkgomon.Interrupt(bbsProcess)
		})

		It("starts", func() {
			auctioneerProcess = ginkgomon.Invoke(runner)
			Consistently(runner).ShouldNot(Exit())
		})
	})

	Context("when the auctioneer starts up", func() {
		Context("when a debug address is specified", func() {
			BeforeEach(func() {
				port, err := portAllocator.ClaimPorts(1)
				Expect(err).NotTo(HaveOccurred())
				auctioneerConfig.DebugAddress = fmt.Sprintf("0.0.0.0:%d", port)
			})

			It("starts the debug server", func() {
				auctioneerProcess = ginkgomon.Invoke(runner)

				_, err := net.Dial("tcp", auctioneerConfig.DebugAddress)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Context("when the auctioneer is configured to grab the lock from the sql locking server", func() {
		var (
			task *rep.Task
		)

		BeforeEach(func() {
			task = &rep.Task{
				TaskGuid: "task-guid",
				Domain:   "test",
				Resource: rep.Resource{
					MemoryMB: 124,
					DiskMB:   456,
				},
				PlacementConstraint: rep.PlacementConstraint{
					RootFs: "some-rootfs",
				},
			}
		})

		JustBeforeEach(func() {
			auctioneerProcess = ifrit.Background(runner)
		})

		AfterEach(func() {
			ginkgomon.Interrupt(auctioneerProcess)
		})

		It("acquires the lock and becomes active", func() {
			Eventually(func() error {
				return auctioneerClient.RequestTaskAuctions(logger, "some-request-id", []*auctioneer.TaskStartRequest{
					&auctioneer.TaskStartRequest{Task: *task},
				})
			}).ShouldNot(HaveOccurred())
		})

		It("uses the configured UUID as the owner", func() {
			locketClient, err := locket.NewClient(logger, auctioneerConfig.ClientLocketConfig)
			Expect(err).NotTo(HaveOccurred())

			var lock *locketmodels.FetchResponse
			Eventually(func() error {
				lock, err = locketClient.Fetch(context.Background(), &locketmodels.FetchRequest{
					Key: "auctioneer",
				})
				return err
			}).ShouldNot(HaveOccurred())

			Expect(lock.Resource.Owner).To(Equal(auctioneerConfig.UUID))
		})

		It("emits metric about holding lock", func() {
			Eventually(func() error {
				return auctioneerClient.RequestTaskAuctions(logger, "some-request-id", []*auctioneer.TaskStartRequest{
					&auctioneer.TaskStartRequest{Task: *task},
				})
			}).ShouldNot(HaveOccurred())

			Eventually(testMetricsChan).Should(Receive(testhelpers.MatchV2MetricAndValue(testhelpers.MetricAndValue{
				Name:  "LockHeld",
				Value: 1,
			})))
		})

		Context("and the locking server becomes unreachable after grabbing the lock", func() {
			It("exits", func() {
				ginkgomon.Interrupt(locketProcess)
				Eventually(auctioneerProcess.Wait()).Should(Receive())
			})
		})

		Context("when the lock is not available", func() {
			var competingProcess ifrit.Process

			BeforeEach(func() {
				locketClient, err := locket.NewClient(logger, auctioneerConfig.ClientLocketConfig)
				Expect(err).NotTo(HaveOccurred())

				lockIdentifier := &locketmodels.Resource{
					Key:      "auctioneer",
					Owner:    "Your worst enemy.",
					Value:    "Something",
					TypeCode: locketmodels.LOCK,
				}

				clock := clock.NewClock()
				competingRunner := lock.NewLockRunner(
					logger,
					locketClient,
					lockIdentifier,
					locket.DefaultSessionTTLInSeconds,
					clock,
					locket.RetryInterval,
				)
				competingProcess = ginkgomon.Invoke(competingRunner)
			})

			AfterEach(func() {
				ginkgomon.Interrupt(competingProcess)
			})

			It("starts but does not accept auctions", func() {
				Consistently(func() error {
					return auctioneerClient.RequestTaskAuctions(logger, "some-request-id", []*auctioneer.TaskStartRequest{
						&auctioneer.TaskStartRequest{Task: *task},
					})
				}).Should(HaveOccurred())
			})

			It("emits metric about not holding lock", func() {
				Eventually(runner.Buffer()).Should(gbytes.Say("failed-to-acquire-lock"))

				Eventually(testMetricsChan).Should(Receive(testhelpers.MatchV2MetricAndValue(testhelpers.MetricAndValue{
					Name:  "LockHeld",
					Value: 0,
				})))
			})

			Context("and continues to be unavailable", func() {
				It("exits", func() {
					Eventually(auctioneerProcess.Wait(), locket.DefaultSessionTTL*2).Should(Receive())
				})
			})

			Context("and the lock becomes available", func() {
				JustBeforeEach(func() {
					Eventually(runner.Buffer()).Should(gbytes.Say(
						"failed-to-acquire-lock"))
					ginkgomon.Interrupt(competingProcess)
				})

				It("acquires the lock and becomes active", func() {
					Eventually(func() error {
						return auctioneerClient.RequestTaskAuctions(logger, "some-request-id", []*auctioneer.TaskStartRequest{
							&auctioneer.TaskStartRequest{Task: *task},
						})
					}, 2*time.Second).ShouldNot(HaveOccurred())
				})
			})
		})

		Context("and the locket address is invalid", func() {
			BeforeEach(func() {
				auctioneerConfig.LocketAddress = "{{{}}}}{{{{"
			})

			It("exits with an error", func() {
				Eventually(auctioneerProcess.Wait()).Should(Receive(Not(BeNil())))
			})
		})

		Context("when the locket addess isn't set", func() {
			BeforeEach(func() {
				auctioneerConfig.LocketAddress = ""
			})

			It("exits with an error", func() {
				Eventually(auctioneerProcess.Wait()).Should(Receive(Not(BeNil())))
			})
		})

		Context("and the UUID is not present", func() {
			BeforeEach(func() {
				auctioneerConfig.UUID = ""
			})

			It("exits with an error", func() {
				Eventually(auctioneerProcess.Wait()).Should(Receive())
			})
		})

	})

	Context("when the auctioneer is configured with TLS options", func() {
		var caCertFile, serverCertFile, serverKeyFile string

		BeforeEach(func() {
			caCertFile = "fixtures/green-certs/ca.crt"
			serverCertFile = "fixtures/green-certs/server.crt"
			serverKeyFile = "fixtures/green-certs/server.key"

			auctioneerConfig.CACertFile = caCertFile
			auctioneerConfig.ServerCertFile = serverCertFile
			auctioneerConfig.ServerKeyFile = serverKeyFile
		})

		JustBeforeEach(func() {
			auctioneerProcess = ifrit.Background(runner)
		})

		AfterEach(func() {
			ginkgomon.Interrupt(auctioneerProcess)
		})

		Context("when invalid values for the certificates are supplied", func() {
			BeforeEach(func() {
				auctioneerConfig.CACertFile = caCertFile
				auctioneerConfig.ServerCertFile = "invalid-certs/server.crt"
				auctioneerConfig.ServerKeyFile = serverKeyFile
			})

			It("fails", func() {
				Eventually(runner.Buffer()).Should(gbytes.Say(
					"invalid-tls-config"))
				Eventually(runner.ExitCode()).ShouldNot(Equal(0))
			})
		})

		Context("when invalid combinations of the certificates are supplied", func() {
			Context("when the server cert file isn't specified", func() {
				BeforeEach(func() {
					auctioneerConfig.CACertFile = caCertFile
					auctioneerConfig.ServerCertFile = ""
					auctioneerConfig.ServerKeyFile = serverKeyFile
				})

				It("fails", func() {
					Eventually(runner.Buffer()).Should(gbytes.Say(
						"invalid-tls-config"))
					Eventually(runner.ExitCode()).ShouldNot(Equal(0))
				})
			})

			Context("when the server cert file and server key file aren't specified", func() {
				BeforeEach(func() {
					auctioneerConfig.CACertFile = caCertFile
					auctioneerConfig.ServerCertFile = ""
					auctioneerConfig.ServerKeyFile = ""
				})

				It("fails", func() {
					Eventually(runner.Buffer()).Should(gbytes.Say(
						"invalid-tls-config"))
					Eventually(runner.ExitCode()).ShouldNot(Equal(0))
				})
			})

			Context("when the server key file isn't specified", func() {
				BeforeEach(func() {
					auctioneerConfig.CACertFile = caCertFile
					auctioneerConfig.ServerCertFile = serverCertFile
					auctioneerConfig.ServerKeyFile = ""
				})

				It("fails", func() {
					Eventually(runner.Buffer()).Should(gbytes.Say(
						"invalid-tls-config"))
					Eventually(runner.ExitCode()).ShouldNot(Equal(0))
				})
			})
		})

		Context("when the server key and the CA cert don't match", func() {
			BeforeEach(func() {
				auctioneerConfig.CACertFile = caCertFile
				auctioneerConfig.ServerCertFile = serverCertFile
				auctioneerConfig.ServerKeyFile = "fixtures/blue-certs/server.key"
			})

			It("fails", func() {
				Eventually(runner.Buffer()).Should(gbytes.Say(
					"invalid-tls-config"))
				Eventually(runner.ExitCode()).ShouldNot(Equal(0))
			})
		})

		Context("when correct TLS options are supplied", func() {
			It("starts", func() {
				Eventually(auctioneerProcess.Ready()).Should(BeClosed())
				Consistently(runner).ShouldNot(Exit())
			})

			It("responds successfully to a TLS client", func() {
				Eventually(auctioneerProcess.Ready()).Should(BeClosed())

				secureAuctioneerClient, err := auctioneer.NewSecureClient("https://"+auctioneerLocation, caCertFile, serverCertFile, serverKeyFile, false, defaultAuctioneerClientRequestTimeout)
				Expect(err).NotTo(HaveOccurred())

				err = secureAuctioneerClient.RequestLRPAuctions(logger, "", nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Context("Auctioneer Client", func() {
		var client auctioneer.Client

		JustBeforeEach(func() {
			auctioneerProcess = ginkgomon.Invoke(runner)
		})

		Context("when the auctioneer is configured with TLS", func() {
			BeforeEach(func() {
				auctioneerConfig.CACertFile = "fixtures/green-certs/ca.crt"
				auctioneerConfig.ServerCertFile = "fixtures/green-certs/server.crt"
				auctioneerConfig.ServerKeyFile = "fixtures/green-certs/server.key"
			})

			Context("and the auctioneer client is not configured with TLS", func() {
				BeforeEach(func() {
					client = auctioneer.NewClient("http://"+auctioneerLocation, defaultAuctioneerClientRequestTimeout)
				})

				It("does not work", func() {
					err := client.RequestLRPAuctions(logger, "", []*auctioneer.LRPStartRequest{})
					Expect(err).To(HaveOccurred())

					err = client.RequestTaskAuctions(logger, "", []*auctioneer.TaskStartRequest{})
					Expect(err).To(HaveOccurred())
				})
			})

			Context("and the auctioneer client is configured with tls", func() {
				BeforeEach(func() {
					var err error
					client, err = auctioneer.NewSecureClient(
						"https://"+auctioneerLocation,
						"fixtures/green-certs/ca.crt",
						"fixtures/green-certs/client.crt",
						"fixtures/green-certs/client.key",
						true,
						defaultAuctioneerClientRequestTimeout,
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("works", func() {
					err := client.RequestLRPAuctions(logger, "", []*auctioneer.LRPStartRequest{})
					Expect(err).NotTo(HaveOccurred())

					err = client.RequestTaskAuctions(logger, "", []*auctioneer.TaskStartRequest{})
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Context("when the auctioneer is not configured with TLS", func() {
			Context("and the auctioneer client is not configured with TLS", func() {
				BeforeEach(func() {
					client = auctioneer.NewClient("http://"+auctioneerLocation, defaultAuctioneerClientRequestTimeout)
				})

				It("works", func() {
					err := client.RequestLRPAuctions(logger, "", []*auctioneer.LRPStartRequest{})
					Expect(err).NotTo(HaveOccurred())

					err = client.RequestTaskAuctions(logger, "", []*auctioneer.TaskStartRequest{})
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("and the auctioneer client is configured with TLS", func() {
				Context("and the client requires tls", func() {
					BeforeEach(func() {
						var err error
						client, err = auctioneer.NewSecureClient(
							"https://"+auctioneerLocation,
							"fixtures/green-certs/ca.crt",
							"fixtures/green-certs/client.crt",
							"fixtures/green-certs/client.key",
							true,
							defaultAuctioneerClientRequestTimeout,
						)
						Expect(err).NotTo(HaveOccurred())
					})

					It("does not work", func() {
						err := client.RequestLRPAuctions(logger, "", []*auctioneer.LRPStartRequest{})
						Expect(err).To(HaveOccurred())

						err = client.RequestTaskAuctions(logger, "", []*auctioneer.TaskStartRequest{})
						Expect(err).To(HaveOccurred())
					})
				})

				Context("and the client does not require tls", func() {
					BeforeEach(func() {
						var err error
						client, err = auctioneer.NewSecureClient(
							"https://"+auctioneerLocation,
							"fixtures/green-certs/ca.crt",
							"fixtures/green-certs/client.crt",
							"fixtures/green-certs/client.key",
							false,
							defaultAuctioneerClientRequestTimeout,
						)
						Expect(err).NotTo(HaveOccurred())
					})

					It("falls back to http and does work", func() {
						err := client.RequestLRPAuctions(logger, "", []*auctioneer.LRPStartRequest{})
						Expect(err).NotTo(HaveOccurred())

						err = client.RequestTaskAuctions(logger, "", []*auctioneer.TaskStartRequest{})
						Expect(err).NotTo(HaveOccurred())
					})
				})
			})
		})
	})
})
