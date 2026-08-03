package log_streamer_test

import (
	"bytes"

	"code.cloudfoundry.org/executor/depot/log_streamer"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BufferStreamer", func() {
	var (
		outBuffer *bytes.Buffer
		errBuffer *bytes.Buffer
		streamer  log_streamer.LogStreamer
	)

	BeforeEach(func() {
		outBuffer = new(bytes.Buffer)
		errBuffer = new(bytes.Buffer)
		streamer = log_streamer.NewBufferStreamer(outBuffer, errBuffer)
	})

	It("writes stdout to the buffer", func() {
		streamer.Stdout().Write([]byte("stdout"))

		output := make([]byte, 6)
		_, err := outBuffer.Read(output)
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(Equal([]byte("stdout")))
	})

	It("writes stderr to the buffer", func() {
		streamer.Stderr().Write([]byte("stdout"))

		output := make([]byte, 6)
		_, err := errBuffer.Read(output)
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(Equal([]byte("stdout")))
	})

	Describe("SourceName", func() {
		Context("when WithSource has not been called", func() {
			It("returns empty string", func() {
				Expect(streamer.SourceName()).To(Equal(log_streamer.DefaultLogSource))
			})
		})

		Context("when WithSource has previously been called", func() {
			It("returns the source name passed to WithSource", func() {
				streamer = streamer.WithSource("test-log-source")
				Expect(streamer.SourceName()).To(Equal("test-log-source"))
			})
		})
	})
})
