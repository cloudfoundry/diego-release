package vizzini_test

import (
	"strconv"

	"time"

	. "code.cloudfoundry.org/vizzini/matchers"

	"code.cloudfoundry.org/bbs/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Actions", func() {
	var taskDef *models.TaskDefinition

	Describe("Timeout action", func() {
		BeforeEach(func() {
			taskDef = Task()
			taskDef.Action = models.WrapAction(models.Timeout(
				&models.RunAction{
					Path: "bash",
					Args: []string{"-c", "sleep 1000"},
					User: "vcap",
				},
				2*time.Second,
			))
			taskDef.ResultFile = ""

			Expect(bbsClient.DesireTask(logger, traceID, guid, domain, taskDef)).To(Succeed())
		})

		It("should fail the Task within the timeout window", func() {
			Eventually(TaskGetter(logger, guid)).Should(HaveTaskState(models.Task_Running))
			Eventually(TaskGetter(logger, guid), 10).Should(HaveTaskState(models.Task_Completed))
			task, err := bbsClient.TaskByGuid(logger, traceID, guid)
			Expect(err).NotTo(HaveOccurred())
			Expect(task.GetFailed()).To(BeTrue())
			Expect(task.GetFailureReason()).To(ContainSubstring("timeout"))

			Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
			Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
		})
	})

	Describe("Run action", func() {
		BeforeEach(func() {
			taskDef = Task()
			taskDef.Action = models.WrapAction(&models.RunAction{
				Path: "bash",
				Dir:  "/etc",
				Args: []string{"-c", "echo $PWD > /tmp/bar"},
				User: "vcap",
			})

			Expect(bbsClient.DesireTask(logger, traceID, guid, domain, taskDef)).To(Succeed())
		})

		It("should be possible to specify a working directory", func() {
			Eventually(TaskGetter(logger, guid)).Should(HaveTaskState(models.Task_Completed))
			task, err := bbsClient.TaskByGuid(logger, traceID, guid)
			Expect(err).NotTo(HaveOccurred())
			Expect(task.GetFailed()).To(BeFalse())
			Expect(task.GetResult()).To(ContainSubstring("/etc"))

			Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
			Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
		})

	})

	Describe("Run action resource limits", func() {
		var processLimit uint64 = 23790
		BeforeEach(func() {
			taskDef = Task()
			rl := &models.ResourceLimits{}
			rl.SetNofile(processLimit)
			taskDef.Action = models.WrapAction(&models.RunAction{
				Path:           "bash",
				Dir:            "/etc",
				Args:           []string{"-c", "ulimit -n > /tmp/bar"},
				User:           "vcap",
				ResourceLimits: rl,
			})

			Expect(bbsClient.DesireTask(logger, traceID, guid, domain, taskDef)).To(Succeed())
		})

		It("is possible to limit the number of processes", func() {
			Eventually(TaskGetter(logger, guid)).Should(HaveTaskState(models.Task_Completed))
			task, err := bbsClient.TaskByGuid(logger, traceID, guid)
			Expect(err).NotTo(HaveOccurred())
			Expect(task.GetFailed()).To(BeFalse())
			Expect(task.GetResult()).To(ContainSubstring(strconv.FormatUint(processLimit, 10)))

			Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
			Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
		})
	})

	Describe("Cancelling Downloads", func() {
		It("should cancel the download", func() {
			desiredLRP := &models.DesiredLRP{
				PlacementTags: PlacementTags(),
				ProcessGuid:   guid,
				RootFs:        config.DefaultRootFS,
				Domain:        domain,
				Instances:     1,
				Action: models.WrapAction(&models.DownloadAction{
					From: "https://storage.googleapis.com/diego-assets/large-file.txt",
					To:   "/tmp",
					User: "vcap",
				}),
				MemoryMb:   128,
				DiskMb:     128,
				CpuWeight:  100,
				LogGuid:    guid,
				LogSource:  "VIZ",
				Annotation: "arbitrary-data",
				MetricTags: map[string]*models.MetricTagValue{"source_id": {Static: guid}},
			}

			Expect(bbsClient.DesireLRP(logger, traceID, desiredLRP)).To(Succeed())
			Eventually(ActualGetter(logger, desiredLRP.ProcessGuid, 0)).Should(BeActualLRPWithState(desiredLRP.ProcessGuid, 0, "RUNNING"))
			Expect(bbsClient.RemoveDesiredLRP(logger, traceID, desiredLRP.ProcessGuid)).To(Succeed())
			Eventually(ActualByProcessGuidGetter(logger, desiredLRP.ProcessGuid), 5).Should(BeEmpty())
		})
	})
})
