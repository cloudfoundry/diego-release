package cell_test

import (
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	archive_helper "code.cloudfoundry.org/archiver/extractor/test_helper"
	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	logging "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"code.cloudfoundry.org/inigo/fixtures"
	"code.cloudfoundry.org/inigo/helpers"
	"code.cloudfoundry.org/inigo/helpers/certauthority"
	"code.cloudfoundry.org/inigo/world"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/localip"
	"code.cloudfoundry.org/rep/cmd/rep/config"

	"crypto/tls"
	"crypto/x509"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/matchers"
	"github.com/onsi/gomega/types"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/tedsuo/ifrit/grouper"
)

const GraceBusyboxImageURL = "docker:///cloudfoundry/grace"

var _ = Describe("InstanceIdentity", func() {
	var (
		validityPeriod                              time.Duration
		cellProcess                                 ifrit.Process
		fileServerStaticDir                         string
		intermediateCACertPath, intermediateKeyPath string
		rootCAs                                     *x509.CertPool
		client                                      http.Client
		lrp                                         *models.DesiredLRP
		processGUID                                 string
		organizationalUnit                          []string
		rep, fileServer, metronAgent                ifrit.Runner
		logger                                      lager.Logger
		configRepCerts                              func(cfg *config.RepConfig)
	)

	BeforeEach(func() {
		// We can only do one OrganizationalUnit at the moment until go1.8
		// Make this 2 organizational units after we update to go1.8
		// https://github.com/golang/go/issues/18654
		organizationalUnit = []string{"jim:radical"}

		var err error
		credDir := world.TempDirWithParent(suiteTempDir, "instance-creds")

		certAuthority, err := certauthority.NewCertAuthority(credDir, "ca-with-no-max-path-length")
		Expect(err).NotTo(HaveOccurred())
		_, caCertPath := certAuthority.CAAndKey()

		intermediateKeyPath, intermediateCACertPath, err = certAuthority.GenerateSelfSignedCertAndKey("instance-identity", []string{"instance-identity"}, true)
		Expect(err).NotTo(HaveOccurred())
		caCertContent, err := os.ReadFile(caCertPath)
		Expect(err).NotTo(HaveOccurred())
		caCert := parseCertificate(caCertContent, true)
		rootCAs = x509.NewCertPool()
		rootCAs.AddCert(caCert)

		validityPeriod = time.Minute

		configRepCerts = func(cfg *config.RepConfig) {
			cfg.InstanceIdentityCredDir = credDir
			cfg.InstanceIdentityCAPath = intermediateCACertPath
			cfg.InstanceIdentityPrivateKeyPath = intermediateKeyPath
			cfg.InstanceIdentityValidityPeriod = durationjson.Duration(validityPeriod)
		}

		client = http.Client{}
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
				RootCAs:            rootCAs,
			},
		}

		processGUID = helpers.GenerateGuid()
		lrp = helpers.DefaultDeclaritiveHealthcheckLRPCreateRequest(componentMaker.Addresses(), processGUID, "log-guid", 1)
		lrp.Setup = nil
		lrp.CachedDependencies = []*models.CachedDependency{{
			From:      fmt.Sprintf("http://%s/v1/static/%s", componentMaker.Addresses().FileServer, "lrp.zip"),
			To:        "/tmp/diego",
			Name:      "lrp bits",
			CacheKey:  "lrp-cache-key",
			LogSource: "APP",
		}}
		lrp.Ports = []uint32{8080, 8081}
		lrp.Action = models.WrapAction(&models.RunAction{
			User: "vcap",
			Path: "/tmp/diego/go-server",
			Env: []*models.EnvironmentVariable{
				{Name: "PORT", Value: "8080"},
				{Name: "HTTPS_PORT", Value: "8081"},
			},
		})
		lrp.CertificateProperties = &models.CertificateProperties{
			OrganizationalUnit: organizationalUnit,
		}

		fileServer, fileServerStaticDir = componentMaker.FileServer()
		archiveFiles := fixtures.GoServerApp()
		archive_helper.CreateTarGZArchive(filepath.Join(fileServerStaticDir, "lrp.tgz"), archiveFiles)
		archive_helper.CreateZipArchive(
			filepath.Join(fileServerStaticDir, "lrp.zip"),
			archiveFiles,
		)

		rep = componentMaker.Rep(configRepCerts)
		logger = lagertest.NewTestLogger("metron-agent")
		metronAgent = ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)
			<-signals
			logger.Info("signaled")
			return nil
		})
	})

	JustBeforeEach(func() {
		cellGroup := grouper.Members{
			{Name: "metron-agent", Runner: metronAgent},
			{Name: "file-server", Runner: fileServer},
			{Name: "rep", Runner: rep},
			{Name: "auctioneer", Runner: componentMaker.Auctioneer()},
		}
		terminationSignal := os.Interrupt
		if runtime.GOOS == "windows" {
			terminationSignal = os.Kill
		}
		cellProcess = ginkgomon.Invoke(grouper.NewParallel(terminationSignal, cellGroup))

		Eventually(func() (models.CellSet, error) { return bbsServiceClient.Cells(lgr) }).Should(HaveLen(1))
	})

	AfterEach(func() {
		helpers.StopProcesses(cellProcess)
	})

	verifyCertAndKey := func(data []byte, organizationalUnit []string) {
		block, rest := pem.Decode(data)
		Expect(rest).NotTo(BeEmpty())
		Expect(block).NotTo(BeNil())
		containerCert := block.Bytes

		// skip the intermediate cert which is concatenated to the container cert
		block, rest = pem.Decode(rest)
		Expect(block).NotTo(BeNil())

		block, rest = pem.Decode(rest)
		Expect(rest).To(BeEmpty())
		Expect(block).NotTo(BeNil())
		containerKey := block.Bytes

		By("verify the certificate is signed properly")
		cert := parseCertificate(containerCert, false)
		Expect(cert.Subject.OrganizationalUnit).To(Equal(organizationalUnit))
		Expect(cert.NotAfter.Sub(cert.NotBefore)).To(Equal(validityPeriod))

		caCertContent, err := os.ReadFile(intermediateCACertPath)
		Expect(err).NotTo(HaveOccurred())

		caCert := parseCertificate(caCertContent, true)
		verifyCertificateIsSignedBy(cert, caCert)

		By("verify the private key matches the cert public key")
		key, err := x509.ParsePKCS1PrivateKey(containerKey)
		Expect(err).NotTo(HaveOccurred())
		Expect(&key.PublicKey).To(Equal(cert.PublicKey))
	}

	verifyCertAndKeyArePresentForTask := func(certPath, keyPath string, organizationalUnit []string) {
		By("running the task and getting the concatenated pem cert and key")
		var commandTemplate string
		if runtime.GOOS == "windows" {
			commandTemplate = "cat %s,%s"
		} else {
			commandTemplate = "cat %s %s"
		}
		result := runTaskAndGetCommandOutput(fmt.Sprintf(commandTemplate, certPath, keyPath), organizationalUnit)
		verifyCertAndKey([]byte(result), organizationalUnit)
	}

	verifyCertAndKeyArePresentForLRP := func(ipAddress string, organizationalUnit []string) {
		resp, err := client.Get(fmt.Sprintf("https://%s:8081/cf-instance-cert", ipAddress))
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		certData, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())

		resp, err = client.Get(fmt.Sprintf("https://%s:8081/cf-instance-key", ipAddress))
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		keyData, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())

		data := append(certData, keyData...)
		verifyCertAndKey(data, organizationalUnit)
	}

	Context("tasks", func() {
		It("should add instance identity certificate and key in the right location", func() {
			verifyCertAndKeyArePresentForTask("/etc/cf-instance-credentials/instance.crt", "/etc/cf-instance-credentials/instance.key", organizationalUnit)
		})

		It("should add instance identity environment variables to the container", func() {
			var instanceCertVarName, instanceKeyVarName string
			if runtime.GOOS == "windows" {
				instanceCertVarName = "$env:CF_INSTANCE_CERT"
				instanceKeyVarName = "$env:CF_INSTANCE_KEY"
			} else {
				instanceCertVarName = "$CF_INSTANCE_CERT"
				instanceKeyVarName = "$CF_INSTANCE_KEY"
			}
			verifyCertAndKeyArePresentForTask(instanceCertVarName, instanceKeyVarName, organizationalUnit)
		})
	})

	Context("lrps", func() {
		var address, ipAddress string

		JustBeforeEach(func() {
			err := bbsClient.DesireLRP(lgr, "", lrp)
			Expect(err).NotTo(HaveOccurred())
			Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGUID, nil)).Should(Equal(models.ActualLRPStateRunning))

			address = getContainerInternalAddress(bbsClient, processGUID, 8081, false)
			ipAddress, _, err = net.SplitHostPort(address)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should add instance identity certificate and key in the right location", func() {
			verifyCertAndKeyArePresentForLRP(ipAddress, organizationalUnit)
		})

		It("does not write container proxy config files", func() {
			resp, err := client.Get(fmt.Sprintf("https://%s:8081/cat?file=/etc/cf-assets/envoy_config/envoy.yaml", ipAddress))
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	Context("when a server uses the provided cert and key", func() {
		var address string

		JustBeforeEach(func() {
			err := bbsClient.DesireLRP(lgr, "", lrp)
			Expect(err).NotTo(HaveOccurred())
			Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGUID, nil)).Should(Equal(models.ActualLRPStateRunning))

			address = getContainerInternalAddress(bbsClient, processGUID, 8081, false)
		})

		Context("and a client app tries to connect using the root ca cert", func() {
			It("successfully connects and verify the sever identity", func() {
				resp, err := client.Get(fmt.Sprintf("https://%s/env", address))
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				ipAddress, _, err := net.SplitHostPort(address)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(body)).To(ContainSubstring("CF_INSTANCE_INTERNAL_IP=" + ipAddress))
			})
		})
	})

	Context("when a server has client authentication enabled using the root CA", func() {
		var (
			url string
		)

		JustBeforeEach(func() {
			server := ghttp.NewUnstartedServer()
			server.HTTPTestServer.TLS = &tls.Config{
				ClientCAs:  rootCAs,
				ClientAuth: tls.RequireAndVerifyClientCert,
			}
			ipAddress, err := localip.LocalIP()
			Expect(err).NotTo(HaveOccurred())
			listener, err := net.Listen("tcp4", ipAddress+":0")
			Expect(err).NotTo(HaveOccurred())
			server.AppendHandlers(ghttp.RespondWith(http.StatusOK, "hello world"))
			server.HTTPTestServer.Listener = listener
			server.HTTPTestServer.StartTLS()
			url = server.Addr()
		})

		Context("and a client app tries to connect to the server using the instance identity cert", func() {
			var (
				output string
			)

			JustBeforeEach(func() {
				if runtime.GOOS == "windows" {
					Skip("unable to find the equivalant command in windows even after using curl.exe (see https://github.com/curl/curl/issues/2262)")
				}
				output = runTaskAndGetCommandOutput(fmt.Sprintf("curl --silent -k --cert /etc/cf-instance-credentials/instance.crt --key /etc/cf-instance-credentials/instance.key https://%s", url), []string{})
			})

			It("successfully connects", func() {
				Expect(output).To(ContainSubstring("hello world"))
			})
		})
	})

	Context("when running with envoy proxy", func() {
		var (
			address              string
			enableContainerProxy func(cfg *config.RepConfig)
			loggregatorConfig    func(cfg *config.RepConfig)
			testIngressServer    *testhelpers.TestIngressServer
		)

		BeforeEach(func() {

			enableContainerProxy = func(config *config.RepConfig) {
				config.EnableContainerProxy = true
				config.EnvoyConfigRefreshDelay = durationjson.Duration(time.Second)
				config.ContainerProxyPath = filepath.Dir(os.Getenv("PROXY_BINARY"))

				envoyConfigDir := world.TempDirWithParent(suiteTempDir, "envoy_config")

				config.ContainerProxyConfigPath = envoyConfigDir
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

			loggregatorConfig = func(cfg *config.RepConfig) {
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

			rep = componentMaker.Rep(configRepCerts, enableContainerProxy, loggregatorConfig)

			logger := lagertest.NewTestLogger("metron-agent")
			metronAgent = ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
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
		})

		AfterEach(func() {
			testIngressServer.Stop()
		})

		connect := func() error {
			resp, err := client.Get(fmt.Sprintf("https://%s/env", address))
			if err != nil {
				return fmt.Errorf("Get returned error: %s", err.Error())
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return errors.New("not statusok")
			}

			return nil
		}

		Context("and the app starts successfully", func() {
			JustBeforeEach(func() {
				err := bbsClient.DesireLRP(lgr, "", lrp)
				Expect(err).NotTo(HaveOccurred())
				Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGUID, nil)).Should(Equal(models.ActualLRPStateRunning))

				address = getContainerInternalAddress(bbsClient, processGUID, 8080, true)
			})

			Context("when an invalid cipher is used", func() {
				BeforeEach(func() {
					client.Transport = &http.Transport{
						TLSClientConfig: &tls.Config{
							InsecureSkipVerify: false,
							RootCAs:            rootCAs,
							CipherSuites:       []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256},
							MaxVersion:         tls.VersionTLS12,
						},
					}
				})

				It("should fail", func() {
					Eventually(connect, 10*time.Second).Should(MatchError(ContainSubstring("tls: handshake failure")))
				})
			})

			It("should have a container with envoy enabled on it", func() {
				Eventually(connect, 10*time.Second).Should(Succeed())
			})

			Context("when rep is configured for mutual tls", func() {
				var (
					caCertContent   []byte
					mutualTLSConfig func(*config.RepConfig)
				)

				BeforeEach(func() {
					serverCaCertPath, err := filepath.Abs("../fixtures/certs/server/ca.crt")
					Expect(err).NotTo(HaveOccurred())
					serverCert, err := filepath.Abs("../fixtures/certs/server/server.crt")
					Expect(err).NotTo(HaveOccurred())
					serverKey, err := filepath.Abs("../fixtures/certs/server/server.key")
					Expect(err).NotTo(HaveOccurred())
					caCertContent, err = os.ReadFile(serverCaCertPath)
					Expect(err).NotTo(HaveOccurred())

					tlsCert, err := tls.LoadX509KeyPair(string(serverCert), string(serverKey))
					Expect(err).NotTo(HaveOccurred())

					client.Transport = &http.Transport{
						TLSClientConfig: &tls.Config{
							InsecureSkipVerify: false,
							RootCAs:            rootCAs,
							Certificates:       []tls.Certificate{tlsCert},
						},
					}

					mutualTLSConfig = func(cfg *config.RepConfig) {
						cfg.ContainerProxyTrustedCACerts = []string{string(caCertContent)}
						cfg.ContainerProxyRequireClientCerts = true
					}

					rep = componentMaker.Rep(configRepCerts, enableContainerProxy, loggregatorConfig, mutualTLSConfig)
				})

				Context("with subject_altname", func() {
					Context("when the subject_alt_name is missing", func() {
						BeforeEach(func() {
							noAltNameConfig := func(cfg *config.RepConfig) {
								cfg.ContainerProxyVerifySubjectAltName = []string{}
							}
							rep = componentMaker.Rep(configRepCerts, enableContainerProxy, loggregatorConfig, mutualTLSConfig, noAltNameConfig)
						})

						It("should connect successfully", func() {
							Eventually(connect, 10*time.Second).Should(Succeed())
						})
					})

					Context("with the wrong subject_alt_name", func() {
						BeforeEach(func() {
							if runtime.GOOS == "windows" {
								Skip("subject alt name verification is not yet supported on windows")
							}
							wrongAltNameConfig := func(cfg *config.RepConfig) {
								cfg.ContainerProxyVerifySubjectAltName = []string{"random-subject-name"}
							}
							rep = componentMaker.Rep(configRepCerts, enableContainerProxy, loggregatorConfig, mutualTLSConfig, wrongAltNameConfig)
						})

						It("should fail to connect", func() {
							Eventually(connect, 10*time.Second).Should(MatchError(ContainSubstring("tls: unknown certificate")))
						})
					})

					Context("with the correct subject_alt_name", func() {
						BeforeEach(func() {
							correctAltNameConfig := func(cfg *config.RepConfig) {
								cfg.ContainerProxyVerifySubjectAltName = []string{"gorouter.cf.service.internal"}
							}
							rep = componentMaker.Rep(configRepCerts, enableContainerProxy, loggregatorConfig, mutualTLSConfig, correctAltNameConfig)
						})

						It("should connect successfully", func() {
							Eventually(connect, 10*time.Second).Should(Succeed())
						})
					})
				})

				Context("when the envoy configured ca cert is wrong", func() {
					BeforeEach(func() {
						if runtime.GOOS == "windows" {
							Skip("not supported with envoy-nginx on windows")
						}
						serverCaCertPath, err := filepath.Abs("../fixtures/certs/server/wrong-ca.crt")
						Expect(err).NotTo(HaveOccurred())
						caCertContent, err = os.ReadFile(serverCaCertPath)
						Expect(err).NotTo(HaveOccurred())

						mutualTLSConfig = func(cfg *config.RepConfig) {
							cfg.ContainerProxyTrustedCACerts = []string{string(caCertContent)}
							cfg.ContainerProxyRequireClientCerts = true
						}
						rep = componentMaker.Rep(configRepCerts, enableContainerProxy, loggregatorConfig, mutualTLSConfig)
					})

					It("should fail to connect with the wrong server cert", func() {
						Eventually(connect, 10*time.Second).Should(MatchError(ContainSubstring("tls: certificate required")))
					})
				})

				Context("when the client has the wrong cert for mutual tls", func() {
					BeforeEach(func() {
						if runtime.GOOS == "windows" {
							Skip("not supported with envoy-nginx on windows")
						}
						wrongServerCert, err := filepath.Abs("../fixtures/certs/server/wrong-server.crt")
						Expect(err).NotTo(HaveOccurred())
						wrongServerKey, err := filepath.Abs("../fixtures/certs/server/wrong-server.key")
						Expect(err).NotTo(HaveOccurred())
						wrongTlsCert, err := tls.LoadX509KeyPair(string(wrongServerCert), string(wrongServerKey))
						Expect(err).NotTo(HaveOccurred())

						client.Transport = &http.Transport{
							TLSClientConfig: &tls.Config{
								InsecureSkipVerify: false,
								RootCAs:            rootCAs,
								Certificates:       []tls.Certificate{wrongTlsCert},
							},
						}
					})

					It("should fail to connect", func() {
						Eventually(connect, 10*time.Second).Should(MatchError(ContainSubstring("tls: certificate required")))
					})
				})
			})

			Context("when the app listens on the container interface and does not listen on localhost", func() {
				BeforeEach(func() {
					lrp.EnvironmentVariables = append(lrp.EnvironmentVariables, &models.EnvironmentVariable{
						Name:  "SKIP_LOCALHOST_LISTEN",
						Value: "true",
					})
					lrp.CheckDefinition = nil
					if runtime.GOOS == "windows" {
						cmd := `$ErrorActionPreference = 'Stop'; trap { $host.SetShouldExit(1) }; $result=(Test-NetConnection -ComputerName "$env:CF_INSTANCE_INTERNAL_IP" -Port 8080).TcpTestSucceeded; if ($result) { exit 0 } else { exit 1}`
						lrp.Monitor = models.WrapAction(&models.RunAction{
							User: "vcap",
							Path: "powershell",
							Args: []string{"-command", cmd},
						})
					} else {
						lrp.Monitor = models.WrapAction(&models.RunAction{
							User: "vcap",
							Path: "sh",
							Args: []string{"-c", "nc -z $CF_INSTANCE_INTERNAL_IP 8080"},
						})
					}

				})

				It("should have a container with envoy enabled on it", func() {
					Eventually(connect, 10*time.Second).Should(Succeed())
				})
			})

			Context("when the container uses a docker image", func() {
				BeforeEach(func() {
					if runtime.GOOS == "windows" {
						Skip("docker image is not yet supported for windows")
					}
					lrp = helpers.DockerLRPCreateRequest(componentMaker.Addresses(), processGUID)
				})

				It("should have a container with envoy enabled on it", func() {
					Eventually(connect, 10*time.Second).Should(Succeed())
				})

				Context("and the app ignores SIGTERM", func() {
					BeforeEach(func() {
						lrp.RootFs = GraceBusyboxImageURL
						lrp.Monitor = nil
						lrp.Setup = nil
						lrp.Action = models.WrapAction(&models.RunAction{
							Path: "/grace",
							User: "root",
							Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
							Args: []string{"-catchTerminate"},
						})
					})

					Context("and is killed", func() {
						JustBeforeEach(func() {
							Eventually(connect, 10*time.Second).Should(Succeed())
							err := bbsClient.RemoveDesiredLRP(lgr, "", lrp.ProcessGuid)
							Expect(err).NotTo(HaveOccurred())
						})

						It("continues to serve traffic", func() {
							Consistently(connect, 5*time.Second).Should(Succeed())
						})
					})
				})
			})

			Context("when the container is privileged", func() {
				BeforeEach(func() {
					if runtime.GOOS == "windows" {
						Skip("privileged containers is not supported on windows")
					}

					lrp.Privileged = true
				})

				It("should have a container with envoy enabled on it", func() {
					Eventually(connect, 10*time.Second).Should(Succeed())
				})
			})

			Context("when the container uses OCI preloaded rootfs", func() {
				BeforeEach(func() {
					if runtime.GOOS == "windows" {
						Skip("OCI mode is not supported on windows")
					}
					lrp.CachedDependencies = nil
					layer := fmt.Sprintf("http://%s/v1/static/%s", componentMaker.Addresses().FileServer, "lrp.tgz")
					lrp.RootFs = "preloaded+layer:" + world.DefaultStack + "?layer=" + layer + "&layer_path=/" + "&layer_digest="
					lrp.Action = models.WrapAction(&models.RunAction{
						User: "vcap",
						Path: "/go-server",
						Env: []*models.EnvironmentVariable{
							{Name: "PORT", Value: "8080"},
							{Name: "HTTPS_PORT", Value: "8081"},
						},
					})
				})

				It("should have a container with envoy enabled on it", func() {
					Eventually(connect, 10*time.Second).Should(Succeed())
				})
			})

			Context("when certs are rotated", func() {
				var credRotationPeriod = 64 * time.Second

				BeforeEach(func() {
					alterCredRotation := func(config *config.RepConfig) {
						config.InstanceIdentityValidityPeriod = durationjson.Duration(credRotationPeriod)
					}

					rep = componentMaker.Rep(configRepCerts, enableContainerProxy, loggregatorConfig, alterCredRotation)
				})

				It("should be able to reconnect with the updated certs", func() {
					Eventually(connect).Should(Succeed())
					Consistently(connect, 90*time.Second, 20*time.Millisecond).Should(Succeed())
				})
			})

			Context("when the additional memory allocation is defined", func() {
				var (
					container                garden.Container
					setProxyMemoryAllocation func(cfg *config.RepConfig)
					containerMutex           sync.Mutex
				)

				BeforeEach(func() {
					containerMutex.Lock()
					defer containerMutex.Unlock()
					container = nil

					if runtime.GOOS == "windows" {
						lrp.MemoryMb = 564
					} else {
						lrp.MemoryMb = 64
					}
					setProxyMemoryAllocation = func(config *config.RepConfig) {
						config.ProxyMemoryAllocationMB = 5
					}

					rep = componentMaker.Rep(configRepCerts, enableContainerProxy, setProxyMemoryAllocation)
				})

				JustBeforeEach(func() {
					containerMutex.Lock()
					defer containerMutex.Unlock()
					actualLRPs, err := bbsClient.ActualLRPs(logger, "", models.ActualLRPFilter{ProcessGuid: processGUID})
					Expect(err).NotTo(HaveOccurred())
					actualLRP := actualLRPs[0]
					container, err = gardenClient.Lookup(actualLRP.ActualLrpInstanceKey.InstanceGuid)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when emitting app metrics", func() {
					var (
						metricsChan chan map[string]uint64
						memoryLimit uint64
					)

					BeforeEach(func() {
						metricsChan = make(chan map[string]uint64, 10)
						memoryLimit = uint64(lrp.MemoryMb)

						metronAgent = ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
							close(ready)
							testMetricsChan, signalMetricsChan := testhelpers.TestMetricChan(testIngressServer.Receivers())
							defer close(signalMetricsChan)
							for {
								select {
								case envelope := <-testMetricsChan:
									metric := getContainerMetricEnvelope(logger, envelope)
									if metric == nil {
										continue
									}

									// lock to avoid the annoying race detector errors
									containerMutex.Lock()
									c := container
									containerMutex.Unlock()
									actualMemoryUsage := getContainerMemoryUsage(logger, c)
									if actualMemoryUsage == nil {
										continue
									}

									metricsChan <- map[string]uint64{
										"memory":        uint64(metric.Metrics["memory"].Value),
										"actual_memory": *actualMemoryUsage,
										"memory_quota":  uint64(metric.Metrics["memory_quota"].Value),
									}
								case <-signals:
									return nil
								}
							}
						})

						rep = componentMaker.Rep(
							configRepCerts,
							enableContainerProxy, setProxyMemoryAllocation,
							loggregatorConfig,
						)

						lrp.Action.RunAction.Args = []string{"-allocate-memory-mb=30"}
					})

					It("should receive rescaled memory usage", func() {
						Eventually(metricsChan, 10*time.Second).Should(Receive(scaledDownMemory(memoryLimit, 5)))
					})

					It("should receive rescaled memory limit", func() {
						Eventually(metricsChan, 10*time.Second).Should(Receive(HaveKeyWithValue("memory_quota", memoryInBytes(memoryLimit))))
					})

					Context("when additional memory is set and the LRP has unlimited memory", func() {
						BeforeEach(func() {
							lrp.MemoryMb = 0
						})

						It("should have unlimited memory", func() {
							Eventually(metricsChan, 10*time.Second).Should(Receive(unlimitedMemory()))
						})
					})

					Context("when additional memory is set but container proxy is not enabled", func() {
						BeforeEach(func() {
							if runtime.GOOS == "windows" {
								Skip("TODO: use buildpack LRP. docker lrp is not supported on windows")
							}
							rep = componentMaker.Rep(
								configRepCerts,
								setProxyMemoryAllocation,
								loggregatorConfig,
							)

							lrp = helpers.DockerLRPCreateRequest(componentMaker.Addresses(), processGUID)
							lrp.MemoryMb = int32(memoryLimit)
						})

						It("should not scale the memory usage", func() {
							Eventually(metricsChan, 10*time.Second).Should(Receive(unscaledDownMemory()))
						})

						It("should receive the right memory limit", func() {
							Eventually(metricsChan, 10*time.Second).Should(Receive(HaveKeyWithValue("memory_quota", memoryInBytes(memoryLimit))))
						})
					})

					Context("when the lrp is using docker rootfs", func() {
						BeforeEach(func() {
							if runtime.GOOS == "windows" {
								Skip("TODO: use buildpack LRP. docker lrp is not supported on windows")
							}
							lrp = helpers.DockerLRPCreateRequest(componentMaker.Addresses(), processGUID)
							lrp.MemoryMb = int32(memoryLimit)
						})

						It("should receive rescaled memory limit", func() {
							Eventually(metricsChan, 10*time.Second).Should(Receive(scaledDownMemory(memoryLimit, 5)))
						})

						It("should receive the rescaled memory usage", func() {
							Eventually(metricsChan, 10*time.Second).Should(Receive(HaveKeyWithValue("memory_quota", memoryInBytes(memoryLimit))))
						})
					})
				})
			})
		})

		Context("and envoy takes longer to start", func() {
			var sleepyEnvoyDir string

			BeforeEach(func() {
				if runtime.GOOS == "windows" {
					Skip("TODO: figure out a way to create .exe or .bat file that emulates the slep behavior in windows")
				}
				sleepyEnvoyDir = createSleepyEnvoy(suiteTempDir)

				setSleepEnvoy := func(config *config.RepConfig) {
					config.ContainerProxyPath = sleepyEnvoyDir
				}

				enableDeclarativeHealthChecks := func(config *config.RepConfig) {
					config.EnableDeclarativeHealthcheck = true
					config.DeclarativeHealthCheckDefaultTimeout = durationjson.Duration(1 * time.Second)
					config.DeclarativeHealthcheckPath = componentMaker.Artifacts().Healthcheck
					config.HealthCheckWorkPoolSize = 1
				}

				rep = componentMaker.Rep(configRepCerts, enableContainerProxy, setSleepEnvoy, loggregatorConfig, enableDeclarativeHealthChecks)
			})

			JustBeforeEach(func() {
				err := bbsClient.DesireLRP(lgr, "", lrp)
				Expect(err).NotTo(HaveOccurred())
			})

			envoyIsHealthChecked := func() {
				It("should be marked running only when both envoy and the app are available", func() {
					Eventually(helpers.LRPStatePoller(lgr, bbsClient, processGUID, nil)).Should(Equal(models.ActualLRPStateRunning))
					address = getContainerInternalAddress(bbsClient, processGUID, 8080, true)

					Consistently(connect).Should(Succeed())
				})

				Context("and envoy startup time exceeds the start timeout", func() {
					BeforeEach(func() {
						lrp.StartTimeoutMs = 3000
					})

					It("crashes the lrp with a descriptive error", func() {
						Eventually(func() *models.ActualLRP {
							index := int32(0)
							lrps, err := bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: processGUID, Index: &index})
							Expect(err).NotTo(HaveOccurred())
							Expect(len(lrps)).To(Equal(1))
							return lrps[0]
						}).Should(gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
							"CrashReason": MatchRegexp("Instance never healthy after .*: instance proxy failed to start"),
						})))
					})
				})
			}

			envoyIsHealthChecked()

			Context("when the lrp uses declarative healthchecks", func() {
				BeforeEach(func() {
					lrp = helpers.DefaultDeclaritiveHealthcheckLRPCreateRequest(componentMaker.Addresses(), processGUID, "log-guid", 1)
				})

				envoyIsHealthChecked()
			})
		})
	})
})

func unscaledDownMemory() types.GomegaMatcher {
	someNonZeroValue := uint64(1) // otherwise the scaledDownMemory will attempt to divide by 0
	return scaledDownMemory(someNonZeroValue, 0)
}

func unlimitedMemory() types.GomegaMatcher {
	const maxMemory uint64 = math.MaxUint64
	transform := func(m map[string]uint64) uint64 {
		return m["memory_quota"]
	}
	// Note: the memory_quota returned by the garden.Container is very near MaxUnit64,
	// but due to being converted to and from JSON and float64 lose precision in an
	// non-deterministic manner.  So we simply check that the memory_quota is within
	// 5% of MaxUint64.
	return And(
		HaveKey("memory_quota"),
		matchers.NewWithTransformMatcher(transform, BeNumerically("~", maxMemory, maxMemory/20)),
	)
}

func scaledDownMemory(memoryLimit, proxyAllocatedMemory uint64) types.GomegaMatcher {
	return And(
		HaveKey("memory"),
		HaveKey("actual_memory"),
		matchers.NewWithTransformMatcher(func(m map[string]uint64) int64 {
			reportedMemory := int64(m["memory"])
			actualMemoryUsage := m["actual_memory"]
			scaledDownActualMemroy := int64(actualMemoryUsage * memoryLimit / (memoryLimit + proxyAllocatedMemory))
			return reportedMemory - scaledDownActualMemroy
		}, BeNumerically("~", 0, 102400)),
	)
}

func memoryInBytes(memoryMb uint64) uint64 {
	return memoryMb * 1024 * 1024
}

func getContainerInternalAddress(client bbs.Client, processGuid string, port uint32, tls bool) string {
	By("getting the internal ip address of the container")
	lrps, err := client.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: processGuid})
	Expect(err).NotTo(HaveOccurred())
	Expect(lrps).To(HaveLen(1))
	netInfo := lrps[0].ActualLrpNetInfo
	address := netInfo.InstanceAddress
	for _, mapping := range netInfo.Ports {
		if mapping.ContainerPort == port {
			if tls {
				return address + ":" + strconv.Itoa(int(mapping.ContainerTlsProxyPort))
			}
			return address + ":" + strconv.Itoa(int(mapping.ContainerPort))
		}
	}
	Fail("cannot find port mapping for port "+strconv.Itoa(int(port)), 1)
	panic("unreachable")
}

func runTaskAndGetCommandOutput(command string, organizationalUnits []string) string {
	guid := helpers.GenerateGuid()

	var (
		shell      string
		args       []string
		resultFile string
	)

	if runtime.GOOS == "windows" {
		shell = "powershell"
		args = []string{"-Command", fmt.Sprintf("%s | Set-Content -Encoding Ascii -Path thingy", command)}
		resultFile = "/Users/vcap/thingy"
	} else {
		shell = "sh"
		args = []string{"-c", fmt.Sprintf("%s > thingy", command)}
		resultFile = "/home/vcap/thingy"
	}

	expectedTask := helpers.TaskCreateRequestWithCertificateProperties(
		guid,
		&models.RunAction{
			User: "vcap",
			Path: shell,
			Args: args,
		},
		&models.CertificateProperties{
			OrganizationalUnit: organizationalUnits,
		},
	)
	expectedTask.TaskDefinition.ResultFile = resultFile

	err := bbsClient.DesireTask(lgr, "", expectedTask.TaskGuid, expectedTask.Domain, expectedTask.TaskDefinition)
	Expect(err).NotTo(HaveOccurred())

	var task *models.Task
	Eventually(func() interface{} {
		var err error

		task, err = bbsClient.TaskByGuid(lgr, "", guid)
		Expect(err).NotTo(HaveOccurred())

		return task.State
	}).Should(Equal(models.Task_Completed))

	Expect(task.Failed).To(BeFalse(), "Task Should've succeeded")

	return task.Result
}

func parseCertificate(cert []byte, pemEncoded bool) *x509.Certificate {
	if pemEncoded {
		block, _ := pem.Decode(cert)
		Expect(block).NotTo(BeNil())
		cert = block.Bytes
	}
	certs, err := x509.ParseCertificates(cert)
	Expect(err).NotTo(HaveOccurred())
	Expect(certs).To(HaveLen(1))
	return certs[0]
}

func verifyCertificateIsSignedBy(cert, parentCert *x509.Certificate) {
	certPool := x509.NewCertPool()
	certPool.AddCert(parentCert)
	certs, err := cert.Verify(x509.VerifyOptions{
		Roots: certPool,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(certs).To(HaveLen(1))
	Expect(certs[0]).To(ContainElement(parentCert))
}

func createSleepyEnvoy(parentDir string) string {
	envoyPath := os.Getenv("PROXY_BINARY")

	dir := world.TempDirWithParent(parentDir, "sleepy-envoy")

	copyFile := func(dst, src string) {
		dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		Expect(err).NotTo(HaveOccurred())
		defer dstFile.Close()
		srcFile, err := os.Open(src)
		Expect(err).NotTo(HaveOccurred())
		defer srcFile.Close()
		_, err = io.Copy(dstFile, srcFile)
		Expect(err).NotTo(HaveOccurred())
	}

	copyFile(filepath.Join(dir, "orig_envoy"), envoyPath)

	newEnvoy, err := os.OpenFile(filepath.Join(dir, "envoy"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	Expect(err).NotTo(HaveOccurred())
	defer newEnvoy.Close()
	fmt.Fprintf(newEnvoy, `#!/usr/bin/env bash
sleep 5
dir=$(dirname $0)
exec $dir/orig_envoy "$@"
`)
	return dir
}

func getContainerMetricEnvelope(logger lager.Logger, envelope *loggregator_v2.Envelope) *loggregator_v2.Gauge {
	if envelope.GetInstanceId() == "" {
		// not app metrics
		logger.Info("no-instance-id")
		return nil
	}

	metric := envelope.GetGauge()
	if metric == nil {
		// not app metrics
		logger.Info("no-envelop-gauge")
		return nil
	}

	if _, ok := metric.Metrics["memory"]; !ok {
		// this is the separate garden cpu entitlement, container age gauge.
		return nil
	}

	return metric
}

func getContainerMemoryUsage(logger lager.Logger, c garden.Container) *uint64 {
	if c == nil {
		// container can be nil if we get a container metric while
		// the JustBeforeEach is still getting the actual lrp and
		// container infromation
		logger.Info("container-is-not-set")
		return nil
	}

	metrics, err := c.Metrics()
	if err != nil {
		// do not return the error since garden will initially
		logger.Info("no-container-metrics")
		return nil
	}
	stats := metrics.MemoryStat
	actualMemoryUsage := stats.TotalRss + stats.TotalCache - stats.TotalInactiveFile
	return &actualMemoryUsage
}
