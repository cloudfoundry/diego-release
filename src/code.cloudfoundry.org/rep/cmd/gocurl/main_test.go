package main_test

import (
	"bytes"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/tlsconfig"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Drain", func() {
	var (
		apiAddress string
	)

	drainCmd := func(args ...string) error {
		cmd := exec.Command(drainPath, args...)
		cmd.Stdout = GinkgoWriter
		cmd.Stderr = GinkgoWriter

		return cmd.Run()
	}

	Context("when no API address is given", func() {
		BeforeEach(func() {
			apiAddress = ""
		})

		It("fails with non-zero exit code", func() {
			err := drainCmd()
			Expect(err).To(HaveOccurred())

			var exitErr *exec.ExitError
			Expect(err).To(BeAssignableToTypeOf(exitErr))
			exitErr = err.(*exec.ExitError)
			Expect(exitErr.Success()).To(BeFalse())
		})
	})

	Context("when an API address is given", func() {
		BeforeEach(func() {
			apiServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/ping"),
					ghttp.RespondWith(http.StatusAccepted, ""),
				),
			)
			apiServer.Start()
			apiAddress = fmt.Sprintf("%s%s", apiServer.URL(), "/ping")
		})

		It(fmt.Sprintf("hits the %s endpoint over HTTP", "/ping"), func() {
			err := drainCmd(apiAddress)
			Expect(err).NotTo(HaveOccurred())

			Expect(apiServer.ReceivedRequests()).To(HaveLen(1))
		})

		Context("when a random 2xx status code is received", func() {
			BeforeEach(func() {
				apiServer.SetHandler(
					0,
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/ping"),
						ghttp.RespondWith(299, ""),
					),
				)
			})

			It(fmt.Sprintf("hits the %s endpoint over HTTP", "/ping"), func() {
				err := drainCmd(apiAddress)
				Expect(err).NotTo(HaveOccurred())

				Expect(apiServer.ReceivedRequests()).To(HaveLen(1))
			})
		})
	})

	Context("when TLS flags are passed in", func() {
		var (
			tlsCACertPath string
			tlsCertPath   string
			tlsKeyPath    string
		)
		BeforeEach(func() {
			tlsCACertPath = filepath.Join(fixturesPath, "testCA.crt")
			tlsCertPath = filepath.Join(fixturesPath, "localhost.crt")
			tlsKeyPath = filepath.Join(fixturesPath, "localhost.key")

			tlsConfig, err := tlsconfig.Build(
				tlsconfig.WithInternalServiceDefaults(),
				tlsconfig.WithIdentityFromFile(tlsCertPath, tlsKeyPath),
			).Server(tlsconfig.WithClientAuthenticationFromFile(tlsCACertPath))
			Expect(err).NotTo(HaveOccurred())

			apiServer.HTTPTestServer.TLS = tlsConfig
			apiServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/ping"),
					ghttp.RespondWith(http.StatusOK, ""),
				),
			)

			apiServer.HTTPTestServer.StartTLS()
			apiAddress = fmt.Sprintf("%s%s", apiServer.URL(), "/ping")
		})

		It(fmt.Sprintf("hits the %s endpoint over HTTPS", "/ping"), func() {
			err := drainCmd("--cacert", tlsCACertPath, "--cert", tlsCertPath, "--key", tlsKeyPath, apiAddress)
			Expect(err).NotTo(HaveOccurred())

			Expect(apiServer.ReceivedRequests()).To(HaveLen(1))
		})

		Context("when only some flags are provided", func() {
			It("fails with non-zero exit code", func() {
				err := drainCmd("--key", tlsKeyPath, apiAddress)
				var exitErr *exec.ExitError
				Expect(err).To(BeAssignableToTypeOf(exitErr))
				Expect(apiServer.ReceivedRequests()).To(HaveLen(0))
			})
		})
	})

	Context("when the API server is not running", func() {
		BeforeEach(func() {
			apiAddress = fmt.Sprintf("http://%s", "/ping")
		})

		It("fails with non-zero exit code", func() {
			err := drainCmd(apiAddress)
			Expect(err).To(HaveOccurred())
			var exitErr *exec.ExitError
			Expect(err).To(BeAssignableToTypeOf(exitErr))
			exitErr = err.(*exec.ExitError)
			Expect(exitErr.Success()).To(BeFalse())

			Expect(apiServer.ReceivedRequests()).To(HaveLen(0))
		})
	})

	Context("when the API server responds with an error", func() {
		BeforeEach(func() {
			apiServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/ping"),
					ghttp.RespondWith(http.StatusBadGateway, ""),
				),
			)
			apiServer.Start()
			apiAddress = fmt.Sprintf("%s%s", apiServer.URL(), "/ping")
		})

		It("fails with non-zero exit code", func() {
			err := drainCmd(apiAddress)
			Expect(err).To(HaveOccurred())
			var exitErr *exec.ExitError
			Expect(err).To(BeAssignableToTypeOf(exitErr))
			exitErr = err.(*exec.ExitError)
			Expect(exitErr.Success()).To(BeFalse())

			Expect(apiServer.ReceivedRequests()).To(HaveLen(1))
		})
	})

	Context("when the method is specified as POST", func() {
		BeforeEach(func() {
			apiServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/evacuate"),
					ghttp.RespondWith(http.StatusOK, ""),
				),
			)
			apiServer.Start()
			apiAddress = fmt.Sprintf("%s%s", apiServer.URL(), "/evacuate")
		})

		It("posts", func() {
			err := drainCmd("-X", "POST", apiAddress)
			Expect(err).NotTo(HaveOccurred())

			Expect(apiServer.ReceivedRequests()).To(HaveLen(1))
		})
	})

	Context("when the max-time is defined", func() {
		BeforeEach(func() {
			apiServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/ping"),
					func(w http.ResponseWriter, r *http.Request) {
						time.Sleep(10 * time.Second)
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("Ok"))
					},
				),
			)
			apiServer.Start()
			apiAddress = fmt.Sprintf("%s%s", apiServer.URL(), "/ping")
		})

		It("gets", func() {
			var stdout, stderr bytes.Buffer
			cmd := exec.Command(drainPath, "-max-time", "0.1s", apiAddress)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			Expect(err).To(HaveOccurred())
			Expect(stderr.String()).To(ContainSubstring("Client.Timeout exceeded while"))

			Expect(apiServer.ReceivedRequests()).To(HaveLen(1))
		})
	})

	Context("when the customer header is defined", func() {
		BeforeEach(func() {
			apiServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/ping"),
					func(w http.ResponseWriter, r *http.Request) {
						w.Write([]byte(r.Header.Get("Custom")))
					},
				),
			)
			apiServer.Start()
			apiAddress = fmt.Sprintf("%s%s", apiServer.URL(), "/ping")
		})

		It("gets", func() {
			var stdout, stderr bytes.Buffer
			cmd := exec.Command(drainPath, "-H", "Custom=something", apiAddress)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			Expect(err).NotTo(HaveOccurred(), stderr.String())
			Expect(stdout.String()).To(Equal("something"))
			Expect(apiServer.ReceivedRequests()).To(HaveLen(1))
		})
	})
})
