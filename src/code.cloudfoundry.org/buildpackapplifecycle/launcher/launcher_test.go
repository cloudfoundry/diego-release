package main_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"code.cloudfoundry.org/buildpackapplifecycle/buildpackrunner"
	"code.cloudfoundry.org/buildpackapplifecycle/containerpath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Launcher", func() {
	var extractDir string
	var appDir string
	var launcherCmd *exec.Cmd
	var session *gexec.Session
	var startCommand string

	removeFromLauncherEnv := func(keys ...string) {
		var newEnv []string
		for _, env := range launcherCmd.Env {
			found := false
			for _, key := range keys {
				if strings.HasPrefix(env, key) {
					found = true
					break
				}
			}
			if !found {
				newEnv = append(newEnv, env)
			}
		}
		launcherCmd.Env = newEnv
	}

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			startCommand = "cmd /C set && echo PWD=%cd% && echo running app && " + hello
		} else {
			startCommand = "env; echo running app; " + hello
		}

		var err error
		extractDir, err = os.MkdirTemp("", "vcap")
		Expect(err).NotTo(HaveOccurred())

		appDir = filepath.Join(extractDir, "app")
		Expect(os.MkdirAll(appDir, 0755)).To(Succeed())

		launcherCmd = &exec.Cmd{
			Path: launcher,
			Dir:  extractDir,
			Args: []string{"-credhubRetryDelay=0s"},
			Env: append(
				os.Environ(),
				"CALLERENV=some-value",
				"TEST_CREDENTIAL_FILTER_WHITELIST=CALLERENV,DEPS_DIR,VCAP_APPLICATION,VCAP_SERVICES,A,B,C,INSTANCE_GUID,INSTANCE_INDEX,PORT,DATABASE_URL",
				"PORT=8080",
				"INSTANCE_GUID=some-instance-guid",
				"INSTANCE_INDEX=123",
				`VCAP_APPLICATION={"foo":1}`,
			),
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(extractDir)).To(Succeed())
	})

	JustBeforeEach(func() {
		var err error
		session, err = gexec.Start(launcherCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
	})

	var ItExecutesTheCommandWithTheRightEnvironment = func() {
		It("executes with the environment of the caller", func() {
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring("CALLERENV=some-value"))
		})

		It("executes the start command with $HOME as the given dir", func() {
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring("HOME=" + appDir))
		})

		It("changes to the app directory when running", func() {
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring("PWD=" + appDir))
		})

		It("executes the start command with $TMPDIR as the extract directory + /tmp", func() {
			absDir, err := filepath.Abs(filepath.Join(appDir, "..", "tmp"))
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring("TMPDIR=" + absDir))
		})

		It("executes the start command with $DEPS_DIR as the extract directory + /deps", func() {
			absDir, err := filepath.Abs(filepath.Join(appDir, "..", "deps"))
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring("DEPS_DIR=" + absDir))
		})

		It("munges VCAP_APPLICATION appropriately", func() {
			Eventually(session).Should(gexec.Exit(0))

			vcapAppPattern := regexp.MustCompile("VCAP_APPLICATION=(.*)")
			vcapApplicationBytes := vcapAppPattern.FindSubmatch(session.Out.Contents())[1]

			vcapApplication := map[string]interface{}{}
			Expect(json.Unmarshal(vcapApplicationBytes, &vcapApplication)).To(Succeed())

			Expect(vcapApplication["host"]).To(Equal("0.0.0.0"))
			Expect(vcapApplication["port"]).To(Equal(float64(8080)))
			Expect(vcapApplication["instance_index"]).To(Equal(float64(123)))
			Expect(vcapApplication["instance_id"]).To(Equal("some-instance-guid"))
			Expect(vcapApplication["foo"]).To(Equal(float64(1)))
		})

		Context("when the given dir has .profile.d with scripts in it", func() {
			BeforeEach(func() {
				profileDir := filepath.Join(appDir, ".profile.d")

				Expect(os.MkdirAll(profileDir, 0755)).To(Succeed())

				if runtime.GOOS == "windows" {
					Expect(os.WriteFile(filepath.Join(profileDir, "a.bat"), []byte("@echo off\necho sourcing a.bat\nset A=1\n"), 0644)).To(Succeed())
					Expect(os.WriteFile(filepath.Join(profileDir, "b.bat"), []byte("@echo off\necho sourcing b.bat\nset B=1\n"), 0644)).To(Succeed())
					Expect(os.WriteFile(filepath.Join(appDir, ".profile.bat"), []byte("@echo off\necho sourcing .profile.bat\nset C=%A%%B%\n"), 0644)).To(Succeed())
				} else {
					Expect(os.WriteFile(filepath.Join(profileDir, "a.sh"), []byte("echo sourcing a.sh\nexport A=1\n"), 0644)).To(Succeed())
					Expect(os.WriteFile(filepath.Join(profileDir, "b.sh"), []byte("echo sourcing b.sh\nexport B=1\n"), 0644)).To(Succeed())
					Expect(os.WriteFile(filepath.Join(appDir, ".profile"), []byte("echo sourcing .profile\nexport C=$A$B\n"), 0644)).To(Succeed())
				}
			})

			It("sources them before sourcing .profile and before executing", func() {
				Eventually(session).Should(gexec.Exit(0))
				if runtime.GOOS == "windows" {
					Eventually(session).Should(gbytes.Say("sourcing a.bat"))
					Eventually(session).Should(gbytes.Say("sourcing b.bat"))
					Eventually(session).Should(gbytes.Say("sourcing .profile.bat"))
				} else {
					Eventually(session).Should(gbytes.Say("sourcing a.sh"))
					Eventually(session).Should(gbytes.Say("sourcing b.sh"))
					Eventually(session).Should(gbytes.Say("sourcing .profile"))
				}
				Eventually(string(session.Out.Contents())).Should(ContainSubstring("B=1"))
				Eventually(string(session.Out.Contents())).Should(ContainSubstring("C=11"))
				Eventually(string(session.Out.Contents())).Should(ContainSubstring("running app"))
			})

			It("prints to the app/task logs when profile scripts are being invoked", func() {
				Eventually(session).Should(gexec.Exit(0))
				Eventually(session).Should(gbytes.Say("Invoking pre-start scripts."))
			})

			It("prints to the app/task logs when the entrypoint command is being invoked", func() {
				Eventually(session).Should(gexec.Exit(0))
				Eventually(session).Should(gbytes.Say("Invoking start command."))
			})

			Context("hello is on path", func() {
				BeforeEach(func() {
					profileDir := filepath.Join(appDir, ".profile.d")
					Expect(os.MkdirAll(profileDir, 0755)).To(Succeed())

					destDir := filepath.Join(appDir, "tmp")
					Expect(os.MkdirAll(destDir, 0777)).To(Succeed())

					helloFilename, err := copyExe(destDir, hello)
					Expect(err).NotTo(HaveOccurred())

					if runtime.GOOS == "windows" {
						Expect(os.WriteFile(filepath.Join(profileDir, "a.bat"), []byte(fmt.Sprintf("@echo off\necho sourcing a.bat\nset PATH=%%PATH%%;%s\n", destDir)), 0644)).To(Succeed())
					} else {
						Expect(os.WriteFile(filepath.Join(profileDir, "a.sh"), []byte(fmt.Sprintf("echo sourcing a.sh\nexport PATH=$PATH:%s\n", destDir)), 0644)).To(Succeed())
					}

					launcherCmd.Args = []string{
						"launcher",
						appDir,
						helloFilename,
						`{ "start_command": "echo should not run this" }`,
					}
				})

				It("finds the app executable", func() {
					Eventually(session).Should(gexec.Exit(0))
					if runtime.GOOS == "windows" {
						Eventually(session).Should(gbytes.Say("sourcing a.bat"))
					} else {
						Eventually(session).Should(gbytes.Say("sourcing a.sh"))
					}
					Expect(string(session.Out.Contents())).To(ContainSubstring("app is running"))
				})
			})
		})

		Context("when the given dir does not have .profile.d", func() {
			It("does not report errors about missing .profile.d", func() {
				Eventually(session).Should(gexec.Exit(0))
				Expect(string(session.Err.Contents())).To(BeEmpty())
			})
		})

		Context("when the given dir has an empty .profile.d", func() {
			BeforeEach(func() {
				Expect(os.MkdirAll(filepath.Join(appDir, ".profile.d"), 0755)).To(Succeed())
			})

			It("does not report errors about missing .profile.d", func() {
				Eventually(session).Should(gexec.Exit(0))
				Expect(string(session.Err.Contents())).To(BeEmpty())
			})
		})

		Context("when the given dir has ../profile.d with scripts in it", func() {
			BeforeEach(func() {
				profileDir := filepath.Join(appDir, "..", "profile.d")

				Expect(os.MkdirAll(profileDir, 0755)).To(Succeed())

				if runtime.GOOS == "windows" {
					Expect(os.WriteFile(filepath.Join(profileDir, "a.bat"), []byte("@echo off\necho sourcing a.bat\nset A=1\n"), 0644)).To(Succeed())
					Expect(os.WriteFile(filepath.Join(profileDir, "b.bat"), []byte("@echo off\necho sourcing b.bat\nset B=1\n"), 0644)).To(Succeed())
					Expect(os.MkdirAll(filepath.Join(appDir, ".profile.d"), 0755)).To(Succeed())
					Expect(os.WriteFile(filepath.Join(appDir, ".profile.d", "c.bat"), []byte("@echo off\necho sourcing c.bat\nset C=%A%%B%\n"), 0644)).To(Succeed())
				} else {
					Expect(os.WriteFile(filepath.Join(profileDir, "a.sh"), []byte("echo sourcing a.sh\nexport A=1\n"), 0644)).To(Succeed())
					Expect(os.WriteFile(filepath.Join(profileDir, "b.sh"), []byte("echo sourcing b.sh\nexport B=1\n"), 0644)).To(Succeed())
					Expect(os.MkdirAll(filepath.Join(appDir, ".profile.d"), 0755)).To(Succeed())
					Expect(os.WriteFile(filepath.Join(appDir, ".profile.d", "c.sh"), []byte("echo sourcing c.sh\nexport C=$A$B\n"), 0644)).To(Succeed())
				}
			})

			It("sources them before sourcing .profile.d/* and before executing", func() {
				Eventually(session).Should(gexec.Exit(0))
				if runtime.GOOS == "windows" {
					Eventually(session).Should(gbytes.Say("sourcing a.bat"))
					Eventually(session).Should(gbytes.Say("sourcing b.bat"))
					Eventually(session).Should(gbytes.Say("sourcing c.bat"))
				} else {
					Eventually(session).Should(gbytes.Say("sourcing a.sh"))
					Eventually(session).Should(gbytes.Say("sourcing b.sh"))
					Eventually(session).Should(gbytes.Say("sourcing c.sh"))
				}
				Eventually(string(session.Out.Contents())).Should(ContainSubstring("A=1"))
				Eventually(string(session.Out.Contents())).Should(ContainSubstring("B=1"))
				Eventually(string(session.Out.Contents())).Should(ContainSubstring("C=11"))
				Eventually(string(session.Out.Contents())).Should(ContainSubstring("running app"))
			})
		})

		Context("when the given dir does not have ../profile.d", func() {
			It("does not report errors about missing ../profile.d", func() {
				Eventually(session).Should(gexec.Exit(0))
				Expect(string(session.Err.Contents())).To(BeEmpty())
			})
		})

		Context("when the given dir has an empty ../profile.d", func() {
			BeforeEach(func() {
				Expect(os.MkdirAll(filepath.Join(appDir, "../profile.d"), 0755)).To(Succeed())
			})

			It("does not report errors about missing ../profile.d", func() {
				Eventually(session).Should(gexec.Exit(0))
				Expect(string(session.Err.Contents())).To(BeEmpty())
			})
		})
	}

	Context("the app executable is in vcap/app", func() {
		BeforeEach(func() {
			helloFilename, err := copyExe(appDir, hello)
			Expect(err).NotTo(HaveOccurred())

			var executable string
			if runtime.GOOS == "windows" {
				executable = fmt.Sprintf(".\\%s", helloFilename)
			} else {
				executable = fmt.Sprintf("./%s", helloFilename)
			}
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				executable,
				`{ "start_command": "echo should not run this" }`,
			}
		})

		It("finds the app executable", func() {
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring("app is running"))
		})
	})

	Context("the app executable path contains a space", func() {
		BeforeEach(func() {
			if runtime.GOOS != "windows" {
				Skip("file/directory names with spaces should be escaped on non-Windows OSes")
			}

			appDirWithSpaces := filepath.Join(appDir, "space dir")
			Expect(os.MkdirAll(appDirWithSpaces, 0755)).To(Succeed())

			helloFilename, err := copyExe(appDirWithSpaces, hello)
			Expect(err).NotTo(HaveOccurred())

			launcherCmd.Args = []string{
				"launcher",
				appDir,
				filepath.Join(appDirWithSpaces, helloFilename),
				`{ "start_command": "echo should not run this" }`,
			}
		})

		It("finds the app executable", func() {
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(ContainSubstring("app is running"))
		})
	})

	Context("when the app exits", func() {
		BeforeEach(func() {
			if runtime.GOOS == "windows" {
				startCommand = "cmd.exe /C exit 26"
			} else {
				startCommand = "exit 26"
			}

			launcherCmd.Args = []string{
				"launcher",
				appDir,
				startCommand,
				`{ "start_command": "echo should not run this" }`,
			}
		})

		It("exits with the exit code of the app", func() {
			Eventually(session).Should(gexec.Exit(26))
		})
	})

	Context("when the start command starts a subprocess", func() {
		Context("the subprocess outputs to stdout", func() {
			BeforeEach(func() {
				launcherCmd.Args = []string{
					"launcher",
					appDir,
					startCommand,
					`{ "start_command": "echo should not run this" }`,
				}
			})

			It("captures stdout", func() {
				Eventually(session).Should(gexec.Exit(0))
				Expect(string(session.Out.Contents())).To(ContainSubstring("app is running"))
			})
		})

		Context("the subprocess outputs to stderr", func() {
			BeforeEach(func() {
				launcherCmd.Args = []string{
					"launcher",
					appDir,
					startCommand + " 1>&2",
					`{ "start_command": "echo should not run this" }`,
				}
			})

			It("captures stderr", func() {
				Eventually(session).Should(gexec.Exit(0))
				Expect(string(session.Err.Contents())).To(ContainSubstring("app is running"))
			})
		})
	})

	Context("when a start command is given", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				startCommand,
				`{ "start_command": "echo should not run this" }`,
			}
		})

		ItExecutesTheCommandWithTheRightEnvironment()

		Context("when the staging_info.yml specifies an entrypoint prefix", func() {
			Context("When running on Windows", func() {
				BeforeEach(func() {
					if runtime.GOOS != "windows" {
						Skip("entrypoint_prefix is supported, skipping windows specific 'anti-tests'")
					}
				})

				Context("when the custom entrypoint is an absolute path", func() {
					BeforeEach(func() {
						destDir := filepath.Join(appDir, "tmp")
						Expect(os.MkdirAll(destDir, 0777)).To(Succeed())
						customEntrypointFile, err := copyExe(destDir, customEntrypoint)
						Expect(err).NotTo(HaveOccurred())

						writeStagingInfo(extractDir, fmt.Sprintf("config:\n  entrypoint_prefix: %s", filepath.Join(destDir, customEntrypointFile)))
					})

					It("ignores the custom entrypoint and executes the start command", func() {
						Eventually(session).Should(gexec.Exit(0))
						Expect(string(session.Out.Contents())).To(ContainSubstring("app is running"))
						Expect(string(session.Out.Contents())).NotTo(ContainSubstring("I'm a custom entrypoint"))
						Expect(string(session.Out.Contents())).NotTo(ContainSubstring(fmt.Sprintf("I was called with: '[%s]'", startCommand)))
					})
				})
			})

			Context("when running on Linux", func() {
				BeforeEach(func() {
					if runtime.GOOS != "linux" {
						Skip("entrypoint_prefix not supported on windows")
					}
				})

				Context("when the custom entrypoint is an absolute path", func() {
					BeforeEach(func() {
						destDir := filepath.Join(appDir, "tmp")
						Expect(os.MkdirAll(destDir, 0777)).To(Succeed())
						customEntrypointFile, err := copyExe(destDir, customEntrypoint)
						Expect(err).NotTo(HaveOccurred())

						writeStagingInfo(extractDir, fmt.Sprintf("config:\n  entrypoint_prefix: %s", filepath.Join(destDir, customEntrypointFile)))
					})

					It("invokes the custom entrypoint and passes the start command to it", func() {
						Eventually(session).Should(gexec.Exit(0))
						Expect(string(session.Out.Contents())).To(ContainSubstring("I'm a custom entrypoint"))
						Expect(string(session.Out.Contents())).To(ContainSubstring(fmt.Sprintf("I was called with: '[%s]'", startCommand)))
					})
				})

				Context("when the custom entrypoint is on the PATH", func() {
					BeforeEach(func() {
						profileDir := filepath.Join(appDir, ".profile.d")
						Expect(os.MkdirAll(profileDir, 0755)).To(Succeed())

						destDir := filepath.Join(appDir, "tmp")
						Expect(os.MkdirAll(destDir, 0777)).To(Succeed())

						customEntrypointFile, err := copyExe(destDir, customEntrypoint)
						Expect(err).NotTo(HaveOccurred())

						writeStagingInfo(extractDir, fmt.Sprintf("config:\n  entrypoint_prefix: %s", customEntrypointFile))

						Expect(os.WriteFile(filepath.Join(profileDir, "set_path.sh"), []byte(fmt.Sprintf("echo Setting path\nexport PATH=$PATH:%s\n", destDir)), 0644)).To(Succeed())
					})

					It("invokes the custom entrypoint and passes the start command to it", func() {
						Eventually(session).Should(gexec.Exit(0))
						Expect(string(session.Out.Contents())).To(ContainSubstring("I'm a custom entrypoint"))
						Expect(string(session.Out.Contents())).To(ContainSubstring(fmt.Sprintf("I was called with: '[%s]'", startCommand)))
					})
				})

				Context("when the custom entrypoint cannot be found", func() {
					var executableName string

					BeforeEach(func() {
						executableName = filepath.Base(customEntrypoint)
						writeStagingInfo(extractDir, fmt.Sprintf("config:\n  entrypoint_prefix: \"%s\"", executableName))
					})

					It("reports 'command not found' and continues execution of the start command", func() {
						Eventually(session).Should(gexec.Exit(127))

						Expect(string(session.Err.Contents())).To(ContainSubstring(fmt.Sprintf("exec: %s: not found", executableName)))

						Expect(string(session.Out.Contents())).NotTo(ContainSubstring("I'm a custom entrypoint"))
						Expect(string(session.Out.Contents())).NotTo(ContainSubstring("I was called with:"))
					})
				})
			})
		})
	})

	Describe("interpolation of credhub-ref in VCAP_SERVICES", func() {
		var (
			server         *ghttp.Server
			fixturesSslDir string
			err            error
		)

		VerifyClientCerts := func() http.HandlerFunc {
			return func(w http.ResponseWriter, req *http.Request) {
				tlsConnectionState := req.TLS
				Expect(tlsConnectionState).NotTo(BeNil())
				Expect(tlsConnectionState.PeerCertificates).NotTo(BeEmpty())
				Expect(tlsConnectionState.PeerCertificates[0].Subject.CommonName).To(Equal("client"))
			}
		}

		BeforeEach(func() {
			fixturesSslDir, err = filepath.Abs(filepath.Join("..", "fixtures"))
			Expect(err).NotTo(HaveOccurred())

			cert, err := tls.LoadX509KeyPair(filepath.Join(fixturesSslDir, "certs", "server.crt"), filepath.Join(fixturesSslDir, "certs", "server.key"))
			Expect(err).NotTo(HaveOccurred())

			caCerts := x509.NewCertPool()

			caCertBytes, err := os.ReadFile(filepath.Join(fixturesSslDir, "cacerts", "CA.crt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(caCerts.AppendCertsFromPEM(caCertBytes)).To(BeTrue())

			server = ghttp.NewUnstartedServer()
			server.HTTPTestServer.TLS = &tls.Config{
				ClientAuth:   tls.RequireAndVerifyClientCert,
				Certificates: []tls.Certificate{cert},
				ClientCAs:    caCerts,
			}
			server.HTTPTestServer.StartTLS()

			removeFromLauncherEnv("USERPROFILE")
			launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf("USERPROFILE=%s", fixturesSslDir))
			cpath := containerpath.New(fixturesSslDir)
			if cpath.For("/") == fixturesSslDir {
				launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf("CF_INSTANCE_CERT=%s", filepath.Join("/certs", "client.crt")))
				launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf("CF_INSTANCE_KEY=%s", filepath.Join("/certs", "client.key")))
				launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf("CF_SYSTEM_CERT_PATH=%s", "/cacerts"))
			} else {
				launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf("CF_INSTANCE_CERT=%s", filepath.Join(fixturesSslDir, "certs", "client.crt")))
				launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf("CF_INSTANCE_KEY=%s", filepath.Join(fixturesSslDir, "certs", "client.key")))
				launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf("CF_SYSTEM_CERT_PATH=%s", filepath.Join(fixturesSslDir, "cacerts")))
			}

			launcherCmd.Args = []string{
				"launcher",
				appDir,
				startCommand,
				"-credhubRetryDelay=0s",
				"-credhubConnectAttempts=1",
			}
		})

		AfterEach(func() {
			server.Close()
		})

		Context("when VCAP_SERVICES contains credhub refs", func() {
			var vcapServicesValue string
			var vcapPlatformOptionsValue string

			BeforeEach(func() {
				vcapServicesValue = `{"my-server":[{"credentials":{"credhub-ref":"(//my-server/creds)"}}]}`
				launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf("VCAP_SERVICES=%s", vcapServicesValue))
			})

			Context("when the credhub location is passed to the launcher's platform options", func() {
				BeforeEach(func() {
					vcapPlatformOptionsValue = `{ "credhub-uri": "` + server.URL() + `"}`
					launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf(`VCAP_PLATFORM_OPTIONS=%s`, vcapPlatformOptionsValue))
				})

				Context("when credhub successfully interpolates", func() {
					BeforeEach(func() {
						server.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("POST", "/api/v1/interpolate"),
								ghttp.VerifyBody([]byte(vcapServicesValue)),
								VerifyClientCerts(),
								ghttp.RespondWith(http.StatusOK, "INTERPOLATED_JSON"),
							))
					})

					It("updates VCAP_SERVICES with the interpolated content and runs the process without VCAP_PLATFORM_OPTIONS", func() {
						Eventually(session).Should(gexec.Exit(0))
						Eventually(session.Out).Should(gbytes.Say("VCAP_SERVICES=INTERPOLATED_JSON"))
						Eventually(session.Out).ShouldNot(gbytes.Say("VCAP_PLATFORM_OPTIONS"))
					})
				})

				Context("when credhub fails", func() {
					BeforeEach(func() {
						for attempt := 1; attempt <= 3; attempt++ {
							server.AppendHandlers(
								ghttp.CombineHandlers(
									ghttp.VerifyRequest("POST", "/api/v1/interpolate"),
									ghttp.VerifyBody([]byte(vcapServicesValue)),
									ghttp.RespondWith(http.StatusInternalServerError, "{}"),
								))
						}
					})

					It("prints an error message", func() {
						Eventually(session).Should(gexec.Exit(3))
						Eventually(session.Err).Should(gbytes.Say("Unable to interpolate credhub references"))
					})
				})
			})

			Context("when an empty string is passed for the launcher platform options", func() {
				BeforeEach(func() {
					launcherCmd.Env = append(launcherCmd.Env, `VCAP_PLATFORM_OPTIONS=`)
				})

				It("does not attempt to do any credhub interpolation", func() {
					Eventually(session).Should(gexec.Exit(0))
					Eventually(string(session.Out.Contents())).Should(ContainSubstring(fmt.Sprintf("VCAP_SERVICES=%s", vcapServicesValue)))
					Eventually(session.Out).ShouldNot(gbytes.Say("VCAP_PLATFORM_OPTIONS"))
				})
			})

			Context("when an empty JSON is passed for the launcher platform options", func() {
				BeforeEach(func() {
					launcherCmd.Env = append(launcherCmd.Env, `VCAP_PLATFORM_OPTIONS={}`)
				})

				It("does not attempt to do any credhub interpolation", func() {
					Eventually(session).Should(gexec.Exit(0))
					Eventually(string(session.Out.Contents())).Should(ContainSubstring(fmt.Sprintf("VCAP_SERVICES=%s", vcapServicesValue)))
					Eventually(session.Out).ShouldNot(gbytes.Say("VCAP_PLATFORM_OPTIONS"))
				})
			})

			Context("when invalid JSON is passed for the launcher platform options", func() {
				BeforeEach(func() {
					launcherCmd.Env = append(launcherCmd.Env, `VCAP_PLATFORM_OPTIONS='{"credhub-uri":"missing quote and brace'`)
				})

				It("prints an error message", func() {
					Eventually(session).Should(gexec.Exit(3))
					Eventually(session.Err).Should(gbytes.Say("Invalid platform options"))
				})
			})
		})

		Context("VCAP_SERVICES does not have an appropriate credential", func() {
			It("DATABASE_URL is not set", func() {
				Eventually(session).Should(gexec.Exit(0))
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("DATABASE_URL="))
			})
		})

		Context("VCAP_SERVICES has an appropriate credential", func() {
			const databaseURL = "postgres://thing.com/special"

			BeforeEach(func() {
				vcapServicesValue := `{"my-server":[{"credentials":{"credhub-ref":"(//my-server/creds)"}}]}`
				vcapPlatformOptionsValue := `{ "credhub-uri": "` + server.URL() + `"}`
				launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf(`VCAP_PLATFORM_OPTIONS=%s`, vcapPlatformOptionsValue))
				launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf("VCAP_SERVICES=%s", vcapServicesValue))
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/api/v1/interpolate"),
						ghttp.RespondWith(http.StatusOK, `{"my-server":[{"credentials":{"uri":"`+databaseURL+`"}}]}`),
					))
			})

			It("sets DATABASE_URL", func() {
				Eventually(session).Should(gexec.Exit(0))
				Eventually(string(session.Out.Contents())).Should(ContainSubstring(fmt.Sprintf("DATABASE_URL=%s", databaseURL)))
			})

			Context("DATABASE_URL was set before running launcher", func() {
				BeforeEach(func() {
					launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf("DATABASE_URL=%s", "original content"))
				})

				It("overrides DATABASE_URL", func() {
					Eventually(session).Should(gexec.Exit(0))
					Eventually(string(session.Out.Contents())).Should(ContainSubstring(fmt.Sprintf("DATABASE_URL=%s", databaseURL)))
					Expect(string(session.Out.Contents())).ToNot(ContainSubstring("DATABASE_URL=original content"))
				})
			})
		})
	})

	Describe("setting DATABASE_URL env variable", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				startCommand,
				"",
				"",
			}
		})

		Context("VCAP_SERVICES does not have an appropriate credential", func() {
			It("DATABASE_URL is not set", func() {
				Eventually(session).Should(gexec.Exit(0))
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("DATABASE_URL="))
			})
		})

		Context("VCAP_SERVICES has an appropriate credential", func() {
			Context("VCAP_SERVICES is NOT encrypted", func() {
				const databaseURL = "postgres://thing.com/special"
				BeforeEach(func() {
					vcapServicesValue := `{"my-server":[{"credentials":{"uri":"` + databaseURL + `"}}]}`
					launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf("VCAP_SERVICES=%s", vcapServicesValue))
				})

				It("sets DATABASE_URL", func() {
					Eventually(session).Should(gexec.Exit(0))
					Eventually(string(session.Out.Contents())).Should(ContainSubstring(fmt.Sprintf("DATABASE_URL=%s", databaseURL)))
				})

				Context("DATABASE_URL was set before running builder", func() {
					BeforeEach(func() {
						launcherCmd.Env = append(launcherCmd.Env, fmt.Sprintf("DATABASE_URL=%s", "original text"))
					})

					It("overrides DATABASE_URL", func() {
						Eventually(session).Should(gexec.Exit(0))
						Eventually(string(session.Out.Contents())).Should(ContainSubstring(fmt.Sprintf("DATABASE_URL=%s", databaseURL)))
						Expect(string(session.Out.Contents())).ToNot(ContainSubstring("DATABASE_URL=original content"))
					})
				})
			})
		})
	})

	var ItPrintsMissingStartCommandInformation = func() {
		It("fails and reports no start command", func() {
			Eventually(session).Should(gexec.Exit(1))
			Eventually(session.Err).Should(gbytes.Say("launcher: no start command specified or detected in droplet"))
		})
	}

	Context("when no start command is given", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"",
				"",
			}
		})

		Context("when the app package does not contain staging_info.yml", func() {
			ItPrintsMissingStartCommandInformation()
		})

		Context("when the app package has a staging_info.yml", func() {

			Context("when it is missing a start command", func() {
				BeforeEach(func() {
					writeStagingInfo(extractDir, "detected_buildpack: Ruby")
				})

				ItPrintsMissingStartCommandInformation()
			})

			Context("when it contains a start command", func() {
				BeforeEach(func() {
					writeStagingInfo(extractDir, "detected_buildpack: Ruby\nstart_command: "+startCommand)
				})

				ItExecutesTheCommandWithTheRightEnvironment()
			})

			Context("when it references unresolvable types in non-essential fields", func() {
				BeforeEach(func() {
					writeStagingInfo(
						extractDir,
						"---\nbuildpack_path: !ruby/object:Pathname\n  path: /tmp/buildpacks/null-buildpack\ndetected_buildpack: \nstart_command: "+startCommand+"\n",
					)
				})

				ItExecutesTheCommandWithTheRightEnvironment()
			})

			Context("when it is not valid YAML", func() {
				BeforeEach(func() {
					writeStagingInfo(extractDir, "start_command: &ruby/object:Pathname")
				})

				It("prints an error message", func() {
					Eventually(session).Should(gexec.Exit(1))
					Eventually(session.Err).Should(gbytes.Say("Invalid staging info"))
				})
			})
		})
	})

	Context("when arguments are missing", func() {
		BeforeEach(func() {
			launcherCmd.Args = []string{
				"launcher",
				appDir,
				"env",
			}
		})

		It("fails with an indication that too few arguments were passed", func() {
			Eventually(session).Should(gexec.Exit(1))
			Eventually(session.Err).Should(gbytes.Say("launcher: received only 2 arguments\n"))
			Eventually(session.Err).Should(gbytes.Say("Usage: launcher <app-directory> <start-command> <metadata>"))
		})
	})
})

func writeStagingInfo(extractDir, stagingInfo string) {
	Expect(os.WriteFile(filepath.Join(extractDir, buildpackrunner.DeaStagingInfoFilename), []byte(stagingInfo), 0644)).To(Succeed())
}

func copyExe(dstDir, src string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()

	exeName := filepath.Base(src)

	dst := filepath.Join(dstDir, exeName)
	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return "", err
	}

	return exeName, nil
}
