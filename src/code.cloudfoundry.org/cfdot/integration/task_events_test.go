package integration_test

import (
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("task-events", func() {
	itValidatesBBSFlags("task-events")
	itHasNoArgs("task-events", false)

	Context("when the server responds with events", func() {
		BeforeEach(func() {
			task := models.Task{TaskGuid: "some-guid"}
			taskEvent := models.NewTaskRemovedEvent(&task)
			sseEvent, err := events.NewEventFromModelEvent(1, taskEvent)
			Expect(err).ToNot(HaveOccurred())

			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/events/tasks.r1"),
					ghttp.RespondWith(200, sseEvent.Encode()),
				),
			)
		})

		It("prints out the event stream", func() {
			sess := RunCFDot("task-events")
			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say("some-guid"))
		})
	})

	Context("when there is a BBS error", func() {
		BeforeEach(func() {
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/events/tasks.r1"),
					ghttp.RespondWith(418, ""),
				),
			)
		})

		It("responds with a status code 4", func() {
			sess := RunCFDot("task-events")
			Eventually(sess).Should(gexec.Exit(4))
		})
	})
})
