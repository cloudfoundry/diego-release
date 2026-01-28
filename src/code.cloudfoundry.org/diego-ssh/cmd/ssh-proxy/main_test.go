package main_test

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/diego-ssh/authenticators"
	"code.cloudfoundry.org/diego-ssh/cmd/ssh-proxy/config"
	"code.cloudfoundry.org/diego-ssh/cmd/ssh-proxy/testrunner"
	sshdtestrunner "code.cloudfoundry.org/diego-ssh/cmd/sshd/testrunner"
	"code.cloudfoundry.org/diego-ssh/helpers"
	"code.cloudfoundry.org/diego-ssh/routes"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"code.cloudfoundry.org/inigo/helpers/certauthority"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/tlsconfig"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"
)

var _ = Describe("SSH proxy", Serial, func() {
	var (
		fakeBBS            *ghttp.Server
		fakeUAA            *ghttp.Server
		fakeCC             *ghttp.Server
		runner             ifrit.Runner
		process            ifrit.Process
		sshProxyConfig     *config.SSHProxyConfig
		sshProxyConfigPath string
		certDepoDir        string
		ca                 certauthority.CertAuthority

		address                     string
		healthCheckAddress          string
		diegoCredentials            string
		hostKeyFingerprint          string
		expectedGetActualLRPRequest *models.ActualLRPsRequest
		actualLRPsResponse          *models.ActualLRPsResponse
		getDesiredLRPRequest        *models.DesiredLRPByProcessGuidRequest
		desiredLRPResponse          *models.DesiredLRPResponse

		processGuid  string
		clientConfig *ssh.ClientConfig
	)

	BeforeEach(func() {
		var err error
		certDepoDir, err = os.MkdirTemp("", "ssh-proxy-certs-")
		Expect(err).NotTo(HaveOccurred())

		ca, err = certauthority.NewCertAuthority(certDepoDir, "ssh-proxy-ca")
		Expect(err).NotTo(HaveOccurred())

		serverKeyFile, serverCertFile, err := ca.GenerateSelfSignedCertAndKey("server", []string{}, false)
		Expect(err).NotTo(HaveOccurred())
		_, serverCAFile := ca.CAAndKey()

		fakeBBS = ghttp.NewUnstartedServer()
		fakeBBS.HTTPTestServer.TLS, err = tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(serverCertFile, serverKeyFile),
		).Server(tlsconfig.WithClientAuthenticationFromFile(serverCAFile))
		Expect(err).NotTo(HaveOccurred())
		fakeBBS.HTTPTestServer.StartTLS()

		fakeUAA = ghttp.NewUnstartedServer()
		fakeUAA.HTTPTestServer.TLS, err = tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(serverCertFile, serverKeyFile),
		).Server(tlsconfig.WithClientAuthenticationFromFile(serverCAFile))
		Expect(err).NotTo(HaveOccurred())
		fakeUAA.HTTPTestServer.TLS.ClientAuth = tls.NoClientCert
		fakeUAA.HTTPTestServer.StartTLS()

		fakeCC = ghttp.NewUnstartedServer()
		fakeCC.HTTPTestServer.TLS, err = tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(serverCertFile, serverKeyFile),
		).Server(tlsconfig.WithClientAuthenticationFromFile(serverCAFile))
		Expect(err).NotTo(HaveOccurred())
		fakeCC.HTTPTestServer.TLS.ClientAuth = tls.NoClientCert
		fakeCC.HTTPTestServer.StartTLS()

		privateKey, err := ssh.ParsePrivateKey([]byte(hostKeyPem))
		Expect(err).NotTo(HaveOccurred())
		hostKeyFingerprint = helpers.MD5Fingerprint(privateKey.PublicKey())

		address = fmt.Sprintf("127.0.0.1:%d", sshProxyPort)
		healthCheckAddress = fmt.Sprintf("127.0.0.1:%d", healthCheckProxyPort)
		diegoCredentials = "some-creds"
		processGuid = "app-guid-app-version"

		u, err := url.Parse(fakeUAA.URL())
		Expect(err).NotTo(HaveOccurred())

		u.Path = "/oauth/token"

		sshProxyConfig = &config.SSHProxyConfig{}
		sshProxyConfig.Address = address
		sshProxyConfig.HealthCheckAddress = healthCheckAddress
		sshProxyConfig.BBSAddress = fakeBBS.URL()
		sshProxyConfig.BBSCACert = serverCAFile
		sshProxyConfig.BBSClientCert = serverCertFile
		sshProxyConfig.BBSClientKey = serverKeyFile
		sshProxyConfig.CCAPIURL = fakeCC.URL()
		sshProxyConfig.CCAPICACert = serverCAFile
		sshProxyConfig.DiegoCredentials = diegoCredentials
		sshProxyConfig.EnableCFAuth = true
		sshProxyConfig.EnableDiegoAuth = true
		sshProxyConfig.HostKey = hostKeyPem
		sshProxyConfig.SkipCertVerify = false
		sshProxyConfig.UAATokenURL = u.String()
		sshProxyConfig.UAAPassword = "password1"
		sshProxyConfig.UAAUsername = "amandaplease"
		sshProxyConfig.UAACACert = serverCAFile
		sshProxyConfig.IdleConnectionTimeout = durationjson.Duration(500 * time.Millisecond)
		sshProxyConfig.CommunicationTimeout = durationjson.Duration(10 * time.Second)
		sshProxyConfig.ConnectToInstanceAddress = false
		sshProxyConfig.LagerConfig = lagerflags.DefaultLagerConfig()

		index := int32(99)
		expectedGetActualLRPRequest = &models.ActualLRPsRequest{
			ProcessGuid: processGuid,
			Index:       &index,
		}

		actualLRPsResponse = &models.ActualLRPsResponse{
			Error: nil,
			ActualLrps: []*models.ActualLRP{
				&models.ActualLRP{
					ActualLrpKey:         models.NewActualLRPKey(processGuid, 99, "some-domain"),
					ActualLrpInstanceKey: models.NewActualLRPInstanceKey("some-instance-guid", "some-cell-id"),
					ActualLrpNetInfo:     models.NewActualLRPNetInfo("127.0.0.1", "127.0.0.1", models.ActualLRPNetInfo_PreferredAddressUnknown, models.NewPortMappingWithTLSProxy(uint32(sshdPort), uint32(sshdContainerPort), uint32(sshdTLSPort), uint32(sshdContainerTLSPort))),
				},
			},
		}

		getDesiredLRPRequest = &models.DesiredLRPByProcessGuidRequest{
			ProcessGuid: processGuid,
		}

		sshRoute, err := json.Marshal(routes.SSHRoute{
			ContainerPort:   uint32(sshdContainerPort),
			PrivateKey:      privateKeyPem,
			HostFingerprint: hostKeyFingerprint,
		})
		Expect(err).NotTo(HaveOccurred())

		sshRouteMessage := json.RawMessage(sshRoute)
		desiredLRPResponse = &models.DesiredLRPResponse{
			Error: nil,
			DesiredLrp: &models.DesiredLRP{
				ProcessGuid: processGuid,
				Instances:   100,
				Routes:      &models.Routes{routes.DIEGO_SSH: &sshRouteMessage},
			},
		}

		clientConfig = &ssh.ClientConfig{}
	})

	JustBeforeEach(func() {
		protoActualLrpRequest := expectedGetActualLRPRequest.ToProto()
		// protoDesiredLrpRequest := getDesiredLRPRequest.ToProto()
		fakeBBS.RouteToHandler("POST", "/v1/actual_lrps/list", ghttp.CombineHandlers(
			ghttp.VerifyRequest("POST", "/v1/actual_lrps/list"),
			VerifyProto(protoActualLrpRequest),
			RespondWithProto(actualLRPsResponse.ToProto()),
		))
		fakeBBS.RouteToHandler("POST", "/v1/desired_lrps/get_by_process_guid.r3", ghttp.CombineHandlers(
			ghttp.VerifyRequest("POST", "/v1/desired_lrps/get_by_process_guid.r3"),
			VerifyProto(getDesiredLRPRequest.ToProto()),
			RespondWithProto(desiredLRPResponse.ToProto()),
		))

		configData, err := json.Marshal(&sshProxyConfig)
		Expect(err).NotTo(HaveOccurred())

		configFile, err := os.CreateTemp("", "ssh-proxy-config")
		Expect(err).NotTo(HaveOccurred())

		n, err := configFile.Write(configData)
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(len(configData)))

		sshProxyConfigPath = configFile.Name()

		err = configFile.Close()
		Expect(err).NotTo(HaveOccurred())

		runner = testrunner.New(sshProxyPath, sshProxyConfigPath)
		process = ifrit.Invoke(runner)
	})

	AfterEach(func() {
		ginkgomon.Kill(process, 3*time.Second)

		err := os.RemoveAll(sshProxyConfigPath)
		Expect(err).NotTo(HaveOccurred())

		Expect(os.RemoveAll(certDepoDir)).To(Succeed())

		fakeBBS.Close()
		fakeUAA.Close()
		fakeCC.Close()
	})

	Describe("argument validation", func() {
		Context("when the host key is not provided", func() {
			BeforeEach(func() {
				sshProxyConfig.HostKey = ""
			})

			It("reports the problem and terminates", func() {
				Expect(runner).To(gbytes.Say("hostKey is required"))
				Expect(runner).NotTo(gexec.Exit(0))
			})
		})

		Context("when an ill-formed host key is provided", func() {
			BeforeEach(func() {
				sshProxyConfig.HostKey = "host-key"
			})

			It("reports the problem and terminates", func() {
				Expect(runner).To(gbytes.Say("failed-to-parse-host-key"))
				Expect(runner).NotTo(gexec.Exit(0))
			})
		})

		Context("when the BBS address is missing", func() {
			BeforeEach(func() {
				sshProxyConfig.BBSAddress = ""
			})

			It("reports the problem and terminates", func() {
				Expect(runner).To(gbytes.Say("bbsAddress is required"))
				Expect(runner).NotTo(gexec.Exit(0))
			})
		})

		Context("when the BBS address cannot be parsed", func() {
			BeforeEach(func() {
				sshProxyConfig.BBSAddress = ":://goober-swallow#yuck"
			})

			It("reports the problem and terminates", func() {
				Expect(runner).To(gbytes.Say("failed-to-parse-bbs-address"))
				Expect(runner).NotTo(gexec.Exit(0))
			})
		})

		Context("when CF authentication is enabled", func() {
			BeforeEach(func() {
				sshProxyConfig.EnableCFAuth = true
			})

			Context("when the cc URL is missing", func() {
				BeforeEach(func() {
					sshProxyConfig.CCAPIURL = ""
				})

				It("reports the problem and terminates", func() {
					Expect(runner).To(gbytes.Say("ccAPIURL is required for Cloud Foundry authentication"))
					Expect(runner).NotTo(gexec.Exit(0))
				})
			})

			Context("when the cc URL cannot be parsed", func() {
				BeforeEach(func() {
					sshProxyConfig.CCAPIURL = ":://goober-swallow#yuck"
				})

				It("reports the problem and terminates", func() {
					Expect(runner).To(gbytes.Say("configure-failed"))
					Expect(runner).To(gexec.Exit(1))
				})
			})

			Context("when cc ca cert does not exist", func() {
				BeforeEach(func() {
					sshProxyConfig.CCAPICACert = "doesnotexist"
				})

				It("exits with an error", func() {
					Expect(runner).To(gbytes.Say("failed to read ca cert"))
					Expect(runner).To(gexec.Exit(1))
				})
			})

			Context("when the uaa URL is missing", func() {
				BeforeEach(func() {
					sshProxyConfig.UAATokenURL = ""
				})

				It("reports the problem and terminates", func() {
					Expect(runner).To(gbytes.Say("uaaTokenURL is required for Cloud Foundry authentication"))
					Expect(runner).To(gexec.Exit(1))
				})
			})

			Context("when the UAA password is missing", func() {
				BeforeEach(func() {
					sshProxyConfig.UAAPassword = ""
				})

				It("exits with an error", func() {
					Expect(runner).To(gbytes.Say("UAA password is required for Cloud Foundry authentication"))
					Expect(runner).To(gexec.Exit(1))
				})
			})

			Context("when the UAA username is missing", func() {
				BeforeEach(func() {
					sshProxyConfig.UAAUsername = ""
				})

				It("exits with an error", func() {
					Expect(runner).To(gbytes.Say("UAA username is required for Cloud Foundry authentication"))
					Expect(runner).To(gexec.Exit(1))
				})
			})

			Context("when the UAA URL cannot be parsed", func() {
				BeforeEach(func() {
					sshProxyConfig.UAATokenURL = ":://spitting#nickles"
				})

				It("reports the problem and terminates", func() {
					Expect(runner).To(gbytes.Say("configure-failed"))
					Expect(runner).To(gexec.Exit(1))
				})
			})

			Context("when UAA ca cert does not exist", func() {
				BeforeEach(func() {
					sshProxyConfig.UAACACert = "doesnotexist"
				})

				It("exits with an error", func() {
					Expect(runner).To(gbytes.Say("failed to read ca cert"))
					Expect(runner).To(gexec.Exit(1))
				})
			})
		})
	})

	It("presents the correct host key", func() {
		var handshakeHostKey ssh.PublicKey
		_, err := ssh.Dial("tcp", address, &ssh.ClientConfig{
			User: "user",
			Auth: []ssh.AuthMethod{ssh.Password("")},
			HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
				handshakeHostKey = key
				return errors.New("Short-circuit the handshake")
			},
		})
		Expect(err).To(HaveOccurred())

		proxyHostKey, err := ssh.ParsePrivateKey([]byte(hostKeyPem))
		Expect(err).NotTo(HaveOccurred())
		Expect(proxyHostKey.PublicKey().Marshal()).To(Equal(handshakeHostKey.Marshal()))
	})

	Describe("Disabled http healthcheck server", func() {
		BeforeEach(func() {
			sshProxyConfig.DisableHealthCheckServer = true
		})

		It("is not running the healthcheck process", func() {
			req, err := http.NewRequest("GET", "http://"+healthCheckAddress, nil)
			Expect(err).NotTo(HaveOccurred())
			_, err = http.DefaultClient.Do(req)
			e, ok := err.(net.Error)
			Expect(ok).To(BeTrue())
			Expect(e.Error()).To(MatchRegexp(".*connection refused"))
		})
	})

	Describe("http healthcheck server", func() {
		var (
			method, path string
			resp         *http.Response
		)

		JustBeforeEach(func() {
			req, err := http.NewRequest(method, "http://"+healthCheckAddress+path, nil)
			Expect(err).NotTo(HaveOccurred())
			resp, err = http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("valid requests", func() {
			BeforeEach(func() {
				method = "GET"
				path = "/"
			})

			It("returns 200", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			})
		})

		Context("invalid requests", func() {
			Context("invalid method", func() {
				BeforeEach(func() {
					method = "POST"
					path = "/"
				})

				It("returns 405", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusMethodNotAllowed))
				})
			})

			Context("invalid path", func() {
				BeforeEach(func() {
					method = "GET"
					path = "/foo/bar"
				})

				It("returns 404", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})
	})

	Describe("attempting authentication without a realm", func() {
		BeforeEach(func() {
			clientConfig = &ssh.ClientConfig{
				User:            processGuid + "/99",
				Auth:            []ssh.AuthMethod{ssh.Password(diegoCredentials)},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}
		})

		It("fails the authentication", func() {
			_, err := ssh.Dial("tcp", address, clientConfig)
			Expect(err).To(MatchError(ContainSubstring("ssh: handshake failed")))
			Expect(fakeBBS.ReceivedRequests()).To(HaveLen(0))
		})
	})

	Describe("attempting authentication with an unknown realm", func() {
		BeforeEach(func() {
			clientConfig = &ssh.ClientConfig{
				User:            "goo:" + processGuid + "/99",
				Auth:            []ssh.AuthMethod{ssh.Password(diegoCredentials)},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}
		})

		It("fails the authentication", func() {
			_, err := ssh.Dial("tcp", address, clientConfig)
			Expect(err).To(MatchError(ContainSubstring("ssh: handshake failed")))
			Expect(fakeBBS.ReceivedRequests()).To(HaveLen(0))
		})
	})

	Describe("authenticating with the diego realm", func() {
		var (
			intermediaryTLSConfig *tls.Config
			intermediaryListener  net.Listener
			connectedToTLS        chan struct{}
			forwardServer         *forwardTLSServer
		)

		BeforeEach(func() {
			clientConfig = &ssh.ClientConfig{
				User:            "diego:" + processGuid + "/99",
				Auth:            []ssh.AuthMethod{ssh.Password(diegoCredentials)},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}

			serverKeyFile, serverCertFile, err := ca.GenerateSelfSignedCertAndKey("server", []string{"some-instance-guid"}, false)
			Expect(err).NotTo(HaveOccurred())
			_, serverCAFile := ca.CAAndKey()

			intermediaryTLSConfig, err = tlsconfig.Build(
				tlsconfig.WithInternalServiceDefaults(),
				tlsconfig.WithIdentityFromFile(serverCertFile, serverKeyFile),
			).Server(tlsconfig.WithClientAuthenticationFromFile(serverCAFile))
			Expect(err).NotTo(HaveOccurred())

			intermediaryListener, err = tls.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", sshdTLSPort), intermediaryTLSConfig)
			Expect(err).NotTo(HaveOccurred())

			connectedToTLS = make(chan struct{}, 1)
			logger := lagertest.NewTestLogger("ssh-proxy-test")
			forwardServer = NewForwardTLSServer(logger, intermediaryListener, sshdAddress)
		})

		JustBeforeEach(func() {
			go forwardServer.Start(connectedToTLS)
		})

		AfterEach(func() {
			forwardServer.Stop()
			close(connectedToTLS)
		})

		It("acquires the desired and actual LRP info from the BBS", func() {
			client, err := ssh.Dial("tcp", address, clientConfig)
			Expect(err).NotTo(HaveOccurred())

			err = client.Close()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBBS.ReceivedRequests()).To(HaveLen(2))
		})

		It("connects to the target daemon", func() {
			client, err := ssh.Dial("tcp", address, clientConfig)
			Expect(err).NotTo(HaveOccurred())

			session, err := client.NewSession()
			Expect(err).NotTo(HaveOccurred())
			output, err := session.Output("echo -n hello")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(output)).To(Equal("hello"))
		})

		Context("when a tls intermediary is configured", func() {
			Context("when ssh-proxy is configured to connect to the intermediary", func() {
				BeforeEach(func() {
					sshProxyConfig.BackendsTLSEnabled = true
				})

				Context("when the tls handshake is via non-MTLS", func() {
					BeforeEach(func() {
						_, serverCAFile := ca.CAAndKey()
						sshProxyConfig.BackendsTLSCACerts = serverCAFile

						intermediaryTLSConfig.ClientAuth = tls.NoClientCert
					})

					It("connects to the target daemon using tls", func() {
						client, err := ssh.Dial("tcp", address, clientConfig)
						Expect(err).NotTo(HaveOccurred())
						Eventually(connectedToTLS).Should(Receive())

						_, err = client.NewSession()
						Expect(err).NotTo(HaveOccurred())
					})
				})

				Context("when the tls handshake is via MTLS", func() {
					BeforeEach(func() {
						serverKeyFile, serverCertFile, err := ca.GenerateSelfSignedCertAndKey("server", []string{}, false)
						Expect(err).NotTo(HaveOccurred())
						_, serverCAFile := ca.CAAndKey()

						sshProxyConfig.BackendsTLSCACerts = serverCAFile
						sshProxyConfig.BackendsTLSClientCert = serverCertFile
						sshProxyConfig.BackendsTLSClientKey = serverKeyFile

						intermediaryTLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
					})

					It("connects to the target daemon using MTLS", func() {
						client, err := ssh.Dial("tcp", address, clientConfig)
						Expect(err).NotTo(HaveOccurred())
						Eventually(connectedToTLS).Should(Receive())

						_, err = client.NewSession()
						Expect(err).NotTo(HaveOccurred())
					})
				})

				Context("when connecting using TLS fails", func() {
					BeforeEach(func() {
						// force TLS handshake to fail
						otherCA, err := certauthority.NewCertAuthority(certDepoDir, "other_server_ca")
						Expect(err).NotTo(HaveOccurred())

						_, otherCAFile := otherCA.CAAndKey()

						sshProxyConfig.BackendsTLSCACerts = otherCAFile

						intermediaryTLSConfig.ClientAuth = tls.NoClientCert
					})

					It("connects to the daemon without TLS", func() {
						client, err := ssh.Dial("tcp", address, clientConfig)
						Expect(err).NotTo(HaveOccurred())

						Consistently(connectedToTLS).ShouldNot(Receive())

						session, err := client.NewSession()
						Expect(err).NotTo(HaveOccurred())
						output, err := session.Output("echo -n hello")
						Expect(err).NotTo(HaveOccurred())
						Expect(string(output)).To(Equal("hello"))
					})
				})
			})

			Context("when ssh-proxy is NOT configured to connect to the intermediary", func() {
				BeforeEach(func() {
					sshProxyConfig.BackendsTLSEnabled = false
				})

				It("connects to the target daemon without using tls", func() {
					client, err := ssh.Dial("tcp", address, clientConfig)
					Expect(err).NotTo(HaveOccurred())

					Consistently(connectedToTLS).ShouldNot(Receive())

					session, err := client.NewSession()
					Expect(err).NotTo(HaveOccurred())
					output, err := session.Output("echo -n hello")
					Expect(err).NotTo(HaveOccurred())
					Expect(string(output)).To(Equal("hello"))
				})
			})
		})

		Context("when there is NO tls intermediary configured", func() {
			BeforeEach(func() {
				intermediaryListener.Close()
			})

			Context("when ssh-proxy is configured to connect to a tls intermediary", func() {
				BeforeEach(func() {
					sshProxyConfig.BackendsTLSEnabled = true
					_, serverCAFile := ca.CAAndKey()
					sshProxyConfig.BackendsTLSCACerts = serverCAFile
				})

				It("connects to the daemon without using tls and logs appropriately", func() {
					client, err := ssh.Dial("tcp", address, clientConfig)
					Expect(err).NotTo(HaveOccurred())

					Consistently(connectedToTLS).ShouldNot(Receive())

					session, err := client.NewSession()
					Expect(err).NotTo(HaveOccurred())
					output, err := session.Output("echo -n hello")
					Expect(err).NotTo(HaveOccurred())
					Expect(string(output)).To(Equal("hello"))
				})
			})
		})

		It("identifies itself as a Diego SSH proxy server", func() {
			client, err := ssh.Dial("tcp", address, clientConfig)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(client.Conn.ServerVersion())).To(Equal("SSH-2.0-diego-ssh-proxy"))
		})

		Context("when dealing with an idle connection", func() {
			It("eventually times out", func() {
				client, err := net.Dial("tcp", address)
				Expect(err).NotTo(HaveOccurred())

				errs := make(chan error)
				go func() {
					defer GinkgoRecover()
					for {
						bs := make([]byte, 10)
						_, err := client.Read(bs)
						errs <- err
					}
				}()
				Eventually(errs).Should(Receive(MatchError("EOF")))
			})
		})

		Context("metrics", func() {
			var (
				testMetricsChan   = make(chan *loggregator_v2.Envelope, 10)
				signalMetricsChan = make(chan struct{})
				testIngressServer *testhelpers.TestIngressServer
			)

			BeforeEach(func() {
				serverKeyFile, serverCertFile, err := ca.GenerateSelfSignedCertAndKey("metron", []string{"metron"}, false)
				Expect(err).NotTo(HaveOccurred())
				_, serverCAFile := ca.CAAndKey()

				testIngressServer, err = testhelpers.NewTestIngressServer(serverCertFile, serverKeyFile, serverCAFile)
				Expect(err).NotTo(HaveOccurred())

				receiversChan := testIngressServer.Receivers()
				Expect(testIngressServer.Start()).To(Succeed())
				port, err := strconv.Atoi(strings.TrimPrefix(testIngressServer.Addr(), "127.0.0.1:"))
				Expect(err).NotTo(HaveOccurred())
				sshProxyConfig.LoggregatorConfig.BatchFlushInterval = 10 * time.Millisecond
				sshProxyConfig.LoggregatorConfig.BatchMaxSize = 1
				sshProxyConfig.LoggregatorConfig.APIPort = port
				sshProxyConfig.LoggregatorConfig.UseV2API = true
				sshProxyConfig.LoggregatorConfig.CACertPath = serverCAFile
				sshProxyConfig.LoggregatorConfig.KeyPath = serverKeyFile
				sshProxyConfig.LoggregatorConfig.CertPath = serverCertFile

				testMetricsChan, signalMetricsChan = testhelpers.TestMetricChan(receiversChan)
			})

			AfterEach(func() {
				testIngressServer.Stop()
				close(signalMetricsChan)
			})

			Context("when the loggregator server isn't up", func() {
				BeforeEach(func() {
					testIngressServer.Stop()
				})

				It("exits with non-zero status code", func() {
					Eventually(process.Wait()).Should(Receive(HaveOccurred()))
				})
			})

			Context("when the loggregator agent is up", func() {
				JustBeforeEach(func() {
					client, err := ssh.Dial("tcp", address, clientConfig)
					Expect(err).NotTo(HaveOccurred())
					_, err = client.NewSession()
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when using loggregator v2 api", func() {
					BeforeEach(func() {
						sshProxyConfig.LoggregatorConfig.UseV2API = true
					})

					It("emits the number of current ssh-connections", func() {
						Eventually(testMetricsChan).Should(Receive(testhelpers.MatchV2MetricAndValue(testhelpers.MetricAndValue{Name: "ssh-connections", Value: int32(1)})))
					})
				})

				Context("when not using the loggregator v2 api", func() {
					BeforeEach(func() {
						sshProxyConfig.LoggregatorConfig.UseV2API = false
					})

					It("doesn't emit any metrics", func() {
						Consistently(testMetricsChan).ShouldNot(Receive())
					})
				})
			})
		})

		Context("when the proxy provides an unsupported cipher algorithm", func() {
			BeforeEach(func() {
				sshProxyConfig.AllowedCiphers = "unsupported"
			})

			It("rejects the cipher algorithm", func() {
				_, err := ssh.Dial("tcp", address, clientConfig)
				Expect(err).To(MatchError(ContainSubstring("ssh: no common algorithm for client to server cipher")))
				Expect(fakeBBS.ReceivedRequests()).To(HaveLen(0))
			})
		})

		Context("when the proxy provides the default cipher algorithms", func() {
			BeforeEach(func() {
				clientConfig.Ciphers = []string{"arcfour128"}
			})

			It("errors when the client doesn't provide any of the algorithms: 'aes128-gcm@openssh.com', 'aes128-gcm@openssh.com', 'aes256-ctr', 'aes192-ctr', 'aes128-ctr'", func() {
				_, err := ssh.Dial("tcp", address, clientConfig)
				Expect(err).To(MatchError("ssh: handshake failed: ssh: no common algorithm for client to server cipher; client offered: [arcfour128], server offered: [aes128-gcm@openssh.com aes256-ctr aes192-ctr aes128-ctr]"))
				Expect(fakeBBS.ReceivedRequests()).To(HaveLen(0))
			})
		})

		Context("when the proxy provides a supported cipher algorithm", func() {
			BeforeEach(func() {
				sshProxyConfig.AllowedCiphers = "aes128-ctr,aes256-ctr"
				clientConfig = &ssh.ClientConfig{
					User:            "diego:" + processGuid + "/99",
					Auth:            []ssh.AuthMethod{ssh.Password(diegoCredentials)},
					HostKeyCallback: ssh.InsecureIgnoreHostKey(),
				}
			})

			It("allows a client to complete a handshake", func() {
				client, err := ssh.Dial("tcp", address, clientConfig)
				Expect(err).NotTo(HaveOccurred())

				err = client.Close()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the proxy provides an unsupported MAC algorithm", func() {
			BeforeEach(func() {
				sshProxyConfig.AllowedMACs = "unsupported"
			})

			Context("and the cipher is an AEAD cipher", func() {
				BeforeEach(func() {
					sshProxyConfig.AllowedCiphers = "aes128-gcm@openssh.com"
				})

				It("does not error", func() {
					client, err := ssh.Dial("tcp", address, clientConfig)
					Expect(err).NotTo(HaveOccurred())

					err = client.Close()
					Expect(err).NotTo(HaveOccurred())

				})
			})

			Context("and the cipher is not an AEAD cipher", func() {
				BeforeEach(func() {
					sshProxyConfig.AllowedCiphers = "aes256-ctr"
				})

				It("rejects the MAC algorithm", func() {
					_, err := ssh.Dial("tcp", address, clientConfig)
					Expect(err).To(MatchError(ContainSubstring("ssh: no common algorithm for client to server MAC")))
					Expect(fakeBBS.ReceivedRequests()).To(HaveLen(0))
				})
			})
		})

		Context("when the proxy provides a supported MAC algorithm", func() {
			BeforeEach(func() {
				sshProxyConfig.AllowedMACs = "hmac-sha2-256,hmac-sha1"
				clientConfig = &ssh.ClientConfig{
					User:            "diego:" + processGuid + "/99",
					Auth:            []ssh.AuthMethod{ssh.Password(diegoCredentials)},
					HostKeyCallback: ssh.InsecureIgnoreHostKey(),
				}
			})

			It("allows a client to complete a handshake", func() {
				client, err := ssh.Dial("tcp", address, clientConfig)
				Expect(err).NotTo(HaveOccurred())

				err = client.Close()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the proxy provides the default MAC algorithm", func() {
			BeforeEach(func() {
				clientConfig.MACs = []string{"hmac-sha1"}
			})

			Context("and the cipher is an AEAD cipher", func() {
				BeforeEach(func() {
					sshProxyConfig.AllowedCiphers = "aes128-gcm@openssh.com"
				})

				It("does not error", func() {
					client, err := ssh.Dial("tcp", address, clientConfig)
					Expect(err).NotTo(HaveOccurred())

					err = client.Close()
					Expect(err).NotTo(HaveOccurred())

				})
			})

			Context("and the cipher is not an AEAD cipher", func() {
				BeforeEach(func() {
					sshProxyConfig.AllowedCiphers = "aes256-ctr"
				})

				It("errors when the client doesn't provide one of the algorithms: 'hmac-sha2-256-etm@openssh.com', 'hmac-sha2-256'", func() {
					_, err := ssh.Dial("tcp", address, clientConfig)
					Expect(err).To(MatchError("ssh: handshake failed: ssh: no common algorithm for client to server MAC; client offered: [hmac-sha1], server offered: [hmac-sha2-256-etm@openssh.com hmac-sha2-256]"))
					Expect(fakeBBS.ReceivedRequests()).To(HaveLen(0))
				})
			})
		})

		Context("when the proxy provides an unsupported key exchange algorithm", func() {
			BeforeEach(func() {
				sshProxyConfig.AllowedKeyExchanges = "unsupported"
			})

			It("rejects the key exchange algorithm", func() {
				_, err := ssh.Dial("tcp", address, clientConfig)
				Expect(err).To(MatchError(ContainSubstring("ssh: no common algorithm for key exchange")))
				Expect(fakeBBS.ReceivedRequests()).To(HaveLen(0))
			})
		})

		Context("when the proxy provides a supported key exchange algorithm", func() {
			BeforeEach(func() {
				sshProxyConfig.AllowedKeyExchanges = "curve25519-sha256@libssh.org,ecdh-sha2-nistp384,diffie-hellman-group14-sha1"
				clientConfig = &ssh.ClientConfig{
					User:            "diego:" + processGuid + "/99",
					Auth:            []ssh.AuthMethod{ssh.Password(diegoCredentials)},
					HostKeyCallback: ssh.InsecureIgnoreHostKey(),
				}
			})

			It("allows a client to complete a handshake", func() {
				client, err := ssh.Dial("tcp", address, clientConfig)
				Expect(err).NotTo(HaveOccurred())

				err = client.Close()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the proxy provides the default KeyExchange algorithm", func() {
			BeforeEach(func() {
				clientConfig.KeyExchanges = []string{"diffie-hellman-group14-sha1"}
			})

			It("errors when the client doesn't provide the algorithm: 'curve25519-sha256@libssh.org'", func() {
				_, err := ssh.Dial("tcp", address, clientConfig)
				Expect(err).To(MatchError("ssh: handshake failed: ssh: no common algorithm for key exchange; client offered: [diffie-hellman-group14-sha1 ext-info-c kex-strict-c-v00@openssh.com], server offered: [curve25519-sha256@libssh.org kex-strict-s-v00@openssh.com]"))
				Expect(fakeBBS.ReceivedRequests()).To(HaveLen(0))
			})
		})

		Context("when a non-existent process guid is used", func() {
			BeforeEach(func() {
				clientConfig.User = "diego:bad-process-guid/999"
				index := int32(999)
				expectedGetActualLRPRequest = &models.ActualLRPsRequest{
					ProcessGuid: "bad-process-guid",
					Index:       &index,
				}
				actualLRPsResponse = &models.ActualLRPsResponse{
					Error: models.ErrResourceNotFound,
				}
			})

			It("attempts to acquire the lrp info from the BBS", func() {
				_, _ = ssh.Dial("tcp", address, clientConfig)
				Expect(fakeBBS.ReceivedRequests()).To(HaveLen(1))
			})

			It("fails the authentication", func() {
				_, err := ssh.Dial("tcp", address, clientConfig)
				Expect(err).To(MatchError(ContainSubstring("ssh: handshake failed")))
			})
		})

		Context("when invalid credentials are presented", func() {
			BeforeEach(func() {
				clientConfig.Auth = []ssh.AuthMethod{
					ssh.Password("bogus-password"),
				}
			})

			It("fails diego authentication when the wrong credentials are used", func() {
				_, err := ssh.Dial("tcp", address, clientConfig)
				Expect(err).To(MatchError(ContainSubstring("ssh: handshake failed")))
			})
		})

		Context("and the enableDiegoAuth flag is set to false", func() {
			BeforeEach(func() {
				sshProxyConfig.EnableDiegoAuth = false
			})

			It("fails the authentication", func() {
				_, err := ssh.Dial("tcp", address, clientConfig)
				Expect(err).To(MatchError(ContainSubstring("ssh: handshake failed")))
				Expect(fakeBBS.ReceivedRequests()).To(HaveLen(0))
			})
		})
	})

	Describe("authenticating with the cf realm with a one time code", Serial, func() {
		BeforeEach(func() {
			clientConfig = &ssh.ClientConfig{
				User:            "cf:60f0f26e-86b3-4487-8f19-9e94f848f3d2/99",
				Auth:            []ssh.AuthMethod{ssh.Password("abc123")},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}

			fakeUAA.RouteToHandler("POST", "/oauth/token", ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/oauth/token"),
				ghttp.VerifyBasicAuth("amandaplease", "password1"),
				ghttp.VerifyContentType("application/x-www-form-urlencoded"),
				ghttp.VerifyFormKV("grant_type", "authorization_code"),
				ghttp.VerifyFormKV("code", "abc123"),
				ghttp.RespondWithJSONEncoded(http.StatusOK, authenticators.UAAAuthTokenResponse{
					AccessToken: "eyJhbGciOiJSUzI1NiIsImtpZCI6ImxlZ2FjeS10b2tlbi1rZXkiLCJ0eXAiOiJKV1QifQ.eyJqdGkiOiJmMGMyYWRkN2E5MDI0NTQyOWExZTdiMjNjZGVlZjkyZiIsInN1YiI6IjM2YmExMWZmLTBmNmEtNGM1MC1hYjM0LTZmYmQyODZhNjQzZSIsInNjb3BlIjpbInJvdXRpbmcucm91dGVyX2dyb3Vwcy5yZWFkIiwiY2xvdWRfY29udHJvbGxlci5yZWFkIiwicGFzc3dvcmQud3JpdGUiLCJjbG91ZF9jb250cm9sbGVyLndyaXRlIiwib3BlbmlkIiwicm91dGluZy5yb3V0ZXJfZ3JvdXBzLndyaXRlIiwiZG9wcGxlci5maXJlaG9zZSIsInNjaW0ud3JpdGUiLCJzY2ltLnJlYWQiLCJjbG91ZF9jb250cm9sbGVyLmFkbWluIiwidWFhLnVzZXIiXSwiY2xpZW50X2lkIjoiY2YiLCJjaWQiOiJjZiIsImF6cCI6ImNmIiwiZ3JhbnRfdHlwZSI6InBhc3N3b3JkIiwidXNlcl9pZCI6IjM2YmExMWZmLTBmNmEtNGM1MC1hYjM0LTZmYmQyODZhNjQzZSIsIm9yaWdpbiI6InVhYSIsInVzZXJfbmFtZSI6ImFkbWluIiwiZW1haWwiOiJhZG1pbiIsInJldl9zaWciOiJiMzUyMDU5ZiIsImlhdCI6MTQ3ODUxMzI3NywiZXhwIjoxNDc4NTEzODc3LCJpc3MiOiJodHRwczovL3VhYS5ib3NoLWxpdGUuY29tL29hdXRoL3Rva2VuIiwiemlkIjoidWFhIiwiYXVkIjpbInNjaW0iLCJjbG91ZF9jb250cm9sbGVyIiwicGFzc3dvcmQiLCJjZiIsInVhYSIsIm9wZW5pZCIsImRvcHBsZXIiLCJyb3V0aW5nLnJvdXRlcl9ncm91cHMiXX0.d8YS9HYM2QJ7f3xXjwHjZsGHCD2a4hM3tNQdGUQCJzT45YQkFZAJJDFIn4rai0YXJyswHmNT3K9pwKBzzcVzbe2HoMyI2HhCn3vW45OA7r55ATYmA88F1KkOtGitO_qi5NPhqDlQwg55kr6PzWAE84BXgWwivMXDDcwkyQosVYA",
					TokenType:   "bearer",
				}),
			))

			fakeCC.RouteToHandler("GET", "/internal/apps/60f0f26e-86b3-4487-8f19-9e94f848f3d2/ssh_access/99", ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/internal/apps/60f0f26e-86b3-4487-8f19-9e94f848f3d2/ssh_access/99"),
				ghttp.VerifyHeader(http.Header{"Authorization": []string{"bearer eyJhbGciOiJSUzI1NiIsImtpZCI6ImxlZ2FjeS10b2tlbi1rZXkiLCJ0eXAiOiJKV1QifQ.eyJqdGkiOiJmMGMyYWRkN2E5MDI0NTQyOWExZTdiMjNjZGVlZjkyZiIsInN1YiI6IjM2YmExMWZmLTBmNmEtNGM1MC1hYjM0LTZmYmQyODZhNjQzZSIsInNjb3BlIjpbInJvdXRpbmcucm91dGVyX2dyb3Vwcy5yZWFkIiwiY2xvdWRfY29udHJvbGxlci5yZWFkIiwicGFzc3dvcmQud3JpdGUiLCJjbG91ZF9jb250cm9sbGVyLndyaXRlIiwib3BlbmlkIiwicm91dGluZy5yb3V0ZXJfZ3JvdXBzLndyaXRlIiwiZG9wcGxlci5maXJlaG9zZSIsInNjaW0ud3JpdGUiLCJzY2ltLnJlYWQiLCJjbG91ZF9jb250cm9sbGVyLmFkbWluIiwidWFhLnVzZXIiXSwiY2xpZW50X2lkIjoiY2YiLCJjaWQiOiJjZiIsImF6cCI6ImNmIiwiZ3JhbnRfdHlwZSI6InBhc3N3b3JkIiwidXNlcl9pZCI6IjM2YmExMWZmLTBmNmEtNGM1MC1hYjM0LTZmYmQyODZhNjQzZSIsIm9yaWdpbiI6InVhYSIsInVzZXJfbmFtZSI6ImFkbWluIiwiZW1haWwiOiJhZG1pbiIsInJldl9zaWciOiJiMzUyMDU5ZiIsImlhdCI6MTQ3ODUxMzI3NywiZXhwIjoxNDc4NTEzODc3LCJpc3MiOiJodHRwczovL3VhYS5ib3NoLWxpdGUuY29tL29hdXRoL3Rva2VuIiwiemlkIjoidWFhIiwiYXVkIjpbInNjaW0iLCJjbG91ZF9jb250cm9sbGVyIiwicGFzc3dvcmQiLCJjZiIsInVhYSIsIm9wZW5pZCIsImRvcHBsZXIiLCJyb3V0aW5nLnJvdXRlcl9ncm91cHMiXX0.d8YS9HYM2QJ7f3xXjwHjZsGHCD2a4hM3tNQdGUQCJzT45YQkFZAJJDFIn4rai0YXJyswHmNT3K9pwKBzzcVzbe2HoMyI2HhCn3vW45OA7r55ATYmA88F1KkOtGitO_qi5NPhqDlQwg55kr6PzWAE84BXgWwivMXDDcwkyQosVYA"}}),
				ghttp.RespondWithJSONEncoded(http.StatusOK, authenticators.AppSSHResponse{
					ProcessGuid: processGuid,
				}),
			))
		})

		It("provides the access code to the UAA and and gets an access token", func() {
			client, err := ssh.Dial("tcp", address, clientConfig)
			Expect(err).NotTo(HaveOccurred())

			err = client.Close()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeUAA.ReceivedRequests()).To(HaveLen(1))
		})

		It("provides a bearer token to the CC and gets the process guid", func() {
			client, err := ssh.Dial("tcp", address, clientConfig)
			Expect(err).NotTo(HaveOccurred())

			err = client.Close()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCC.ReceivedRequests()).To(HaveLen(1))
		})

		It("acquires the lrp info from the BBS using the process guid from the CC", func() {
			client, err := ssh.Dial("tcp", address, clientConfig)
			Expect(err).NotTo(HaveOccurred())

			err = client.Close()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBBS.ReceivedRequests()).To(HaveLen(2))
		})

		It("connects to the target daemon", func() {
			client, err := ssh.Dial("tcp", address, clientConfig)
			Expect(err).NotTo(HaveOccurred())

			session, err := client.NewSession()
			Expect(err).NotTo(HaveOccurred())

			output, err := session.Output("echo -n hello")
			Expect(err).NotTo(HaveOccurred())

			Expect(string(output)).To(Equal("hello"))
		})

		Context("when the proxy is configured to use direct instance address", Serial, func() {
			BeforeEach(func() {
				sshProxyConfig.ConnectToInstanceAddress = true

				ginkgomon.Kill(sshdProcess)
				sshdArgs := sshdtestrunner.Args{
					Address:       fmt.Sprintf("127.0.0.1:%d", uint32(sshdContainerPort)),
					HostKey:       hostKeyPem,
					AuthorizedKey: publicAuthorizedKey,
				}

				runner := sshdtestrunner.New(sshdPath, sshdArgs)
				sshdProcess = ifrit.Invoke(runner)
			})

			It("connects to the target daemon", func() {
				client, err := ssh.Dial("tcp", address, clientConfig)
				Expect(err).NotTo(HaveOccurred())

				session, err := client.NewSession()
				Expect(err).NotTo(HaveOccurred())

				output, err := session.Output("echo -n hello")
				Expect(err).NotTo(HaveOccurred())

				Expect(string(output)).To(Equal("hello"))
			})
		})
	})
})

func VerifyProto(expected proto.Message) http.HandlerFunc {
	return ghttp.CombineHandlers(
		ghttp.VerifyContentType("application/x-protobuf"),

		func(w http.ResponseWriter, req *http.Request) {
			defer GinkgoRecover()
			body, err := io.ReadAll(req.Body)
			Expect(err).ToNot(HaveOccurred())

			err = req.Body.Close()
			Expect(err).NotTo(HaveOccurred())

			expectedType := reflect.TypeOf(expected)
			actualValuePtr := reflect.New(expectedType.Elem())

			actual, ok := actualValuePtr.Interface().(proto.Message)
			Expect(ok).To(BeTrue())

			err = proto.Unmarshal(body, actual)
			Expect(err).ToNot(HaveOccurred())

			Expect(proto.Equal(actual, expected)).To(BeTrue())
		},
	)
}

func RespondWithProto(message proto.Message) http.HandlerFunc {
	data, err := proto.Marshal(message)
	Expect(err).ToNot(HaveOccurred())

	var headers = make(http.Header)
	headers["Content-Type"] = []string{"application/x-protobuf"}
	return ghttp.RespondWith(200, string(data), headers)
}

type forwardTLSServer struct {
	logger  lager.Logger
	proxy   net.Listener
	stopCh  chan struct{}
	address string
}

func NewForwardTLSServer(logger lager.Logger, proxy net.Listener, address string) *forwardTLSServer {
	return &forwardTLSServer{
		logger:  logger.Session("forward-tls-server"),
		proxy:   proxy,
		address: address,
		stopCh:  make(chan struct{}),
	}
}

func (s *forwardTLSServer) Start(onConnectionReceived chan struct{}) error {
	for {
		select {
		case <-s.stopCh:
			return nil
		default:
			conn, err := s.proxy.Accept()
			if err != nil {
				select {
				case <-s.stopCh:
					return nil
				default:
					s.logger.Error("failed-to-receive-connection", err)
					return err
				}
			}

			tlsConn := conn.(*tls.Conn)
			err = tlsConn.Handshake()
			if err != nil {
				select {
				case <-s.stopCh:
					return nil
				default:
					s.logger.Error("failed-to-tls-handshake", err)
					return err
				}
			}

			if onConnectionReceived != nil {
				onConnectionReceived <- struct{}{}
			}

			proxyConn, err := net.Dial("tcp", s.address)
			if err != nil {
				select {
				case <-s.stopCh:
					return nil
				default:
					s.logger.Error("failed-to-dial", err)
					return err
				}
			}

			wg := sync.WaitGroup{}
			wg.Add(2)

			go func() {
				_, _ = io.Copy(conn, proxyConn)
				wg.Done()
			}()

			go func() {
				_, _ = io.Copy(proxyConn, conn)
				wg.Done()
			}()

			wg.Wait()
		}
	}
}

func (s *forwardTLSServer) Stop() {
	close(s.stopCh)
	s.proxy.Close()
}
