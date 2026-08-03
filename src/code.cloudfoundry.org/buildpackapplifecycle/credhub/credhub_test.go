package credhub_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/buildpackapplifecycle/containerpath"
	"code.cloudfoundry.org/buildpackapplifecycle/credhub"
	"code.cloudfoundry.org/goshims/osshim/os_fake"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("credhub", func() {
	Describe("InterpolateServiceRefs", func() {
		var (
			vcapServicesValue string
			server            *ghttp.Server
			fixturesSslDir    string
			err               error

			fakeOs  *os_fake.FakeOs
			subject *credhub.Credhub

			maxConnectAttempts int
			retryDelay         time.Duration
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
			fakeOs = &os_fake.FakeOs{}
			fakeEnv := make(map[string]string)
			fakeOs.SetenvStub = func(key, value string) error {
				fakeEnv[key] = value
				return nil
			}
			fakeOs.GetenvStub = func(key string) string {
				return fakeEnv[key]
			}
			fakeOs.UnsetenvStub = func(key string) error {
				delete(fakeEnv, key)
				return nil
			}

			fixturesSslDir, err := filepath.Abs(filepath.Join("..", "fixtures"))
			Expect(err).NotTo(HaveOccurred())

			server = ghttp.NewUnstartedServer()

			cert, err := tls.LoadX509KeyPair(filepath.Join(fixturesSslDir, "certs", "server.crt"), filepath.Join(fixturesSslDir, "certs", "server.key"))
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

			cpath := containerpath.New(fixturesSslDir)
			fakeOs.Setenv("USERPROFILE", fixturesSslDir)
			if cpath.For("/") == fixturesSslDir {
				fakeOs.Setenv("CF_INSTANCE_CERT", filepath.Join("/certs", "client.crt"))
				fakeOs.Setenv("CF_INSTANCE_KEY", filepath.Join("/certs", "client.key"))
				fakeOs.Setenv("CF_SYSTEM_CERT_PATH", "/cacerts")
			} else {
				fakeOs.Setenv("CF_INSTANCE_CERT", filepath.Join(fixturesSslDir, "certs", "client.crt"))
				fakeOs.Setenv("CF_INSTANCE_KEY", filepath.Join(fixturesSslDir, "certs", "client.key"))
				fakeOs.Setenv("CF_SYSTEM_CERT_PATH", filepath.Join(fixturesSslDir, "cacerts"))
			}

			maxConnectAttempts = 5
			retryDelay = 0 * time.Second
			subject = credhub.New(fakeOs, maxConnectAttempts, retryDelay)
		})

		AfterEach(func() {
			server.Close()
		})

		BeforeEach(func() {
			vcapServicesValue = `{"my-server":[{"credentials":{"credhub-ref":"(//my-server/creds)"}}]}`
			fakeOs.Setenv("VCAP_SERVICES", vcapServicesValue)
		})

		JustBeforeEach(func() {
			err = subject.InterpolateServiceRefs(server.URL())
		})

		Context("when there are no credhub refs in VCAP_SERVICES and no TLS environment variables are present", func() {
			BeforeEach(func() {
				fakeOs.Unsetenv("CF_INSTANCE_CERT")
				fakeOs.Unsetenv("CF_INSTANCE_KEY")
				fakeOs.Unsetenv("CF_SYSTEM_CERT_PATH")

				vcapServicesValue = `{"my-server":[{"credentials":{"no refs here":"and this string containing credhub-ref doesnt count"}}]}`
				fakeOs.Setenv("VCAP_SERVICES", vcapServicesValue)
			})

			It("does not fail and does not change VCAP_SERVICES", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeOs.Getenv("VCAP_SERVICES")).To(Equal(vcapServicesValue))
			})
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
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeOs.Getenv("VCAP_SERVICES")).To(Equal("INTERPOLATED_JSON"))
			})

			Context("when updating VCAP_SERVICES fails", func() {
				BeforeEach(func() {
					fakeOs.SetenvStub = func(key, value string) error {
						return fmt.Errorf("Setenv: setting %s failed", key)
					}
				})

				It("returns an error", func() {
					Expect(err).To(MatchError(MatchRegexp("Unable to update VCAP_SERVICES with interpolated credhub references")))
				})
			})
		})

		Context("when credhub fails initially, but eventually succeeds", func() {
			BeforeEach(func() {

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/api/v1/interpolate"),
						ghttp.VerifyBody([]byte(vcapServicesValue)),
						ghttp.RespondWith(http.StatusInternalServerError, "{}"),
					))

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/api/v1/interpolate"),
						ghttp.VerifyBody([]byte(vcapServicesValue)),
						VerifyClientCerts(),
						ghttp.RespondWith(http.StatusOK, "INTERPOLATED_JSON"),
					))
			})

			It("updates VCAP_SERVICES with the interpolated content and runs the process without VCAP_PLATFORM_OPTIONS", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeOs.Getenv("VCAP_SERVICES")).To(Equal("INTERPOLATED_JSON"))
			})
		})

		Context("when credhub always fails", func() {
			BeforeEach(func() {
				for attempt := 1; attempt <= maxConnectAttempts; attempt++ {
					server.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/api/v1/interpolate"),
							ghttp.VerifyBody([]byte(vcapServicesValue)),
							ghttp.RespondWith(http.StatusInternalServerError, "{}"),
						))
				}
			})

			Context("and it never succeeds", func() {
				It("returns an error and doesn't change VCAP_SERVICES", func() {
					Expect(err).To(MatchError(MatchRegexp("Unable to interpolate credhub references")))
					Expect(fakeOs.Getenv("VCAP_SERVICES")).To(Equal(vcapServicesValue))
				})
			})
		})

		Context("when the instance cert and key are invalid", func() {
			BeforeEach(func() {
				cpath := containerpath.New(fixturesSslDir)
				if cpath.For("/") == fixturesSslDir {
					fakeOs.Setenv("CF_INSTANCE_CERT", "not_a_cert")
					fakeOs.Setenv("CF_INSTANCE_KEY", "not_a_cert")
				} else {
					fakeOs.Setenv("CF_INSTANCE_CERT", filepath.Join(fixturesSslDir, "not_a_cert"))
					fakeOs.Setenv("CF_INSTANCE_KEY", filepath.Join(fixturesSslDir, "not_a_cert"))
				}
			})

			It("returns an error and doesn't change VCAP_SERVICES", func() {
				Expect(err).To(MatchError(MatchRegexp("Unable to set up credhub client")))
				Expect(fakeOs.Getenv("VCAP_SERVICES")).To(Equal(vcapServicesValue))
			})
		})

		Context("when the instance cert and key aren't set", func() {
			BeforeEach(func() {
				fakeOs.Unsetenv("CF_INSTANCE_CERT")
				fakeOs.Unsetenv("CF_INSTANCE_KEY")
			})

			It("returns an error and doesn't change VCAP_SERVICES", func() {
				Expect(err).To(MatchError(MatchRegexp("Missing CF_INSTANCE_CERT and/or CF_INSTANCE_KEY")))
				Expect(fakeOs.Getenv("VCAP_SERVICES")).To(Equal(vcapServicesValue))
			})
		})

		Context("when the system certs path isn't set", func() {
			BeforeEach(func() {
				fakeOs.Unsetenv("CF_SYSTEM_CERT_PATH")
			})

			It("prints an error message", func() {
				Expect(err).To(MatchError(MatchRegexp("Missing CF_SYSTEM_CERT_PATH")))
				Expect(fakeOs.Getenv("VCAP_SERVICES")).To(Equal(vcapServicesValue))
			})
		})

		Context("when credhub skip interpolation is set", func() {
			var originalVCAPServices string

			BeforeEach(func() {
				originalVCAPServices = fakeOs.Getenv("VCAP_SERVICES")
				fakeOs.Setenv("CREDHUB_SKIP_INTERPOLATION", "true")
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/api/v1/interpolate"),
						ghttp.VerifyBody([]byte(vcapServicesValue)),
						VerifyClientCerts(),
						ghttp.RespondWith(http.StatusOK, "JSON_RESPONSE"),
					))
			})

			It("does not change VCAP_SERVICES", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeOs.Getenv("VCAP_SERVICES")).To(Equal(originalVCAPServices))
			})
		})
	})
})
