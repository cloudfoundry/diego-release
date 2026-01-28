package helpers_test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/dockerapplifecycle"
	"code.cloudfoundry.org/dockerapplifecycle/helpers"
	"code.cloudfoundry.org/dockerapplifecycle/protocol"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"
	digest "github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type serverResponseConfig struct {
	ImageConfig            v1.ImageConfig
	ImageTag               string
	WithPrivateRegistry    bool
	WithSlowImageConfig    bool
	WithSlowImageManifest  bool
	WithTokenAuthorization bool
	WithBasicAuthorization bool
}

var _ = Describe("Builder helpers", func() {
	var server *ghttp.Server

	v2Schema1Manifest := func(serverConfig serverResponseConfig) {
		json := `
{
   "schemaVersion": 1,
   "name": "cloudfoundry/diego-docker-app",
   "tag": "latest",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:c7b0764dcaa8ce81ef7e949b8029195dd763182cf45df5bfd62983797c62d8bd"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:6dbfe8710faa57533c018007eaabefd7d83c0b705ef2faa6c6f35d0e67da4bc6"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:add3ddb21edebb102b552fc129273216bf6312f5f1519d7c1401864a2810738b"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"architecture\":\"amd64\",\"author\":\"https://github.com/cloudfoundry-incubator/diego-dockerfiles\",\"config\":{\"Hostname\":\"5fd1ff7f1c41\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":{\"8080/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/myapp/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\",\"HOME=/home/some_docker_user\",\"SOME_VAR=some_docker_value\"],\"Cmd\":[\"dockerapp\"],\"ArgsEscaped\":true,\"Image\":\"sha256:a0a486475836a2512e6a2ca6b1f8a41eedf50be2492ad98143f299649f4941d1\",\"Volumes\":null,\"WorkingDir\":\"/myapp\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":{}},\"container\":\"f4591248d8558a17d793015c0d7ed759c61af4c5b2bcb92c330797a8033a5e38\",\"container_config\":{\"Hostname\":\"5fd1ff7f1c41\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":{\"8080/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/myapp/bin\",\"VCAP_APPLICATION={}\",\"BAD_QUOTE='\",\"BAD_SHELL=$1\",\"HOME=/home/some_docker_user\",\"SOME_VAR=some_docker_value\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) \",\"CMD [\\\"dockerapp\\\"]\"],\"ArgsEscaped\":true,\"Image\":\"sha256:a0a486475836a2512e6a2ca6b1f8a41eedf50be2492ad98143f299649f4941d1\",\"Volumes\":null,\"WorkingDir\":\"/myapp\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":{}},\"created\":\"2017-10-18T16:36:09.414939809Z\",\"docker_version\":\"17.05.0-ce\",\"id\":\"e7d3549e7b27af175c403021174c60d7abef583e04776ace50c75f6d69a29b2e\",\"os\":\"linux\",\"parent\":\"f8cbcf226d6a01a5ebb15b8390cff83b8b5dffc226761e968f9d3a01312551b9\",\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"f8cbcf226d6a01a5ebb15b8390cff83b8b5dffc226761e968f9d3a01312551b9\",\"parent\":\"9e1ba4e8dbf925ac47b982f24c2f156ae394f7f2571ec411e31b5b367824651c\",\"created\":\"2017-10-18T16:36:08.719100141Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c adduser -D vcap\"]},\"author\":\"https://github.com/cloudfoundry-incubator/diego-dockerfiles\"}"
      },
      {
         "v1Compatibility": "{\"id\":\"9e1ba4e8dbf925ac47b982f24c2f156ae394f7f2571ec411e31b5b367824651c\",\"parent\":\"8685b78dbd9fc22dea58974c96d753b0ca0604368eed6107dcce7abdd4626eb0\",\"created\":\"2017-10-18T16:36:05.25752693Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) WORKDIR /myapp\"]},\"author\":\"https://github.com/cloudfoundry-incubator/diego-dockerfiles\",\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"8685b78dbd9fc22dea58974c96d753b0ca0604368eed6107dcce7abdd4626eb0\",\"parent\":\"1b44caaf935ecccfdd03d89bf0ebd0a4e76b0f168402f01b3cecab664daee7bc\",\"created\":\"2017-10-18T16:36:04.333209162Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) COPY file:ef22e82646058a157c84bdaaec02c987845793bea1a98c1b7d74d0f27f7b68df in /myapp/bin/dockerapp \"]},\"author\":\"https://github.com/cloudfoundry-incubator/diego-dockerfiles\"}"
      },
      {
         "v1Compatibility": "{\"id\":\"1b44caaf935ecccfdd03d89bf0ebd0a4e76b0f168402f01b3cecab664daee7bc\",\"parent\":\"e033d6187f812fd1722312764f80e7fa0974575f7cf613a6faf28ab8b325523b\",\"created\":\"2017-10-18T16:36:03.193549148Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop)  EXPOSE 8080/tcp\"]},\"author\":\"https://github.com/cloudfoundry-incubator/diego-dockerfiles\",\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"e033d6187f812fd1722312764f80e7fa0974575f7cf613a6faf28ab8b325523b\",\"parent\":\"8350b6ca18c1c41a2e07042439e8f217c1275a540deaf5c730b05360722af670\",\"created\":\"2017-10-18T16:36:02.213855538Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop)  ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/myapp/bin\"]},\"author\":\"https://github.com/cloudfoundry-incubator/diego-dockerfiles\",\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"8350b6ca18c1c41a2e07042439e8f217c1275a540deaf5c730b05360722af670\",\"parent\":\"49a0e8685cd1ec97f8ef816b5eea34cb2a88ea7e078892f4be654aa4533bf208\",\"created\":\"2017-10-18T16:36:01.197619326Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop)  ENV SOME_VAR=some_docker_value\"]},\"author\":\"https://github.com/cloudfoundry-incubator/diego-dockerfiles\",\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"49a0e8685cd1ec97f8ef816b5eea34cb2a88ea7e078892f4be654aa4533bf208\",\"parent\":\"c222acf717e0aec1d4bf72d7766246fe51de88c507b2a3c37c29e66e6ab8688c\",\"created\":\"2017-10-18T16:36:00.197858282Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop)  ENV HOME=/home/some_docker_user\"]},\"author\":\"https://github.com/cloudfoundry-incubator/diego-dockerfiles\",\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"c222acf717e0aec1d4bf72d7766246fe51de88c507b2a3c37c29e66e6ab8688c\",\"parent\":\"00cbb9f2d7526b727d5100a8c10b773e7f78c169e4cfb68e4e48dce9e3fc13f0\",\"created\":\"2017-10-18T16:35:59.97446728Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop)  ENV BAD_SHELL=$1\"]},\"author\":\"https://github.com/cloudfoundry-incubator/diego-dockerfiles\",\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"00cbb9f2d7526b727d5100a8c10b773e7f78c169e4cfb68e4e48dce9e3fc13f0\",\"parent\":\"292c3a2179c41e490f3d81f407e8c4ba5fd71b32fd3a755740932359194a830e\",\"created\":\"2017-10-18T16:35:57.247129913Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop)  ENV BAD_QUOTE='\"]},\"author\":\"https://github.com/cloudfoundry-incubator/diego-dockerfiles\",\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"292c3a2179c41e490f3d81f407e8c4ba5fd71b32fd3a755740932359194a830e\",\"parent\":\"a4dbe56c2172a4b4032e22486425f430190be90301d5cede87f483456218f21b\",\"created\":\"2017-10-18T16:35:56.194954801Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop)  ENV VCAP_APPLICATION={}\"]},\"author\":\"https://github.com/cloudfoundry-incubator/diego-dockerfiles\",\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"a4dbe56c2172a4b4032e22486425f430190be90301d5cede87f483456218f21b\",\"parent\":\"79896794f176d957c647e79688a575fd1ccd897d00793d79fd2ba87fd8f38db0\",\"created\":\"2017-10-18T16:35:55.673329246Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop)  MAINTAINER https://github.com/cloudfoundry-incubator/diego-dockerfiles\"]},\"author\":\"https://github.com/cloudfoundry-incubator/diego-dockerfiles\",\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"79896794f176d957c647e79688a575fd1ccd897d00793d79fd2ba87fd8f38db0\",\"parent\":\"4560cca0c1f213ea635329144073cb0d7ba6736bfadc12246a46564b2ccd1ca6\",\"created\":\"2017-09-13T10:14:16.620085795Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop)  CMD [\\\"sh\\\"]\"]},\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"4560cca0c1f213ea635329144073cb0d7ba6736bfadc12246a46564b2ccd1ca6\",\"created\":\"2017-09-13T10:14:16.436015799Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) ADD file:645231abe6e10e7282a6e78b49723a3ba35b62741fc08228b4086ffb95128f98 in / \"]}}"
      }
   ],
   "signatures": [
      {
         "header": {
            "jwk": {
               "crv": "P-256",
               "kid": "7EPZ:A6ZJ:NPFY:EVMW:OARM:XU3W:KFX4:KECJ:43LF:MSJ7:WSGX:XEKO",
               "kty": "EC",
               "x": "YThbbdERZsLizuJrzpZZSBiFhjw9CniQn_rn8_vXO28",
               "y": "76Ye0shPacLrI7xblR0ICXTS9hfgjVnN0Ar5Fq4ANdU"
            },
            "alg": "ES256"
         },
         "signature": "e-QO7mMcDWuR4eMu5yYZBe2e0D71IZP-gQvsXeZ-16jf2nPGaQM0VXf6T2es8Pmu-U09dnu1FtT1iMc-2Yo9lQ",
         "protected": "eyJmb3JtYXRMZW5ndGgiOjkwNTIsImZvcm1hdFRhaWwiOiJDbjAiLCJ0aW1lIjoiMjAxNy0xMC0xOVQxNTo0MToyM1oifQ"
      }
   ]
}
`
		server.AllowUnhandledRequests = true
		server.AppendHandlers(
			ghttp.VerifyRequest("GET", "/v2/"),
		)

		server.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v2/some_user/some_repo/manifests/"+serverConfig.ImageTag),
				ghttp.VerifyHeaderKV(
					"Accept",
					v1.MediaTypeImageIndex,
					v1.MediaTypeImageManifest,
					manifest.DockerV2Schema2MediaType,
					manifest.DockerV2Schema1SignedMediaType,
					manifest.DockerV2Schema1MediaType,
				),
				http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					w.Header().Set("X-Docker-Token", "token-1,token-2")
					w.Header().Set("Content-Type", manifest.DockerV2Schema1MediaType)
					w.Write([]byte(json))
				}),
			),
		)
	}

	v2Schema2Manifest := func(serverConfig serverResponseConfig) {
		if serverConfig.WithTokenAuthorization {
			authenticateHeader := http.Header{}
			authenticateHeader.Add("WWW-Authenticate", fmt.Sprintf(`Bearer realm="https://%s/token"`, server.Addr()))
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(401, "", authenticateHeader),
				),
			)
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/token"),
					ghttp.VerifyBasicAuth("username", "password"),
					ghttp.RespondWith(200, `{"token":"tokenstring"}`),
				),
			)
		} else if serverConfig.WithBasicAuthorization {
			authenticateHeader := http.Header{}
			authenticateHeader.Add("WWW-Authenticate", fmt.Sprintf(`Basic realm="http://%s/token"`, server.Addr()))
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v2/"),
					ghttp.RespondWith(401, "", authenticateHeader),
				),
			)
		} else {
			server.AllowUnhandledRequests = true
			server.AppendHandlers(
				ghttp.VerifyRequest("GET", "/v2/"),
			)
		}

		config := v1.Image{
			Config: serverConfig.ImageConfig,
		}

		configBytes, err := json.Marshal(config)
		Expect(err).ToNot(HaveOccurred())
		configDigest := digest.FromBytes(configBytes)

		m := manifest.Schema2{
			SchemaVersion: 2,
			MediaType:     manifest.DockerV2Schema2MediaType,
			ConfigDescriptor: manifest.Schema2Descriptor{
				MediaType: manifest.DockerV2Schema2ConfigMediaType,
				Size:      int64(len(configBytes)),
				Digest:    configDigest,
			},
		}
		manifestBytes, err := json.Marshal(m)
		Expect(err).ToNot(HaveOccurred())

		if serverConfig.WithSlowImageManifest {
			slowImageManifestHandler := ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/v2/some_user/some_repo/manifests/"+serverConfig.ImageTag),
				ghttp.RespondWith(418, "{\"details\": \"Push failed due to a network error. Please try again. If the problem persists, it may be due to a slow connection.\"}"),
			)
			server.AppendHandlers(
				slowImageManifestHandler,
				ghttp.VerifyRequest("GET", "/v2/"),
				slowImageManifestHandler,
				ghttp.VerifyRequest("GET", "/v2/"),
				slowImageManifestHandler,
				ghttp.VerifyRequest("GET", "/v2/"),
			)
		}
		verifyRequests := []http.HandlerFunc{
			ghttp.VerifyRequest("GET", "/v2/some_user/some_repo/manifests/"+serverConfig.ImageTag),
			ghttp.VerifyHeaderKV(
				"Accept",
				v1.MediaTypeImageIndex,
				v1.MediaTypeImageManifest,
				manifest.DockerV2Schema2MediaType,
				manifest.DockerV2Schema1SignedMediaType,
				manifest.DockerV2Schema1MediaType,
			),
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("X-Docker-Token", "token-1,token-2")
				w.Header().Set("Content-Type", manifest.DockerV2Schema2MediaType)
				w.Write(manifestBytes)
			}),
		}
		if serverConfig.WithTokenAuthorization {
			verifyRequests = append(
				verifyRequests,
				ghttp.VerifyHeaderKV("Authorization", "Bearer tokenstring"),
			)
		}
		if serverConfig.WithBasicAuthorization {
			verifyRequests = append(
				verifyRequests,
				ghttp.VerifyBasicAuth("username", "password"),
			)
		}
		server.AppendHandlers(
			ghttp.CombineHandlers(verifyRequests...),
		)

		if serverConfig.WithSlowImageConfig {
			slowImageConfigHandler := ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", fmt.Sprintf("/v2/some_user/some_repo/blobs/%s", configDigest)),
				ghttp.RespondWith(418, "{\"details\": \"Push failed due to a network error. Please try again. If the problem persists, it may be due to a slow connection.\"}"),
			)
			server.AppendHandlers(
				slowImageConfigHandler,
				slowImageConfigHandler,
				slowImageConfigHandler,
			)
		}
		verifyRequests = []http.HandlerFunc{
			ghttp.VerifyRequest("GET", fmt.Sprintf("/v2/some_user/some_repo/blobs/%s", configDigest)),
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("X-Docker-Token", "token-1,token-2")
				w.Write([]byte(configBytes))
			}),
		}
		if serverConfig.WithTokenAuthorization {
			verifyRequests = append(
				verifyRequests,
				ghttp.VerifyHeaderKV("Authorization", "Bearer tokenstring"),
			)
		}
		if serverConfig.WithBasicAuthorization {
			verifyRequests = append(
				verifyRequests,
				ghttp.VerifyBasicAuth("username", "password"),
			)
		}
		server.AppendHandlers(
			ghttp.CombineHandlers(verifyRequests...),
		)
	}

	resultJSON := func(filename string) []byte {
		resultInfo, err := os.ReadFile(filename)
		Expect(err).NotTo(HaveOccurred())

		return resultInfo
	}

	Describe("ParseDockerRef", func() {
		Context("when the repo image is from dockerhub", func() {
			It("prepends 'library/' to the repo Name if there is no '/' character", func() {
				repositoryURL, repoName, _ := helpers.ParseDockerRef("redis")
				Expect(repositoryURL).To(Equal("registry-1.docker.io"))
				Expect(repoName).To(Equal("library/redis"))
			})

			It("does not prepends 'library/' to the repo Name if there is a '/' ", func() {
				repositoryURL, repoName, _ := helpers.ParseDockerRef("b/c")
				Expect(repositoryURL).To(Equal("registry-1.docker.io"))
				Expect(repoName).To(Equal("b/c"))
			})
		})

		Context("When the registryURL is not dockerhub", func() {
			It("does not add a '/' character to a single repo name", func() {
				repositoryURL, repoName, _ := helpers.ParseDockerRef("foobar:5123/baz")
				Expect(repositoryURL).To(Equal("foobar:5123"))
				Expect(repoName).To(Equal("baz"))
			})
		})

		Context("Parsing tags", func() {
			It("should parse tags based off the last colon", func() {
				_, _, tag := helpers.ParseDockerRef("baz/bot:test")
				Expect(tag).To(Equal("test"))
			})

			It("should default the tag to latest", func() {
				_, _, tag := helpers.ParseDockerRef("redis")
				Expect(tag).To(Equal("latest"))
			})
		})
	})

	Describe("FetchMetadata", func() {
		var registryURL string
		var repoName string
		var tag string
		var insecureRegistries []string
		var ctx *types.SystemContext

		BeforeEach(func() {
			server = ghttp.NewUnstartedServer()

			registryURL = server.Addr()

			repoName = "some_user/some_repo"
			tag = "latest"

			insecureRegistries = []string{}
		})

		JustBeforeEach(func() {
			fixturesPath := "fixtures"
			tlsCA := filepath.Join(fixturesPath, "testCA.crt")
			tlsCert := filepath.Join(fixturesPath, "localhost.cert")
			tlsKey := filepath.Join(fixturesPath, "localhost.key")

			ctx = &types.SystemContext{
				DockerAuthConfig: &types.DockerAuthConfig{
					Username: "username",
					Password: "password",
				},
				DockerCertPath: fixturesPath,
			}
			for _, insecure := range insecureRegistries {
				if registryURL == insecure {
					ctx.DockerInsecureSkipTLSVerify = types.OptionalBoolTrue
				}
			}

			if server.HTTPTestServer.TLS == nil {
				tlsConfig, err := tlsconfig.Build(
					tlsconfig.WithInternalServiceDefaults(),
					tlsconfig.WithIdentityFromFile(tlsCert, tlsKey),
				).Server(tlsconfig.WithClientAuthenticationFromFile(tlsCA))
				Expect(err).NotTo(HaveOccurred())
				tlsConfig.ClientAuth = tls.NoClientCert
				server.HTTPTestServer.TLS = tlsConfig
			}
			server.HTTPTestServer.StartTLS()
			var err error
			Expect(err).NotTo(HaveOccurred())
		})

		Context("with an invalid host", func() {
			BeforeEach(func() {
				registryURL = "qewr:5431"
			})

			It("should error", func() {
				_, err := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with an unknown repository", func() {
			BeforeEach(func() {
				server.AllowUnhandledRequests = true
				repoName = "some_user/not_some_repo"
			})

			It("should error", func() {
				_, err := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with an unknown tag", func() {
			BeforeEach(func() {
				server.AllowUnhandledRequests = true
				tag = "not_some_tag"
			})

			It("should error", func() {
				_, err := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with a valid repository reference", func() {
			Context("with manifest schema 1", func() {
				BeforeEach(func() {
					v2Schema1Manifest(serverResponseConfig{ImageTag: "latest"})
				})

				It("should not error", func() {
					_, err := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return the top-most image layer metadata", func() {
					imgConfig, _ := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
					Expect(imgConfig).NotTo(BeNil())
					Expect(imgConfig.Cmd).NotTo(BeNil())
					Expect(imgConfig.Cmd).To(Equal([]string{"dockerapp"}))
				})
			})

			Context("with manifest schema 2", func() {
				BeforeEach(func() {
					v2Schema2Manifest(serverResponseConfig{
						ImageConfig: v1.ImageConfig{Cmd: []string{"dockerapp"}},
						ImageTag:    "latest",
					})
				})

				It("should not error", func() {
					_, err := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return the top-most image layer metadata", func() {
					imgConfig, _ := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
					Expect(imgConfig).NotTo(BeNil())
					Expect(imgConfig.Cmd).NotTo(BeNil())
					Expect(imgConfig.Cmd).To(Equal([]string{"dockerapp"}))
				})
			})
		})

		Context("when the network connection is slow getting the image manifest", func() {
			BeforeEach(func() {
				v2Schema2Manifest(serverResponseConfig{
					ImageConfig:           v1.ImageConfig{},
					ImageTag:              tag,
					WithSlowImageManifest: true,
				})
			})

			It("should retry 3 times", func() {
				stderr := gbytes.NewBuffer()
				_, err := helpers.FetchMetadata(registryURL, repoName, tag, ctx, stderr)
				Expect(err).NotTo(HaveOccurred())

				Expect(stderr).To(gbytes.Say(`Failed getting docker image manifest by tag: .* retry attempt: 1`))
				Expect(stderr).To(gbytes.Say(`Failed getting docker image manifest by tag: .* retry attempt: 2`))
				Expect(stderr).To(gbytes.Say(`Failed getting docker image manifest by tag: .* retry attempt: 3`))
			})
		})

		Context("when the network connection is slow getting the image config", func() {
			BeforeEach(func() {
				v2Schema2Manifest(serverResponseConfig{
					ImageConfig:         v1.ImageConfig{},
					ImageTag:            tag,
					WithSlowImageConfig: true,
				})
			})

			It("should retry 3 times", func() {
				stderr := gbytes.NewBuffer()
				_, err := helpers.FetchMetadata(registryURL, repoName, tag, ctx, stderr)
				Expect(err).NotTo(HaveOccurred())

				Expect(stderr).To(gbytes.Say(`Failed getting docker image config by tag: .* retry attempt: 1`))
				Expect(stderr).To(gbytes.Say(`Failed getting docker image config by tag: .* retry attempt: 2`))
				Expect(stderr).To(gbytes.Say(`Failed getting docker image config by tag: .* retry attempt: 3`))
			})
		})

		Context("with a valid repository:tag reference", func() {
			BeforeEach(func() {
				v2Schema2Manifest(serverResponseConfig{
					ImageConfig: v1.ImageConfig{Cmd: []string{"dockerapp"}},
					ImageTag:    tag,
				})
			})

			It("should not error", func() {
				_, err := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the top-most image layer metadata", func() {
				imgConfig, _ := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
				Expect(imgConfig).NotTo(BeNil())
				Expect(imgConfig.Cmd).To(Equal([]string{"dockerapp"}))
			})
		})

		Context("when the image exposes custom ports", func() {
			BeforeEach(func() {
				v2Schema2Manifest(serverResponseConfig{
					ImageConfig: v1.ImageConfig{ExposedPorts: map[string]struct{}{"8080/tcp": {}}},
					ImageTag:    tag,
				})
			})

			It("should not error", func() {
				_, err := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the exposed ports", func() {
				imgConfig, _ := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
				Expect(imgConfig).NotTo(BeNil())
				Expect(imgConfig.ExposedPorts).To(HaveKeyWithValue("8080/tcp", struct{}{}))
			})
		})

		Context("with an insecure registry", func() {
			BeforeEach(func() {
				v2Schema2Manifest(serverResponseConfig{
					ImageConfig: v1.ImageConfig{Cmd: []string{"dockerapp"}},
					ImageTag:    tag,
				})
				server.HTTPTestServer.TLS = &tls.Config{InsecureSkipVerify: false}
				insecureRegistries = append(insecureRegistries, server.Addr())
			})

			It("should not error", func() {
				_, err := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the top-most image layer metadata", func() {
				imgConfig, _ := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
				Expect(imgConfig).NotTo(BeNil())
				Expect(imgConfig.Cmd).NotTo(BeNil())
				Expect(imgConfig.Cmd).To(Equal([]string{"dockerapp"}))
			})

			Context("that is not in the insecureRegistries list", func() {
				BeforeEach(func() {
					insecureRegistries = []string{}
				})

				It("should error", func() {
					_, err := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("https"))
				})
			})
		})

		Context("with a private registry with token authorization", func() {
			BeforeEach(func() {
				v2Schema2Manifest(serverResponseConfig{
					ImageConfig:            v1.ImageConfig{Cmd: []string{"dockerapp"}},
					ImageTag:               tag,
					WithTokenAuthorization: true,
				})
			})

			Context("with a valid repository:tag reference", func() {
				It("should not error", func() {
					_, err := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return the top-most image layer metadata", func() {
					imgConfig, _ := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
					Expect(imgConfig).NotTo(BeNil())
					Expect(imgConfig.Cmd).To(Equal([]string{"dockerapp"}))
				})
			})
		})

		Context("with a private registry with basic authorization", func() {
			BeforeEach(func() {
				v2Schema2Manifest(serverResponseConfig{
					ImageConfig:            v1.ImageConfig{Cmd: []string{"dockerapp"}},
					ImageTag:               tag,
					WithBasicAuthorization: true,
				})
			})

			Context("with a valid repository:tag reference", func() {
				It("should not error", func() {
					_, err := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return the top-most image layer metadata", func() {
					imgConfig, _ := helpers.FetchMetadata(registryURL, repoName, tag, ctx, os.Stderr)
					Expect(imgConfig).NotTo(BeNil())
					Expect(imgConfig.Cmd).To(Equal([]string{"dockerapp"}))
				})
			})
		})
	})

	Context("SaveMetadata", func() {
		var metadata protocol.DockerImageMetadata
		var outputDir string

		BeforeEach(func() {
			metadata = protocol.DockerImageMetadata{
				ExecutionMetadata: protocol.ExecutionMetadata{
					Cmd:        []string{"fake-arg1", "fake-arg2"},
					Entrypoint: []string{"fake-cmd", "fake-arg0"},
					Workdir:    "/fake-workdir",
				},
				DockerImage: "cloudfoundry/diego-docker-app",
			}
		})

		Context("to an unwritable path on disk", func() {
			It("should error", func() {
				err := helpers.SaveMetadata("////tmp/", &metadata)
				Expect(err).To(HaveOccurred())
			})
		})
		Context("with a writable path on disk", func() {

			BeforeEach(func() {
				var err error
				outputDir, err = os.MkdirTemp(os.TempDir(), "metadata")
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				os.RemoveAll(outputDir)
			})

			It("should output a json file", func() {
				filename := path.Join(outputDir, "result.json")
				err := helpers.SaveMetadata(filename, &metadata)
				Expect(err).NotTo(HaveOccurred())
				_, err = os.Stat(filename)
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("the json", func() {
				verifyMetadata := func(expectedEntryPoint []string, expectedStartCmd string) {
					err := helpers.SaveMetadata(path.Join(outputDir, "result.json"), &metadata)
					Expect(err).NotTo(HaveOccurred())
					result := resultJSON(path.Join(outputDir, "result.json"))

					var stagingResult dockerapplifecycle.StagingResult
					err = json.Unmarshal(result, &stagingResult)
					Expect(err).NotTo(HaveOccurred())

					Expect(stagingResult.ExecutionMetadata).NotTo(BeEmpty())
					Expect(stagingResult.ProcessTypes).NotTo(BeEmpty())

					var executionMetadata protocol.ExecutionMetadata
					err = json.Unmarshal([]byte(stagingResult.ExecutionMetadata), &executionMetadata)
					Expect(err).NotTo(HaveOccurred())

					Expect(executionMetadata.Cmd).To(Equal(metadata.ExecutionMetadata.Cmd))
					Expect(executionMetadata.Entrypoint).To(Equal(expectedEntryPoint))
					Expect(executionMetadata.Workdir).To(Equal(metadata.ExecutionMetadata.Workdir))

					Expect(stagingResult.ProcessTypes).To(HaveLen(1))
					Expect(stagingResult.ProcessTypes).To(HaveKeyWithValue("web", expectedStartCmd))

					Expect(stagingResult.LifecycleMetadata.DockerImage).To(Equal(metadata.DockerImage))
				}

				It("should contain the metadata", func() {
					verifyMetadata(metadata.ExecutionMetadata.Entrypoint, "fake-cmd fake-arg0 fake-arg1 fake-arg2")
				})

				Context("when the EntryPoint is empty", func() {
					BeforeEach(func() {
						metadata.ExecutionMetadata.Entrypoint = []string{}
					})

					It("contains all but the EntryPoint", func() {
						verifyMetadata(nil, "fake-arg1 fake-arg2")
					})
				})

				Context("when the EntryPoint is nil", func() {
					BeforeEach(func() {
						metadata.ExecutionMetadata.Entrypoint = nil
					})

					It("contains all but the EntryPoint", func() {
						verifyMetadata(metadata.ExecutionMetadata.Entrypoint, "fake-arg1 fake-arg2")
					})
				})
			})
		})
	})
})
