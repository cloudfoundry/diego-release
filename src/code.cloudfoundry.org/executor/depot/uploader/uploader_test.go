package uploader_test

import (
	"crypto/md5"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"code.cloudfoundry.org/executor/depot/uploader"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/tlsconfig"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Uploader", func() {
	var (
		upldr             uploader.Uploader
		testServer        *httptest.Server
		serverRequests    []*http.Request
		serverRequestBody []string

		logger         *lagertest.TestLogger
		url            *url.URL
		file           *os.File
		expectedBytes  int
		expectedMD5    string
		expectedSHA256 string
		defaultTimeout time.Duration
	)

	BeforeEach(func() {
		testServer = nil
		serverRequestBody = []string{}
		serverRequests = []*http.Request{}
		logger = lagertest.NewTestLogger("test")

		file, _ = os.CreateTemp("", "foo")
		contentString := "content that we can check later"
		expectedBytes, _ = file.WriteString(contentString)
		rawMD5 := md5.Sum([]byte(contentString))
		expectedMD5 = base64.StdEncoding.EncodeToString(rawMD5[:])
		rawSHA256 := sha256.Sum256([]byte(contentString))
		expectedSHA256 = base64.StdEncoding.EncodeToString(rawSHA256[:])
		file.Close()
		defaultTimeout = 500 * time.Millisecond
	})

	AfterEach(func() {
		file.Close()
		if testServer != nil {
			testServer.Close()
		}
		os.Remove(file.Name())
	})

	Describe("Insecure Upload", func() {
		BeforeEach(func() {
			upldr = uploader.New(logger, defaultTimeout, nil)
		})

		Context("when the upload is successful", func() {
			BeforeEach(func() {
				testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					serverRequests = append(serverRequests, r)

					data, err := io.ReadAll(r.Body)
					Expect(err).NotTo(HaveOccurred())
					serverRequestBody = append(serverRequestBody, string(data))

					fmt.Fprintln(w, "Hello, client")
				}))

				serverUrl := testServer.URL + "/somepath"
				url, _ = url.Parse(serverUrl)
			})

			var err error
			var numBytes int64
			JustBeforeEach(func() {
				numBytes, err = upldr.Upload(file.Name(), url, nil)
			})

			It("uploads the file to the url providing Content-MD5 and Content-Digest headers", func() {
				Expect(len(serverRequests)).To(Equal(1))

				request := serverRequests[0]
				data := serverRequestBody[0]

				Expect(request.URL.Path).To(Equal("/somepath"))
				Expect(request.Header.Get("Content-Type")).To(Equal("application/octet-stream"))
				Expect(request.Header.Get("Content-MD5")).To(Equal(expectedMD5))
				Expect(request.Header.Get("Content-Digest")).To(Equal(fmt.Sprintf("sha-256=:%s:", expectedSHA256)))
				Expect(strconv.Atoi(request.Header.Get("Content-Length"))).To(BeNumerically("==", 31))
				Expect(string(data)).To(Equal("content that we can check later"))
			})

			It("returns the number of bytes written", func() {
				Expect(numBytes).To(Equal(int64(expectedBytes)))
			})

			It("does not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the upload is canceled", func() {
			var flushRequests chan struct{}
			var requestsInFlight *sync.WaitGroup

			BeforeEach(func() {
				flushRequests = make(chan struct{})

				requestsInFlight = new(sync.WaitGroup)
				testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer requestsInFlight.Done()
					<-flushRequests
				}))

				serverUrl := testServer.URL + "/somepath"
				url, _ = url.Parse(serverUrl)
			})

			AfterEach(func() {
				close(flushRequests)
				requestsInFlight.Wait()
			})

			It("interrupts the client and returns an error", func() {
				upldrWithoutTimeout := uploader.New(logger, 0, nil)

				cancel := make(chan struct{})
				errs := make(chan error)

				requestsInFlight.Add(1)

				go func() {
					_, err := upldrWithoutTimeout.Upload(file.Name(), url, cancel)
					errs <- err
				}()

				Consistently(errs).ShouldNot(Receive())

				close(cancel)

				Eventually(errs).Should(Receive(Equal(uploader.ErrUploadCancelled)))
			})
		})

		Context("when the upload times out", func() {
			var requestInitiated chan struct{}

			BeforeEach(func() {
				requestInitiated = make(chan struct{})

				testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requestInitiated <- struct{}{}

					time.Sleep(3 * defaultTimeout)
					fmt.Fprintln(w, "Hello, client")
				}))

				serverUrl := testServer.URL + "/somepath"
				url, _ = url.Parse(serverUrl)
			})

			It("should retry and log 3 times and return an error", func() {
				errs := make(chan error)

				go func() {
					_, err := upldr.Upload(file.Name(), url, nil)
					errs <- err
				}()

				Eventually(logger.TestSink.Buffer).Should(gbytes.Say("attempt"))
				Eventually(requestInitiated).Should(Receive())

				Eventually(logger.TestSink.Buffer).Should(gbytes.Say("attempt"))
				Eventually(requestInitiated).Should(Receive())

				Eventually(logger.TestSink.Buffer).Should(gbytes.Say("attempt"))
				Eventually(requestInitiated).Should(Receive())

				Eventually(logger.TestSink.Buffer).Should(gbytes.Say("failed-upload"))

				Expect(<-errs).To(HaveOccurred())
			})
		})

		Context("when the upload fails with a protocol error", func() {
			BeforeEach(func() {
				// No server to handle things!

				serverUrl := "http://127.0.0.1:54321/somepath"
				url, _ = url.Parse(serverUrl)
			})

			It("should return the error", func() {
				_, err := upldr.Upload(file.Name(), url, nil)
				Expect(err).NotTo(BeNil())
			})
		})

		Context("when the upload fails with a status code error", func() {
			BeforeEach(func() {
				testServer = httptest.NewServer(http.NotFoundHandler())

				serverUrl := testServer.URL + "/somepath"
				url, _ = url.Parse(serverUrl)
			})

			It("should return the error", func() {
				_, err := upldr.Upload(file.Name(), url, nil)
				Expect(err).NotTo(BeNil())
			})
		})
	})

	Describe("Secure Upload", func() {
		Context("when the server supports tls", func() {
			var (
				err                 error
				numBytes            int64
				fileserverTLSConfig *tls.Config
				tlsConfig           *tls.Config
			)

			BeforeEach(func() {
				testServer = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					serverRequests = append(serverRequests, r)

					data, err := io.ReadAll(r.Body)
					Expect(err).NotTo(HaveOccurred())
					serverRequestBody = append(serverRequestBody, string(data))

					fmt.Fprintln(w, "Hello, client")
				}))

				fileserverTLSConfig, err = tlsconfig.Build(
					tlsconfig.WithInternalServiceDefaults(),
					tlsconfig.WithIdentityFromFile("fixtures/correct/server.crt", "fixtures/correct/server.key"),
				).Server(
					tlsconfig.WithClientAuthenticationFromFile("fixtures/correct/server-ca.crt"),
				)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				testServer.TLS = fileserverTLSConfig
				testServer.StartTLS()
				serverUrl := testServer.URL + "/somepath"
				url, err = url.Parse(serverUrl)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the client has the correct credentials", func() {
				BeforeEach(func() {
					tlsConfig, err = tlsconfig.Build(
						tlsconfig.WithInternalServiceDefaults(),
						tlsconfig.WithIdentityFromFile("fixtures/correct/client.crt", "fixtures/correct/client.key"),
					).Client(
						tlsconfig.WithAuthorityFromFile("fixtures/correct/server-ca.crt"),
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("uploads the file to the url", func() {
					upldr = uploader.New(logger, defaultTimeout, tlsConfig)
					numBytes, err = upldr.Upload(file.Name(), url, nil)
					Expect(err).NotTo(HaveOccurred())

					Expect(len(serverRequests)).To(Equal(1))

					request := serverRequests[0]
					data := serverRequestBody[0]

					Expect(request.URL.Path).To(Equal("/somepath"))
					Expect(request.Header.Get("Content-Type")).To(Equal("application/octet-stream"))
					Expect(request.Header.Get("Content-MD5")).To(Equal(expectedMD5))
					Expect(strconv.Atoi(request.Header.Get("Content-Length"))).To(BeNumerically("==", 31))
					Expect(string(data)).To(Equal("content that we can check later"))
				})

				It("returns the number of bytes written", func() {
					upldr = uploader.New(logger, defaultTimeout, tlsConfig)
					numBytes, err = upldr.Upload(file.Name(), url, nil)
					Expect(err).NotTo(HaveOccurred())

					Expect(numBytes).To(Equal(int64(expectedBytes)))
				})
			})

			Context("when the client has a CA, but no keypair", func() {
				BeforeEach(func() {
					fileserverTLSConfig.ClientAuth = tls.NoClientCert

					tlsConfig, err = tlsconfig.Build(
						tlsconfig.WithInternalServiceDefaults(),
					).Client(
						tlsconfig.WithAuthorityFromFile("fixtures/correct/server-ca.crt"),
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("can communicate with the fileserver via one-sided TLS", func() {
					upldr = uploader.New(logger, defaultTimeout, tlsConfig)
					numBytes, err = upldr.Upload(file.Name(), url, nil)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when the client has incorrect certs", func() {
				It("fails when no certs are provided", func() {
					upldr = uploader.New(logger, defaultTimeout, nil)
					numBytes, err = upldr.Upload(file.Name(), url, nil)
					Expect(err).To(HaveOccurred())
				})

				It("fails when wrong cert/keypair is provided", func() {
					tlsConfig, err = tlsconfig.Build(
						tlsconfig.WithInternalServiceDefaults(),
						tlsconfig.WithIdentityFromFile("fixtures/incorrect/client.crt", "fixtures/incorrect/client.key"),
					).Client(
						tlsconfig.WithAuthorityFromFile("fixtures/correct/server-ca.crt"),
					)
					Expect(err).NotTo(HaveOccurred())
					upldr = uploader.New(logger, defaultTimeout, tlsConfig)
					numBytes, err = upldr.Upload(file.Name(), url, nil)
					Expect(err).To(HaveOccurred())
				})

				It("fails when ca cert is wrong", func() {
					tlsConfig, err = tlsconfig.Build(
						tlsconfig.WithInternalServiceDefaults(),
						tlsconfig.WithIdentityFromFile("fixtures/incorrect/client.crt", "fixtures/incorrect/client.key"),
					).Client(
						tlsconfig.WithAuthorityFromFile("fixtures/incorrect/server-ca.crt"),
					)
					Expect(err).NotTo(HaveOccurred())
					upldr = uploader.New(logger, defaultTimeout, tlsConfig)
					numBytes, err = upldr.Upload(file.Name(), url, nil)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
