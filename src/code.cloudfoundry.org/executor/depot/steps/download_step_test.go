package steps_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"net/url"
	"os"
	"strings"

	"code.cloudfoundry.org/cacheddownloader"
	cdfakes "code.cloudfoundry.org/cacheddownloader/cacheddownloaderfakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/garden"

	"code.cloudfoundry.org/executor/depot/log_streamer/fake_log_streamer"
	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/executor/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"

	archiveHelper "code.cloudfoundry.org/archiver/extractor/test_helper"
)

var _ = Describe("DownloadAction", func() {
	var (
		step ifrit.Runner

		downloadAction models.DownloadAction
		cache          *cdfakes.FakeCachedDownloader
		gardenClient   *fakes.FakeGardenClient
		fakeStreamer   *fake_log_streamer.FakeLogStreamer
		logger         *lagertest.TestLogger
		rateLimiter    chan struct{}
	)

	handle := "some-container-handle"

	BeforeEach(func() {
		cache = &cdfakes.FakeCachedDownloader{}
		cache.FetchReturns(io.NopCloser(new(bytes.Buffer)), 42, nil)

		downloadAction = models.DownloadAction{
			From:     "http://mr_jones",
			To:       "/tmp/Antarctica",
			CacheKey: "the-cache-key",
			User:     "notroot",
		}

		gardenClient = fakes.NewGardenClient()

		fakeStreamer = newFakeStreamer()
		logger = lagertest.NewTestLogger("test")

		rateLimiter = make(chan struct{}, 1)
	})

	Describe("Run", func() {
		var (
			stepErr error
		)

		JustBeforeEach(func() {
			container, err := gardenClient.Create(garden.ContainerSpec{
				Handle: handle,
			})
			Expect(err).NotTo(HaveOccurred())

			step = steps.NewDownload(
				container,
				downloadAction,
				cache,
				rateLimiter,
				fakeStreamer,
				logger,
			)

			stepErr = <-ifrit.Invoke(step).Wait()
		})

		var tarReader *tar.Reader

		It("downloads via the cache with a tar transformer", func() {
			Expect(cache.FetchCallCount()).To(Equal(1))

			_, url, cacheKey, checksumInfo, cancelChan := cache.FetchArgsForCall(0)
			Expect(url.Host).To(ContainSubstring("mr_jones"))
			Expect(cacheKey).To(Equal("the-cache-key"))
			Expect(checksumInfo.Algorithm).To(Equal(""))
			Expect(checksumInfo.Value).To(Equal(""))
			Expect(cancelChan).NotTo(BeNil())
		})

		Context("when checksum is provided", func() {
			BeforeEach(func() {
				downloadAction.ChecksumAlgorithm = "md5"
				downloadAction.ChecksumValue = "checksum-value"
			})

			It("downloads via the cache with a tar tranformer and specified checksum", func() {
				Expect(cache.FetchCallCount()).To(Equal(1))

				_, url, cacheKey, checksumInfo, cancelChan := cache.FetchArgsForCall(0)
				Expect(url.Host).To(ContainSubstring("mr_jones"))
				Expect(cacheKey).To(Equal("the-cache-key"))
				Expect(checksumInfo.Algorithm).To(Equal("md5"))
				Expect(checksumInfo.Value).To(Equal("checksum-value"))
				Expect(cancelChan).NotTo(BeNil())
			})
		})

		It("logs the step", func() {
			Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
				"test.download-step.acquiring-limiter",
				"test.download-step.acquired-limiter",
				"test.download-step.fetch-starting",
				"test.download-step.fetch-complete",
				"test.download-step.stream-in-starting",
				"test.download-step.stream-in-complete",
			}))
		})

		Context("when an artifact is not specified", func() {
			It("does not stream the download information", func() {
				err := <-ifrit.Invoke(step).Wait()
				Expect(err).NotTo(HaveOccurred())

				stdout := fakeStreamer.Stdout().(*gbytes.Buffer)
				Expect(stdout.Contents()).To(BeEmpty())
			})
		})

		Context("when an artifact is specified", func() {
			BeforeEach(func() {
				downloadAction.Artifact = "artifact"
			})

			Describe("logging the size", func() {
				Context("when nothing had to be downloaded", func() {
					BeforeEach(func() {
						cache.FetchReturns(gbytes.NewBuffer(), 0, nil) // 0 bytes downlaoded
					})

					It("streams unknown when the Fetch does not return a File", func() {
						Expect(stepErr).NotTo(HaveOccurred())

						stdout := fakeStreamer.Stdout().(*gbytes.Buffer)
						Expect(stdout.Contents()).To(ContainSubstring("Downloaded artifact\n"))
					})
				})

				Context("when data was downloaded", func() {
					BeforeEach(func() {
						cache.FetchReturns(gbytes.NewBuffer(), 42, nil)
					})

					It("streams the size when the Fetch returns a File", func() {
						Expect(stepErr).NotTo(HaveOccurred())

						stdout := fakeStreamer.Stdout().(*gbytes.Buffer)
						Expect(stdout.Contents()).To(ContainSubstring("Downloaded artifact (42B)"))
					})
				})
			})
		})

		Context("when there is an error parsing the download url", func() {
			BeforeEach(func() {
				downloadAction.From = "foo/bar"
			})

			It("returns an error", func() {
				Expect(stepErr).To(HaveOccurred())
			})

			It("logs the step", func() {
				Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
					"test.download-step.acquiring-limiter",
					"test.download-step.acquired-limiter",
					"test.download-step.fetch-starting",
					"test.download-step.parse-request-uri-error",
				}))
			})
		})

		Context("and the fetched bits are a valid tarball", func() {
			BeforeEach(func() {
				tarFile := createTempTar()
				defer os.Remove(tarFile.Name())

				cache.FetchReturns(tarFile, 42, nil)
			})

			Context("and streaming in succeeds", func() {
				BeforeEach(func() {
					buffer := &bytes.Buffer{}
					tarReader = tar.NewReader(buffer)

					gardenClient.Connection.StreamInStub = func(handle string, spec garden.StreamInSpec) error {
						Expect(spec.Path).To(Equal("/tmp/Antarctica"))
						Expect(spec.User).To(Equal("notroot"))

						_, err := io.Copy(buffer, spec.TarStream)
						Expect(err).NotTo(HaveOccurred())

						return nil
					}
				})

				It("does not return an error", func() {
					Expect(stepErr).NotTo(HaveOccurred())
				})

				It("places the file in the container under the destination", func() {
					header, err := tarReader.Next()
					Expect(err).NotTo(HaveOccurred())
					Expect(header.Name).To(Equal("file1"))
				})
			})

			Context("when there is an error copying the extracted files into the container", func() {
				var expectedErr error

				Context("when the error message is under 1kb", func() {
					BeforeEach(func() {
						expectedErr = errors.New("oh no!")
						gardenClient.Connection.StreamInReturns(expectedErr)
					})

					It("returns an error", func() {
						Expect(stepErr.Error()).To(ContainSubstring("Copying into the container failed: oh no!"))
					})

					It("streams an error", func() {
						stderr := fakeStreamer.Stderr().(*gbytes.Buffer)
						Expect(stderr.Contents()).To(ContainSubstring("Copying into the container failed: oh no!\n"))
					})

					It("logs the step", func() {
						Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
							"test.download-step.acquiring-limiter",
							"test.download-step.acquired-limiter",
							"test.download-step.fetch-starting",
							"test.download-step.fetch-complete",
							"test.download-step.stream-in-starting",
							"test.download-step.stream-in-failed",
						}))
					})

					Context("when the artifact has a name", func() {
						BeforeEach(func() {
							downloadAction.Artifact = "artifact"
						})

						It("returns an error", func() {
							Expect(stepErr.Error()).To(ContainSubstring("Copying artifact into the container failed: oh no!"))
						})

						It("streams an error", func() {
							stderr := fakeStreamer.Stderr().(*gbytes.Buffer)
							Expect(stderr.Contents()).To(ContainSubstring("Copying artifact into the container failed: oh no!\n"))
						})
					})
				})

				Context("when the error message is over 1kb", func() {
					BeforeEach(func() {
						error_message := strings.Repeat("error", 1024)
						expectedErr = errors.New(error_message)

						gardenClient.Connection.StreamInReturns(expectedErr)

					})

					It("truncates the error", func() {
						stderr := fakeStreamer.Stderr().(*gbytes.Buffer)
						Expect(stderr.Contents()).To(ContainSubstring("Copying into the container failed"))
						Expect(stderr.Contents()).To(ContainSubstring("(error truncated)"))
						Expect([]byte(stderr.Contents())).Should(HaveLen(1024))
					})
				})
			})
		})

		Context("when there is an error fetching the file", func() {
			BeforeEach(func() {
				cache.FetchReturns(nil, 0, errors.New("oh no!"))
			})

			It("returns an error", func() {
				Expect(stepErr.Error()).To(ContainSubstring("Downloading failed"))
			})

			It("does not stream an error", func() {
				stderr := fakeStreamer.Stderr().(*gbytes.Buffer)
				Expect(stderr.Contents()).NotTo(ContainSubstring("Downloading.*failed"))
			})

			It("logs the step", func() {
				Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
					"test.download-step.acquiring-limiter",
					"test.download-step.acquired-limiter",
					"test.download-step.fetch-starting",
					"test.download-step.fetch-failed",
				}))

			})

			Context("when the artifact has a name", func() {
				BeforeEach(func() {
					downloadAction.Artifact = "artifact"
				})

				It("returns an error", func() {
					Expect(stepErr.Error()).To(ContainSubstring("Downloading artifact failed"))
				})

				It("streams an error", func() {
					stderr := fakeStreamer.Stderr().(*gbytes.Buffer)
					Expect(stderr.Contents()).To(ContainSubstring("Downloading artifact failed\n"))
				})
			})
		})
	})

	Describe("Ready", func() {
		var (
			p ifrit.Process
		)

		BeforeEach(func() {
			cache.FetchStub = func(_ lager.Logger, u *url.URL, key string, checksumInfo cacheddownloader.ChecksumInfoType, cancelCh <-chan struct{}) (io.ReadCloser, int64, error) {
				<-cancelCh
				return nil, 0, errors.New("some error indicating a cancel")
			}

			container, err := gardenClient.Create(garden.ContainerSpec{
				Handle: handle,
			})
			Expect(err).NotTo(HaveOccurred())

			step = steps.NewDownload(
				container,
				downloadAction,
				cache,
				rateLimiter,
				fakeStreamer,
				logger,
			)
		})

		JustBeforeEach(func() {
			p = ifrit.Background(step)
		})

		AfterEach(func() {
			p.Signal(os.Interrupt)
		})

		It("becomes ready immediately", func() {
			Eventually(p.Ready()).Should(BeClosed())
		})
	})

	Describe("Cancel", func() {
		var (
			p ifrit.Process
		)

		BeforeEach(func() {
			container, err := gardenClient.Create(garden.ContainerSpec{
				Handle: handle,
			})
			Expect(err).NotTo(HaveOccurred())

			step = steps.NewDownload(
				container,
				downloadAction,
				cache,
				rateLimiter,
				fakeStreamer,
				logger,
			)
		})

		JustBeforeEach(func() {
			p = ifrit.Background(step)
		})

		Context("when waiting on the rate limiter", func() {
			BeforeEach(func() {
				rateLimiter <- struct{}{}
			})

			It("cancels the wait", func() {
				p.Signal(os.Interrupt)
				Eventually(p.Wait()).Should(Receive(Equal(new(steps.CancelledError))))
			})

			It("does not fetch the download artifact", func() {
				p.Signal(os.Interrupt)
				Eventually(p.Wait()).Should(Receive(Equal(new(steps.CancelledError))))
				Expect(cache.FetchCallCount()).To(Equal(0))
			})
		})

		Context("when downloading the file", func() {
			var (
				calledChan chan struct{}
			)

			BeforeEach(func() {
				calledChan = make(chan struct{})

				cache.FetchStub = func(_ lager.Logger, u *url.URL, key string, checksumInfo cacheddownloader.ChecksumInfoType, cancelCh <-chan struct{}) (io.ReadCloser, int64, error) {
					defer GinkgoRecover()

					Expect(cancelCh).NotTo(BeNil())
					Expect(cancelCh).NotTo(BeClosed())

					close(calledChan)
					<-cancelCh

					Expect(cancelCh).To(BeClosed())

					return nil, 0, errors.New("some error indicating a cancel")
				}
			})

			It("closes the cancel channel and propagates the cancel error", func() {
				Eventually(calledChan).Should(BeClosed())
				p.Signal(os.Interrupt)

				Eventually(p.Wait()).Should(Receive(Equal(new(steps.CancelledError))))
			})
		})

		Context("when streaming the file into the container", func() {
			var (
				calledChan  chan struct{}
				barrierChan chan struct{}
			)

			BeforeEach(func() {
				tarFile := createTempTar()
				defer os.Remove(tarFile.Name())
				cache.FetchReturns(tarFile, 0, nil)

				calledChan = make(chan struct{})
				barrierChan = make(chan struct{})

				gardenClient.Connection.StreamInStub = func(handle string, spec garden.StreamInSpec) error {
					writer := func(p []byte) (n int, err error) {
						close(calledChan)

						<-barrierChan
						return 1, nil
					}
					_, err := io.Copy(WriteFunc(writer), spec.TarStream)
					return err
				}
			})

			AfterEach(func() {
				close(barrierChan)
			})

			It("aborts the streaming", func() {
				Eventually(calledChan).Should(BeClosed())
				p.Signal(os.Interrupt)

				Eventually(p.Wait()).Should(Receive(Equal(new(steps.CancelledError))))
			})
		})
	})

	Describe("the downloads are rate limited", func() {
		var container garden.Container

		BeforeEach(func() {
			var err error
			container, err = gardenClient.Create(garden.ContainerSpec{
				Handle: handle,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("allows only N concurrent downloads", func() {
			rateLimiter := make(chan struct{}, 2)

			downloadAction1 := models.DownloadAction{
				From: "http://mr_jones1",
				To:   "/tmp/Antarctica",
			}

			step1 := steps.NewDownload(
				container,
				downloadAction1,
				cache,
				rateLimiter,
				fakeStreamer,
				logger,
			)

			downloadAction2 := models.DownloadAction{
				From: "http://mr_jones2",
				To:   "/tmp/Antarctica",
			}

			step2 := steps.NewDownload(
				container,
				downloadAction2,
				cache,
				rateLimiter,
				fakeStreamer,
				logger,
			)

			downloadAction3 := models.DownloadAction{
				From: "http://mr_jones3",
				To:   "/tmp/Antarctica",
			}

			step3 := steps.NewDownload(
				container,
				downloadAction3,
				cache,
				rateLimiter,
				fakeStreamer,
				logger,
			)

			fetchCh := make(chan struct{}, 3)
			barrier := make(chan struct{})
			nopCloser := io.NopCloser(new(bytes.Buffer))
			cache.FetchStub = func(_ lager.Logger, urlToFetch *url.URL, cacheKey string, checksumInfo cacheddownloader.ChecksumInfoType, cancelChan <-chan struct{}) (io.ReadCloser, int64, error) {
				fetchCh <- struct{}{}
				<-barrier
				return nopCloser, 42, nil
			}

			ifrit.Background(step1)
			ifrit.Background(step2)
			ifrit.Background(step3)

			Eventually(fetchCh).Should(Receive())
			Eventually(fetchCh).Should(Receive())
			Consistently(fetchCh).ShouldNot(Receive())

			barrier <- struct{}{}

			Eventually(fetchCh).Should(Receive())

			close(barrier)
		})
	})
})

var _ = Describe("ReadSizer", func() {
	Describe("BytesRead", func() {
		It("returns the number of bytes read", func() {
			base := bytes.NewBufferString("FooBar BazQux")
			readSizer := &steps.ReadSizer{Reader: base}
			buf := make([]byte, 7)
			n, err := readSizer.Read(buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(7))
			Expect(readSizer.BytesRead()).To(Equal(7))

			n, err = readSizer.Read(buf)
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(6))
			Expect(readSizer.BytesRead()).To(Equal(13))
		})
	})
})

func createTempTar() *os.File {
	tarFile, err := os.CreateTemp("", "some-tar")
	Expect(err).NotTo(HaveOccurred())

	archiveHelper.CreateTarArchive(
		tarFile.Name(),
		[]archiveHelper.ArchiveFile{{Name: "file1"}},
	)

	_, err = tarFile.Seek(0, 0)
	Expect(err).NotTo(HaveOccurred())

	return tarFile
}

type WriteFunc func(p []byte) (n int, err error)

func (wf WriteFunc) Write(p []byte) (n int, err error) {
	return wf(p)
}
