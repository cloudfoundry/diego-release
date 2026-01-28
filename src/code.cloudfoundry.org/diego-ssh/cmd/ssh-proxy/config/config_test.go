package config_test

import (
	"crypto/tls"
	"os"
	"time"

	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/diego-ssh/cmd/ssh-proxy/config"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/inigo/helpers/certauthority"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SSHProxyConfig", func() {
	Describe("#NewSSHProxyConfig", func() {
		var configFilePath, configData string

		BeforeEach(func() {
			configData = `{
			"address": "1.1.1.1",
			"health_check_address": "2.2.2.2",
			"disable_health_check_server": true,
			"host_key": "I am a host key.",
			"bbs_address": "3.3.3.3",
			"cc_api_url": "4.4.4.4",
			"cc_api_ca_cert": "I am a cc ca cert.",
			"uaa_token_url": "5.5.5.5",
			"uaa_password": "uaa-password",
			"uaa_username": "uaa-username",
			"skip_cert_verify": true,
			"communication_timeout": "5s",
			"enable_cf_auth": true,
			"enable_diego_auth": true,
			"diego_credentials": "diego-password",
			"bbs_ca_cert": "I am a bbs ca cert.",
			"bbs_client_cert": "I am a bbs client cert.",
			"bbs_client_key": "I am a bbs client key.",
			"bbs_client_session_cache_size": 10,
			"bbs_max_idle_conns_per_host": 20,
			"allowed_ciphers": "cipher1,cipher2,cipher3",
			"allowed_macs": "mac1,mac2,mac3",
			"allowed_key_exchanges": "exchange1,exchange2,exchange3",
			"log_level": "debug",
			"debug_address": "5.5.5.5:9090",
			"connect_to_instance_address": true,
			"idle_connection_timeout": "5ms",

			"backends_tls_enabled": true,
			"backends_tls_ca_certificates": "./some_filepath/ca.crt",
			"backends_tls_client_certificate": "./some_filepath/client.crt",
			"backends_tls_client_private_key": "./some_filepath/client.key"
		}`
		})

		JustBeforeEach(func() {
			configFile, err := os.CreateTemp("", "ssh-proxy-config")
			Expect(err).NotTo(HaveOccurred())

			n, err := configFile.WriteString(configData)
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(len(configData)))

			err = configFile.Close()
			Expect(err).NotTo(HaveOccurred())

			configFilePath = configFile.Name()
		})

		AfterEach(func() {
			err := os.RemoveAll(configFilePath)
			Expect(err).NotTo(HaveOccurred())
		})

		It("correctly parses the config file", func() {
			proxyConfig, err := config.NewSSHProxyConfig(configFilePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(proxyConfig).To(Equal(config.SSHProxyConfig{
				Address:                   "1.1.1.1",
				HealthCheckAddress:        "2.2.2.2",
				DisableHealthCheckServer:  true,
				HostKey:                   "I am a host key.",
				BBSAddress:                "3.3.3.3",
				CCAPIURL:                  "4.4.4.4",
				CCAPICACert:               "I am a cc ca cert.",
				UAATokenURL:               "5.5.5.5",
				UAAPassword:               "uaa-password",
				UAAUsername:               "uaa-username",
				SkipCertVerify:            true,
				CommunicationTimeout:      durationjson.Duration(5 * time.Second),
				EnableCFAuth:              true,
				EnableDiegoAuth:           true,
				DiegoCredentials:          "diego-password",
				BBSCACert:                 "I am a bbs ca cert.",
				BBSClientCert:             "I am a bbs client cert.",
				BBSClientKey:              "I am a bbs client key.",
				BBSClientSessionCacheSize: 10,
				BBSMaxIdleConnsPerHost:    20,
				AllowedCiphers:            "cipher1,cipher2,cipher3",
				AllowedMACs:               "mac1,mac2,mac3",
				AllowedKeyExchanges:       "exchange1,exchange2,exchange3",
				ConnectToInstanceAddress:  true,
				IdleConnectionTimeout:     durationjson.Duration(5 * time.Millisecond),
				LagerConfig: lagerflags.LagerConfig{
					LogLevel: lagerflags.DEBUG,
				},
				DebugServerConfig: debugserver.DebugServerConfig{
					DebugAddress: "5.5.5.5:9090",
				},

				BackendsTLSEnabled:    true,
				BackendsTLSCACerts:    "./some_filepath/ca.crt",
				BackendsTLSClientCert: "./some_filepath/client.crt",
				BackendsTLSClientKey:  "./some_filepath/client.key",
			}))
		})

		Context("when the file does not exist", func() {
			It("returns an error", func() {
				_, err := config.NewSSHProxyConfig("foobar")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the file does not contain valid json", func() {
			BeforeEach(func() {
				configData = "{{"
			})

			It("returns an error", func() {
				_, err := config.NewSSHProxyConfig(configFilePath)
				Expect(err).To(HaveOccurred())
			})

			Context("because the communication_timeout is not valid", func() {
				BeforeEach(func() {
					configData = `{"communication_timeout": 4234342342}`
				})

				It("returns an error", func() {
					_, err := config.NewSSHProxyConfig(configFilePath)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})

	Describe("#BackendsTLSConfig", func() {
		var (
			sshProxyConfig config.SSHProxyConfig
			tlsConfig      *tls.Config
			getConfigErr   error
			ca             certauthority.CertAuthority
			certDepoDir    string
		)

		JustBeforeEach(func() {
			tlsConfig, getConfigErr = sshProxyConfig.BackendsTLSConfig()
		})

		BeforeEach(func() {
			var err error

			certDepoDir, err = os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			ca, err = certauthority.NewCertAuthority(certDepoDir, "ssh-proxy-ca")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(certDepoDir)).To(Succeed())
		})

		Context("when backends tls is disabled", func() {
			BeforeEach(func() {
				sshProxyConfig.BackendsTLSEnabled = false
			})

			It("returns an empty tls config", func() {
				Expect(getConfigErr).ToNot(HaveOccurred())
				Expect(tlsConfig).To(BeNil())
			})
		})

		Context("when backends tls is enabled", func() {
			BeforeEach(func() {
				_, serverCAFile := ca.CAAndKey()

				clientKeyFile, clientCertFile, err := ca.GenerateSelfSignedCertAndKey("client", []string{}, false)
				Expect(err).NotTo(HaveOccurred())

				sshProxyConfig.BackendsTLSEnabled = true
				sshProxyConfig.BackendsTLSCACerts = serverCAFile
				sshProxyConfig.BackendsTLSClientCert = clientCertFile
				sshProxyConfig.BackendsTLSClientKey = clientKeyFile
			})

			It("returns a tls config", func() {
				Expect(getConfigErr).ToNot(HaveOccurred())
				//lint:ignore SA1019 - ignoring tlsCert.RootCAs.Subjects is deprecated ERR because cert does not come from SystemCertPool.
				Expect(len(tlsConfig.RootCAs.Subjects())).To(BeNumerically(">", 0))
				Expect(len(tlsConfig.Certificates)).To(BeNumerically(">", 0))
			})

			Context("when the CA cert file is NOT provided", func() {
				BeforeEach(func() {
					sshProxyConfig.BackendsTLSCACerts = ""
				})

				It("returns an error", func() {
					Expect(getConfigErr).To(MatchError(ContainSubstring("backend tls ca certificates must be specified")))
				})
			})

			Context("when the CA cert file is provided but unreadable", func() {
				BeforeEach(func() {
					sshProxyConfig.BackendsTLSCACerts = "non-existent-path/ca.crt"
				})

				It("returns an error", func() {
					Expect(getConfigErr).To(HaveOccurred())
					Expect(tlsConfig).To(BeNil())
				})
			})

			Context("when the CA cert is not valid PEM encoded", func() {
				var invalidCAPath string

				BeforeEach(func() {
					invalidCA, err := os.CreateTemp("", "invalid-ca.crt")
					Expect(err).NotTo(HaveOccurred())

					_, err = invalidCA.WriteString("invalid PEM")
					Expect(err).NotTo(HaveOccurred())

					err = invalidCA.Close()
					Expect(err).NotTo(HaveOccurred())

					invalidCAPath = invalidCA.Name()
					sshProxyConfig.BackendsTLSCACerts = invalidCAPath
				})

				AfterEach(func() {
					err := os.Remove(invalidCAPath)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns an error", func() {
					Expect(getConfigErr).To(MatchError("Failed to parse backends_tls_ca_certificates"))
					Expect(tlsConfig).To(BeNil())
				})
			})

			Context("when the client cert file is NOT provided", func() {
				BeforeEach(func() {
					sshProxyConfig.BackendsTLSClientCert = ""
				})

				It("should NOT set the client certificate in the TLS config", func() {
					Expect(getConfigErr).ToNot(HaveOccurred())
					Expect(tlsConfig.Certificates).To(HaveLen(0))
					//lint:ignore SA1019 - ignoring tlsCert.RootCAs.Subjects is deprecated ERR because cert does not come from SystemCertPool.
					Expect(len(tlsConfig.RootCAs.Subjects())).To(BeNumerically(">", 0))
				})
			})

			Context("when the client key file is NOT provided", func() {
				BeforeEach(func() {
					sshProxyConfig.BackendsTLSClientKey = ""
				})

				It("should NOT set the client certificate in the TLS config", func() {
					Expect(getConfigErr).ToNot(HaveOccurred())
					Expect(tlsConfig.Certificates).To(HaveLen(0))
					//lint:ignore SA1019 - ignoring tlsCert.RootCAs.Subjects is deprecated ERR because cert does not come from SystemCertPool.
					Expect(len(tlsConfig.RootCAs.Subjects())).To(BeNumerically(">", 0))
				})
			})

			Context("when the client cert file and the key file are both provided", func() {
				Context("when the client cert file cannot be read", func() {
					BeforeEach(func() {
						sshProxyConfig.BackendsTLSClientCert = "non-existant-path/client.crt"
					})

					It("returns an error", func() {
						Expect(getConfigErr).To(HaveOccurred())
						Expect(tlsConfig).To(BeNil())
					})
				})

				Context("when the client key file cannot be read", func() {
					BeforeEach(func() {
						sshProxyConfig.BackendsTLSClientKey = "non-existant-path/client.key"
					})

					It("returns an error", func() {
						Expect(getConfigErr).To(HaveOccurred())
						Expect(tlsConfig).To(BeNil())
					})
				})

				Context("when the client certificate is not valid PEM encoded", func() {
					var invalidCertPath string

					BeforeEach(func() {
						invalidCert, err := os.CreateTemp("", "invalid-cert.crt")
						Expect(err).NotTo(HaveOccurred())

						_, err = invalidCert.WriteString("invalid PEM")
						Expect(err).NotTo(HaveOccurred())

						err = invalidCert.Close()
						Expect(err).NotTo(HaveOccurred())

						invalidCertPath = invalidCert.Name()
						sshProxyConfig.BackendsTLSClientCert = invalidCertPath
					})

					AfterEach(func() {
						err := os.Remove(invalidCertPath)
						Expect(err).NotTo(HaveOccurred())
					})

					It("returns an error", func() {
						Expect(getConfigErr).To(MatchError(ContainSubstring("failed to load keypair")))
						Expect(tlsConfig).To(BeNil())
					})
				})

				Context("when the client key is not valid PEM encoded", func() {
					var invalidKeyPath string

					BeforeEach(func() {
						invalidKey, err := os.CreateTemp("", "invalid-key.key")
						Expect(err).NotTo(HaveOccurred())

						_, err = invalidKey.WriteString("invalid PEM")
						Expect(err).NotTo(HaveOccurred())

						err = invalidKey.Close()
						Expect(err).NotTo(HaveOccurred())

						invalidKeyPath = invalidKey.Name()
						sshProxyConfig.BackendsTLSClientKey = invalidKeyPath
					})

					AfterEach(func() {
						err := os.Remove(invalidKeyPath)
						Expect(err).NotTo(HaveOccurred())
					})

					It("returns an error", func() {
						Expect(getConfigErr).To(MatchError(ContainSubstring("failed to load keypair")))
						Expect(tlsConfig).To(BeNil())
					})
				})
			})
		})
	})
})
