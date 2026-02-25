package main_test

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/debugserver"
	loggingclient "code.cloudfoundry.org/diego-logging-client"

	"code.cloudfoundry.org/fileserver/cmd/file-server/config"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/tlsconfig"
	"code.cloudfoundry.org/tlsconfig/certtest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("File server", func() {
	var (
		port            int
		servedDirectory string
		session         *gexec.Session
		err             error
		configPath      string
		cfg             config.FileServerConfig
	)

	start := func(extras ...string) *gexec.Session {
		args := []string{"-config", configPath}
		session, err = gexec.Start(exec.Command(fileServerBinary, args...), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gbytes.Say("file-server.ready"))

		return session
	}

	AfterEach(func() {
		session.Kill().Wait()
		os.RemoveAll(servedDirectory)
		os.RemoveAll(configPath)
	})

	Context("when started without any arguments", func() {
		It("should fail", func() {
			session, err = gexec.Start(exec.Command(fileServerBinary), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(2))
			Eventually(session.Out).Should(gbytes.Say("failed-to-parse-config"))
		})
	})

	Context("when started correctly", func() {
		BeforeEach(func() {
			servedDirectory, err = os.MkdirTemp("", "file_server-test")
			Expect(err).NotTo(HaveOccurred())

			port = 8182 + GinkgoParallelProcess()
			cfg = config.FileServerConfig{
				LagerConfig: lagerflags.LagerConfig{
					LogLevel:   lagerflags.INFO,
					TimeFormat: lagerflags.FormatUnixEpoch,
				},
				LoggregatorConfig: loggingclient.Config{
					CACertPath: metronCAFile,
					CertPath:   metronServerCertFile,
					KeyPath:    metronServerKeyFile,
				},
				StaticDirectory: servedDirectory,
				ServerAddress:   fmt.Sprintf("localhost:%d", port),
			}
		})

		JustBeforeEach(func() {
			cfg.LoggregatorConfig.APIPort, _ = testIngressServer.Port()
			configFile, err := os.CreateTemp("", "file_server-test-config")
			Expect(err).NotTo(HaveOccurred())
			configPath = configFile.Name()

			encoder := json.NewEncoder(configFile)
			err = encoder.Encode(&cfg)
			Expect(err).NotTo(HaveOccurred())

			session = start()
			os.WriteFile(filepath.Join(servedDirectory, "test"), []byte("hello"), os.ModePerm)
		})

		It("should return that file on GET request", func() {
			resp, err := http.Get(fmt.Sprintf("http://localhost:%d/v1/static/test", port))
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			sha256bytes := sha256.Sum256([]byte("hello"))
			Expect(resp.Header.Get("ETag")).To(Equal(fmt.Sprintf(`"%s"`, hex.EncodeToString(sha256bytes[:]))))

			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal("hello"))
		})
	})

	Context("when HTTPS server is enabled", func() {
		var tlsPort int
		BeforeEach(func() {
			servedDirectory, err = os.MkdirTemp("", "file_server-test")
			Expect(err).NotTo(HaveOccurred())

			port = 8182 + GinkgoParallelProcess()
			tlsPort = 8282 + GinkgoParallelProcess()
			cfg = config.FileServerConfig{
				LagerConfig: lagerflags.LagerConfig{
					LogLevel:   lagerflags.INFO,
					TimeFormat: lagerflags.FormatUnixEpoch,
				},
				LoggregatorConfig: loggingclient.Config{
					CACertPath: metronCAFile,
					CertPath:   metronServerCertFile,
					KeyPath:    metronServerKeyFile,
				},
				HTTPSServerEnabled: true,
				StaticDirectory:    servedDirectory,
				ServerAddress:      fmt.Sprintf("localhost:%d", port),
			}
		})

		JustBeforeEach(func() {
			cfg.LoggregatorConfig.APIPort, _ = testIngressServer.Port()
			cfg.LoggregatorConfig.CACertPath = metronCAFile
			cfg.LoggregatorConfig.CertPath = metronServerCertFile
			cfg.LoggregatorConfig.KeyPath = metronServerKeyFile
			configFile, err := os.CreateTemp("", "file_server-test-config")
			Expect(err).NotTo(HaveOccurred())
			configPath = configFile.Name()

			encoder := json.NewEncoder(configFile)
			err = encoder.Encode(&cfg)
			Expect(err).NotTo(HaveOccurred())
		})

		It("fails if none of the required HTTPS configuration is provided", func() {
			args := []string{"-config", configPath}
			session, err = gexec.Start(exec.Command(fileServerBinary, args...), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(2))
			Eventually(session.Out).Should(gbytes.Say("invalid-https-configuration"))
		})

		Context("when just the server address is provided", func() {
			BeforeEach(func() {
				cfg.HTTPSListenAddr = fmt.Sprintf("localhost:%d", tlsPort)
			})

			It("fails if the server cert is not provided", func() {
				args := []string{"-config", configPath}
				session, err = gexec.Start(exec.Command(fileServerBinary, args...), GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(2))
				Eventually(session.Out).Should(gbytes.Say("failed-to-create-tls-config"))
			})
		})

		Context("when all required HTTPS configuration is provided", func() {
			var (
				caCertPool                    *x509.CertPool
				certFile, keyFile, caFile     *os.File
				clientCertFile, clientKeyFile *os.File
			)

			BeforeEach(func() {
				ca, err := certtest.BuildCA("test-ca")
				Expect(err).NotTo(HaveOccurred())
				cert, err := ca.BuildSignedCertificate("fileserver")
				Expect(err).NotTo(HaveOccurred())
				clientCert, err := ca.BuildSignedCertificate("client")
				Expect(err).NotTo(HaveOccurred())
				caCertPool, err = ca.CertPool()
				Expect(err).NotTo(HaveOccurred())

				caPem, err := ca.CertificatePEM()
				Expect(err).NotTo(HaveOccurred())
				pem, privKey, err := cert.CertificatePEMAndPrivateKey()
				Expect(err).NotTo(HaveOccurred())
				clientPem, clientPrivKey, err := clientCert.CertificatePEMAndPrivateKey()
				Expect(err).NotTo(HaveOccurred())

				certFile, err = os.CreateTemp("", "testcert")
				Expect(err).NotTo(HaveOccurred())
				keyFile, err = os.CreateTemp("", "testkey")
				Expect(err).NotTo(HaveOccurred())
				caFile, err = os.CreateTemp("", "testca")
				Expect(err).NotTo(HaveOccurred())
				clientCertFile, err = os.CreateTemp("", "testclientcert")
				Expect(err).NotTo(HaveOccurred())
				clientKeyFile, err = os.CreateTemp("", "testclientkey")
				Expect(err).NotTo(HaveOccurred())

				_, err = certFile.Write(pem)
				Expect(err).NotTo(HaveOccurred())
				_, err = keyFile.Write(privKey)
				Expect(err).NotTo(HaveOccurred())
				_, err = caFile.Write(caPem)
				Expect(err).NotTo(HaveOccurred())
				_, err = clientCertFile.Write(clientPem)
				Expect(err).NotTo(HaveOccurred())
				_, err = clientKeyFile.Write(clientPrivKey)
				Expect(err).NotTo(HaveOccurred())

				Expect(certFile.Close()).To(Succeed())
				Expect(keyFile.Close()).To(Succeed())
				Expect(caFile.Close()).To(Succeed())
				Expect(clientCertFile.Close()).To(Succeed())
				Expect(clientKeyFile.Close()).To(Succeed())

				cfg.HTTPSListenAddr = fmt.Sprintf("localhost:%d", tlsPort)
				cfg.CertFile = certFile.Name()
				cfg.KeyFile = keyFile.Name()
				cfg.ClientCACertFile = caFile.Name()
			})

			JustBeforeEach(func() {
				session = start()
				Expect(os.WriteFile(filepath.Join(servedDirectory, "test"), []byte("hello"), os.ModePerm)).To(Succeed())
			})

			AfterEach(func() {
				Expect(os.Remove(certFile.Name())).To(Succeed())
				Expect(os.Remove(keyFile.Name())).To(Succeed())
				Expect(os.Remove(caFile.Name())).To(Succeed())
				Expect(os.Remove(clientCertFile.Name())).To(Succeed())
				Expect(os.Remove(clientKeyFile.Name())).To(Succeed())
			})

			It("should successfully return the test file on an HTTPS GET request", func() {
				clientTLSConfig, err := tlsconfig.Build(
					tlsconfig.WithIdentityFromFile(clientCertFile.Name(), clientKeyFile.Name()),
				).Client(tlsconfig.WithAuthority(caCertPool))
				Expect(err).NotTo(HaveOccurred())

				httpClient := &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: clientTLSConfig,
					},
				}
				resp, err := httpClient.Get(fmt.Sprintf("https://localhost:%d/v1/static/test", tlsPort))
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()

				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				sha256bytes := sha256.Sum256([]byte("hello"))
				Expect(resp.Header.Get("ETag")).To(Equal(fmt.Sprintf(`"%s"`, hex.EncodeToString(sha256bytes[:]))))

				body, err := io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(body)).To(Equal("hello"))
			})

			It("fails to return test when caCertPool is missing", func() {
				clientTLSConfig, err := tlsconfig.Build(
					tlsconfig.WithIdentityFromFile(clientCertFile.Name(), clientKeyFile.Name()),
				).Client()
				Expect(err).NotTo(HaveOccurred())

				httpClient := &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: clientTLSConfig,
					},
				}
				_, err = httpClient.Get(fmt.Sprintf("https://localhost:%d/v1/static/test", tlsPort))
				Expect(err.Error()).To(ContainSubstring("tls: failed to verify certificate: x509:"))
			})

			It("fails to return test when client certificate is not provided (mTLS required)", func() {
				clientTLSConfig, err := tlsconfig.Build(
					tlsconfig.WithInternalServiceDefaults(),
				).Client(tlsconfig.WithAuthority(caCertPool))
				Expect(err).NotTo(HaveOccurred())

				httpClient := &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: clientTLSConfig,
					},
				}
				_, err = httpClient.Get(fmt.Sprintf("https://localhost:%d/v1/static/test", tlsPort))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("tls: certificate required"))
			})

			It("should return a 301 redirect to the HTTPS URL when making an HTTP Get request", func() {
				clientTLSConfig, err := tlsconfig.Build(
					tlsconfig.WithIdentityFromFile(clientCertFile.Name(), clientKeyFile.Name()),
				).Client(tlsconfig.WithAuthority(caCertPool))
				Expect(err).NotTo(HaveOccurred())

				httpClient := &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: clientTLSConfig,
					},
					CheckRedirect: func(req *http.Request, via []*http.Request) error {
						return http.ErrUseLastResponse
					},
				}

				req, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d/v1/static/test", port), nil)
				Expect(err).NotTo(HaveOccurred())
				req.Host = "file-server.service.test.com"
				resp, err := httpClient.Do(req)
				Expect(err).NotTo(HaveOccurred())

				Expect(resp.StatusCode).To(Equal(http.StatusMovedPermanently))
				location, err := resp.Location()
				Expect(err).NotTo(HaveOccurred())
				Expect(location.String()).To(Equal(fmt.Sprintf("https://file-server.service.test.com:%d/v1/static/test", tlsPort)))
			})
		})
	})

	Context("debug server", func() {
		var debugPort int

		BeforeEach(func() {
			servedDirectory, err = os.MkdirTemp("", "file_server-test")
			Expect(err).NotTo(HaveOccurred())

			port = 8182 + GinkgoParallelProcess()
			debugPort = 17005 + GinkgoParallelProcess()
		})

		Context("when debug_address is configured", func() {
			BeforeEach(func() {
				cfg = config.FileServerConfig{
					LagerConfig: lagerflags.LagerConfig{
						LogLevel:   lagerflags.INFO,
						TimeFormat: lagerflags.FormatUnixEpoch,
					},
					LoggregatorConfig: loggingclient.Config{
						CACertPath: metronCAFile,
						CertPath:   metronServerCertFile,
						KeyPath:    metronServerKeyFile,
					},
					StaticDirectory: servedDirectory,
					ServerAddress:   fmt.Sprintf("localhost:%d", port),
				}
				cfg.DebugAddress = fmt.Sprintf("127.0.0.1:%d", debugPort)
			})

			JustBeforeEach(func() {
				cfg.LoggregatorConfig.APIPort, _ = testIngressServer.Port()
				configFile, err := os.CreateTemp("", "file_server-test-config")
				Expect(err).NotTo(HaveOccurred())
				configPath = configFile.Name()

				encoder := json.NewEncoder(configFile)
				err = encoder.Encode(&cfg)
				Expect(err).NotTo(HaveOccurred())

				session = start()
			})

			It("should start the debug server and respond to requests", func() {
				Eventually(func() error {
					resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/debug/pprof/", debugPort))
					if err != nil {
						return err
					}
					defer resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						return fmt.Errorf("expected status 200, got %d", resp.StatusCode)
					}
					return nil
				}).Should(Succeed())
			})
		})

		Context("when debug_address is not configured", func() {
			BeforeEach(func() {
				cfg = config.FileServerConfig{
					LagerConfig: lagerflags.LagerConfig{
						LogLevel:   lagerflags.INFO,
						TimeFormat: lagerflags.FormatUnixEpoch,
					},
					LoggregatorConfig: loggingclient.Config{
						CACertPath: metronCAFile,
						CertPath:   metronServerCertFile,
						KeyPath:    metronServerKeyFile,
					},
					StaticDirectory: servedDirectory,
					ServerAddress:   fmt.Sprintf("localhost:%d", port),
					DebugServerConfig: debugserver.DebugServerConfig{
						DebugAddress: "",
					},
				}
			})

			JustBeforeEach(func() {
				cfg.LoggregatorConfig.APIPort, _ = testIngressServer.Port()
				configFile, err := os.CreateTemp("", "file_server-test-config")
				Expect(err).NotTo(HaveOccurred())
				configPath = configFile.Name()

				encoder := json.NewEncoder(configFile)
				err = encoder.Encode(&cfg)
				Expect(err).NotTo(HaveOccurred())

				session = start()
			})

			It("should not start the debug server", func() {
				Consistently(func() error {
					resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/debug/pprof/", debugPort))
					if err != nil {
						return err
					}
					defer resp.Body.Close()
					return nil
				}).Should(HaveOccurred())
			})
		})
	})
})
