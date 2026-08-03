package integration_test

import (
	"encoding/json"
	"net/http"
	"time"

	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("task", func() {
	itValidatesBBSFlags("task", "task-guid")

	Context("when the server responds for task", func() {
		var task = &models.Task{
			TaskGuid: "task-guid",
		}

		var (
			serverTimeout int
		)

		BeforeEach(func() {
			serverTimeout = 0
		})

		JustBeforeEach(func() {
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/tasks/get_by_task_guid.r3"),
					func(w http.ResponseWriter, req *http.Request) {
						time.Sleep(time.Duration(serverTimeout) * time.Second)
					},
					ghttp.VerifyProtoRepresenting(&models.TaskByGuidRequest{TaskGuid: "task-guid"}),
					ghttp.RespondWithProto(200, &models.TaskResponse{Task: task}),
				),
			)
		})

		It("task prints a json representation of the task", func() {
			sess := RunCFDot("task", "task-guid")
			Eventually(sess).Should(gexec.Exit(0))

			taskJSON, err := json.Marshal(task)
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.Out.Contents()).To(MatchJSON(taskJSON))
		})

		Context("when timeout flag is present", func() {
			Context("when request exceeds timeout", func() {
				BeforeEach(func() {
					serverTimeout = 2
				})

				It("exits with code 4 and a timeout message", func() {
					sess := RunCFDot("task", "task-guid", "--timeout", "1")
					Eventually(sess, 2).Should(gexec.Exit(4))
					Expect(sess.Err).To(gbytes.Say(`Timeout exceeded`))
				})
			})

			Context("when request is within the timeout", func() {
				It("exits with status code of 0", func() {
					sess := RunCFDot("task", "task-guid", "--timeout", "1")
					Eventually(sess).Should(gexec.Exit(0))
				})
			})
		})
	})

	Context("when the server responds with error", func() {
		It("exits with status code 4", func() {
			bbsServer.RouteToHandler(
				"POST",
				"/v1/tasks/get_by_task_guid.r3",
				ghttp.RespondWithProto(200, &models.TaskResponse{
					Error: models.ErrUnknownError,
				}),
			)

			sess := RunCFDot("task", "task-guid")
			Eventually(sess).Should(gexec.Exit(4))

			Expect(sess.Err).To(gbytes.Say("UnknownError"))
		})
	})

	Context("validates that exactly one guid is passed in", func() {
		It("fails with no arguments and prints the usage", func() {
			sess := RunCFDot("task")
			Eventually(sess).Should(gexec.Exit(3))
			Expect(sess.Err).To(gbytes.Say("Error: Missing arguments"))
			Expect(sess.Err).To(gbytes.Say("cfdot task TASK_GUID \\[flags\\]"))
		})

		It("fails with 2+ arguments", func() {
			sess := RunCFDot("task", "task-guid", "arg1", "arg2")
			Eventually(sess).Should(gexec.Exit(3))
			Expect(sess.Err).To(gbytes.Say("Error: Too many arguments specified"))
			Expect(sess.Err).To(gbytes.Say("cfdot task TASK_GUID \\[flags\\]"))
		})
	})
})
