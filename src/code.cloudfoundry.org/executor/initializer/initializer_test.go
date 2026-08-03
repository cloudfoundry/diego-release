package initializer_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"code.cloudfoundry.org/cacheddownloader"

	"code.cloudfoundry.org/clock/fakeclock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/containerstore"
	"code.cloudfoundry.org/executor/depot/containerstore/containerstorefakes"
	"code.cloudfoundry.org/executor/gardenhealth"
	"code.cloudfoundry.org/executor/initializer"
	"code.cloudfoundry.org/executor/initializer/configuration"
	"code.cloudfoundry.org/executor/initializer/fakes"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/go-loggregator/v9"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Initializer", func() {
	const StalledGardenDuration = "StalledGardenDuration"

	var (
		initialTime      time.Time
		fakeGarden       *ghttp.Server
		fakeClock        *fakeclock.FakeClock
		errCh            chan error
		done             chan struct{}
		config           initializer.ExecutorConfig
		logger           lager.Logger
		fakeMetronClient *mfakes.FakeIngressClient
		metricMap        map[string]time.Duration
		mutex            sync.RWMutex
	)

	BeforeEach(func() {
		initialTime = time.Now()
		fakeGarden = ghttp.NewUnstartedServer()
		fakeClock = fakeclock.NewFakeClock(initialTime)
		errCh = make(chan error, 1)
		done = make(chan struct{})
		logger = lagertest.NewTestLogger("test")

		fakeGarden.RouteToHandler("GET", "/ping", ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{}))
		fakeGarden.RouteToHandler("GET", "/containers", ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{}))
		fakeGarden.RouteToHandler("GET", "/containers/bulk_metrics", ghttp.RespondWithJSONEncoded(http.StatusOK,
			garden.ContainerMetricsEntry{}))
		fakeGarden.RouteToHandler("GET", "/capacity", ghttp.RespondWithJSONEncoded(http.StatusOK,
			garden.Capacity{MemoryInBytes: 1024 * 1024 * 1024, DiskInBytes: 20 * 1048 * 1024 * 1024, MaxContainers: 4}))
		fakeGarden.RouteToHandler("GET", "/containers/bulk_info", ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{}))
		config = initializer.ExecutorConfig{
			AdvertisePreferenceForInstanceAddress: false,
			AutoDiskOverheadMB:                    1,
			CachePath:                             fmt.Sprintf("%s-%d", "/tmp/cache", GinkgoParallelProcess()),
			ContainerInodeLimit:                   200000,
			ContainerMaxCpuShares:                 0,
			ContainerMetricsReportInterval:        durationjson.Duration(15 * time.Second),
			ContainerOwnerName:                    "executor",
			ContainerProxyADSServers:              []string{"10.0.0.2:15010"},
			ContainerReapInterval:                 durationjson.Duration(time.Minute),
			CreateWorkPoolSize:                    32,
			DeleteWorkPoolSize:                    32,
			DiskMB:                                configuration.Automatic,
			EnableContainerProxy:                  false,
			GardenAddr:                            "/tmp/garden.sock",
			GardenHealthcheckCommandRetryPause:    durationjson.Duration(1 * time.Second),
			GardenHealthcheckEmissionInterval:     durationjson.Duration(30 * time.Second),
			GardenHealthcheckInterval:             durationjson.Duration(10 * time.Minute),
			GardenHealthcheckProcessArgs:          []string{},
			GardenHealthcheckProcessEnv:           []string{},
			GardenHealthcheckTimeout:              durationjson.Duration(10 * time.Minute),
			GardenNetwork:                         "unix",
			GracefulShutdownInterval:              durationjson.Duration(1 * time.Second),
			HealthCheckContainerOwnerName:         "executor-health-check",
			HealthCheckWorkPoolSize:               64,
			HealthyMonitoringInterval:             durationjson.Duration(30 * time.Second),
			MaxCacheSizeInBytes:                   10 * 1024 * 1024 * 1024,
			MaxConcurrentDownloads:                5,
			MaxLogLinesPerSecond:                  200,
			MemoryMB:                              configuration.Automatic,
			MetricsWorkPoolSize:                   8,
			ReadWorkPoolSize:                      64,
			ReservedExpirationTime:                durationjson.Duration(time.Minute),
			SkipCertVerify:                        false,
			TempDir:                               "/tmp",
			UnhealthyMonitoringInterval:           durationjson.Duration(500 * time.Millisecond),
			VolmanDriverPaths:                     "/tmpvolman1:/tmp/volman2",
		}

		fakeMetronClient = new(mfakes.FakeIngressClient)

		mutex = sync.RWMutex{}
	})

	AfterEach(func() {
		Eventually(done, 10*time.Second).Should(BeClosed())
		fakeGarden.Close()
	})

	getMetrics := func() map[string]time.Duration {
		mutex.Lock()
		defer mutex.Unlock()
		m := make(map[string]time.Duration, len(metricMap))
		for k, v := range metricMap {
			m[k] = v
		}
		return m
	}

	JustBeforeEach(func() {
		config.GardenAddr = fakeGarden.HTTPTestServer.Listener.Addr().String()
		config.GardenNetwork = "tcp"
		go func() {
			rootFSes := map[string]string{}
			sidecarRootFSPath := ""
			_, _, _, err := initializer.Initialize(logger, config, "cell-id", "some-zone", rootFSes, sidecarRootFSPath, fakeMetronClient, fakeClock)
			errCh <- err
			close(done)
		}()

		metricMap = make(map[string]time.Duration)
		fakeMetronClient.SendDurationStub = func(name string, time time.Duration, opts ...loggregator.EmitGaugeOption) error {
			mutex.Lock()
			metricMap[name] = time
			mutex.Unlock()
			return nil
		}

		fakeGarden.Start()
	})

	Context("when garden doesn't respond", func() {
		var waitChan chan struct{}

		BeforeEach(func() {
			waitChan = make(chan struct{})
			fakeGarden.RouteToHandler("GET", "/ping", func(w http.ResponseWriter, req *http.Request) {
				<-waitChan
				ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{})(w, req)
			})
		})

		AfterEach(func() {
			close(waitChan)
		})

		It("emits metrics when garden doesn't respond", func() {
			Consistently(getMetrics, 10*time.Millisecond).ShouldNot(HaveKey(StalledGardenDuration))

			fakeClock.WaitForWatcherAndIncrement(initializer.StalledMetricHeartbeatInterval)
			Eventually(fakeMetronClient.SendDurationCallCount).Should(Equal(1))

			Eventually(getMetrics).Should(HaveKeyWithValue(StalledGardenDuration, fakeClock.Since(initialTime)))
		})
	})

	Context("when garden responds", func() {
		It("emits 0", func() {
			Eventually(fakeMetronClient.SendDurationCallCount).Should(Equal(1))

			Eventually(getMetrics).Should(HaveKeyWithValue(StalledGardenDuration, BeEquivalentTo(0)))

			Consistently(errCh).ShouldNot(Receive(HaveOccurred()))
		})
	})

	Context("when there are leftover containers while initializing", func() {
		BeforeEach(func() {
			fakeGarden.RouteToHandler("GET", "/containers",
				func(w http.ResponseWriter, r *http.Request) {
					r.ParseForm()
					gardenState := r.URL.Query()["garden.state"]
					Expect(gardenState).To(HaveLen(1))
					Expect(gardenState[0]).To(Equal("all"))
					healthcheckTagQueryParam := gardenhealth.HealthcheckTag
					if r.FormValue(healthcheckTagQueryParam) == gardenhealth.HealthcheckTagValue {
						ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{})(w, r)
					} else {
						ghttp.RespondWithJSONEncoded(http.StatusOK, map[string][]string{"handles": {"cnr1", "cnr2"}})(w, r)
					}
				},
			)
		})

		Context("when containers are deleted successfully", func() {
			var deleteChan, doneChan chan struct{}

			BeforeEach(func() {
				deleteChan = make(chan struct{}, 2)
				doneChan = make(chan struct{})
				fakeGarden.RouteToHandler("DELETE", "/containers/cnr1",
					ghttp.CombineHandlers(
						func(http.ResponseWriter, *http.Request) {
							deleteChan <- struct{}{}
							<-doneChan
						},
						ghttp.RespondWithJSONEncoded(http.StatusOK, &struct{}{})))
				fakeGarden.RouteToHandler("DELETE", "/containers/cnr2",
					ghttp.CombineHandlers(
						func(http.ResponseWriter, *http.Request) {
							deleteChan <- struct{}{}
							<-doneChan
						},
						ghttp.RespondWithJSONEncoded(http.StatusOK, &struct{}{})))
			})

			AfterEach(func() {
				close(doneChan)
			})

			It("should delete them concurrently", func() {
				Eventually(func() int {
					return len(deleteChan)
				}).Should(Equal(2))
			})

			Context("when the number of containers exceeds the number of deletion workers", func() {
				BeforeEach(func() {
					config.DeleteWorkPoolSize = 1
				})

				It("should only delete size of deletion pool containers at a time", func() {
					Eventually(func() int {
						return len(deleteChan)
					}).Should(Equal(1))
					Consistently(func() int {
						return len(deleteChan)
					}).Should(Equal(1))

					doneChan <- struct{}{}

					Eventually(func() int {
						return len(deleteChan)
					}).Should(Equal(2))
				})
			})
		})

		Context("when garden fails to delete leftover containers", func() {
			BeforeEach(func() {
				fakeGarden.RouteToHandler(
					"DELETE",
					"/containers/cnr1",
					ghttp.RespondWith(http.StatusInternalServerError, ""),
				)
				fakeGarden.RouteToHandler(
					"DELETE",
					"/containers/cnr2",
					ghttp.RespondWithJSONEncoded(http.StatusOK, &struct{}{}),
				)
			})

			It("should fail the initializer", func() {
				Eventually(errCh).Should(Receive(HaveOccurred()))
			})
		})
	})

	Context("when garden responds with an error", func() {
		var retried chan struct{}

		BeforeEach(func() {
			callCount := 0
			retried = make(chan struct{})
			fakeGarden.RouteToHandler("GET", "/ping", func(w http.ResponseWriter, req *http.Request) {
				callCount++
				if callCount == 1 {
					ghttp.RespondWith(http.StatusInternalServerError, "")(w, req)
				} else if callCount == 2 {
					ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{})(w, req)
					close(retried)
				}
			})
		})

		It("retries on a timer until it succeeds", func() {
			Consistently(retried).ShouldNot(BeClosed())
			fakeClock.Increment(initializer.PingGardenInterval)
			Eventually(retried).Should(BeClosed())
		})

		It("emits zero once it succeeds", func() {
			Consistently(getMetrics).ShouldNot(HaveKey(StalledGardenDuration))

			fakeClock.Increment(initializer.PingGardenInterval)
			Eventually(fakeMetronClient.SendDurationCallCount).Should(Equal(1))

			Eventually(getMetrics).Should(HaveKeyWithValue(StalledGardenDuration, BeEquivalentTo(0)))
		})

		Context("when the error is unrecoverable", func() {
			BeforeEach(func() {
				fakeGarden.RouteToHandler(
					"GET",
					"/ping",
					ghttp.RespondWith(http.StatusGatewayTimeout, `{ "Type": "UnrecoverableError" , "Message": "Extra Special Error Message"}`),
				)
			})

			It("returns an error", func() {
				Eventually(errCh).Should(Receive(BeAssignableToTypeOf(garden.UnrecoverableError{})))
			})
		})
	})

	Context("when the post setup hook is invalid", func() {
		BeforeEach(func() {
			config.PostSetupHook = "unescaped quote\\"
		})

		It("fails fast", func() {
			Eventually(errCh).Should(Receive(MatchError("EOF found after escape character")))
		})
	})

	Describe("with the TLS configuration", func() {
		Context("when the TLS config is valid", func() {
			BeforeEach(func() {
				config.PathToTLSCert = "fixtures/downloader/client.crt"
				config.PathToTLSKey = "fixtures/downloader/client.key"
				config.PathToTLSCACert = "fixtures/downloader/ca.crt"
			})

			It("uses the certs for the uploader and cacheddownloader", func() {
				// not really an easy way to check this at this layer -- inigo
				// let's just check that our validation passes
				Consistently(errCh).ShouldNot(Receive(HaveOccurred()))
			})

			Context("when no CA cert is provided", func() {
				BeforeEach(func() {
					config.PathToTLSCACert = ""
				})

				It("still passes validation", func() {
					Consistently(errCh).ShouldNot(Receive(HaveOccurred()))
				})
			})

			Context("when a CA cert is provided, but no keypair", func() {
				BeforeEach(func() {
					config.PathToTLSCert = ""
					config.PathToTLSKey = ""
				})

				It("passes still passes validation", func() {
					Consistently(errCh).ShouldNot(Receive(HaveOccurred()))
				})
			})
		})

		Context("when the certs are invalid", func() {
			BeforeEach(func() {
				config.PathToTLSCert = "fixtures/ca-certs-invalid"
				config.PathToTLSKey = "fixtures/downloader/client.key"
				config.PathToTLSCACert = "fixtures/downloader/ca.crt"
			})

			It("fails", func() {
				Eventually(errCh).Should(Receive(MatchError(ContainSubstring("failed to find any PEM data in certificate input"))))
			})

			Context("when the cert is missing", func() {
				BeforeEach(func() {
					config.PathToTLSCert = ""
				})

				It("fails", func() {
					Eventually(errCh).Should(Receive(MatchError(ContainSubstring("The TLS certificate or key is missing"))))
				})
			})

			Context("when the key is missing", func() {
				BeforeEach(func() {
					config.PathToTLSKey = ""
				})

				It("fails", func() {
					Eventually(errCh).Should(Receive(MatchError(ContainSubstring("The TLS certificate or key is missing"))))
				})
			})
		})

		Context("when the TLS properties are missing", func() {
			It("succeeds", func() {
				// not really an easy way to check this at this layer -- inigo
				// let's just check that our validation passes
				Consistently(errCh).ShouldNot(Receive(HaveOccurred()))
			})
		})
	})

	Describe("configuring trusted CA bundle", func() {
		Context("when valid", func() {
			BeforeEach(func() {
				config.PathToCACertsForDownloads = "fixtures/ca-certs"
			})

			It("uses it for the cached downloader", func() {
				// not really an easy way to check this at this layer -- inigo
				// let's just check that our validation passes
				Consistently(errCh).ShouldNot(Receive(HaveOccurred()))
			})

			Context("when the cert bundle has extra leading and trailing spaces", func() {
				BeforeEach(func() {
					config.PathToCACertsForDownloads = "fixtures/ca-certs-with-spaces.crt"
				})

				It("does not error", func() {
					Consistently(errCh).ShouldNot(Receive(HaveOccurred()))
				})
			})

			Context("when the cert bundle is empty", func() {
				BeforeEach(func() {
					config.PathToCACertsForDownloads = "fixtures/ca-certs-empty"
				})

				It("does not error", func() {
					Consistently(errCh).ShouldNot(Receive(HaveOccurred()))
				})
			})
		})

		Context("when certs are invalid", func() {
			BeforeEach(func() {
				config.PathToCACertsForDownloads = "fixtures/ca-certs-invalid"
			})

			It("fails", func() {
				Eventually(errCh, 2*time.Second).Should(Receive(MatchError("unable to load CA certificate")))
			})
		})

		Context("when path is invalid", func() {
			BeforeEach(func() {
				config.PathToCACertsForDownloads = "sandwich"
			})

			It("fails", func() {
				Eventually(errCh).Should(Receive(MatchError("Unable to open CA cert bundle 'sandwich'")))
			})
		})
	})

	Describe("TLSConfigFromConfig", func() {
		var (
			tlsConfig             *tls.Config
			caCert                *x509.Certificate
			tlsClientCert         tls.Certificate
			fakeCertPoolRetriever *fakes.FakeCertPoolRetriever
			err                   error
			logger                *lagertest.TestLogger
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("executor")
			fakeCertPoolRetriever = &fakes.FakeCertPoolRetriever{}
			config.PathToTLSCert = "fixtures/downloader/client.crt"
			config.PathToTLSKey = "fixtures/downloader/client.key"
			config.PathToTLSCACert = "fixtures/downloader/ca.crt"

			fakeCertPoolRetriever.SystemCertsReturns(x509.NewCertPool(), nil)

			certBytes, err := os.ReadFile(config.PathToTLSCACert)
			Expect(err).NotTo(HaveOccurred())
			block, _ := pem.Decode(certBytes)
			caCert, err = x509.ParseCertificate(block.Bytes)
			Expect(err).NotTo(HaveOccurred())

			tlsClientCert, err = tls.LoadX509KeyPair(config.PathToTLSCert, config.PathToTLSKey)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns a valid mutual TLS config", func() {
			tlsConfig, err = initializer.TLSConfigFromConfig(logger, fakeCertPoolRetriever, config)
			Expect(err).To(Succeed())
			Expect(tlsConfig).NotTo(BeNil())
			Expect(tlsConfig.MinVersion).To(BeEquivalentTo(tls.VersionTLS12))
			Expect(tlsConfig.InsecureSkipVerify).To(Equal(config.SkipCertVerify))
			Expect(tlsConfig.Certificates).To(ContainElement(tlsClientCert))
			//lint:ignore SA1019 - ignoring tlsCert.RootCAs.Subjects is deprecated ERR because cert does not come from SystemCertPool.
			Expect(tlsConfig.RootCAs.Subjects()).To(ContainElement(caCert.RawSubject))
		})

		It("adds any system certs to the CA pools", func() {
			certBytes, err := os.ReadFile("fixtures/systemcerts/extra-ca.crt")
			Expect(err).NotTo(HaveOccurred())
			block, _ := pem.Decode(certBytes)
			caCert, err = x509.ParseCertificate(block.Bytes)
			Expect(err).NotTo(HaveOccurred())

			systemCAs := x509.NewCertPool()
			ok := systemCAs.AppendCertsFromPEM(certBytes)
			Expect(ok).To(BeTrue())
			fakeCertPoolRetriever.SystemCertsReturns(systemCAs, nil)

			tlsConfig, err = initializer.TLSConfigFromConfig(logger, fakeCertPoolRetriever, config)
			Expect(err).To(Succeed())
			Expect(tlsConfig).NotTo(BeNil())

			Expect(fakeCertPoolRetriever.SystemCertsCallCount()).To(Equal(1))
			//lint:ignore SA1019 - ignoring tlsCert.RootCAs.Subjects is deprecated ERR because cert does not come from SystemCertPool.
			Expect(tlsConfig.RootCAs.Subjects()).To(ContainElement(caCert.RawSubject))
		})

		Context("when the cert pool retriever fails", func() {
			BeforeEach(func() {
				fakeCertPoolRetriever.SystemCertsReturns(nil, errors.New("failed retrieving certs"))
			})

			It("errors", func() {
				tlsConfig, err = initializer.TLSConfigFromConfig(logger, fakeCertPoolRetriever, config)
				Expect(err).To(MatchError("failed retrieving certs"))
				Expect(tlsConfig).To(BeNil())
			})
		})

		It("does not restrict the cipher suites", func() {
			tlsConfig, err = initializer.TLSConfigFromConfig(logger, fakeCertPoolRetriever, config)
			Expect(err).To(Succeed())
			Expect(tlsConfig.CipherSuites).To(BeNil())
		})

		Context("when mutual is using PathToCACertsForDownloads", func() {
			BeforeEach(func() {
				config.PathToTLSCACert = ""
				config.PathToCACertsForDownloads = "fixtures/downloader/ca.crt"

				certBytes, err := os.ReadFile(config.PathToCACertsForDownloads)
				Expect(err).NotTo(HaveOccurred())
				block, _ := pem.Decode(certBytes)
				caCert, err = x509.ParseCertificate(block.Bytes)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a valid mutual TLS config", func() {
				tlsConfig, err = initializer.TLSConfigFromConfig(logger, fakeCertPoolRetriever, config)
				Expect(err).To(Succeed())
				Expect(tlsConfig).NotTo(BeNil())
				//lint:ignore SA1019 - ignoring tlsCert.RootCAs.Subjects is deprecated ERR because cert does not come from SystemCertPool.
				Expect(tlsConfig.RootCAs.Subjects()).To(ContainElement(caCert.RawSubject))
			})
		})

		Context("when tls cert and key are not configured", func() {
			BeforeEach(func() {
				logger = lagertest.NewTestLogger("executor")
				config.PathToTLSKey = ""
				config.PathToTLSCert = ""
			})

			It("returns a valid, non-mutual TLS enabled, config", func() {
				tlsConfig, err = initializer.TLSConfigFromConfig(logger, fakeCertPoolRetriever, config)
				Expect(err).To(Succeed())
				Expect(tlsConfig).NotTo(BeNil())
				//lint:ignore SA1019 - ignoring tlsCert.RootCAs.Subjects is deprecated ERR because cert does not come from SystemCertPool.
				Expect(tlsConfig.RootCAs.Subjects()).To(ContainElement(caCert.RawSubject))
			})
		})
	})

	Describe("CredManagerFromConfig", func() {
		var credManager containerstore.CredManager
		var err error
		var container executor.Container
		var logger *lagertest.TestLogger

		JustBeforeEach(func() {
			logger = lagertest.NewTestLogger("executor")
			container = executor.Container{
				Guid: "1234",
			}

			mounts := []garden.BindMount{
				{
					Origin:  garden.BindMountOriginHost,
					SrcPath: "some-path",
					DstPath: "/etc/cf-assets/envoy",
				},
			}
			fakeCredHandler := &containerstorefakes.FakeCredentialHandler{}
			fakeCredHandler.CreateDirReturns(mounts, nil, nil)
			credManager, err = initializer.CredManagerFromConfig(logger, fakeMetronClient, config, fakeClock, fakeCredHandler)
		})

		Describe("when instance identity creds directory is not set", func() {
			BeforeEach(func() {
				config.InstanceIdentityCredDir = ""
			})

			It("returns a noop credential manager", func() {
				bindMounts, _, err := credManager.CreateCredDir(logger, container)
				Expect(bindMounts).To(BeEmpty())
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("when the instance identity creds directory is set", func() {
			BeforeEach(func() {
				config.InstanceIdentityCredDir = "fixtures/instance-id/"
				config.InstanceIdentityCAPath = "fixtures/instance-id/ca.crt"
				config.InstanceIdentityPrivateKeyPath = "fixtures/instance-id/ca.key"
				config.InstanceIdentityValidityPeriod = durationjson.Duration(1 * time.Minute)
			})

			It("returns a credential manager", func() {
				bindMounts, _, err := credManager.CreateCredDir(logger, container)
				defer os.RemoveAll(filepath.Join(config.InstanceIdentityCredDir, container.Guid))
				Expect(err).NotTo(HaveOccurred())
				Expect(bindMounts).NotTo(BeEmpty())
			})

			Context("when the private key does not exist", func() {
				BeforeEach(func() {
					config.InstanceIdentityPrivateKeyPath = "fixtures/instance-id/notexist.key"
				})

				It("fails", func() {
					Eventually(os.IsNotExist(err)).Should(BeTrue(), "Private key does not exist")
				})
			})

			Context("when the private key is not PEM-encoded", func() {
				BeforeEach(func() {
					config.InstanceIdentityPrivateKeyPath = "fixtures/instance-id/non-pem.key"
				})

				It("fails", func() {
					Eventually(err).Should(MatchError(ContainSubstring("instance ID key is not PEM-encoded")))
				})
			})

			Context("when the private key is invalid", func() {
				BeforeEach(func() {
					config.InstanceIdentityPrivateKeyPath = "fixtures/instance-id/invalid.key"
				})

				It("fails", func() {
					Eventually(err).Should(BeAssignableToTypeOf(asn1.StructuralError{}))
				})
			})

			Context("when the certificate does not exist", func() {
				BeforeEach(func() {
					config.InstanceIdentityCAPath = "fixtures/instance-id/notexist.crt"
				})

				It("fails", func() {
					Eventually(os.IsNotExist(err)).Should(BeTrue(), "Instance certificate does not exist")
				})
			})

			Context("when the certificate is not PEM-encoded", func() {
				BeforeEach(func() {
					config.InstanceIdentityCAPath = "fixtures/instance-id/non-pem.crt"
				})

				It("fails", func() {
					Eventually(err).Should(MatchError(ContainSubstring("instance ID CA is not PEM-encoded")))
				})
			})

			Context("when the certificate is invalid", func() {
				BeforeEach(func() {
					config.InstanceIdentityCAPath = "fixtures/instance-id/invalid.crt"
				})

				It("fails", func() {
					Eventually(err).Should(MatchError("x509: malformed tbs certificate"))
				})
			})

			Context("when the validity period is not set", func() {
				BeforeEach(func() {
					config.InstanceIdentityValidityPeriod = 0
				})

				It("fails", func() {
					Eventually(err).Should(MatchError(ContainSubstring("instance ID validity period needs to be set and positive")))
				})
			})
		})
	})

	Describe("CachedDownloader", func() {
		Context("when cacheddownloader.New receives a malformed cache.CachedPath", func() {
			var (
				logger                lager.Logger
				fakeCertPoolRetriever *fakes.FakeCertPoolRetriever
			)

			BeforeEach(func() {
				logger = lagertest.NewTestLogger("executor")
				fakeCertPoolRetriever = &fakes.FakeCertPoolRetriever{}
				config.PathToTLSCert = "fixtures/downloader/client.crt"
				config.PathToTLSKey = "fixtures/downloader/client.key"
				config.PathToTLSCACert = "fixtures/downloader/ca.crt"
				config.CachePath = ""

				fakeCertPoolRetriever.SystemCertsReturns(x509.NewCertPool(), nil)
			})
			It("returns an error", func() {
				certBytes, err := os.ReadFile(config.PathToTLSCACert)
				Expect(err).NotTo(HaveOccurred())
				block, _ := pem.Decode(certBytes)
				_, err = x509.ParseCertificate(block.Bytes)
				Expect(err).NotTo(HaveOccurred())

				tlsConfig, err := initializer.TLSConfigFromConfig(logger, fakeCertPoolRetriever, config)
				Expect(err).To(Succeed())
				Expect(tlsConfig).NotTo(BeNil())

				newDownloader := cacheddownloader.NewDownloader(10*time.Minute, math.MaxInt8, tlsConfig)
				newCache := cacheddownloader.NewCache(config.CachePath, int64(config.MaxCacheSizeInBytes), 0)

				newCachedDownloader, err := cacheddownloader.New(
					newDownloader,
					newCache,
					cacheddownloader.TarTransform,
				)

				Expect(newCachedDownloader).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("could not create cache path"))
			})
		})
	})
})
