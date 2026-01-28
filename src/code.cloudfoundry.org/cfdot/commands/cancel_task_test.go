package commands_test

import (
	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfdot/commands"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/openzipkin/zipkin-go/model"
)

var _ = Describe("CancelTask", func() {
	Describe("CancelTaskByGuid", func() {
		var (
			fakeBBSClient  *fake_bbs.FakeClient
			stdout, stderr *gbytes.Buffer
		)

		BeforeEach(func() {
			stdout = gbytes.NewBuffer()
			stderr = gbytes.NewBuffer()
			fakeBBSClient = &fake_bbs.FakeClient{}
		})

		It("passes through the task guid to the BBS", func() {
			taskGuid := "task-guid"

			err := commands.CancelTaskByGuid(stdout, stderr, fakeBBSClient, taskGuid)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBBSClient.CancelTaskCallCount()).To(Equal(1))

			_, traceID, guid := fakeBBSClient.CancelTaskArgsForCall(0)
			Expect(guid).To(Equal(taskGuid))
			_, err = model.TraceIDFromHex(traceID)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the bbs client errors", func() {
			It("returns an error back", func() {
				fakeBBSClient.CancelTaskReturns(models.ErrResourceNotFound)

				err := commands.CancelTaskByGuid(stdout, stderr, fakeBBSClient, "broken")
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})
		})
	})

	Describe("ValidateCancelTaskArgs", func() {
		Context("when one argument is passed", func() {
			It("returns the argument and no error", func() {
				guid, err := commands.ValidateCancelTaskArgs([]string{"guid"})
				Expect(err).NotTo(HaveOccurred())
				Expect(guid).To(Equal("guid"))
			})
		})

		Context("when no arguments are passed", func() {
			It("returns an error", func() {
				_, err := commands.ValidateCancelTaskArgs([]string{})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when two arguments are passed", func() {
			It("returns an error", func() {
				_, err := commands.ValidateCancelTaskArgs([]string{"guid1", "guid2"})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when one empty argument is passed", func() {
			It("returns an error", func() {
				_, err := commands.ValidateCancelTaskArgs([]string{""})
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
