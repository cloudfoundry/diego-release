package main_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"

	"code.cloudfoundry.org/buildpackapplifecycle"
	"code.cloudfoundry.org/buildpackapplifecycle/buildpackrunner"
	"code.cloudfoundry.org/buildpackapplifecycle/test_helpers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Building", func() {

	var (
		builderCmd *exec.Cmd

		tmpDir                    string
		buildDir                  string
		buildpacksDir             string
		outputDroplet             string
		buildpackOrder            string
		buildArtifactsCacheDir    string
		outputMetadata            string
		outputBuildArtifactsCache string
		skipDetect                bool

		sessionEnv []string
	)

	builder := func() *gexec.Session {
		session, err := gexec.Start(
			builderCmd,
			GinkgoWriter,
			GinkgoWriter,
		)
		Expect(err).NotTo(HaveOccurred())

		return session
	}

	cpBuildpack := func(buildpack string) {
		cp(filepath.Join(buildpackFixtures, buildpack), filepath.Join(buildpacksDir, buildpackHash(buildpack)))
	}

	BeforeEach(func() {
		var err error

		tmpDir, err = os.MkdirTemp("", "building-tmp")
		Expect(err).NotTo(HaveOccurred())

		buildDir, err = os.MkdirTemp(tmpDir, "building-app")
		Expect(err).NotTo(HaveOccurred())

		if runtime.GOOS == "windows" {
			test_helpers.CopyFile(tarPath, filepath.Join(tmpDir, "tmp", "lifecycle", "tar.exe"))
		}

		buildpacksDir, err = os.MkdirTemp(tmpDir, "building-buildpacks")
		Expect(err).NotTo(HaveOccurred())

		outputDropletFile, err := os.CreateTemp(tmpDir, "building-droplet")
		Expect(err).NotTo(HaveOccurred())
		outputDroplet = outputDropletFile.Name()
		Expect(outputDropletFile.Close()).To(Succeed())

		outputBuildArtifactsCacheDir, err := os.MkdirTemp(tmpDir, "building-cache-output")
		Expect(err).NotTo(HaveOccurred())
		outputBuildArtifactsCache = filepath.Join(outputBuildArtifactsCacheDir, "cache.tgz")

		buildArtifactsCacheDir, err = os.MkdirTemp(tmpDir, "building-cache")
		Expect(err).NotTo(HaveOccurred())

		outputMetadataFile, err := os.CreateTemp(tmpDir, "building-result")
		Expect(err).NotTo(HaveOccurred())
		outputMetadata = outputMetadataFile.Name()
		Expect(outputMetadataFile.Close()).To(Succeed())

		buildpackOrder = ""

		skipDetect = false

		sessionEnv = append(os.Environ(), "TEST_CREDENTIAL_FILTER_WHITELIST=DATABASE_URL,VCAP_SERVICES", "TMPDIR="+tmpDir)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	JustBeforeEach(func() {
		builderCmd = exec.Command(builderPath,
			"-buildDir", buildDir,
			"-buildpacksDir", buildpacksDir,
			"-outputDroplet", outputDroplet,
			"-outputBuildArtifactsCache", outputBuildArtifactsCache,
			"-buildArtifactsCacheDir", buildArtifactsCacheDir,
			"-buildpackOrder", buildpackOrder,
			"-outputMetadata", outputMetadata,
			"-skipDetect="+strconv.FormatBool(skipDetect),
			"-credhubRetryDelay=0s",
		)

		builderCmd.Env = sessionEnv
		builderCmd.Dir = tmpDir
	})

	resultJSON := func() []byte {
		resultInfo, err := os.ReadFile(outputMetadata)
		Expect(err).NotTo(HaveOccurred())

		return resultInfo
	}

	resultJSONbuildpacks := func() []byte {
		result := resultJSON()
		var stagingResult buildpackapplifecycle.StagingResult
		Expect(json.Unmarshal(result, &stagingResult)).To(Succeed())
		bytes, err := json.Marshal(stagingResult.LifecycleMetadata.Buildpacks)
		Expect(err).ToNot(HaveOccurred())
		return bytes
	}

	sessionSetEnv := func(key, value string) {
		newEnv := []string{}
		for _, env := range sessionEnv {
			if !strings.HasPrefix(env, key+"=") {
				newEnv = append(newEnv, env)
			}
		}
		sessionEnv = append(newEnv, key+"="+value)
	}

	Describe("interpolation of credhub-ref in VCAP_SERVICES", func() {
		var (
			server         *ghttp.Server
			session        *gexec.Session
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

			server = ghttp.NewUnstartedServer()

			cert, err := tls.LoadX509KeyPair(
				filepath.Join(fixturesSslDir, "certs", "server.crt"),
				filepath.Join(fixturesSslDir, "certs", "server.key"),
			)
			Expect(err).NotTo(HaveOccurred())

			caCerts := x509.NewCertPool()

			caCertBytes, err := os.ReadFile(filepath.Join(fixturesSslDir, "cacerts", "CA.crt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(caCerts.AppendCertsFromPEM(caCertBytes)).To(BeTrue())

			server.HTTPTestServer.TLS = &tls.Config{
				ClientAuth:   tls.RequireAndVerifyClientCert,
				Certificates: []tls.Certificate{cert},
				ClientCAs:    caCerts,
			}
			server.HTTPTestServer.StartTLS()

			sessionSetEnv("USERPROFILE", fixturesSslDir)
			if runtime.GOOS == "windows" {
				// windows2012
				sessionSetEnv("CF_INSTANCE_CERT", filepath.Join("/", "certs", "client.crt"))
				sessionSetEnv("CF_INSTANCE_KEY", filepath.Join("/", "certs", "client.key"))
				sessionSetEnv("CF_SYSTEM_CERT_PATH", filepath.Join("/", "cacerts"))
			} else {
				// all others
				sessionSetEnv("CF_INSTANCE_CERT", filepath.Join(fixturesSslDir, "certs", "client.crt"))
				sessionSetEnv("CF_INSTANCE_KEY", filepath.Join(fixturesSslDir, "certs", "client.key"))
				sessionSetEnv("CF_SYSTEM_CERT_PATH", filepath.Join(fixturesSslDir, "cacerts"))
			}

			buildpackOrder = "always-detects"
			cpBuildpack("always-detects")
		})

		AfterEach(func() {
			server.Close()
		})

		JustBeforeEach(func() {
			var err error
			session, err = gexec.Start(builderCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when VCAP_SERVICES contains credhub refs", func() {
			var vcapServicesValue string
			BeforeEach(func() {
				vcapServicesValue = `{"my-server":[{"credentials":{"credhub-ref":"(//my-server/creds)"}}]}`
				sessionSetEnv("VCAP_SERVICES", vcapServicesValue)
			})

			Context("when the credhub location is passed to the launcher's platform options", func() {
				BeforeEach(func() {
					sessionSetEnv("VCAP_PLATFORM_OPTIONS", `{ "credhub-uri": "`+server.URL()+`"}`)
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
						Eventually(session).Should(gexec.Exit(4))
						Eventually(session.Err).Should(gbytes.Say("Unable to interpolate credhub references"))
					})
				})
			})

			Context("when an empty string is passed for the launcher platform options", func() {
				BeforeEach(func() {
					sessionSetEnv("VCAP_PLATFORM_OPTIONS", "")
				})

				It("does not attempt to do any credhub interpolation", func() {
					Eventually(session).Should(gexec.Exit(0))
					Eventually(string(session.Out.Contents())).Should(ContainSubstring(fmt.Sprintf("VCAP_SERVICES=%s", vcapServicesValue)))
				})
			})

			Context("when an empty JSON is passed for the launcher platform options", func() {
				BeforeEach(func() {
					sessionSetEnv("VCAP_PLATFORM_OPTIONS", "{}")
				})

				It("does not attempt to do any credhub interpolation", func() {
					Eventually(session).Should(gexec.Exit(0))
					Eventually(string(session.Out.Contents())).Should(ContainSubstring(fmt.Sprintf("VCAP_SERVICES=%s", vcapServicesValue)))
				})
			})

			Context("when invalid JSON is passed for the launcher platform options", func() {
				BeforeEach(func() {
					sessionSetEnv("VCAP_PLATFORM_OPTIONS", `{"credhub-uri":"missing quote and brace`)
				})

				It("prints an error message", func() {
					Eventually(session).Should(gexec.Exit(3))
					Eventually(session.Err).Should(gbytes.Say("Invalid platform options"))
				})
			})
		})

		Context("VCAP_SERVICES has an appropriate database", func() {
			const databaseURL = "postgres://thing.com/special"
			BeforeEach(func() {
				vcapServicesValue := `{"my-server":[{"credentials":{"credhub-ref":"(//my-server/creds)"}}]}`
				sessionSetEnv("VCAP_PLATFORM_OPTIONS", `{ "credhub-uri": "`+server.URL()+`"}`)
				sessionSetEnv("VCAP_SERVICES", vcapServicesValue)
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

			Context("DATABASE_URL was set before running builder", func() {
				BeforeEach(func() {
					sessionSetEnv("DATABASE_URL", "original text")
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
		var session *gexec.Session
		BeforeEach(func() {
			buildpackOrder = "always-detects"
			cpBuildpack("always-detects")
		})

		JustBeforeEach(func() {
			var err error
			session, err = gexec.Start(builderCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("VCAP_SERVICES does not have an appropriate credential", func() {
			const databaseURL = "datastore://thing.com/special"
			BeforeEach(func() {
				vcapServicesValue := `{"my-server":[{"credentials":{"uri":"` + databaseURL + `"}}]}`
				sessionSetEnv("VCAP_SERVICES", vcapServicesValue)
			})
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
					sessionSetEnv("VCAP_SERVICES", vcapServicesValue)
				})
				It("sets DATABASE_URL", func() {
					Eventually(session).Should(gexec.Exit(0))
					Eventually(string(session.Out.Contents())).Should(ContainSubstring(fmt.Sprintf("DATABASE_URL=%s", databaseURL)))
				})
				Context("DATABASE_URL was set before running builder", func() {
					BeforeEach(func() {
						sessionSetEnv("DATABASE_URL", "original text")
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

	Context("run detect", func() {
		var session *gexec.Session
		BeforeEach(func() {
			buildpackOrder = "always-detects,also-always-detects"

			cpBuildpack("always-detects")
			cpBuildpack("also-always-detects")
			cp(path.Join(appFixtures, "bash-app", "app.sh"), buildDir)
		})

		JustBeforeEach(func() {
			session = builder()
			Eventually(session, 5*time.Second).Should(gexec.Exit(0))
		})

		Context("first buildpack detect is not executable", func() {
			BeforeEach(func() {
				if runtime.GOOS == "windows" {
					Skip("warning is unnecessary on Windows")
				}

				binDetect := filepath.Join(buildpacksDir, buildpackHash("always-detects"), "bin", "detect")
				Expect(os.Chmod(binDetect, 0644)).To(Succeed())
			})

			It("should warn that detect is not executable", func() {
				Eventually(session.Out).Should(gbytes.Say("WARNING: buildpack script '/bin/detect' is not executable"))
			})

			It("should have chosen the second buildpack detect", func() {
				data := &struct {
					LifeCycle struct {
						Key string `json:"buildpack_key"`
					} `json:"lifecycle_metadata"`
				}{}
				Expect(json.Unmarshal(resultJSON(), data)).To(Succeed())

				Expect(data.LifeCycle.Key).To(Equal("also-always-detects"))
			})

		})

		Describe("the contents of the output tgz", func() {
			var files []string

			JustBeforeEach(func() {
				result, err := exec.Command("tar", "-tzf", outputDroplet).Output()
				Expect(err).NotTo(HaveOccurred())

				files = removeTrailingSpace(strings.Split(string(result), "\n"))
			})

			It("should contain an /app dir with the contents of the compilation", func() {
				Expect(files).To(ContainElement("./app/"))
				Expect(files).To(ContainElement("./app/app.sh"))
				Expect(files).To(ContainElement("./app/compiled"))
			})

			It("should contain an empty /tmp directory", func() {
				Expect(files).To(ContainElement("./tmp/"))
				Expect(files).NotTo(ContainElement(MatchRegexp("\\./tmp/.+")))
			})

			It("should contain an empty /logs directory", func() {
				Expect(files).To(ContainElement("./logs/"))
				Expect(files).NotTo(ContainElement(MatchRegexp("\\./logs/.+")))
			})

			It("should contain a staging_info.yml with the detected buildpack", func() {
				stagingInfo, err := exec.Command("tar", "-xzf", outputDroplet, "-O", fmt.Sprintf("./%s", buildpackrunner.DeaStagingInfoFilename)).Output()
				Expect(err).NotTo(HaveOccurred())

				expectedYAML := `{"detected_buildpack":"Always Matching","start_command":"the start command"}`
				Expect(string(stagingInfo)).To(MatchJSON(expectedYAML))
			})

			Context("buildpack with supply/finalize", func() {
				BeforeEach(func() {
					buildpackOrder = "has-finalize,always-detects,also-always-detects"
					cpBuildpack("has-finalize")
				})

				It("runs supply/finalize and not compile", func() {
					Expect(files).To(ContainElement("./app/finalized"))
					Expect(files).ToNot(ContainElement("./app/compiled"))
				})

				It("places profile.d scripts in ./profile.d (not app)", func() {
					if runtime.GOOS == "windows" {
						Skip("profile.d not supported on Windows")
					}

					Expect(files).To(ContainElement("./profile.d/finalized.sh"))
				})
			})
		})

		Describe("the build artifacts cache output tgz", func() {
			BeforeEach(func() {
				buildpackOrder = "always-detects-creates-build-artifacts"

				cpBuildpack("always-detects-creates-build-artifacts")
			})

			It("gets created", func() {
				result, err := exec.Command("tar", "-tzf", outputBuildArtifactsCache).Output()
				Expect(err).NotTo(HaveOccurred())

				Expect(removeTrailingSpace(strings.Split(string(result), "\n"))).To(ContainElement("./final/build-artifact"))
			})
		})

		Describe("the result.json, which is used to communicate back to the stager", func() {
			It("exists, and contains the detected buildpack", func() {
				Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"web":"the start command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata":{
							"detected_buildpack": "Always Matching",
							"buildpack_key": "always-detects",
							"buildpacks": [
								{"key": "always-detects", "name": "Always Matching"}
							]
						},
						"processes": [
          		{
            		"type": "web",
            		"command": "the start command"
          		}
        		],
						"execution_metadata": ""
				}`))
			})

			Context("when the app has a Procfile", func() {
				BeforeEach(func() {
					cp(filepath.Join(appFixtures, "with-procfile-with-web", "Procfile"), buildDir)
				})

				It("uses the Procfile processes in the execution metadata", func() {
					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"web":"procfile-provided start-command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata":{
							"detected_buildpack": "Always Matching",
							"buildpack_key": "always-detects",
							"buildpacks": [
								{ "key": "always-detects", "name": "Always Matching" }
							]
						},
						"processes": [
          		{
            		"type": "web",
            		"command": "procfile-provided start-command"
          		}
        		],
						"execution_metadata": ""
				 }`))
				})
			})

			Context("when the app does not have a Procfile", func() {
				BeforeEach(func() {
					cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
				})

				It("uses the default_process_types specified by the buildpack", func() {
					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"web":"the start command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata":{
							"detected_buildpack": "Always Matching",
							"buildpack_key": "always-detects",
							"buildpacks": [
								{ "key": "always-detects", "name": "Always Matching" }
							]
						},
						"processes": [
          		{
            		"type": "web",
            		"command": "the start command"
          		}
        		],
						"execution_metadata": ""
				 }`))
				})
			})
		})
	})

	Context("skip detect", func() {
		BeforeEach(func() {
			skipDetect = true
		})

		JustBeforeEach(func() {
			Eventually(builder(), 5*time.Second).Should(gexec.Exit(0))
		})

		Describe("the contents of the output tgz", func() {
			var files []string

			JustBeforeEach(func() {
				result, err := exec.Command("tar", "-tzf", outputDroplet).Output()
				Expect(err).NotTo(HaveOccurred())

				files = removeTrailingSpace(strings.Split(string(result), "\n"))
			})

			Describe("the result.json, which is used to communicate back to the stager", func() {
				BeforeEach(func() {
					buildpackOrder = "always-detects"
					cpBuildpack("always-detects")
					cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
				})
				It("exists, and contains the final buildpack key", func() {
					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"web":"the start command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata":{
							"detected_buildpack": "",
							"buildpack_key": "always-detects",
							"buildpacks": [
								{ "key": "always-detects", "name": "" }
						  ]
						},
						"processes": [
          		{
            		"type": "web",
            		"command": "the start command"
          		}
        		],
						"execution_metadata": ""
				}`))
				})
			})

			Context("final buildpack does not contain a finalize script", func() {
				BeforeEach(func() {
					buildpackOrder = "always-detects-creates-build-artifacts,always-detects,also-always-detects"

					cpBuildpack("always-detects-creates-build-artifacts")
					cpBuildpack("always-detects")
					cpBuildpack("also-always-detects")
					cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
				})

				It("contains an /deps/xxxxx dir with the contents of the supply commands", func() {
					content, err := exec.Command("tar", "-xzOf", outputDroplet, "./deps/0/supplied").CombinedOutput()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("always-detects-creates-buildpack-artifacts"))

					content, err = exec.Command("tar", "-xzOf", outputDroplet, "./deps/1/supplied").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("always-detects-buildpack"))

					Expect(files).ToNot(ContainElement("./deps/2/supplied"))
				})

				It("contains an /app dir with the contents of the compilation", func() {
					Expect(files).To(ContainElement("./app/"))
					Expect(files).To(ContainElement("./app/app.sh"))
					Expect(files).To(ContainElement("./app/compiled"))

					content, err := exec.Command("tar", "-xzOf", outputDroplet, "./app/compiled").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("also-always-detects-buildpack"))
				})

				It("the /deps dir is not passed to the final compile command", func() {
					Expect(files).ToNot(ContainElement("./deps/compiled"))
				})
			})

			Context("final buildpack contains finalize + supply scripts", func() {
				BeforeEach(func() {
					buildpackOrder = "always-detects-creates-build-artifacts,always-detects,has-finalize"

					cpBuildpack("always-detects-creates-build-artifacts")
					cpBuildpack("always-detects")
					cpBuildpack("has-finalize")
					cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
				})

				It("contains an /deps/xxxxx dir with the contents of the supply commands", func() {
					content, err := exec.Command("tar", "-xzOf", outputDroplet, "./deps/0/supplied").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("always-detects-creates-buildpack-artifacts"))

					content, err = exec.Command("tar", "-xzOf", outputDroplet, "./deps/1/supplied").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("always-detects-buildpack"))

					content, err = exec.Command("tar", "-xzOf", outputDroplet, "./deps/2/supplied").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("has-finalize-buildpack"))
				})

				It("contains an /app dir with the contents of the compilation", func() {
					Expect(files).To(ContainElement("./app/"))
					Expect(files).To(ContainElement("./app/app.sh"))
					Expect(files).To(ContainElement("./app/finalized"))

					content, err := exec.Command("tar", "-xzOf", outputDroplet, "./app/finalized").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("has-finalize-buildpack"))
				})

				It("writes metadata on all buildpacks", func() {
					Expect(resultJSONbuildpacks()).To(MatchJSON(`[
					  { "key": "always-detects-creates-build-artifacts", "name": "Creates Buildpack Artifacts", "version": "9.1.3" },
						{ "key": "always-detects", "name": "" },
						{ "key": "has-finalize", "name": "Finalize" }
					]`))
				})

				Context("a supply outputs a config.yml with a `config:` present", func() {
					BeforeEach(func() {
						if runtime.GOOS == "windows" {
							Skip("support for buildpack configs is not supported in Windows")
						}

						buildpackOrder = "has-buildpack-config"

						cpBuildpack("has-buildpack-config")
						cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
					})

					It("includes the `config:` stanza in the staging_info.yml", func() {
						content, err := exec.Command("tar", "-xzOf", outputDroplet, "./staging_info.yml").Output()
						Expect(err).To(BeNil())
						Expect(string(content)).To(MatchJSON("{\"detected_buildpack\":\"Has Buildpack Config\",\"start_command\":\"the start command\",\"config\":{\"entrypoint_prefix\":\"custom-entrypoint\"}}"))
					})
				})
			})

			Context("final buildpack only contains finalize ", func() {
				BeforeEach(func() {
					buildpackOrder = "always-detects-creates-build-artifacts,always-detects,has-finalize-no-supply"

					cpBuildpack("always-detects-creates-build-artifacts")
					cpBuildpack("always-detects")
					cpBuildpack("has-finalize-no-supply")
					cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
				})

				It("contains an /deps/xxxxx dir with the contents of the supply commands", func() {
					content, err := exec.Command("tar", "-xzOf", outputDroplet, "./deps/0/supplied").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("always-detects-creates-buildpack-artifacts"))

					content, err = exec.Command("tar", "-xzOf", outputDroplet, "./deps/1/supplied").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("always-detects-buildpack"))

					Expect(files).ToNot(ContainElement("./deps/2/supplied"))
				})

				It("contains an /app dir with the contents of the compilation", func() {
					Expect(files).To(ContainElement("./app/"))
					Expect(files).To(ContainElement("./app/app.sh"))
					Expect(files).To(ContainElement("./app/finalized"))

					content, err := exec.Command("tar", "-xzOf", outputDroplet, "./app/finalized").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("has-finalize-no-supply-buildpack"))
				})
			})

			Context("buildpack that fails detect", func() {
				BeforeEach(func() {
					buildpackOrder = "always-fails-detect"

					cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
					cpBuildpack("always-fails-detect")
				})

				It("should run successfully", func() {
					Expect(files).To(ContainElement("./app/compiled"))
				})
			})
		})

		Describe("the contents of the cache tgz", func() {
			var files []string

			BeforeEach(func() {
				buildpackOrder = "always-detects-creates-build-artifacts,always-detects,also-always-detects"

				cpBuildpack("always-detects-creates-build-artifacts")
				cpBuildpack("always-detects")
				cpBuildpack("also-always-detects")
				cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
			})

			JustBeforeEach(func() {
				result, err := exec.Command("tar", "-tzf", outputBuildArtifactsCache).Output()
				Expect(err).NotTo(HaveOccurred())

				files = removeTrailingSpace(strings.Split(string(result), "\n"))
			})

			Context("final buildpack does not contain finalize", func() {
				Describe("the buildArtifactsCacheDir is empty", func() {
					It("the final buildpack caches compile output in $CACHE_DIR/final", func() {
						Expect(files).To(ContainElement("./final/compiled"))

						content, err := exec.Command("tar", "-xzOf", outputBuildArtifactsCache, "./final/compiled").Output()
						Expect(err).To(BeNil())
						Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("also-always-detects-buildpack"))
					})

					It("the supply buildpacks caches supply output as $CACHE_DIR/<xxhash64 of buildpack URL>", func() {
						supplyCacheDir := buildpackHash("always-detects-creates-build-artifacts")
						Expect(files).To(ContainElement("./" + supplyCacheDir + "/supplied"))

						supplyCacheDir = buildpackHash("always-detects")
						Expect(files).To(ContainElement("./" + supplyCacheDir + "/supplied"))

						content, err := exec.Command("tar", "-xzOf", outputBuildArtifactsCache, "./"+supplyCacheDir+"/supplied").Output()
						Expect(err).To(BeNil())
						Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("always-detects-buildpack"))
					})
				})
			})

			Context("final buildpack contains finalize", func() {
				BeforeEach(func() {
					buildpackOrder = "always-detects-creates-build-artifacts,always-detects,has-finalize"

					cpBuildpack("always-detects-creates-build-artifacts")
					cpBuildpack("always-detects")
					cpBuildpack("has-finalize")
					cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
				})

				Describe("the buildArtifactsCacheDir is empty", func() {
					It("the final buildpack caches finalize output in $CACHE_DIR/final", func() {
						Expect(files).To(ContainElement("./final/finalized"))

						content, err := exec.Command("tar", "-xzOf", outputBuildArtifactsCache, "./final/finalized").Output()
						Expect(err).To(BeNil())
						Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("has-finalize-buildpack"))
					})

					It("the final buildpack caches supply output in $CACHE_DIR/final", func() {
						Expect(files).To(ContainElement("./final/supplied"))

						content, err := exec.Command("tar", "-xzOf", outputBuildArtifactsCache, "./final/supplied").Output()
						Expect(err).To(BeNil())
						Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("has-finalize-buildpack"))
					})

					It("the supply buildpacks caches supply output as $CACHE_DIR/<xxhash64 of buildpack URL>", func() {
						supplyCacheDir := buildpackHash("always-detects-creates-build-artifacts")
						Expect(files).To(ContainElement("./" + supplyCacheDir + "/supplied"))

						supplyCacheDir = buildpackHash("always-detects")
						Expect(files).To(ContainElement("./" + supplyCacheDir + "/supplied"))

						content, err := exec.Command("tar", "-xzOf", outputBuildArtifactsCache, "./"+supplyCacheDir+"/supplied").Output()
						Expect(err).To(BeNil())
						Expect(strings.TrimRight(string(content), " \r\n")).To(Equal("always-detects-buildpack"))
					})
				})
			})

			Describe("the buildArtifactsCacheDir contains relevant and old buildpack cache directories", func() {
				//test setup
				var (
					alwaysDetectsHash       string
					notInBuildpackOrderHash string
					cachedSupply            string
					cachedCompile           string
				)

				BeforeEach(func() {
					cachedSupply = fmt.Sprintf("%d", rand.Int())
					alwaysDetectsHash = buildpackHash("always-detects")
					err := os.MkdirAll(filepath.Join(buildArtifactsCacheDir, alwaysDetectsHash), 0755)
					Expect(err).To(BeNil())
					err = os.WriteFile(filepath.Join(buildArtifactsCacheDir, alwaysDetectsHash, "old-supply"), []byte(cachedSupply), 0644)
					Expect(err).To(BeNil())

					notInBuildpackOrderHash = buildpackHash("not-in-buildpack-order")
					err = os.MkdirAll(filepath.Join(buildArtifactsCacheDir, notInBuildpackOrderHash), 0755)
					Expect(err).To(BeNil())

					cachedCompile = fmt.Sprintf("%d", rand.Int())
					err = os.MkdirAll(filepath.Join(buildArtifactsCacheDir, "final"), 0755)
					Expect(err).To(BeNil())
					err = os.WriteFile(filepath.Join(buildArtifactsCacheDir, "final", "old-compile"), []byte(cachedCompile), 0644)
					Expect(err).To(BeNil())

					err = os.WriteFile(filepath.Join(buildArtifactsCacheDir, "pre-multi-file"), []byte("Some Content"), 0644)
					Expect(err).To(BeNil())
				})

				It("does not remove the cached contents of $CACHE_DIR/final", func() {
					Expect(files).To(ContainElement("./final/compiled"))

					content, err := exec.Command("tar", "-xzOf", outputBuildArtifactsCache, "./final/compiled").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal(cachedCompile))
				})

				It("does not remove the cached contents of buildpacks in buildpack order", func() {
					Expect(files).To(ContainElement("./" + alwaysDetectsHash + "/supplied"))

					content, err := exec.Command("tar", "-xzOf", outputBuildArtifactsCache, "./"+alwaysDetectsHash+"/supplied").Output()
					Expect(err).To(BeNil())
					Expect(strings.TrimRight(string(content), " \r\n")).To(Equal(cachedSupply))
				})

				It("removes the cached contents of buildpacks not in buildpack order", func() {
					Expect(files).NotTo(ContainElement("./" + notInBuildpackOrderHash + "/"))
				})

				It("removes any files from pre multi buildpack days", func() {
					Expect(files).NotTo(ContainElement("./pre-multi-file"))
				})
			})
		})
	})

	Context("with a buildpack that has no commands", func() {
		BeforeEach(func() {
			buildpackOrder = "release-without-command"
			cpBuildpack("release-without-command")
		})

		Context("when the app has a Procfile", func() {
			Context("with web defined", func() {
				JustBeforeEach(func() {
					Eventually(builder(), 5*time.Second).Should(gexec.Exit(0))
				})

				BeforeEach(func() {
					cp(filepath.Join(appFixtures, "with-procfile-with-web", "Procfile"), buildDir)
				})

				It("uses the Procfile for execution_metadata", func() {
					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"web":"procfile-provided start-command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata":{
							"detected_buildpack": "Release Without Command",
							"buildpack_key": "release-without-command",
							"buildpacks": [
							  { "key": "release-without-command", "name": "Release Without Command" }
						  ]
						},
						"processes": [
          		{
            		"type": "web",
            		"command": "procfile-provided start-command"
          		}
        		],
						"execution_metadata": ""
					}`))
				})
			})

			Context("without web", func() {
				BeforeEach(func() {
					cp(filepath.Join(appFixtures, "with-procfile", "Procfile"), buildDir)
				})

				It("displays an error and returns the Procfile data without web", func() {
					session := builder()
					Eventually(session.Err).Should(gbytes.Say("No start command specified by buildpack or via Procfile."))
					Eventually(session.Err).Should(gbytes.Say("App will not start unless a command is provided at runtime."))
					Eventually(session, 5*time.Second).Should(gexec.Exit(0))

					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"spider":"bogus command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata": {
							"detected_buildpack": "Release Without Command",
							"buildpack_key": "release-without-command",
							"buildpacks": [
							  { "key": "release-without-command", "name": "Release Without Command" }
						  ]
						},
						"processes": [
          		{
            		"type": "spider",
            		"command": "bogus command"
          		}
        		],
						"execution_metadata": ""
					}`))
				})
			})
		})

		Context("and the app has no Procfile", func() {
			BeforeEach(func() {
				cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
			})

			It("fails", func() {
				session := builder()
				Eventually(session.Err).Should(gbytes.Say("No start command specified by buildpack or via Procfile."))
				Eventually(session.Err).Should(gbytes.Say("App will not start unless a command is provided at runtime."))
				Eventually(session, 5*time.Second).Should(gexec.Exit(0))
			})
		})
	})

	Context("with a buildpack that determines a start web-command", func() {
		BeforeEach(func() {
			buildpackOrder = "always-detects"
			cpBuildpack("always-detects")
		})

		Context("when the app has a Procfile", func() {
			Context("with web defined", func() {
				JustBeforeEach(func() {
					Eventually(builder(), 5*time.Second).Should(gexec.Exit(0))
				})

				BeforeEach(func() {
					cp(filepath.Join(appFixtures, "with-procfile-with-web", "Procfile"), buildDir)
				})

				It("merges the Procfile and the buildpack for execution_metadata", func() {
					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"web":"procfile-provided start-command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata":{
							"detected_buildpack": "Always Matching",
							"buildpack_key": "always-detects",
							"buildpacks": [
							  { "key": "always-detects", "name": "Always Matching" }
						  ]
						},
						"processes": [
          		{
            		"type": "web",
            		"command": "procfile-provided start-command"
          		}
        		],
						"execution_metadata": ""
					}`))
				})
			})

			Context("without web", func() {
				JustBeforeEach(func() {
					Eventually(builder(), 5*time.Second).Should(gexec.Exit(0))
				})

				BeforeEach(func() {
					cp(filepath.Join(appFixtures, "with-procfile", "Procfile"), buildDir)
				})

				It("merges the Procfile but uses the buildpack for execution_metadata", func() {

					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"spider":"bogus command", "web":"the start command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata": {
							"detected_buildpack": "Always Matching",
							"buildpack_key": "always-detects",
							"buildpacks": [
							  { "key": "always-detects", "name": "Always Matching" }
						  ]
						},
						"processes": [
          		{
            		"type": "web",
            		"command": "the start command"
          		},
							{
            		"type": "spider",
            		"command": "bogus command"
          		}
        		],
						"execution_metadata": ""
					}`))
				})
			})
		})

		Context("and the app has no Procfile", func() {

			JustBeforeEach(func() {
				Eventually(builder(), 5*time.Second).Should(gexec.Exit(0))
			})

			BeforeEach(func() {
				cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
			})

			It("merges the Procfile and the buildpack for execution_metadata", func() {
				Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"web":"the start command"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata":{
							"detected_buildpack": "Always Matching",
							"buildpack_key": "always-detects",
							"buildpacks": [
							  { "key": "always-detects", "name": "Always Matching" }
						  ]
						},
						"processes": [
          		{
            		"type": "web",
            		"command": "the start command"
          		}
        		],
						"execution_metadata": ""
					}`))
			})
		})
	})

	Context("with a buildpack that determines a start non-web-command", func() {
		BeforeEach(func() {
			buildpackOrder = "always-detects-non-web"
			cpBuildpack("always-detects-non-web")
		})

		Context("when the app has a Procfile", func() {
			Context("with web defined", func() {
				JustBeforeEach(func() {
					Eventually(builder(), 5*time.Second).Should(gexec.Exit(0))
				})

				BeforeEach(func() {
					cp(filepath.Join(appFixtures, "with-procfile-with-web", "Procfile"), buildDir)
				})

				It("merges the Procfile for execution_metadata", func() {
					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"web":"procfile-provided start-command", "nonweb":"start nonweb buildpack"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata":{
							"detected_buildpack": "Always Detects Non-Web",
							"buildpack_key": "always-detects-non-web",
						  "buildpacks": [
                  { "key": "always-detects-non-web", "name": "Always Detects Non-Web" }
              ]
						},
						"processes": [
							{
            		"type": "nonweb",
            		"command": "start nonweb buildpack"
          		},
          		{
            		"type": "web",
            		"command": "procfile-provided start-command"
          		}
        		],
						"execution_metadata": ""
					}`))
				})
			})

			Context("without web", func() {
				BeforeEach(func() {
					cp(filepath.Join(appFixtures, "with-procfile", "Procfile"), buildDir)
				})

				It("displays an error and returns the Procfile data without web", func() {
					session := builder()
					Eventually(session.Err).Should(gbytes.Say("No start command specified by buildpack or via Procfile."))
					Eventually(session.Err).Should(gbytes.Say("App will not start unless a command is provided at runtime."))
					Eventually(session, 5*time.Second).Should(gexec.Exit(0))

					Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"spider":"bogus command", "nonweb":"start nonweb buildpack"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata": {
							"detected_buildpack": "Always Detects Non-Web",
							"buildpack_key": "always-detects-non-web",
						  "buildpacks": [
                  { "key": "always-detects-non-web", "name": "Always Detects Non-Web" }
              ]
						},
						"processes": [
							{
            		"type": "nonweb",
            		"command": "start nonweb buildpack"
          		},
          		{
            		"type": "spider",
            		"command": "bogus command"
          		}
        		],
						"execution_metadata": ""
					}`))
				})
			})
		})

		Context("and the app has no Procfile", func() {
			BeforeEach(func() {
				cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
			})

			It("fails", func() {
				session := builder()
				Eventually(session.Err).Should(gbytes.Say("No start command specified by buildpack or via Procfile."))
				Eventually(session.Err).Should(gbytes.Say("App will not start unless a command is provided at runtime."))
				Eventually(session, 5*time.Second).Should(gexec.Exit(0))
				Expect(resultJSON()).To(MatchJSON(`{
						"process_types":{"nonweb":"start nonweb buildpack"},
						"lifecycle_type": "buildpack",
						"lifecycle_metadata": {
							"detected_buildpack": "Always Detects Non-Web",
							"buildpack_key": "always-detects-non-web",
						  "buildpacks": [
                  { "key": "always-detects-non-web", "name": "Always Detects Non-Web" }
              ]
						},
						"processes": [
          		{
            		"type": "nonweb",
            		"command": "start nonweb buildpack"
          		}
        		],
						"execution_metadata": ""
					}`))
			})
		})
	})

	Context("with an app with an invalid Procfile", func() {
		BeforeEach(func() {
			buildpackOrder = "always-detects,also-always-detects"

			cpBuildpack("always-detects")
			cpBuildpack("also-always-detects")

			cp(filepath.Join(appFixtures, "bogus-procfile", "Procfile"), buildDir)
		})

		It("fails", func() {
			session := builder()
			Eventually(session.Err).Should(gbytes.Say("Failed to read command from Procfile: invalid YAML"))
			Eventually(session, 5*time.Second).Should(gexec.Exit(1))
		})
	})

	Context("when no buildpacks match", func() {
		BeforeEach(func() {
			buildpackOrder = "always-fails-detect"

			cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
			cpBuildpack("always-fails-detect")
		})

		It("should exit with an error", func() {
			session := builder()
			Eventually(session, 5*time.Second).Should(gexec.Exit(222))
			Expect(session.Err).To(gbytes.Say("None of the buildpacks detected a compatible application"))
		})
	})

	Context("when the buildpack fails in compile", func() {
		BeforeEach(func() {
			buildpackOrder = "fails-to-compile"

			cpBuildpack("fails-to-compile")
			cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
		})

		It("should exit with an error", func() {
			session := builder()
			Eventually(session, 5*time.Second).Should(gexec.Exit(223))
			Expect(session.Err).Should(gbytes.Say("Failed to compile droplet"))
		})
	})

	Context("when a buildpack fails a supply script", func() {
		BeforeEach(func() {
			buildpackOrder = "fails-to-supply,always-detects"
			skipDetect = true

			cpBuildpack("fails-to-supply")
			cpBuildpack("always-detects")
			cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
		})

		It("should exit with an error", func() {
			session := builder()
			Eventually(session, 5*time.Second).Should(gexec.Exit(225))
			Expect(session.Err).Should(gbytes.Say("Failed to run all supply scripts"))
		})
	})

	Context("when a buildpack that isn't last doesn't have a supply script", func() {
		BeforeEach(func() {
			buildpackOrder = "has-finalize-no-supply,has-finalize"
			skipDetect = true

			cpBuildpack("has-finalize-no-supply")
			cpBuildpack("has-finalize")
			cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
		})

		It("should exit with a clear error", func() {
			session := builder()
			Eventually(session, 5*time.Second).Should(gexec.Exit(225))
			Expect(session.Err).Should(gbytes.Say("Error: one of the buildpacks chosen to supply dependencies does not support multi-buildpack apps"))
		})
	})

	Context("when a the final buildpack has compile but not finalize", func() {
		Context("single buildpack", func() {
			BeforeEach(func() {
				buildpackOrder = "always-detects"
				skipDetect = true

				cpBuildpack("always-detects")
				cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
			})

			It("should not display a warning about multi-buildpack compatibility", func() {
				session := builder()
				Eventually(session, 5*time.Second).Should(gexec.Exit(0))
				Expect(session.Err).ToNot(gbytes.Say("Warning: the last buildpack is not compatible with multi-buildpack apps and cannot make use of any dependencies supplied by the buildpacks specified before it"))
			})
		})
		Context("multi-buildpack", func() {
			BeforeEach(func() {
				buildpackOrder = "has-finalize,always-detects"
				skipDetect = true

				cpBuildpack("has-finalize")
				cpBuildpack("always-detects")
				cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
			})

			It("should display a warning about multi-buildpack compatibility", func() {
				session := builder()
				Eventually(session, 5*time.Second).Should(gexec.Exit(0))
				Expect(session.Err).To(gbytes.Say("Warning: the last buildpack is not compatible with multi-buildpack apps and cannot make use of any dependencies supplied by the buildpacks specified before it"))
			})
		})
	})

	Context("when the buildpack release generates invalid yaml", func() {
		BeforeEach(func() {
			buildpackOrder = "release-generates-bad-yaml"

			cpBuildpack("release-generates-bad-yaml")
			cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
		})

		It("should exit with an error", func() {
			session := builder()
			Eventually(session, 5*time.Second).Should(gexec.Exit(224))
			Expect(session.Err).Should(gbytes.Say("buildpack's release output invalid"))
		})
	})

	Context("when the buildpack fails to release", func() {
		BeforeEach(func() {
			buildpackOrder = "fails-to-release"

			cpBuildpack("fails-to-release")
			cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
		})

		It("should exit with an error", func() {
			session := builder()
			Eventually(session, 5*time.Second).Should(gexec.Exit(224))
			Expect(session.Err).Should(gbytes.Say("Failed to build droplet release"))
		})
	})

	Context("with a buildpack using an md5 based path", func() {
		BeforeEach(func() {
			buildpack := "always-detect-buildpack"
			buildpackOrder = buildpack

			buildpackHash := "e05b2d8a59b30d5e9552282465253095"

			buildpackDir := filepath.Join(buildpacksDir, buildpackHash)

			cp(filepath.Join(buildpackFixtures, "always-detects"), buildpackDir)
			cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
		})

		It("should detect the buildpack using an md5 based path", func() {
			Eventually(builder()).Should(gexec.Exit(0))
		})
	})

	Context("with a nested buildpack using an md5 based path", func() {
		BeforeEach(func() {
			nestedBuildpack := "nested-buildpack"
			buildpackOrder = nestedBuildpack

			nestedBuildpackHash := "70d137ae4ee01fbe39058ccdebf48460"

			nestedBuildpackDir := filepath.Join(buildpacksDir, nestedBuildpackHash)
			err := os.MkdirAll(nestedBuildpackDir, 0777)
			Expect(err).NotTo(HaveOccurred())

			cp(filepath.Join(buildpackFixtures, "always-detects"), nestedBuildpackDir)
			cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
		})

		It("should detect the nested buildpack using an md5 based path", func() {
			Eventually(builder()).Should(gexec.Exit(0))
		})
	})

	Context("with a nested buildpack", func() {
		BeforeEach(func() {
			nestedBuildpack := "nested-buildpack"
			buildpackOrder = nestedBuildpack

			nestedBuildpackHash := "ba8ed7547bcbcb08"

			nestedBuildpackDir := filepath.Join(buildpacksDir, nestedBuildpackHash)
			err := os.MkdirAll(nestedBuildpackDir, 0777)
			Expect(err).NotTo(HaveOccurred())

			cp(filepath.Join(buildpackFixtures, "always-detects"), nestedBuildpackDir)
			cp(filepath.Join(appFixtures, "bash-app", "app.sh"), buildDir)
		})

		It("should detect the nested buildpack", func() {
			Eventually(builder()).Should(gexec.Exit(0))
		})
	})
})

func removeTrailingSpace(dirty []string) []string {
	clean := []string{}
	for _, s := range dirty {
		clean = append(clean, strings.TrimRight(s, "\r\n"))
	}

	return clean
}

func buildpackHash(key string) string {
	return fmt.Sprintf("%016x", xxhash.Sum64String(key))
}
