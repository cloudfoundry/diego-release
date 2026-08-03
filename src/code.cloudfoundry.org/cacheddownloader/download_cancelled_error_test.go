package cacheddownloader_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/cacheddownloader"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DownloadCancelledError", func() {
	It("reports an error with source, duration and bytes", func() {
		e := cacheddownloader.NewDownloadCancelledError("here", 30*time.Second, 1, nil)
		Expect(e.Error()).To(Equal("Download cancelled: source 'here', duration '30s', bytes '1'"))
	})

	It("reports an error with source, duration, bytes, and an additional error", func() {
		e := cacheddownloader.NewDownloadCancelledError("here", 30*time.Second, 1, errors.New("Foo"))
		Expect(e.Error()).To(Equal("Download cancelled: source 'here', duration '30s', bytes '1', Error: Foo"))
	})

	Context("when no bytes have been read", func() {
		It("only reports source and duration", func() {
			e := cacheddownloader.NewDownloadCancelledError("here", 30*time.Second, cacheddownloader.NoBytesReceived, nil)
			Expect(e.Error()).To(Equal("Download cancelled: source 'here', duration '30s'"))
		})
	})
})
