package integration_test

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("cancel-task", func() {
	itValidatesBBSFlags("cancel-task", "task-guid")

	Context("when the server responds ok for canceling task", func() {
		var (
			serverTimeout int
		)

		BeforeEach(func() {
			serverTimeout = 0

		})
		JustBeforeEach(func() {
			request := &models.TaskGuidRequest{TaskGuid: "task-guid"}
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/tasks/cancel"),
					func(w http.ResponseWriter, req *http.Request) {
						time.Sleep(time.Duration(serverTimeout) * time.Second)
					},
					ghttp.VerifyProtoRepresenting(request.ToProto()),
					ghttp.RespondWith(200, nil),
				),
			)
		})

		It("exits 0", func() {
			sess := RunCFDot("cancel-task", "task-guid")
			Eventually(sess).Should(gexec.Exit(0))
			Expect(len(bbsServer.ReceivedRequests())).To(Equal(1))
		})

		Context("when timeout flag is present", func() {
			Context("when request exceeds timeout", func() {
				BeforeEach(func() {
					serverTimeout = 2
				})

				It("exits with code 4 and a timeout message", func() {
					sess := RunCFDot("cancel-task", "task-guid", "--timeout", "1")
					Eventually(sess, 2).Should(gexec.Exit(4))
					Expect(sess.Err).To(gbytes.Say(`Timeout exceeded`))
				})
			})

			Context("when request is within the timeout", func() {
				It("exits with status code of 0", func() {
					sess := RunCFDot("cancel-task", "task-guid", "--timeout", "1")
					Eventually(sess).Should(gexec.Exit(0))
					Expect(len(bbsServer.ReceivedRequests())).To(Equal(1))
				})
			})
		})
	})

	Context("when the server responds with error", func() {
		It("exits with status code 4", func() {
			response := &models.TaskResponse{
				Error: models.ErrUnknownError,
			}
			bbsServer.RouteToHandler(
				"POST",
				"/v1/tasks/cancel",
				ghttp.RespondWithProto(200, response.ToProto()),
			)

			sess := RunCFDot("cancel-task", "task-guid")
			Eventually(sess).Should(gexec.Exit(4))

			Expect(sess.Err).To(gbytes.Say("UnknownError"))
		})
	})

	Context("when passed invalid arguments", func() {
		It("fails with no arugments and prints the usage", func() {
			sess := RunCFDot("cancel-task")
			Eventually(sess).Should(gexec.Exit(3))
			Expect(sess.Err).To(gbytes.Say("Error: Missing arguments"))
			Expect(sess.Err).To(gbytes.Say("cfdot cancel-task TASK_GUID \\[flags\\]"))
		})

		It("fails with 2+ arguments", func() {
			sess := RunCFDot("cancel-task", "arg1", "arg2")
			Eventually(sess).Should(gexec.Exit(3))
			Expect(sess.Err).To(gbytes.Say("Error: Too many arguments specified"))
			Expect(sess.Err).To(gbytes.Say("cfdot cancel-task TASK_GUID \\[flags\\]"))
		})
	})
})
