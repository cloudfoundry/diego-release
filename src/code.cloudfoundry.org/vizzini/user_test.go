package vizzini_test

import (
	. "code.cloudfoundry.org/vizzini/matchers"

	"code.cloudfoundry.org/bbs/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Users", func() {
	var task *models.TaskDefinition

	Context("{DOCKER} with an existing 'alice' user in the rootfs", func() {
		BeforeEach(func() {
			task = Task()
			task.RootFs = "docker:///cloudfoundry/busybox-alice"
			task.Action = models.WrapAction(&models.RunAction{
				Path: "sh",
				Args: []string{"-c", "whoami > /tmp/output"},
				User: "alice",
			})
			task.ResultFile = "/tmp/output"

			Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
		})

		It("runs an action as alice", func() {
			Eventually(TaskGetter(logger, guid), 120).Should(HaveTaskState(models.Task_Completed))

			task, err := bbsClient.TaskByGuid(logger, traceID, guid)
			Expect(err).NotTo(HaveOccurred())

			Expect(task.Failed).To(BeFalse())
			Expect(task.Result).To(ContainSubstring("alice"))

			Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
			Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
		})
	})
})
