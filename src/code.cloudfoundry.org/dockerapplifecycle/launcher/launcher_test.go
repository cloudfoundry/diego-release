package main_test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"code.cloudfoundry.org/tlsconfig"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Launcher", func() {
	const (
		vcapServicesWithDB = `{
  "user-provided": [
   {
    "credentials": {
		 "uri": "mysql://username:password@host:3333/db"
    },
    "label": "user-provided",
    "name": "my-db-mine",
    "syslog_drain_url": "",
    "tags": [],
    "volume_mounts": []
   }
  ]
 }
`
	)

	var (
		appDir      string
		launcherCmd *exec.Cmd
		session     *gexec.Session
		workdir     string
	)

	BeforeEach(func() {
		os.Setenv("CALLERENV", "some-value")

		var err error
		appDir, err = os.MkdirTemp("", "app-dir")
		Expect(err).NotTo(HaveOccurred())

		workdir = "/"

		launcherCmd = &exec.Cmd{
			Path: launcher,
			Env: append(
				os.Environ(),
				"PORT=8080",
				"INSTANCE_GUID=some-instance-guid",
				"INSTANCE_INDEX=123",
				`VCAP_APPLICATION={"foo":1}`,
			),
		}
	})

	AfterEach(func() {
		os.RemoveAll(appDir)
	})

	JustBeforeEach(func() {
		var err error
		session, err = gexec.Start(launcherCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
	})

	var ItExecutesTheCommandWithTheRightEnvironment = func() {
		It("executes the start command", func() {
			Eventually(session).Should(gbytes.Say("running app"))
		})

		It("executes with the environment of the caller", func() {
			Eventually(session).Should(gbytes.Say("CALLERENV=some-value"))
		})

		It("changes to the workdir when running", func() {
			// wildcard because PWD expands symlinks and appDir temp folder might be one
			Eventually(session).Should(gbytes.Say("PWD=" + workdir + "\n"))
		})

		It("munges VCAP_APPLICATION appropriately", func() {
			Eventually(session, 3, "100ms").Should(gexec.Exit(0))

			vcapAppPattern := regexp.MustCompile("VCAP_APPLICATION=(.*)")
			vcapApplicationBytes := vcapAppPattern.FindSubmatch(session.Out.Contents())[1]

			vcapApplication := map[string]interface{}{}
			err := json.Unmarshal(vcapApplicationBytes, &vcapApplication)
			Expect(err).NotTo(HaveOccurred())

			Expect(vcapApplication["host"]).To(Equal("0.0.0.0"))
			Expect(vcapApplication["port"]).To(Equal(float64(8080)))
			Expect(vcapApplication["instance_index"]).To(Equal(float64(123)))
			Expect(vcapApplication["instance_id"]).To(Equal("some-instance-guid"))
			Expect(vcapApplication["foo"]).To(Equal(float64(1)))
		})
	}

	Context("when a start command is given", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"env; echo running app",
				`{ "cmd": ["echo should not run this"] }`,
			}
		})

		ItExecutesTheCommandWithTheRightEnvironment()
	})

	Context("when a start command is given with a workdir", func() {
		BeforeEach(func() {
			workdir = "/usr/bin"
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"env; echo running app",
				fmt.Sprintf(`{ "cmd" : ["echo should not run this"],
				   "workdir" : "%s"}`, workdir),
			}
		})

		ItExecutesTheCommandWithTheRightEnvironment()
	})

	Describe("interpolation of credhub-ref in VCAP_SERVICES", func() {
		var (
			startCommand string
			server       *ghttp.Server
		)

		tlsConfig := func() *tls.Config {
			serverCertPath := filepath.Join("fixtures", "credhubserver.crt")
			serverKeyPath := filepath.Join("fixtures", "credhubserver.key")
			caCertPath := filepath.Join("fixtures", "ca-certs", "credhubtest.crt")

			tlsConfig, err := tlsconfig.Build(
				tlsconfig.WithInternalServiceDefaults(),
				tlsconfig.WithIdentityFromFile(serverCertPath, serverKeyPath),
			).Server(tlsconfig.WithClientAuthenticationFromFile(caCertPath))
			Expect(err).NotTo(HaveOccurred())
			return tlsConfig
		}

		BeforeEach(func() {
			server = ghttp.NewUnstartedServer()
			server.HTTPTestServer.TLS = tlsConfig()
			startCommand = "env; echo running app"

			clientCertPath := filepath.Join("fixtures", "credhubclient.crt")
			clientKeyPath := filepath.Join("fixtures", "credhubclient.key")

			env := os.Environ()
			caCertDir := filepath.Join("fixtures", "ca-certs")
			env = append(env, fmt.Sprintf("CF_SYSTEM_CERT_PATH=%s", caCertDir))
			env = append(env, fmt.Sprintf("CF_INSTANCE_CERT=%s", clientCertPath))
			env = append(env, fmt.Sprintf("CF_INSTANCE_KEY=%s", clientKeyPath))
			launcherCmd.Env = env
			server.HTTPTestServer.StartTLS()
		})

		Context("when VCAP_SERVICES contains credhub refs", func() {
			var vcapServicesValue string
			BeforeEach(func() {
				vcapServicesValue = `{"my-server":[{"credentials":{"credhub-ref":"(//my-server/creds)"}}]}`
				launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf("VCAP_SERVICES=%s", vcapServicesValue))
			})

			Context("when the credhub location is passed to the launcher", func() {
				var credhubURI string
				BeforeEach(func() {
					launcherCmd.Args = []string{
						"launcher",
						appDir,
						startCommand,
						"{}",
					}
					credhubURI = server.URL()
					credhubLocation := `{ "credhub-uri": "` + credhubURI + `"}`
					launcherCmd.Env = append(launcherCmd.Env, "VCAP_PLATFORM_OPTIONS="+credhubLocation)
				})

				Context("when credhub successfully interpolates", func() {
					BeforeEach(func() {
						server.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("POST", "/api/v1/interpolate"),
								ghttp.VerifyBody([]byte(vcapServicesValue)),
								ghttp.RespondWith(http.StatusOK, `{"some-service":[]}`),
							))
					})

					It("updates VCAP_SERVICES with the interpolated content", func() {
						Eventually(session).Should(gexec.Exit(0))
						Eventually(session.Out).Should(gbytes.Say(`VCAP_SERVICES={"some-service":\[\]}`))
					})
				})

				Context("and the vcap services has a database uri", func() {
					BeforeEach(func() {
						server.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("POST", "/api/v1/interpolate"),
								ghttp.VerifyBody([]byte(vcapServicesValue)),
								ghttp.RespondWith(http.StatusOK, vcapServicesWithDB),
							))

						launcherCmd.Args = []string{
							"launcher",
							appDir,
							"echo $DATABASE_URL",
							"{}",
						}
					})

					It("includes the database uri as extracted from the interpolated VCAP_SERVICES", func() {
						Eventually(session).Should(gexec.Exit(0))
						Eventually(session.Out).Should(gbytes.Say("mysql2://username:password@host:3333/db"))
					})
				})

				Context("when the launcher is passed an invalid credhub URI", func() {
					BeforeEach(func() {
						launcherCmd.Args = []string{
							"launcher",
							appDir,
							startCommand,
							"{}",
						}
						credhubLocation := `{ "credhub-uri": "https/:notsovalid.com"}`
						launcherCmd.Env = append(launcherCmd.Env, "VCAP_PLATFORM_OPTIONS="+credhubLocation)
					})

					It("prints an error with the invalid URI", func() {
						Eventually(session).Should(gexec.Exit(4))
						Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("Invalid CredHub URI: '%s'", "https/:notsovalid.com")))
					})
				})

				Context("when the launcher cannot connect to the credhub URI", func() {
					BeforeEach(func() {
						server.Close()
					})

					It("prints an error indicating launcher cannot connect to credhub", func() {
						Eventually(session).Should(gexec.Exit(4))
						Eventually(session.Err).Should(gbytes.Say("connection refused"))
					})
				})

				Context("when the credhub server cert is invalid", func() {
					BeforeEach(func() {
						server.Close()
						server = ghttp.NewUnstartedServer()
						server.HTTPTestServer.TLS = tlsConfig()
						invalidCertPath := filepath.Join("fixtures", "invalid.crt")
						invalidKeyPath := filepath.Join("fixtures", "invalid.key")
						cert, err := tls.LoadX509KeyPair(invalidCertPath, invalidKeyPath)
						Expect(err).NotTo(HaveOccurred())
						server.HTTPTestServer.TLS.Certificates = []tls.Certificate{cert}
						server.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("POST", "/api/v1/interpolate"),
								ghttp.VerifyBody([]byte(vcapServicesValue)),
								ghttp.RespondWith(http.StatusOK, "INTERPOLATED_JSON"),
							),
						)
						server.HTTPTestServer.StartTLS()

						credhubLocation := `{ "credhub-uri": "` + server.URL() + `"}`
						launcherCmd.Env = append(launcherCmd.Env, "VCAP_PLATFORM_OPTIONS="+credhubLocation)
					})

					It("returns an error", func() {
						Eventually(session).Should(gexec.Exit(4))
						Eventually(session.Err).Should(gbytes.Say("Unable to interpolate credhub references"))
						Eventually(session.Err).Should(gbytes.Say("certificate signed by unknown authority"))
					})
				})

				Context("when invalid instance identity credentials are supplied", func() {
					Context("when one of the environment variables is not set", func() {
						BeforeEach(func() {
							launcherCmd.Env = append(launcherCmd.Env, "CF_INSTANCE_CERT=")
						})

						It("prints an error message", func() {
							Eventually(session).Should(gexec.Exit(4))
							Eventually(session.Err).Should(gbytes.Say("Unable to load instance identity credentials"))
						})
					})

					Context("when unauthorized", func() {
						BeforeEach(func() {
							server.AppendHandlers(
								ghttp.CombineHandlers(
									ghttp.VerifyRequest("POST", "/api/v1/interpolate"),
									ghttp.VerifyBody([]byte(vcapServicesValue)),
									ghttp.RespondWith(http.StatusUnauthorized, `{"error": "The provided certificate is not authorized to be used for client authentication."}`),
								))
						})

						It("prints an error message", func() {
							Eventually(session).Should(gexec.Exit(4))
							Eventually(session.Err).Should(gbytes.Say("Unable to interpolate credhub references: The provided certificate is not authorized to be used for client authentication."))
						})
					})

					Context("when no instance identity credentials are supplied", func() {
						BeforeEach(func() {
							launcherCmd.Env = append(launcherCmd.Env, "CF_INSTANCE_CERT=", "CF_INSTANCE_KEY=")
						})

						It("prints an error message", func() {
							Eventually(session).Should(gexec.Exit(4))
							Eventually(session.Err).Should(gbytes.Say("Unable to load instance identity credentials"))
						})

						Context("and the app doesn't have credhub in its service bindings", func() {
							BeforeEach(func() {
								vcapServicesValue = `{"my-server":[{"credentials":{"other-cred-manager":"(//my-server/creds)"}}]}`
								launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf("VCAP_SERVICES=%s", vcapServicesValue))
							})

							It("launches the app successfully", func() {
								Eventually(session).Should(gexec.Exit(0))
								Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("VCAP_SERVICES=%s", regexp.QuoteMeta(vcapServicesValue))))
							})
						})
					})

					Context("when credhub interpolation fails on any other error case", func() {
						BeforeEach(func() {
							server.AppendHandlers(
								ghttp.CombineHandlers(
									ghttp.VerifyRequest("POST", "/api/v1/interpolate"),
									ghttp.VerifyBody([]byte(vcapServicesValue)),
									ghttp.RespondWith(http.StatusInternalServerError, "{}"),
								))
						})

						It("prints an error message", func() {
							Eventually(session).Should(gexec.Exit(4))
							Eventually(session.Err).Should(gbytes.Say("Unable to interpolate credhub references"))
						})

						Context("and the app doesn't have credhub in its service bindings", func() {
							BeforeEach(func() {
								vcapServicesValue = `{"my-server":[{"credentials":{"other-cred-manager":"(//my-server/creds)"}}]}`
								launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf("VCAP_SERVICES=%s", vcapServicesValue))
							})

							It("launches the app successfully", func() {
								Eventually(session).Should(gexec.Exit(0))
								Eventually(session.Out).Should(gbytes.Say(fmt.Sprintf("VCAP_SERVICES=%s", regexp.QuoteMeta(vcapServicesValue))))
							})
						})
					})

				})
			})

			Context("when the credhub location is not passed to the launcher", func() {
				BeforeEach(func() {
					launcherCmd.Args = []string{
						"launcher",
						appDir,
						startCommand,
						"{}",
					}
					launcherCmd.Env = append(launcherCmd.Env, "VCAP_PLATFORM_OPTIONS={}")
				})

				It("does not attempt to do any credhub interpolation", func() {
					Eventually(session).Should(gexec.Exit(0))
					Eventually(string(session.Out.Contents())).Should(ContainSubstring(fmt.Sprintf("VCAP_SERVICES=%s", vcapServicesValue)))
				})
			})

			Context("when the platform options is missing", func() {
				BeforeEach(func() {
					launcherCmd.Args = []string{
						"launcher",
						appDir,
						startCommand,
						"{}",
					}
				})

				It("does not attempt to do any credhub interpolation", func() {
					Eventually(session).Should(gexec.Exit(0))
					Eventually(string(session.Out.Contents())).Should(ContainSubstring(fmt.Sprintf("VCAP_SERVICES=%s", vcapServicesValue)))
				})
			})

			Context("when a platform options with invalid JSON is passed to the launcher", func() {
				BeforeEach(func() {
					launcherCmd.Args = []string{
						"launcher",
						appDir,
						startCommand,
						"{}",
					}
					launcherCmd.Env = append(launcherCmd.Env, `VCAP_PLATFORM_OPTIONS={"credhub-uri":"missing quote and brace`)
				})

				It("prints an error message", func() {
					Eventually(session).Should(gexec.Exit(3))
					Eventually(session.Err).Should(gbytes.Say("Invalid platform options"))
				})
			})
		})
	})

	Context("when no start command is given", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"",
				`{ "cmd": ["/bin/sh", "-c", "env; echo running app"] }`,
			}
		})

		ItExecutesTheCommandWithTheRightEnvironment()
	})

	Context("when both an entrypoint and a cmd are in the metadata", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"",
				`{ "entrypoint": ["/bin/echo"], "cmd": ["abc"] }`,
			}
		})

		It("includes the entrypoint before the cmd args", func() {
			Eventually(session).Should(gbytes.Say("abc"))
		})
	})

	Context("when an entrypoint, a cmd, and a workdir are all in the metadata", func() {
		BeforeEach(func() {
			workdir = "/bin"
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"",
				fmt.Sprintf(`{ "entrypoint": ["./echo"], "cmd": ["abc"], "workdir" : "%s"}`, workdir),
			}
		})

		It("runs the composite command in the workdir", func() {
			Eventually(session).Should(gbytes.Say("abc"))
		})
	})

	Context("when no start command or execution metadata is present", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"",
				`{}`,
			}
		})

		It("errors", func() {
			Eventually(session.Err).Should(gbytes.Say("No start command found or specified"))
		})
	})

	ItPrintsUsageInformation := func() {
		It("prints usage information", func() {
			Eventually(session.Err).Should(gbytes.Say("Usage: launcher <ignored> <start command> <metadata>"))
			Eventually(session).Should(gexec.Exit(1))
		})
	}

	Context("when no arguments are given", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
			}
		})

		ItPrintsUsageInformation()
	})

	Context("when the start command and metadata are missing", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
			}
		})

		ItPrintsUsageInformation()
	})

	Context("when the metadata is missing", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"env",
			}
		})

		ItPrintsUsageInformation()
	})

	Context("when the given execution metadata is not valid JSON", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"",
				"{ not-valid-json }",
			}
		})

		It("prints an error message", func() {
			Eventually(session.Err).Should(gbytes.Say("Invalid metadata"))
			Eventually(session).Should(gexec.Exit(1))
		})
	})

	Context("when no start command is given, and exec fails", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"",
				`{ "cmd": ["/bin/sh", "-c", "exit 9"] }`,
			}
		})

		It("correctly bubbles non-zero exit codes", func() {
			Eventually(session).Should(gexec.Exit(9))
		})
	})
})
