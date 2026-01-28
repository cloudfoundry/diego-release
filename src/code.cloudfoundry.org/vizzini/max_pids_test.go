package vizzini_test

import (
	"code.cloudfoundry.org/bbs/models"
	. "code.cloudfoundry.org/vizzini/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MaxPids", func() {
	var lrp *models.DesiredLRP
	BeforeEach(func() {
		lrp = DesiredLRPWithGuid(guid)
	})

	Describe("Max Pid Limits", func() {
		BeforeEach(func() {
			lrp.Setup = models.WrapAction(&models.DownloadAction{
				From:     config.GraceTarballURL,
				To:       ".",
				CacheKey: "grace",
				User:     "vcap",
			})
		})

		Context("when the max pids exceeds the required number of processes", func() {
			Context("when the max pids is a large positive integer", func() {
				It("should start succesfully", func() {
					lrp.MaxPids = 1024
					Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
					Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
				})
			})
			Context("when the max pids is 0", func() {
				//0 is unlimited as per the garden container specs
				It("should not crash, but should start succesfully", func() {
					lrp.MaxPids = 0
					Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
					Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
				})
			})
		})

		Context("when the max pid limit is less than the required number of processes", func() {
			It("should crash when the limits is low positive integer", func() {
				lrp.MaxPids = 1
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPThatHasCrashed(guid, 0))

				//getting all the way helps ensure the tests don't spuriously fail
				//when we delete the DesiredLRP if the application is in the middle of restarting it looks like we need to wiat for a convergence
				//loop to eventually clean it up.  This is likely a bug, though it's not crticial.
				Eventually(ActualGetter(logger, guid, 0), ConvergerInterval).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateCrashed, 3))
			})
			It("should fail call to bbs when the limit is negative integer", func() {
				lrp.MaxPids = -1
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Not(Succeed()))
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
		Context("when the max pids exceeds the required number of processes", func() {
			Context("when the max pids is a large positive integer", func() {
				It("should start succesfully", func() {
					lrp.MaxPids = 1024
					Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
					Eventually(ActualGetter(logger, guid, 0), dockerTimeout).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
				})
			})
			Context("when the max pids is 0", func() {
				//0 is unlimited as per the garden container specs
				It("should not crash, but should start succesfully", func() {
					lrp.MaxPids = 0
					Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
					Eventually(ActualGetter(logger, guid, 0), dockerTimeout).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
				})
			})
		})

		Context("when the max pid limit is less than the required number of processes", func() {
			It("should crash when the limits is low positive integer", func() {
				lrp.MaxPids = 1
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPThatHasCrashed(guid, 0))

				//getting all the way helps ensure the tests don't spuriously fail
				//when we delete the DesiredLRP if the application is in the middle of restarting it looks like we need to wiat for a convergence
				//loop to eventually clean it up.  This is likely a bug, though it's not crticial.
				Eventually(ActualGetter(logger, guid, 0), ConvergerInterval).Should(BeActualLRPWithStateAndCrashCount(guid, 0, models.ActualLRPStateCrashed, 3))
			})
			It("should fail call to bbs when the limit is negative integer", func() {
				lrp.MaxPids = -1
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Not(Succeed()))
			})
		})
	})
})
