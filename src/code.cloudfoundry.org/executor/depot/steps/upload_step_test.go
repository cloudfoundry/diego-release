package steps_test

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/user"
	"testing/iotest"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/garden"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"

	Compressor "code.cloudfoundry.org/archiver/compressor"
	"code.cloudfoundry.org/executor/depot/log_streamer/fake_log_streamer"
	"code.cloudfoundry.org/executor/depot/steps"
	Uploader "code.cloudfoundry.org/executor/depot/uploader"
	"code.cloudfoundry.org/executor/depot/uploader/fake_uploader"
	"code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
)

type fakeUploader struct {
	ready   chan<- struct{}
	barrier <-chan struct{}
}

func (u *fakeUploader) Upload(fileLocation string, destinationUrl *url.URL, cancel <-chan struct{}) (int64, error) {
	u.ready <- struct{}{}
	<-u.barrier
	return 0, nil
}

func newFakeStreamer() *fake_log_streamer.FakeLogStreamer {
	fakeStreamer := new(fake_log_streamer.FakeLogStreamer)

	stdoutBuffer := gbytes.NewBuffer()
	stderrBuffer := gbytes.NewBuffer()
	fakeStreamer.StdoutReturns(stdoutBuffer)
	fakeStreamer.StderrReturns(stderrBuffer)

	return fakeStreamer
}

var _ = Describe("UploadStep", func() {
	var (
		step ifrit.Runner

		uploadAction    *models.UploadAction
		uploader        Uploader.Uploader
		tempDir         string
		gardenClient    *fakes.FakeGardenClient
		logger          *lagertest.TestLogger
		compressor      Compressor.Compressor
		fakeStreamer    *fake_log_streamer.FakeLogStreamer
		uploadTarget    *httptest.Server
		uploadedPayload []byte
	)

	BeforeEach(func() {
		var err error

		uploadTarget = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			var err error

			uploadedPayload, err = io.ReadAll(req.Body)
			Expect(err).NotTo(HaveOccurred())

			w.WriteHeader(http.StatusOK)
		}))

		uploadAction = &models.UploadAction{
			To:   uploadTarget.URL,
			From: "./expected-src.txt",
			User: "notroot",
		}

		tempDir, err = os.MkdirTemp("", "upload-step-tmpdir")
		Expect(err).NotTo(HaveOccurred())

		gardenClient = fakes.NewGardenClient()

		logger = lagertest.NewTestLogger("test")

		compressor = Compressor.NewTgz()
		uploader = Uploader.New(logger, 5*time.Second, nil)

		fakeStreamer = newFakeStreamer()

		_, err = user.Current()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
		uploadTarget.Close()
	})

	handle := "some-container-handle"

	JustBeforeEach(func() {
		gardenClient.Connection.CreateReturns(handle, nil)

		container, err := gardenClient.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		step = steps.NewUpload(
			container,
			*uploadAction,
			uploader,
			compressor,
			tempDir,
			fakeStreamer,
			make(chan struct{}, 1),
			logger,
		)
	})

	Describe("Ready", func() {
		BeforeEach(func() {
			gardenClient.Connection.StreamOutStub = func(handle string, spec garden.StreamOutSpec) (io.ReadCloser, error) {
				buffer := gbytes.NewBuffer()
				return buffer, nil
			}
		})

		It("becomes ready immediately", func() {
			p := ifrit.Background(step)
			Eventually(p.Ready()).Should(BeClosed())
		})
	})

	Describe("Run", func() {
		Context("when streaming out works", func() {
			var buffer *gbytes.Buffer

			writeTarContents := func(w io.Writer) {
				tarWriter := tar.NewWriter(w)

				dropletContents := "expected-contents"

				err := tarWriter.WriteHeader(&tar.Header{
					Name: "./expected-src.txt",
					Size: int64(len(dropletContents)),
				})
				Expect(err).NotTo(HaveOccurred())

				_, err = tarWriter.Write([]byte(dropletContents))
				Expect(err).NotTo(HaveOccurred())

				err = tarWriter.Flush()
				Expect(err).NotTo(HaveOccurred())
			}

			BeforeEach(func() {
				buffer = gbytes.NewBuffer()

				gardenClient.Connection.StreamOutStub = func(handle string, spec garden.StreamOutSpec) (io.ReadCloser, error) {
					Expect(spec.Path).To(Equal("./expected-src.txt"))
					Expect(spec.User).To(Equal("notroot"))
					Expect(handle).To(Equal("some-container-handle"))

					writeTarContents(buffer)

					return buffer, nil
				}
			})

			It("uploads the specified file to the destination", func() {
				err := <-ifrit.Invoke(step).Wait()
				Expect(err).NotTo(HaveOccurred())

				Expect(uploadedPayload).NotTo(BeZero())

				Expect(buffer.Closed()).To(BeTrue())

				Expect(string(uploadedPayload)).To(Equal("expected-contents"))
			})

			It("logs the step", func() {
				err := <-ifrit.Invoke(step).Wait()
				Expect(err).NotTo(HaveOccurred())
				Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
					"test.upload-step.upload-starting",
					"test.URLUploader.uploading",
					"test.URLUploader.succeeded-uploading",
					"test.upload-step.upload-successful",
				}))
			})

			Describe("Signal", func() {
				cancelledErr := errors.New("upload cancelled")

				var (
					cancelled    chan struct{}
					fakeUploader *fake_uploader.FakeUploader
				)

				BeforeEach(func() {
					fakeUploader = new(fake_uploader.FakeUploader)

					cancelled = make(chan struct{})

					fakeUploader.UploadStub = func(from string, dest *url.URL, cancel <-chan struct{}) (int64, error) {
						<-cancel
						close(cancelled)
						return 0, cancelledErr
					}

					uploader = fakeUploader
				})

				It("cancels any in-flight upload", func() {
					p := ifrit.Background(step)

					Eventually(fakeUploader.UploadCallCount).Should(Equal(1))

					Consistently(p.Wait()).ShouldNot(Receive())

					p.Signal(os.Interrupt)

					Eventually(cancelled).Should(BeClosed())

					Eventually(p.Wait()).Should(Receive(Equal(new(steps.CancelledError))))
				})
			})

			Context("when creating a TmpDir fails", func() {
				var stderr *gbytes.Buffer
				BeforeEach(func() {
					tempDir = "doesnotexist"
					stderr = fakeStreamer.Stderr().(*gbytes.Buffer)
				})

				It("logs the step", func() {
					err := <-ifrit.Invoke(step).Wait()
					Expect(err).To(HaveOccurred())
					Expect(logger.TestSink.LogMessages()).To(ContainElement("test.upload-step.failed-to-create-tmp-dir"))
				})

				Context("when there is a named artifact", func() {
					BeforeEach(func() {
						uploadAction.Artifact = "artifact"
					})

					It("should emit an error with the name", func() {
						err := <-ifrit.Invoke(step).Wait()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("Failed to create temp dir for artifact"))
						Expect(stderr).To(gbytes.Say("Failed to create temp dir for artifact\n"))
					})
				})
			})

			Describe("streaming logs for uploads", func() {
				BeforeEach(func() {
					fakeUploader := new(fake_uploader.FakeUploader)
					fakeUploader.UploadReturns(1024, nil)
					uploader = fakeUploader
				})

				Context("when an artifact is specified", func() {
					BeforeEach(func() {
						uploadAction.Artifact = "artifact"
					})

					It("streams the upload filesize", func() {
						err := <-ifrit.Invoke(step).Wait()
						Expect(err).NotTo(HaveOccurred())

						stdout := fakeStreamer.Stdout().(*gbytes.Buffer)
						Expect(stdout.Contents()).To(ContainSubstring("Uploaded artifact (1K)"))
					})
				})

				Context("when an artifact is not specified", func() {
					It("does not stream the upload information", func() {
						err := <-ifrit.Invoke(step).Wait()
						Expect(err).NotTo(HaveOccurred())

						stdout := fakeStreamer.Stdout().(*gbytes.Buffer)
						Expect(stdout.Contents()).To(BeEmpty())
					})
				})

				It("does not stream an error", func() {
					err := <-ifrit.Invoke(step).Wait()
					Expect(err).NotTo(HaveOccurred())

					stderr := fakeStreamer.Stderr().(*gbytes.Buffer)
					Expect(stderr.Contents()).To(BeEmpty())
				})
			})

			Context("when copying to compressed file fails", func() {
				var stderr *gbytes.Buffer
				BeforeEach(func() {
					buffer = gbytes.NewBuffer()
					writeTarContents(buffer)

					brokenStream := io.NopCloser(iotest.TimeoutReader(buffer))

					gardenClient.Connection.StreamOutReturns(brokenStream, nil)

					stderr = fakeStreamer.Stderr().(*gbytes.Buffer)
				})

				It("logs the step", func() {
					err := <-ifrit.Invoke(step).Wait()
					Expect(err).To(HaveOccurred())
					Expect(logger.TestSink.LogMessages()).To(ContainElement("test.upload-step.failed-to-copy-stream"))
				})

				Context("when there is a named artifact", func() {
					BeforeEach(func() {
						uploadAction.Artifact = "artifact"
					})

					It("logs the error", func() {
						err := <-ifrit.Invoke(step).Wait()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("Failed to copy stream contents into temp file for artifact"))
						Expect(stderr).To(gbytes.Say("Failed to copy stream contents into temp file for artifact\n"))
					})
				})

				It("clears out the temporary file", func() {
					err := <-ifrit.Invoke(step).Wait()
					Expect(err).To(HaveOccurred())
					files, err := os.ReadDir(tempDir)
					Expect(err).NotTo(HaveOccurred())
					for _, f := range files {
						Expect(f.Name()).NotTo(HavePrefix("compressed"))
					}
				})
			})

			Context("when there is an error uploading", func() {
				var stderr *gbytes.Buffer
				BeforeEach(func() {
					stderr = fakeStreamer.Stderr().(*gbytes.Buffer)
				})
				errUploadFailed := errors.New("Upload failed!")

				BeforeEach(func() {
					fakeUploader := new(fake_uploader.FakeUploader)
					fakeUploader.UploadReturns(0, errUploadFailed)
					uploader = fakeUploader
				})

				It("returns the appropriate error", func() {
					err := <-ifrit.Invoke(step).Wait()
					Expect(err).To(MatchError(errUploadFailed))
				})

				It("logs the step", func() {
					err := <-ifrit.Invoke(step).Wait()
					Expect(err).To(HaveOccurred())
					Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
						"test.upload-step.upload-starting",
						"test.upload-step.failed-to-upload",
					}))
				})

				Context("when an artifact is specified", func() {
					BeforeEach(func() {
						uploadAction.Artifact = "artifact"
					})

					It("should emit an error with the name", func() {
						err := <-ifrit.Invoke(step).Wait()
						Expect(err).To(HaveOccurred())
						Expect(stderr).To(gbytes.Say("Failed to upload payload for artifact\n"))
						Expect(stderr).NotTo(gbytes.Say(errUploadFailed.Error()))
					})
				})
			})
		})

		Context("when there is an error parsing the upload url", func() {
			var stderr *gbytes.Buffer
			BeforeEach(func() {
				uploadAction.To = "foo/bar"
				stderr = fakeStreamer.Stderr().(*gbytes.Buffer)
			})

			It("returns the appropriate error", func() {
				err := <-ifrit.Invoke(step).Wait()
				Expect(err).To(BeAssignableToTypeOf(&url.Error{}))
			})

			It("logs the step", func() {
				err := <-ifrit.Invoke(step).Wait()
				Expect(err).To(HaveOccurred())
				Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
					"test.upload-step.upload-starting",
					"test.upload-step.failed-to-parse-url",
				}))
			})

			It("should not emit an error", func() {
				err := <-ifrit.Invoke(step).Wait()
				Expect(err).To(HaveOccurred())
				Expect(stderr).NotTo(gbytes.Say("Failed to parse URL"))
			})

			Context("when there is a named artifact", func() {
				BeforeEach(func() {
					uploadAction.Artifact = "artifact"
				})

				It("should emit an error with the name", func() {
					err := <-ifrit.Invoke(step).Wait()
					Expect(err).To(HaveOccurred())
					Expect(stderr).To(gbytes.Say("Failed to parse URL for artifact\n"))
				})

				It("returns the appropriate error", func() {
					err := <-ifrit.Invoke(step).Wait()
					Expect(err).To(BeAssignableToTypeOf(&url.Error{}))
				})
			})
		})

		Context("when there is an error initiating the stream", func() {
			errStream := errors.New("stream error")

			BeforeEach(func() {
				gardenClient.Connection.StreamOutReturns(nil, errStream)
			})

			It("returns the appropriate error", func() {
				err := <-ifrit.Invoke(step).Wait()
				Expect(err).To(MatchError(steps.NewEmittableError(errStream, steps.ErrEstablishStream)))
			})

			It("logs the step", func() {
				err := <-ifrit.Invoke(step).Wait()
				Expect(err).To(HaveOccurred())
				Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
					"test.upload-step.upload-starting",
					"test.upload-step.failed-to-stream-out",
				}))
			})

			Context("when the artifact is named", func() {
				var stderr *gbytes.Buffer
				BeforeEach(func() {
					uploadAction.Artifact = "artifact"
					stderr = fakeStreamer.Stderr().(*gbytes.Buffer)
				})

				It("should emits an error with the artifact name", func() {
					err := <-ifrit.Invoke(step).Wait()
					Expect(err).To(HaveOccurred())
					errString := fmt.Sprintf("%s for %s", steps.ErrEstablishStream, "artifact")
					Expect(err).To(MatchError(steps.NewEmittableError(errStream, "%s", errString)))
				})

				It("should log error with artifact name", func() {
					err := <-ifrit.Invoke(step).Wait()
					Expect(err).To(HaveOccurred())
					Expect(stderr).To(gbytes.Say("Failed to establish stream from container for artifact\n"))
				})
			})
		})

		Context("when there is an error in reading the data from the stream", func() {
			errStream := errors.New("stream error")

			BeforeEach(func() {
				gardenClient.Connection.StreamOutReturns(&errorReader{err: errStream}, nil)
			})

			It("returns the appropriate error", func() {
				err := <-ifrit.Invoke(step).Wait()
				Expect(err).To(MatchError(steps.NewEmittableError(errStream, steps.ErrReadTar)))
			})

			It("logs the step", func() {
				err := <-ifrit.Invoke(step).Wait()
				Expect(err).To(HaveOccurred())
				Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
					"test.upload-step.upload-starting",
					"test.upload-step.failed-to-read-stream",
				}))

			})

			Context("when the artifact is named", func() {
				var stderr *gbytes.Buffer
				BeforeEach(func() {
					uploadAction.Artifact = "artifact"
					stderr = fakeStreamer.Stderr().(*gbytes.Buffer)
				})

				It("should emits an error with the artifact name", func() {
					err := <-ifrit.Invoke(step).Wait()
					Expect(err).To(HaveOccurred())
					errString := fmt.Sprintf("%s for %s", steps.ErrReadTar, "artifact")
					Expect(err).To(MatchError(steps.NewEmittableError(errStream, "%s", errString)))
				})

				It("should log error with artifact name", func() {
					err := <-ifrit.Invoke(step).Wait()
					Expect(err).To(HaveOccurred())
					Expect(stderr).To(gbytes.Say("Failed to find first item in tar stream for artifact\n"))
				})
			})
		})
	})

	Describe("the uploads are rate limited", func() {
		var container garden.Container

		BeforeEach(func() {
			var err error
			container, err = gardenClient.Create(garden.ContainerSpec{
				Handle: handle,
			})
			Expect(err).NotTo(HaveOccurred())

			gardenClient.Connection.StreamOutStub = func(handle string, spec garden.StreamOutSpec) (io.ReadCloser, error) {
				buffer := gbytes.NewBuffer()
				tarWriter := tar.NewWriter(buffer)

				err := tarWriter.WriteHeader(&tar.Header{
					Name: "./does-not-matter.txt",
					Size: int64(0),
				})
				Expect(err).NotTo(HaveOccurred())

				return buffer, nil
			}
		})

		It("allows only N concurrent uploads", func() {
			rateLimiter := make(chan struct{}, 2)

			ready := make(chan struct{}, 3)
			barrier := make(chan struct{})
			uploader := &fakeUploader{
				ready:   ready,
				barrier: barrier,
			}

			uploadAction1 := models.UploadAction{
				To:   "http://mybucket.mf",
				From: "./foo1.txt",
			}

			step1 := steps.NewUpload(
				container,
				uploadAction1,
				uploader,
				compressor,
				tempDir,
				newFakeStreamer(),
				rateLimiter,
				logger,
			)

			uploadAction2 := models.UploadAction{
				To:   "http://mybucket.mf",
				From: "./foo2.txt",
			}

			step2 := steps.NewUpload(
				container,
				uploadAction2,
				uploader,
				compressor,
				tempDir,
				newFakeStreamer(),
				rateLimiter,
				logger,
			)

			uploadAction3 := models.UploadAction{
				To:   "http://mybucket.mf",
				From: "./foo3.txt",
			}

			step3 := steps.NewUpload(
				container,
				uploadAction3,
				uploader,
				compressor,
				tempDir,
				newFakeStreamer(),
				rateLimiter,
				logger,
			)

			ifrit.Background(step1)
			ifrit.Background(step2)
			ifrit.Background(step3)

			Eventually(ready).Should(Receive())
			Eventually(ready).Should(Receive())
			Consistently(ready).ShouldNot(Receive())

			barrier <- struct{}{}

			Eventually(ready).Should(Receive())

			close(barrier)
		})
	})
})

type errorReader struct {
	err error
}

func (r *errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

func (r *errorReader) Close() error {
	return nil
}
