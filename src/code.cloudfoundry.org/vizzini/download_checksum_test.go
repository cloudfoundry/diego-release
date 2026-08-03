package vizzini_test

import (
	"code.cloudfoundry.org/bbs/models"
	. "code.cloudfoundry.org/vizzini/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Download Checksums", func() {
	var lrp *models.DesiredLRP
	BeforeEach(func() {
		lrp = DesiredLRPWithGuid(guid)
	})

	Context("when the checksum is valid but incorrect", func() {
		It("should crash", func() {
			lrp.Setup = models.WrapAction(&models.DownloadAction{
				From:              config.GraceTarballURL,
				To:                ".",
				User:              "vcap",
				ChecksumAlgorithm: "sha1",
				ChecksumValue:     "0123456789abcdef0123456789abcdef01234567",
			})
			Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
			Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPThatHasCrashed(guid, 0))

			// getting all the way helps ensure the tests don't spuriously fail
			// when we delete the DesiredLRP if the application is in the middle of
			// restarting it looks like we need to wait for a convergence loop to
			// eventually clean it up.  This is likely a bug, though it's not critical.
			Eventually(ActualGetter(logger, guid, 0), ConvergerInterval).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateCrashed, 3))
		})
	})
})
