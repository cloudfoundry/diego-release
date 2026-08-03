package internal_test

import (
	"errors"
	"io"
	"strings"

	"code.cloudfoundry.org/archiver/extractor/test_helper"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/rep/generator/internal"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ContainerDelegate", func() {
	var containerDelegate internal.ContainerDelegate
	var executorClient *fakes.FakeClient
	var logger *lagertest.TestLogger
	var expectedGuid = "some-instance-guid"
	const sessionPrefix = "test"

	BeforeEach(func() {
		executorClient = new(fakes.FakeClient)
		containerDelegate = internal.NewContainerDelegate(executorClient)
		logger = lagertest.NewTestLogger(sessionPrefix)
	})

	Describe("RunContainer", func() {
		var result bool
		var runRequest executor.RunRequest

		BeforeEach(func() {
			runRequest = executor.NewRunRequest(expectedGuid, &executor.RunInfo{}, executor.Tags{})
		})

		JustBeforeEach(func() {
			result = containerDelegate.RunContainer(logger, "some-trace-id", &runRequest)
		})

		It("runs the container", func() {
			Expect(executorClient.RunContainerCallCount()).To(Equal(1))
			_, traceID, runReq := executorClient.RunContainerArgsForCall(0)
			Expect(traceID).To(Equal("some-trace-id"))
			Expect(*runReq).To(Equal(runRequest))
		})

		Context("when running succeeds", func() {
			It("returns true", func() {
				Expect(result).To(BeTrue())
			})

			It("logs the running", func() {
				Expect(logger).To(gbytes.Say(sessionPrefix + ".running-container"))
				Expect(logger).To(gbytes.Say(sessionPrefix + ".succeeded-running-container"))
			})
		})

		Context("when running fails", func() {
			BeforeEach(func() {
				executorClient.RunContainerReturns(errors.New("ka-boom"))
			})

			It("returns false", func() {
				Expect(result).To(BeFalse())
			})

			It("logs the failure", func() {
				Expect(logger).To(gbytes.Say(sessionPrefix + ".failed-running-container"))
			})

			It("deletes the container", func() {
				Expect(executorClient.DeleteContainerCallCount()).To(Equal(1))
				_, traceID, containerGuid := executorClient.DeleteContainerArgsForCall(0)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(containerGuid).To(Equal(expectedGuid))
			})

			It("logs the deletion", func() {
				Expect(logger).To(gbytes.Say(sessionPrefix + ".deleting-container"))
				Expect(logger).To(gbytes.Say(sessionPrefix + ".succeeded-deleting-container"))
			})

			Context("when deleting fails", func() {
				BeforeEach(func() {
					executorClient.DeleteContainerReturns(errors.New("boom"))
				})

				It("logs the failure", func() {
					Expect(logger).To(gbytes.Say(sessionPrefix + ".failed-deleting-container"))
				})
			})
		})
	})

	Describe("StopContainer", func() {
		var result bool

		JustBeforeEach(func() {
			result = containerDelegate.StopContainer(logger, "some-trace-id", expectedGuid)
		})

		It("stops the container", func() {
			Expect(executorClient.StopContainerCallCount()).To(Equal(1))
			_, traceID, containerGuid := executorClient.StopContainerArgsForCall(0)
			Expect(traceID).To(Equal("some-trace-id"))
			Expect(containerGuid).To(Equal(expectedGuid))
		})

		Context("when stopping succeeds", func() {
			It("returns true", func() {
				Expect(result).To(BeTrue())
			})

			It("logs the stopping", func() {
				Expect(logger).To(gbytes.Say(sessionPrefix + ".stopping-container"))
				Expect(logger).To(gbytes.Say(sessionPrefix + ".succeeded-stopping-container"))
			})
		})

		Context("when stopping fails", func() {
			BeforeEach(func() {
				executorClient.StopContainerReturns(errors.New("ka-boom"))
			})

			It("returns false", func() {
				Expect(result).To(BeFalse())
			})

			It("logs the failure", func() {
				Expect(logger).To(gbytes.Say(sessionPrefix + ".failed-stopping-container"))
			})
		})
	})

	Describe("DeleteContainer", func() {
		var result bool

		JustBeforeEach(func() {
			result = containerDelegate.DeleteContainer(logger, "some-trace-id", expectedGuid)
		})

		It("deletes the container", func() {
			Expect(executorClient.DeleteContainerCallCount()).To(Equal(1))
			_, traceID, containerGuid := executorClient.DeleteContainerArgsForCall(0)
			Expect(traceID).To(Equal("some-trace-id"))
			Expect(containerGuid).To(Equal(expectedGuid))
		})

		Context("when deleting succeeds", func() {
			It("returns true", func() {
				Expect(result).To(BeTrue())
			})

			It("logs the deleting", func() {
				Expect(logger).To(gbytes.Say(sessionPrefix + ".deleting-container"))
				Expect(logger).To(gbytes.Say(sessionPrefix + ".succeeded-deleting-container"))
			})
		})

		Context("when deleting fails", func() {
			BeforeEach(func() {
				executorClient.DeleteContainerReturns(errors.New("ka-boom"))
			})

			It("returns false", func() {
				Expect(result).To(BeFalse())
			})

			It("logs the failure", func() {
				Expect(logger).To(gbytes.Say(sessionPrefix + ".failed-deleting-container"))
			})
		})
	})

	Describe("FetchContainerResultFile", func() {
		var (
			filename string

			result   string
			fetchErr error
		)

		BeforeEach(func() {
			filename = "some-filename"
		})

		JustBeforeEach(func() {
			result, fetchErr = containerDelegate.FetchContainerResultFile(logger, expectedGuid, filename)
		})

		Context("when fetching the file stream from the container succeeds", func() {
			var fileStream *gbytes.Buffer

			BeforeEach(func() {
				fileStream = gbytes.NewBuffer()
				executorClient.GetFilesReturns(fileStream, nil)
			})

			Context("and the payload is a reasonable size", func() {
				BeforeEach(func() {
					test_helper.WriteTar(
						fileStream,
						[]test_helper.ArchiveFile{{
							Name: "some-file",
							Body: "some result",
							Mode: 0600,
						}},
					)
				})

				It("succeeds", func() {
					Expect(fetchErr).NotTo(HaveOccurred())
				})

				It("returns the result of the first file in the stream", func() {
					Expect(result).To(Equal("some result"))
				})

				It("closes the result stream", func() {
					Expect(fileStream.Closed()).To(BeTrue())
				})

				It("logs the fetching", func() {
					Expect(logger).To(gbytes.Say(sessionPrefix + ".fetching-container-result"))
					Expect(logger).To(gbytes.Say(sessionPrefix + ".succeeded-fetching-container-result"))
				})

				Context("and a single read does not return the whole file contents", func() {
					BeforeEach(func() {
						ir := incompleteReadCloser{r: fileStream, bytesPerRead: 2}
						executorClient.GetFilesReturns(ir, nil)
					})

					It("reads the whole file contents", func() {
						Expect(result).To(Equal("some result"))
					})
				})
			})

			Context("but the payload is too large", func() {
				BeforeEach(func() {
					test_helper.WriteTar(
						fileStream,
						[]test_helper.ArchiveFile{{
							Name: "some-file",
							Body: strings.Repeat("x", internal.MAX_RESULT_SIZE+100),
							Mode: 0600,
						}},
					)
				})

				It("returns an error", func() {
					Expect(fetchErr).To(HaveOccurred())
				})

				It("closes the result stream", func() {
					Expect(fileStream.Closed()).To(BeTrue())
				})

				It("logs the failure", func() {
					Expect(logger).To(gbytes.Say(sessionPrefix + ".failed-fetching-container-result-too-large"))
				})

				Context("and a single read does not return the whole file contents", func() {
					BeforeEach(func() {
						ir := incompleteReadCloser{r: fileStream, bytesPerRead: 1024}
						executorClient.GetFilesReturns(ir, nil)
					})

					It("reads up to the size of the buffer and detects the size is too large", func() {
						Expect(fetchErr).To(HaveOccurred())
						Expect(logger).To(gbytes.Say(sessionPrefix + ".failed-fetching-container-result-too-large"))
					})
				})
			})

			Context("when the reader returns an error", func() {
				var errorReader *errorReadCloser

				BeforeEach(func() {
					test_helper.WriteTar(
						fileStream,
						[]test_helper.ArchiveFile{{
							Name: "some-file",
							Body: "some result",
							Mode: 0600,
						}},
					)

					errorReader = &errorReadCloser{r: fileStream}
					executorClient.GetFilesReturns(errorReader, nil)
				})

				It("returns an error", func() {
					Expect(fetchErr).To(HaveOccurred())
				})

				It("closes the result stream", func() {
					Expect(fileStream.Closed()).To(BeTrue())
				})

				It("logs the failure", func() {
					Expect(logger).To(gbytes.Say(sessionPrefix + ".failed-reading-container-result-file"))
				})
			})

			Context("when the stream is empty for whatever reason", func() {
				It("returns an error", func() {
					Expect(fetchErr).To(HaveOccurred())
				})

				It("closes the result stream", func() {
					Expect(fileStream.Closed()).To(BeTrue())
				})
			})
		})

		Context("when fetching the file stream from the container fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				executorClient.GetFilesReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(fetchErr).To(Equal(disaster))
			})

			It("logs the failure", func() {
				Expect(logger).To(gbytes.Say(sessionPrefix + ".failed-fetching-container-result-stream-from-executor"))
			})
		})
	})
})

type errorReadCloser struct {
	r          io.ReadCloser
	failOnCall bool
}

func (i *errorReadCloser) Read(p []byte) (int, error) {
	if i.failOnCall {
		return 0, errors.New("stubbed-error")
	}
	i.failOnCall = true
	return i.r.Read(p)
}

func (i *errorReadCloser) Close() error {
	return i.r.Close()
}

type incompleteReadCloser struct {
	r            io.ReadCloser
	bytesPerRead int
}

func (i incompleteReadCloser) Read(p []byte) (int, error) {
	smallerBuf := make([]byte, i.bytesPerRead)
	n, err := i.r.Read(smallerBuf)
	if err != nil {
		return n, err
	}
	copied := copy(p, smallerBuf)
	return copied, nil
}

func (i incompleteReadCloser) Close() error {
	return i.r.Close()
}
