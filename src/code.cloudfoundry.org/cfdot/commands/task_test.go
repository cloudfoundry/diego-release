package commands_test

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfdot/commands"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/openzipkin/zipkin-go/model"
)

var _ = Describe("Task", func() {
	Describe("TaskByGuid", func() {
		var (
			fakeBBSClient  *fake_bbs.FakeClient
			stdout, stderr *gbytes.Buffer
		)

		BeforeEach(func() {
			stdout = gbytes.NewBuffer()
			stderr = gbytes.NewBuffer()
			fakeBBSClient = &fake_bbs.FakeClient{}
		})

		Context("when the bbs responds with a task", func() {
			It("prints the task as json", func() {
				var (
					taskGuid = "task-guid"
					task     = &models.Task{TaskGuid: taskGuid}
				)

				fakeBBSClient.TaskByGuidReturns(task, nil)

				err := commands.TaskByGuid(stdout, stderr, fakeBBSClient, taskGuid)
				Expect(err).NotTo(HaveOccurred())

				taskJSON, err := json.Marshal(task)
				Expect(err).NotTo(HaveOccurred())
				Expect(stdout.Contents()).To(MatchJSON(taskJSON))

				_, traceID, guid := fakeBBSClient.TaskByGuidArgsForCall(0)

				_, err = model.TraceIDFromHex(traceID)
				Expect(err).NotTo(HaveOccurred())
				Expect(guid).To(Equal(taskGuid))
			})
		})

		Context("when the bbs client errors", func() {
			It("returns an error back", func() {
				fakeBBSClient.TaskByGuidReturns(nil, models.ErrResourceNotFound)

				err := commands.TaskByGuid(stdout, stderr, fakeBBSClient, "broken")
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})
		})
	})

	Describe("ValidateTaskArgs", func() {
		Context("when one argument is passed", func() {
			It("returns the argument and no error", func() {
				guid, err := commands.ValidateTaskArgs([]string{"guid"})
				Expect(err).NotTo(HaveOccurred())
				Expect(guid).To(Equal("guid"))
			})
		})

		Context("when no arguments are passed", func() {
			It("returns an error", func() {
				_, err := commands.ValidateTaskArgs([]string{})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when two arguments are passed", func() {
			It("returns an error", func() {
				_, err := commands.ValidateTaskArgs([]string{"guid1", "guid2"})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when one empty argument is passed", func() {
			It("returns an error", func() {
				_, err := commands.ValidateTaskArgs([]string{""})
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
