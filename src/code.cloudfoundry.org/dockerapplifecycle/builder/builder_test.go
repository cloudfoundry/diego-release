package main_test

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	"github.com/docker/libtrust"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Building", func() {
	var (
		builderCmd                 *exec.Cmd
		dockerRef                  string
		insecureDockerRegistries   string
		dockerRegistryIPs          string
		dockerRegistryHost         string
		dockerRegistryPort         string
		dockerDaemonExecutablePath string
		cacheDockerImage           bool
		dockerLoginServer          string
		dockerUser                 string
		dockerPassword             string
		dockerEmail                string
		outputMetadataDir          string
		outputMetadataJSONFilename string
		fakeDockerRegistry         *ghttp.Server
	)

	makeResponse := func(resp string) string {
		return `{
			"schemaVersion": 1,
			"name": "cloudfoundry/diego-docker-app",
			"tag": "latest",
			"architecture": "amd64",
			"fsLayers": [
				{
					 "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
				}
			],
			"history": [ { "v1Compatibility": ` + strconv.Quote(resp) + ` } ]
		}`
	}

	signResponse := func(response string) string {
		key, err := libtrust.GenerateECP256PrivateKey()
		Expect(err).NotTo(HaveOccurred())
		jsonSignature, err := libtrust.NewJSONSignature([]byte(response))
		Expect(err).NotTo(HaveOccurred())
		err = jsonSignature.Sign(key)
		Expect(err).NotTo(HaveOccurred())
		keys, err := jsonSignature.Verify()
		Expect(err).NotTo(HaveOccurred())
		Expect(keys).To(HaveLen(1))

		signedResponse, err := jsonSignature.PrettySignature("signatures")
		Expect(err).NotTo(HaveOccurred())
		return string(signedResponse)
	}

	setupBuilder := func() *gexec.Session {
		session, err := gexec.Start(
			builderCmd,
			GinkgoWriter,
			GinkgoWriter,
		)
		Expect(err).NotTo(HaveOccurred())

		return session
	}

	setupFakeDockerRegistry := func() {
		fakeDockerRegistry.AppendHandlers(
			ghttp.VerifyRequest("GET", "/v2/"),
		)
	}

	setupRegistryResponse := func(response string) {
		fakeDockerRegistry.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v2/some-repo/manifests/latest"),
				http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					w.Write([]byte(response))
				}),
			),
		)
	}

	buildDockerRef := func() string {
		parts, err := url.Parse(fakeDockerRegistry.URL())
		Expect(err).NotTo(HaveOccurred())
		return fmt.Sprintf("%s/some-repo", parts.Host)
	}

	resultJSON := func() []byte {
		resultInfo, err := os.ReadFile(outputMetadataJSONFilename)
		Expect(err).NotTo(HaveOccurred())

		return resultInfo
	}

	BeforeEach(func() {
		var err error

		dockerRef = ""
		insecureDockerRegistries = ""
		dockerRegistryIPs = ""
		dockerRegistryHost = ""
		dockerRegistryPort = ""
		dockerDaemonExecutablePath = ""
		cacheDockerImage = false
		dockerLoginServer = ""
		dockerUser = ""
		dockerPassword = ""
		dockerEmail = ""

		outputMetadataDir, err = os.MkdirTemp("", "building-result")
		Expect(err).NotTo(HaveOccurred())

		outputMetadataJSONFilename = path.Join(outputMetadataDir, "result.json")

		fakeDockerRegistry = ghttp.NewServer()
	})

	AfterEach(func() {
		os.RemoveAll(outputMetadataDir)
	})

	JustBeforeEach(func() {
		args := []string{"-outputMetadataJSONFilename", outputMetadataJSONFilename}

		if len(dockerRef) > 0 {
			args = append(args, "-dockerRef", dockerRef)
		}
		if len(insecureDockerRegistries) > 0 {
			args = append(args, "-insecureDockerRegistries", insecureDockerRegistries)
		}
		if len(dockerRegistryIPs) > 0 {
			args = append(args, "-dockerRegistryIPs", dockerRegistryIPs)
		}
		if len(dockerRegistryHost) > 0 {
			args = append(args, "-dockerRegistryHost", dockerRegistryHost)
		}
		if len(dockerRegistryPort) > 0 {
			args = append(args, "-dockerRegistryPort", dockerRegistryPort)
		}
		if cacheDockerImage {
			args = append(args, "-cacheDockerImage")
		}
		if len(dockerDaemonExecutablePath) > 0 {
			args = append(args, "-dockerDaemonExecutablePath", dockerDaemonExecutablePath)
		}
		if len(dockerLoginServer) > 0 {
			args = append(args, "-dockerLoginServer", dockerLoginServer)
		}
		if len(dockerUser) > 0 {
			args = append(args, "-dockerUser", dockerUser)
		}
		if len(dockerPassword) > 0 {
			args = append(args, "-dockerPassword", dockerPassword)
		}
		if len(dockerEmail) > 0 {
			args = append(args, "-dockerEmail", dockerEmail)
		}

		builderCmd = exec.Command(builderPath, args...)

		builderCmd.Env = os.Environ()
	})

	// We rely on a secure docker registry providing a TLS certificate signed by
	// a system CA.
	Context("when an insecure registry is not specified", func() {
		BeforeEach(func() {
			dockerRef = buildDockerRef()
		})

		It("fails to validate an HTTP registry", func() {
			session := setupBuilder()
			Eventually(session.Err).Should(gbytes.Say("server gave HTTP response to HTTPS client"))
			Eventually(session).Should(gexec.Exit(2))
		})
	})

	Context("when insecure registry is specified", func() {
		BeforeEach(func() {
			parts, err := url.Parse(fakeDockerRegistry.URL())
			Expect(err).NotTo(HaveOccurred())
			insecureDockerRegistries = parts.Host
		})

		Context("when running the main", func() {
			Context("with no docker image arg specified", func() {
				It("should exit with an error", func() {
					session := setupBuilder()
					Eventually(session.Err).Should(gbytes.Say("missing flag: dockerRef required"))
					Eventually(session).Should(gexec.Exit(1))
				})
			})

			Context("with an invalid output path", func() {
				It("should exit with an error", func() {
					session := setupBuilder()
					Eventually(session).Should(gexec.Exit(1))
				})
			})

			Context("with an invalid docker registry addesses", func() {
				var invalidRegistryAddress string

				Context("when an address has a scheme", func() {
					BeforeEach(func() {
						invalidRegistryAddress = "://10.244.2.6:5050"
						insecureDockerRegistries = "10.244.2.7:8080, " + invalidRegistryAddress

						dockerRef = buildDockerRef()
					})

					It("should exit with an error", func() {
						session := setupBuilder()
						Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("invalid value \"%s\" for flag -insecureDockerRegistries: no scheme allowed for Docker Registry \\[%s\\]", insecureDockerRegistries, invalidRegistryAddress)))
						Eventually(session).Should(gexec.Exit(2))
					})
				})

				Context("when an address has no port", func() {
					BeforeEach(func() {
						invalidRegistryAddress = "10.244.2.6"
						insecureDockerRegistries = invalidRegistryAddress + " , 10.244.2.7:8080"

						dockerRef = buildDockerRef()
					})

					It("should exit with an error", func() {
						session := setupBuilder()
						Eventually(session.Err).Should(gbytes.Say(fmt.Sprintf("invalid value \"%s\" for flag -insecureDockerRegistries: ip:port expected for Docker Registry \\[%s\\]", insecureDockerRegistries, invalidRegistryAddress)))
						Eventually(session).Should(gexec.Exit(2))
					})
				})
			})

			testValid := func() {
				Context("when the registry returns a signed manifest", func() {
					BeforeEach(func() {
						setupFakeDockerRegistry()
						response := signResponse(
							makeResponse(
								`{"id":"f8cbcf226d6a01a5ebb15b8390cff83b8b5dffc226761e968f9d3a01312551b9","Config":{"Cmd":["-bazbot","-foobar"],"Entrypoint":["/dockerapp","-t"],"WorkingDir":"/workdir"}}`,
							),
						)
						setupRegistryResponse(response)
					})

					It("should exit successfully", func() {
						session := setupBuilder()
						Eventually(session, 10*time.Second).Should(gexec.Exit(0))
					})

					Describe("the json", func() {
						It("should contain the execution metadata", func() {
							session := setupBuilder()
							Eventually(session, 10*time.Second).Should(gexec.Exit(0))

							result := resultJSON()

							Expect(result).To(ContainSubstring(`\"cmd\":[\"-bazbot\",\"-foobar\"]`))
							Expect(result).To(ContainSubstring(`\"entrypoint\":[\"/dockerapp\",\"-t\"]`))
							Expect(result).To(ContainSubstring(`\"workdir\":\"/workdir\"`))
						})
					})
				})

				Context("when the registry returns an unsigned manifest", func() {
					BeforeEach(func() {
						setupFakeDockerRegistry()
						response := makeResponse(
							`{"id":"f8cbcf226d6a01a5ebb15b8390cff83b8b5dffc226761e968f9d3a01312551b9","Config":{"Cmd":["-bazbot","-foobar"],"Entrypoint":["/dockerapp","-t"],"WorkingDir":"/workdir"}}`,
						)
						setupRegistryResponse(response)
						setupRegistryResponse(response)
						setupRegistryResponse(response)
						setupRegistryResponse(response)
					})

					It("should exit successfully", func() {
						session := setupBuilder()
						Eventually(session, 10*time.Second).Should(gexec.Exit(0))
					})
				})
			}

			dockerRefFunc := func() {
				BeforeEach(func() {
					parts, err := url.Parse(fakeDockerRegistry.URL())
					Expect(err).NotTo(HaveOccurred())
					dockerRef = fmt.Sprintf("%s/some-repo", parts.Host)
				})

				testValid()
			}

			Context("with a private docker registry", func() {
				BeforeEach(func() {
					dockerPassword = "fakedockerpassword"
					dockerUser = "fakedockeruser"

					parts, err := url.Parse(fakeDockerRegistry.URL())
					Expect(err).NotTo(HaveOccurred())
					dockerRef = fmt.Sprintf("%s/some-repo", parts.Host)
				})

				Context("with a valid docker ref", func() {
					BeforeEach(func() {
						authenticateHeader := http.Header{}
						authenticateHeader.Add("WWW-Authenticate", `Basic realm="testRegistry"`)
						fakeDockerRegistry.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyRequest("GET", "/v2/"),
								ghttp.RespondWith(401, "", authenticateHeader),
							),
						)

						response := `{
						"id":"f8cbcf226d6a01a5ebb15b8390cff83b8b5dffc226761e968f9d3a01312551b9",
						"Config":{"Cmd":["-bazbot","-foobar"],"Entrypoint":["/dockerapp","-t"],
						"WorkingDir":"/workdir"
					}
				}`
						fakeDockerRegistry.AppendHandlers(
							ghttp.CombineHandlers(
								ghttp.VerifyBasicAuth(dockerUser, dockerPassword),
								ghttp.VerifyRequest("GET", "/v2/some-repo/manifests/latest"),
								http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
									response := makeResponse(response)
									w.Write([]byte(response))
								}),
							),
						)
					})

					It("should exit successfully", func() {
						session := setupBuilder()
						Eventually(session, 10*time.Second).Should(gexec.Exit(0))
					})

					Describe("the json", func() {
						It("should contain the execution metadata", func() {
							session := setupBuilder()
							Eventually(session, 10*time.Second).Should(gexec.Exit(0))

							result := resultJSON()

							Expect(result).To(ContainSubstring(`\"cmd\":[\"-bazbot\",\"-foobar\"]`))
							Expect(result).To(ContainSubstring(`\"entrypoint\":[\"/dockerapp\",\"-t\"]`))
							Expect(result).To(ContainSubstring(`\"workdir\":\"/workdir\"`))
						})
					})
				})
			})

			Context("with an AWS ECR", func() {
				BeforeEach(func() {
					dockerUser = os.Getenv("ECR_TEST_AWS_ACCESS_KEY_ID")
					dockerPassword = os.Getenv("ECR_TEST_AWS_SECRET_ACCESS_KEY")
					dockerRef = os.Getenv("ECR_TEST_REPO_URI")

					if dockerUser == "" ||
						dockerPassword == "" ||
						dockerRef == "" {
						Skip("ECR_TEST_AWS_ACCESS_KEY_ID, ECR_TEST_AWS_SECRET_ACCESS_KEY and ECR_TEST_REPO_URI should be set")
					}
				})

				It("should exit successfully", func() {
					session := setupBuilder()
					Eventually(session, 30*time.Second).Should(gexec.Exit(0))
				})

				Describe("the json", func() {
					It("should contain the execution metadata", func() {
						session := setupBuilder()
						Eventually(session, 30*time.Second).Should(gexec.Exit(0))

						result := resultJSON()

						Expect(result).To(ContainSubstring(`"docker_image":"` + dockerRef + `:latest"`))
						Expect(result).To(ContainSubstring(`\"cmd\":[\"dockerapp\"]`))
						Expect(result).To(ContainSubstring(`\"workdir\":\"/myapp\"`))
					})
				})
			})

			Context("with a valid insecure docker registries", func() {
				BeforeEach(func() {
					parts, err := url.Parse(fakeDockerRegistry.URL())
					Expect(err).NotTo(HaveOccurred())
					insecureDockerRegistries = parts.Host + ",10.244.2.6:80"
					dockerRegistryHost = "docker-registry.service.cf.internal"
				})

				Context("with a valid docker ref", dockerRefFunc)
			})

			Context("with a valid docker registries", func() {
				BeforeEach(func() {
					parts, err := url.Parse(fakeDockerRegistry.URL())
					Expect(err).NotTo(HaveOccurred())
					host, _, err := net.SplitHostPort(parts.Host)
					Expect(err).NotTo(HaveOccurred())

					dockerRegistryIPs = host + ",10.244.2.6"
				})

				Context("with a valid docker ref", dockerRefFunc)
			})

			Context("without docker registries", func() {
				Context("when there is no caching requested", func() {
					Context("with a valid docker ref", dockerRefFunc)
				})
			})

			Context("with exposed ports in image metadata", func() {
				BeforeEach(func() {
					dockerRef = buildDockerRef()
					cacheDockerImage = false

					setupFakeDockerRegistry()
				})

				Context("with correct ports", func() {
					BeforeEach(func() {
						setupRegistryResponse(makeResponse(`{"id":"f8cbcf226d6a01a5ebb15b8390cff83b8b5dffc226761e968f9d3a01312551b9","Config":{"Cmd":["-bazbot","-foobar"],"Entrypoint":["/dockerapp","-t"],"WorkingDir":"/workdir", "ExposedPorts": {"8081/udp":{}, "8081/tcp":{}, "8079/tcp":{}, "8078/udp":{}} }}`))
					})

					Describe("the json", func() {
						It("should contain sorted exposed ports", func() {
							session := setupBuilder()
							Eventually(session, 10*time.Second).Should(gexec.Exit(0))

							result := resultJSON()

							Expect(result).To(ContainSubstring(`\"ports\":[{\"Port\":8078,\"Protocol\":\"udp\"},{\"Port\":8079,\"Protocol\":\"tcp\"},{\"Port\":8081,\"Protocol\":\"tcp\"},{\"Port\":8081,\"Protocol\":\"udp\"}]`))
						})
					})
				})

				Context("with incorrect port", func() {
					Context("bigger than allowed uint16 range", func() {
						BeforeEach(func() {
							setupRegistryResponse(makeResponse(`{"id":"f8cbcf226d6a01a5ebb15b8390cff83b8b5dffc226761e968f9d3a01312551b9","Config":{"Cmd":["-bazbot","-foobar"],"Entrypoint":["/dockerapp","-t"],"WorkingDir":"/workdir", "ExposedPorts": {"8081/tcp":{}, "65536/tcp":{}, "8078/udp":{}} }}`))
						})

						It("should error", func() {
							session := setupBuilder()
							Eventually(session.Err).Should(gbytes.Say("value out of range"))
							Eventually(session, 10*time.Second).Should(gexec.Exit(2))
						})
					})

					Context("negative port", func() {
						BeforeEach(func() {
							setupRegistryResponse(makeResponse(`{"id":"f8cbcf226d6a01a5ebb15b8390cff83b8b5dffc226761e968f9d3a01312551b9","Config":{"Cmd":["-bazbot","-foobar"],"Entrypoint":["/dockerapp","-t"],"WorkingDir":"/workdir", "ExposedPorts": {"8081/tcp":{}, "-8078/udp":{}} }}`))
						})

						It("should error", func() {
							session := setupBuilder()
							Eventually(session.Err).Should(gbytes.Say("invalid syntax"))
							Eventually(session, 10*time.Second).Should(gexec.Exit(2))
						})
					})

					Context("not a number", func() {
						BeforeEach(func() {
							setupRegistryResponse(makeResponse(`{"id":"f8cbcf226d6a01a5ebb15b8390cff83b8b5dffc226761e968f9d3a01312551b9","Config":{"Cmd":["-bazbot","-foobar"],"Entrypoint":["/dockerapp","-t"],"WorkingDir":"/workdir", "ExposedPorts": {"8081/tcp":{}, "8a8a/udp":{}} }}`))
						})

						It("should error", func() {
							session := setupBuilder()
							Eventually(session.Err).Should(gbytes.Say("invalid syntax"))
							Eventually(session, 10*time.Second).Should(gexec.Exit(2))
						})
					})
				})
			})

			Context("with specified user in image metadata", func() {
				BeforeEach(func() {
					dockerRef = buildDockerRef()
					cacheDockerImage = false

					setupFakeDockerRegistry()
					setupRegistryResponse(makeResponse(`{"id":"f8cbcf226d6a01a5ebb15b8390cff83b8b5dffc226761e968f9d3a01312551b9","Config":{"Cmd":["-bazbot","-foobar"],"Entrypoint":["/dockerapp","-t"],"WorkingDir":"/workdir", "User": "custom-user"}}`))
				})

				Describe("the json", func() {
					It("should contain custom user", func() {
						session := setupBuilder()
						Eventually(session, 10*time.Second).Should(gexec.Exit(0))

						result := resultJSON()

						Expect(result).To(ContainSubstring(`\"user\":\"custom-user\"`))
					})
				})

			})
		})
	})
})
