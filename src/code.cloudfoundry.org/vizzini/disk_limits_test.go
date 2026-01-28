package vizzini_test

import (
	"code.cloudfoundry.org/bbs/models"
	. "code.cloudfoundry.org/vizzini/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DiskLimits", func() {
	var lrp *models.DesiredLRP
	BeforeEach(func() {
		lrp = DesiredLRPWithGuid(guid)
	})

	Describe("with a preloaded rootfs, the disk limit is applied to the COW layer", func() {
		BeforeEach(func() {
			lrp.Setup = models.WrapAction(&models.DownloadAction{
				From:     config.GraceTarballURL,
				To:       ".",
				CacheKey: "grace",
				User:     "vcap",
			})
		})

		Context("when the disk limit exceeds the contents to be copied in", func() {
			It("should not crash, but should start succesfully", func() {
				lrp.DiskMb = 64
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
			})
		})

		Context("when the disk limit is less than the contents to be copied in", func() {
			It("should crash", func() {
				lrp.DiskMb = 4
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPThatHasCrashed(guid, 0))

				//getting all the way helps ensure the tests don't spuriously fail
				//when we delete the DesiredLRP if the application is in the middle of restarting it looks like we need to wiat for a convergence
				//loop to eventually clean it up.  This is likely a bug, though it's not crticial.
				Eventually(ActualGetter(logger, guid, 0), ConvergerInterval).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateCrashed, 3))
			})
		})
	})

	Describe("{DOCKER} with a docker-image rootfs", func() {
		BeforeEach(func() {
			lrp.RootFs = config.GraceBusyboxImageURL
			lrp.Setup = nil //note: we copy nothing in, the docker image on its own should cause this failure
			lrp.Action = models.WrapAction(&models.RunAction{
				Path: "/grace",
				User: "root",
				Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
			})
			lrp.Monitor = nil
		})

		Context("when the disk limit exceeds the size of the docker image", func() {
			It("should not crash, but should start succesfully", func() {
				lrp.DiskMb = 64
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(ActualGetter(logger, guid, 0), dockerTimeout).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
			})
		})

		Context("when the disk limit is less than the size of the docker image", func() {
			It("should crash", func() {
				lrp.DiskMb = 4
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPThatHasCrashed(guid, 0))

				//getting all the way helps ensure the tests don't spuriously fail
				//when we delete the DesiredLRP if the application is in the middle of restarting it looks like we need to wiat for a convergence
				//loop to eventually clean it up.  This is likely a bug, though it's not crticial.
				Eventually(ActualGetter(logger, guid, 0), ConvergerInterval).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateCrashed, 3))
			})
		})
	})
})
