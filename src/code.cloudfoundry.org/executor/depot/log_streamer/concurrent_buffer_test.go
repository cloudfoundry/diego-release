package log_streamer_test

import (
	"bytes"
	"io"

	"code.cloudfoundry.org/executor/depot/log_streamer"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("concurrent buffer", func() {
	Describe("NewConcurrentBuffer", func() {
		It("creates a new concurrent buffer out of the given payload", func() {
			payload := bytes.NewBuffer([]byte{3, 4, 5})
			content, err := io.ReadAll(log_streamer.NewConcurrentBuffer(payload))
			Expect(err).NotTo(HaveOccurred())
			Expect(content).To(Equal([]byte{3, 4, 5}))
		})

		It("return nil if the payload is nil", func() {
			Expect(log_streamer.NewConcurrentBuffer(nil)).To(BeNil())
		})
	})

	Describe("Write", func() {
		var payload *bytes.Buffer
		var concurrentBuffer *log_streamer.ConcurrentBuffer

		BeforeEach(func() {
			payload = new(bytes.Buffer)
			concurrentBuffer = log_streamer.NewConcurrentBuffer(payload)
		})

		It("writes stdout to the buffer", func() {
			concurrentBuffer.Write([]byte("hello"))

			output := make([]byte, 5)
			_, err := payload.Read(output)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal([]byte("hello")))
		})
	})

	Describe("Read", func() {
		var payload *bytes.Buffer
		var concurrentBuffer *log_streamer.ConcurrentBuffer

		BeforeEach(func() {
			payload = new(bytes.Buffer)
			concurrentBuffer = log_streamer.NewConcurrentBuffer(payload)
		})

		It("writes stderr to the buffer", func() {
			payload.Write([]byte("hello"))

			output := make([]byte, 5)
			_, err := concurrentBuffer.Read(output)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal([]byte("hello")))
		})
	})

	Describe("Reset", func() {
		var payload *bytes.Buffer
		var concurrentBuffer *log_streamer.ConcurrentBuffer

		BeforeEach(func() {
			payload = new(bytes.Buffer)
			concurrentBuffer = log_streamer.NewConcurrentBuffer(payload)
		})

		It("writes stderr to the buffer", func() {
			payload.Write([]byte("hello"))

			concurrentBuffer.Reset()

			output := make([]byte, 5)
			_, err := payload.Read(output)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(io.EOF))
		})
	})

	Describe("concurrency test", func() {
		var payload *bytes.Buffer
		var concurrentBuffer *log_streamer.ConcurrentBuffer

		BeforeEach(func() {
			payload = new(bytes.Buffer)
			concurrentBuffer = log_streamer.NewConcurrentBuffer(payload)
		})

		// should be run with -race
		It("successfully serializes reads and writes", func() {

			_, err := concurrentBuffer.Write([]byte("hello"))
			Expect(err).NotTo(HaveOccurred())

			go func() {
				concurrentBuffer.Reset()
				_, err := concurrentBuffer.Write([]byte("green"))
				Expect(err).NotTo(HaveOccurred())
			}()

			Eventually(func() []byte {
				output := make([]byte, 5)
				_, _ = concurrentBuffer.Read(output)
				return output
			}).Should(Equal([]byte("green")))
		})
	})
})
