package cacheddownloader_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"code.cloudfoundry.org/cacheddownloader"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	downloadTimeout = 500 * time.Millisecond
)

var _ = Describe("Downloader", func() {
	var (
		downloader        *cacheddownloader.Downloader
		testServer        *httptest.Server
		serverRequestUrls []string
		lock              *sync.Mutex
		cancelChan        chan struct{}

		logger *lagertest.TestLogger
	)

	createDestFile := func() (*os.File, error) {
		return os.CreateTemp("", "foo")
	}

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		testServer = nil
		downloader = cacheddownloader.NewDownloader(downloadTimeout, 10, nil)
		lock = &sync.Mutex{}
		cancelChan = make(chan struct{})
	})

	Describe("Download", func() {
		var serverUrl *url.URL

		BeforeEach(func() {
			serverRequestUrls = []string{}
		})

		AfterEach(func() {
			if testServer != nil {
				testServer.Close()
			}
		})

		Context("when the download is successful", func() {
			var (
				downloadErr    error
				downloadedFile string

				downloadCachingInfo cacheddownloader.CachingInfoType
				expectedCachingInfo cacheddownloader.CachingInfoType
				expectedEtag        string
			)

			AfterEach(func() {
				if downloadedFile != "" {
					os.Remove(downloadedFile)
				}
			})

			JustBeforeEach(func() {
				serverUrl, _ = url.Parse(testServer.URL + "/somepath")
				downloadedFile, downloadCachingInfo, downloadErr = downloader.Download(logger, serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
			})

			Context("and contains a matching MD5 Hash in the Etag", func() {
				var attempts int
				BeforeEach(func() {
					attempts = 0
					testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						lock.Lock()
						serverRequestUrls = append(serverRequestUrls, r.RequestURI)
						attempts++
						lock.Unlock()

						msg := "Hello, client"
						hexMsg, err := cacheddownloader.HexValue("md5", msg)
						Expect(err).NotTo(HaveOccurred())
						expectedCachingInfo = cacheddownloader.CachingInfoType{
							ETag:         hexMsg,
							LastModified: "The 70s",
						}
						w.Header().Set("ETag", expectedCachingInfo.ETag)
						w.Header().Set("Last-Modified", expectedCachingInfo.LastModified)

						fmt.Fprint(w, msg)
					}))
				})

				It("does not return an error", func() {
					Expect(downloadErr).NotTo(HaveOccurred())
				})

				It("only tries once", func() {
					Expect(attempts).To(Equal(1))
				})

				It("claims to have downloaded", func() {
					Expect(downloadedFile).NotTo(BeEmpty())
				})

				It("gets a file from a url", func() {
					lock.Lock()
					urlFromServer := testServer.URL + serverRequestUrls[0]
					Expect(urlFromServer).To(Equal(serverUrl.String()))
					lock.Unlock()
				})

				It("should use the provided file as the download location", func() {
					fileContents, _ := os.ReadFile(downloadedFile)
					Expect(fileContents).To(ContainSubstring("Hello, client"))
				})

				It("returns the ETag", func() {
					Expect(downloadCachingInfo).To(Equal(expectedCachingInfo))
				})
			})

			Context("and contains an Etag that is not an MD5 Hash ", func() {
				BeforeEach(func() {
					expectedEtag = "not the hex you are looking for"
					testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("ETag", expectedEtag)
						fmt.Fprint(w, "Hello, client")
					}))
				})

				It("succeeds without doing a checksum", func() {
					Expect(downloadedFile).NotTo(BeEmpty())
					Expect(downloadErr).NotTo(HaveOccurred())
				})

				It("should returns the ETag in the caching info", func() {
					Expect(downloadCachingInfo.ETag).To(Equal(expectedEtag))
				})
			})

			Context("and contains no Etag at all", func() {
				BeforeEach(func() {
					testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						fmt.Fprint(w, "Hello, client")
					}))
				})

				It("succeeds without doing a checksum", func() {
					Expect(downloadedFile).NotTo(BeEmpty())
					Expect(downloadErr).NotTo(HaveOccurred())
				})

				It("should returns no ETag in the caching info", func() {
					Expect(downloadCachingInfo).To(BeZero())
				})
			})
		})

		Context("when the download times out", func() {
			var requestInitiated chan struct{}

			BeforeEach(func() {
				requestInitiated = make(chan struct{})

				testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requestInitiated <- struct{}{}

					time.Sleep(downloadTimeout * 2)
					fmt.Fprint(w, "Hello, client")
				}))

				serverUrl, _ = url.Parse(testServer.URL + "/somepath")
			})

			It("should retry cacheddownloader.MAX_DOWNLOAD_ATTEMPTS times and return an error", func() {
				errs := make(chan error)
				downloadedFiles := make(chan string)

				go func() {
					downloadedFile, _, err := downloader.Download(logger, serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
					errs <- err
					downloadedFiles <- downloadedFile
				}()

				for i := 0; i < cacheddownloader.MAX_DOWNLOAD_ATTEMPTS; i++ {
					Eventually(requestInitiated, 5*time.Second).Should(Receive())
				}

				Expect(<-errs).To(HaveOccurred())
				Expect(<-downloadedFiles).To(BeEmpty())
			})
		})

		Context("when the download fails with a protocol error", func() {
			BeforeEach(func() {
				// No server to handle things!
				serverUrl, _ = url.Parse("http://127.0.0.1:54321/somepath")
			})

			It("should return the error", func() {
				downloadedFile, _, err := downloader.Download(logger, serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
				Expect(err).To(HaveOccurred())
				Expect(downloadedFile).To(BeEmpty())
			})
		})

		Context("when the download fails with a status code error", func() {
			BeforeEach(func() {
				testServer = httptest.NewServer(http.NotFoundHandler())

				serverUrl, _ = url.Parse(testServer.URL + "/somepath")
			})

			It("should return the error", func() {
				downloadedFile, _, err := downloader.Download(logger, serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
				Expect(err).To(HaveOccurred())
				Expect(downloadedFile).To(BeEmpty())
			})
		})

		Context("when the read exceeds the deadline timeout", func() {
			var done chan struct{}

			BeforeEach(func() {
				done = make(chan struct{}, cacheddownloader.MAX_DOWNLOAD_ATTEMPTS)
				downloader = cacheddownloader.NewDownloaderWithIdleTimeout(1*time.Second, 30*time.Millisecond, 10, nil)

				testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(100 * time.Millisecond)
					done <- struct{}{}
				}))

				serverUrl, _ = url.Parse(testServer.URL + "/somepath")
			})

			AfterEach(func() {
				Eventually(done).Should(HaveLen(cacheddownloader.MAX_DOWNLOAD_ATTEMPTS))
			})

			It("fails with a nested read error", func() {
				errs := make(chan error)

				go func() {
					_, _, err := downloader.Download(logger, serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
					errs <- err
				}()

				var err error
				Eventually(errs, 1*time.Minute).Should(Receive(&err))
				uErr, ok := err.(*url.Error)
				Expect(ok).To(BeTrue())
				opErr, ok := uErr.Err.(*net.OpError)
				Expect(ok).To(BeTrue())
				Expect(opErr.Op).To(Equal("read"))
			})
		})

		Context("when the Content-Length does not match the downloaded file size", func() {
			BeforeEach(func() {
				testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					realMsg := "Hello, client"
					incompleteMsg := "Hello, clientsss"

					w.Header().Set("Content-Length", strconv.Itoa(len(realMsg)))

					fmt.Fprint(w, incompleteMsg)
				}))

				serverUrl, _ = url.Parse(testServer.URL + "/somepath")
			})

			It("should return an error", func() {
				downloadedFile, cachingInfo, err := downloader.Download(logger, serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
				Expect(err).To(HaveOccurred())
				Expect(downloadedFile).To(BeEmpty())
				Expect(cachingInfo).To(BeZero())
			})
		})

		Context("when cancelling", func() {
			var requestInitiated chan struct{}
			var completeRequest chan struct{}

			BeforeEach(func() {
				requestInitiated = make(chan struct{})
				completeRequest = make(chan struct{})

				testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requestInitiated <- struct{}{}
					select {
					case <-completeRequest:
					case <-time.After(30 * time.Second):
						return
					}
					for i := 0; i < 100000; i++ {
						w.Write(bytes.Repeat([]byte("a"), 1024))
					}
					w.(http.Flusher).Flush()
					select {
					case <-completeRequest:
						return
					case <-time.After(30 * time.Second):
						return
					}
				}))

				serverUrl, _ = url.Parse(testServer.URL + "/somepath")
			})

			It("cancels the request and returns the error", func() {
				errs := make(chan error)

				go func() {
					_, _, err := downloader.Download(logger, serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
					errs <- err
				}()

				Eventually(requestInitiated).Should(Receive())
				close(cancelChan)

				Eventually(errs).Should(Receive(SatisfyAll(
					BeAssignableToTypeOf(&cacheddownloader.DownloadCancelledError{}),
					MatchError(MatchRegexp(`Download cancelled: source 'fetch-request', duration .*, Error: .*`)),
				)))

				close(completeRequest)
			})

			It("stops the download", func() {
				errs := make(chan error)
				destFile, err := os.CreateTemp("", "foo")
				Expect(err).NotTo(HaveOccurred())

				createDestFile := func() (*os.File, error) {
					return destFile, nil
				}

				go func() {
					_, _, err := downloader.Download(logger, serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
					errs <- err
				}()

				Eventually(requestInitiated).Should(Receive())
				completeRequest <- struct{}{}

				// Make sure download started to local file
				Eventually(func() int {
					fi, err := os.Stat(destFile.Name())
					Expect(err).NotTo(HaveOccurred())
					return int(fi.Size())
				}).Should(BeNumerically(">", 10))
				close(cancelChan)

				Eventually(errs).Should(Receive(BeAssignableToTypeOf(&cacheddownloader.DownloadCancelledError{})))
				close(completeRequest)
				Consistently(logger).ShouldNot(gbytes.Say(`bytes-written":102400000`))
			})
		})

		Context("when using TLS", func() {
			var (
				downloadErr    error
				downloadedFile string
				downloaderTLS  *tls.Config
			)

			BeforeEach(func() {
				testServer = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, "Hello, client")
				}))

				tlsConfig, err := tlsconfig.Build(
					tlsconfig.WithInternalServiceDefaults(),
					tlsconfig.WithIdentityFromFile("fixtures/server.crt", "fixtures/server.key"),
				).Server(tlsconfig.WithClientAuthenticationFromFile("fixtures/goodCA.crt"))
				Expect(err).NotTo(HaveOccurred())
				testServer.TLS = tlsConfig

				downloaderTLS = &tls.Config{
					RootCAs:            x509.NewCertPool(),
					InsecureSkipVerify: false,
				}
			})

			JustBeforeEach(func() {
				testServer.StartTLS()
				serverUrl, _ = url.Parse(testServer.URL + "/somepath")
				downloadedFile, _, downloadErr = downloader.Download(logger, serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, cacheddownloader.ChecksumInfoType{}, cancelChan)
			})

			AfterEach(func() {
				if downloadedFile != "" {
					err := os.Remove(downloadedFile)
					Expect(err).NotTo(HaveOccurred())
				}
			})

			Context("and setting the correct CA", func() {
				BeforeEach(func() {
					downloaderTLS, err := tlsconfig.Build(
						tlsconfig.WithInternalServiceDefaults(),
						tlsconfig.WithIdentityFromFile("fixtures/goodClient.crt", "fixtures/goodClient.key"),
					).Client(tlsconfig.WithAuthorityFromFile("fixtures/goodCA.crt"))
					Expect(err).NotTo(HaveOccurred())

					downloader = cacheddownloader.NewDownloader(downloadTimeout, 10, downloaderTLS)
				})

				It("succeeds the download", func() {
					Expect(downloadErr).NotTo(HaveOccurred())
					Expect(downloadedFile).NotTo(BeEmpty())
				})
			})

			Context("and setting the incorrect CA", func() {
				BeforeEach(func() {
					downloaderTLS, err := tlsconfig.Build(
						tlsconfig.WithInternalServiceDefaults(),
						tlsconfig.WithIdentityFromFile("fixtures/goodClient.crt", "fixtures/goodClient.key"),
					).Client(tlsconfig.WithAuthorityFromFile("fixtures/badCA.crt"))
					Expect(err).NotTo(HaveOccurred())

					downloader = cacheddownloader.NewDownloader(downloadTimeout, 10, downloaderTLS)
				})

				It("fails the download", func() {
					Expect(downloadErr).To(HaveOccurred())
					Expect(downloadedFile).To(BeEmpty())
				})
			})

			Context("and server does not require client auth", func() {
				BeforeEach(func() {
					testServer.TLS.ClientAuth = tls.NoClientCert

					goodCA, err := os.ReadFile("fixtures/goodCA.crt")
					Expect(err).NotTo(HaveOccurred())

					ok := downloaderTLS.RootCAs.AppendCertsFromPEM(goodCA)
					Expect(ok).To(BeTrue())

					downloader = cacheddownloader.NewDownloader(downloadTimeout, 10, downloaderTLS)
				})

				It("does not need a client certificate and succeeds", func() {
					Expect(downloadErr).NotTo(HaveOccurred())
					Expect(downloadedFile).NotTo(BeEmpty())
				})
			})

			Context("and setting multiple CAs, including the correct one", func() {
				BeforeEach(func() {

					downloaderTLS, err := tlsconfig.Build(
						tlsconfig.WithInternalServiceDefaults(),
						tlsconfig.WithIdentityFromFile("fixtures/goodClient.crt", "fixtures/goodClient.key"),
					).Client(tlsconfig.WithAuthorityFromFile("fixtures/badCA.crt"))
					Expect(err).NotTo(HaveOccurred())

					goodCA, err := os.ReadFile("fixtures/goodCA.crt")
					Expect(err).NotTo(HaveOccurred())

					ok := downloaderTLS.RootCAs.AppendCertsFromPEM(goodCA)
					Expect(ok).To(BeTrue())

					downloader = cacheddownloader.NewDownloader(downloadTimeout, 10, downloaderTLS)
				})

				It("succeeds the download", func() {
					Expect(downloadErr).NotTo(HaveOccurred())
					Expect(downloadedFile).NotTo(BeEmpty())
				})
			})

			Context("and skipping certificate verification", func() {
				BeforeEach(func() {

					downloaderTLS, err := tlsconfig.Build(
						tlsconfig.WithInternalServiceDefaults(),
						tlsconfig.WithIdentityFromFile("fixtures/goodClient.crt", "fixtures/goodClient.key"),
					).Client(tlsconfig.WithAuthorityFromFile("fixtures/badCA.crt"))
					Expect(err).NotTo(HaveOccurred())

					downloaderTLS.InsecureSkipVerify = true

					downloader = cacheddownloader.NewDownloader(downloadTimeout, 10, downloaderTLS)
				})

				It("succeeds without doing checking certificate validity", func() {
					Expect(downloadErr).NotTo(HaveOccurred())
					Expect(downloadedFile).NotTo(BeEmpty())
				})
			})

			// look into this and verify
			Context("without any trusted CAs", func() {
				BeforeEach(func() {
					downloader = cacheddownloader.NewDownloader(downloadTimeout, 10, nil)
				})

				It("fails the download", func() {
					Expect(downloadedFile).To(BeEmpty())
					Expect(downloadErr).To(HaveOccurred())
				})
			})

			Context("without a certificate", func() {
				BeforeEach(func() {
					downloader = cacheddownloader.NewDownloader(downloadTimeout, 10, downloaderTLS)
				})

				It("succeeds without doing checking certificate validity", func() {
					Expect(downloadedFile).To(BeEmpty())
					Expect(downloadErr).To(HaveOccurred())
				})
			})

			Context("with an incorrect certificate", func() {
				BeforeEach(func() {

					downloaderTLS, err := tlsconfig.Build(
						tlsconfig.WithInternalServiceDefaults(),
						tlsconfig.WithIdentityFromFile("fixtures/badClient.crt", "fixtures/badClient.key"),
					).Client(tlsconfig.WithAuthorityFromFile("fixtures/badCA.crt"))
					Expect(err).NotTo(HaveOccurred())

					downloader = cacheddownloader.NewDownloader(downloadTimeout, 10, downloaderTLS)
				})

				It("fails the download", func() {
					Expect(downloadedFile).To(BeEmpty())
					Expect(downloadErr).To(HaveOccurred())
				})
			})
		})

		Context("when checksum info is provided", func() {
			var msg string
			BeforeEach(func() {
				msg = "Hello, client"
				testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, msg)
				}))
				serverUrl, _ = url.Parse(testServer.URL + "/somepath")
			})

			Context("when algorithm is invalid", func() {
				It("should return an algorithm invalid error", func() {
					checksum := cacheddownloader.ChecksumInfoType{Algorithm: "wrong alg", Value: "some value"}
					downloadedFile, _, err := downloader.Download(logger, serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, checksum, cancelChan)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("algorithm invalid"))
					Expect(downloadedFile).To(BeEmpty())
				})
			})

			Context("when the value is invalid", func() {
				It("should return a checksum invalid error", func() {
					checksum := cacheddownloader.ChecksumInfoType{Algorithm: "md5", Value: "wrong value"}
					downloadedFile, _, err := downloader.Download(logger, serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, checksum, cancelChan)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("checksum missing or invalid"))
					Expect(downloadedFile).To(BeEmpty())
				})
			})

			Context("when the checksum algorithm is supported", func() {
				It("with an md5 checksum", func() {
					hexMsg, err := cacheddownloader.HexValue("md5", msg)
					Expect(err).NotTo(HaveOccurred())

					checksum := cacheddownloader.ChecksumInfoType{Algorithm: "md5", Value: hexMsg}
					downloadedFile, _, err := downloader.Download(logger, serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, checksum, cancelChan)
					Expect(err).NotTo(HaveOccurred())
					Expect(downloadedFile).NotTo(BeEmpty())
				})

				It("with a sha1 checksum", func() {
					hexMsg, err := cacheddownloader.HexValue("sha1", msg)
					Expect(err).NotTo(HaveOccurred())

					checksum := cacheddownloader.ChecksumInfoType{Algorithm: "sha1", Value: hexMsg}
					downloadedFile, _, err := downloader.Download(logger, serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, checksum, cancelChan)
					Expect(err).NotTo(HaveOccurred())
					Expect(downloadedFile).NotTo(BeEmpty())
				})

				It("with a sha256 checksum", func() {
					hexMsg, err := cacheddownloader.HexValue("sha256", msg)
					Expect(err).NotTo(HaveOccurred())

					checksum := cacheddownloader.ChecksumInfoType{Algorithm: "sha256", Value: hexMsg}
					downloadedFile, _, err := downloader.Download(logger, serverUrl, createDestFile, cacheddownloader.CachingInfoType{}, checksum, cancelChan)
					Expect(err).NotTo(HaveOccurred())
					Expect(downloadedFile).NotTo(BeEmpty())
				})
			})
		})
	})

	Describe("Concurrent downloads", func() {
		var (
			server    *ghttp.Server
			serverUrl *url.URL
			barrier   chan interface{}
			results   chan bool
			tempDir   string
		)

		BeforeEach(func() {
			barrier = make(chan interface{}, 1)
			results = make(chan bool, 1)

			downloader = cacheddownloader.NewDownloader(1*time.Second, 1, nil)

			var err error
			tempDir, err = os.MkdirTemp("", "temp-dl-dir")
			Expect(err).NotTo(HaveOccurred())

			server = ghttp.NewServer()
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/the-file"),
					http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						barrier <- nil
						Consistently(results, .5).ShouldNot(Receive())
					}),
					ghttp.RespondWith(http.StatusOK, "download content", http.Header{}),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/the-file"),
					http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						results <- true
					}),
					ghttp.RespondWith(http.StatusOK, "download content", http.Header{}),
				),
			)

			serverUrl, _ = url.Parse(server.URL() + "/the-file")
		})

		AfterEach(func() {
			server.Close()
			os.RemoveAll(tempDir)
		})

		downloadTestFile := func(cancelChan <-chan struct{}) (path string, cachingInfoOut cacheddownloader.CachingInfoType, err error) {
			return downloader.Download(
				logger,
				serverUrl,
				func() (*os.File, error) {
					return os.CreateTemp(tempDir, "the-file")
				},
				cacheddownloader.CachingInfoType{},
				cacheddownloader.ChecksumInfoType{},
				cancelChan,
			)
		}

		It("only allows n downloads at the same time", func() {
			go func() {
				downloadTestFile(make(chan struct{}))
				barrier <- nil
			}()

			<-barrier
			downloadTestFile(cancelChan)
			<-barrier
		})

		Context("when cancelling", func() {
			It("bails when waiting", func() {
				go func() {
					downloadTestFile(make(chan struct{}))
					barrier <- nil
				}()

				<-barrier
				cancelChan := make(chan struct{})
				close(cancelChan)
				_, _, err := downloadTestFile(cancelChan)
				Expect(err).To(BeAssignableToTypeOf(&cacheddownloader.DownloadCancelledError{}))
				<-barrier
			})
		})

		Context("Downloading with caching info", func() {
			var (
				server     *ghttp.Server
				cachedInfo cacheddownloader.CachingInfoType
				statusCode int
				serverUrl  *url.URL
				body       string
			)

			BeforeEach(func() {
				cachedInfo = cacheddownloader.CachingInfoType{
					ETag:         "It's Just a Flesh Wound",
					LastModified: "The 60s",
				}

				server = ghttp.NewServer()
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/get-the-file"),
					ghttp.VerifyHeader(http.Header{
						"If-None-Match":     []string{cachedInfo.ETag},
						"If-Modified-Since": []string{cachedInfo.LastModified},
					}),
					ghttp.RespondWithPtr(&statusCode, &body),
				))

				serverUrl, _ = url.Parse(server.URL() + "/get-the-file")
			})

			AfterEach(func() {
				server.Close()
			})

			Context("when the server replies with 304", func() {
				BeforeEach(func() {
					statusCode = http.StatusNotModified
				})

				It("should return that it did not download", func() {
					downloadedFile, _, err := downloader.Download(logger, serverUrl, createDestFile, cachedInfo, cacheddownloader.ChecksumInfoType{}, cancelChan)
					Expect(downloadedFile).To(BeEmpty())
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when the server replies with 200", func() {
				var (
					downloadedFile string
					err            error
				)

				BeforeEach(func() {
					statusCode = http.StatusOK
					body = "quarb!"
				})

				AfterEach(func() {
					if downloadedFile != "" {
						os.Remove(downloadedFile)
					}
				})

				It("should download the file", func() {
					downloadedFile, _, err = downloader.Download(logger, serverUrl, createDestFile, cachedInfo, cacheddownloader.ChecksumInfoType{}, cancelChan)
					Expect(err).NotTo(HaveOccurred())

					info, err := os.Stat(downloadedFile)
					Expect(err).NotTo(HaveOccurred())
					Expect(info.Size()).To(Equal(int64(len(body))))
				})
			})

			Context("for anything else (including a server error)", func() {
				BeforeEach(func() {
					statusCode = http.StatusInternalServerError

					// cope with built in retry
					for i := 0; i < cacheddownloader.MAX_DOWNLOAD_ATTEMPTS; i++ {
						server.AppendHandlers(ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/get-the-file"),
							ghttp.VerifyHeader(http.Header{
								"If-None-Match":     []string{cachedInfo.ETag},
								"If-Modified-Since": []string{cachedInfo.LastModified},
							}),
							ghttp.RespondWithPtr(&statusCode, &body),
						))
					}
				})

				It("should return false with an error", func() {
					downloadedFile, _, err := downloader.Download(logger, serverUrl, createDestFile, cachedInfo, cacheddownloader.ChecksumInfoType{}, cancelChan)
					Expect(downloadedFile).To(BeEmpty())
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
