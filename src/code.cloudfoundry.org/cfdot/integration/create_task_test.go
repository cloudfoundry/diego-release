package integration_test

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/bbs/models"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("create-task", func() {

	itValidatesBBSFlags("create-task")

	Context("when no spec is passed", func() {
		It("exits with status code of 3 and prints the error and usage", func() {
			sess := RunCFDot("create-task")
			Eventually(sess).Should(gexec.Exit(3))
			Expect(sess.Err).To(gbytes.Say(`missing spec`))
			Expect(sess.Err).To(gbytes.Say("cfdot create-task \\(SPEC\\|@FILE\\) .*"))
		})
	})

	Context("when bbs responds with 200 status code", func() {
		var (
			task          *models.Task
			serverTimeout int
		)

		BeforeEach(func() {
			task = &models.Task{
				TaskGuid: "some-task-guid",
			}
			serverTimeout = 0
		})

		JustBeforeEach(func() {
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/tasks/desire.r2"),
					func(w http.ResponseWriter, req *http.Request) {
						time.Sleep(time.Duration(serverTimeout) * time.Second)
					},
					ghttp.VerifyProtoRepresenting(&models.DesireTaskRequest{
						TaskGuid:       task.TaskGuid,
						Domain:         task.Domain,
						TaskDefinition: task.TaskDefinition,
					}),
					ghttp.RespondWithProto(200, &models.TaskLifecycleResponse{
						Error: nil,
					}),
				),
			)
		})

		Context("as json", func() {
			var taskArg string

			BeforeEach(func() {
				spec, err := json.Marshal(task)
				Expect(err).NotTo(HaveOccurred())
				taskArg = string(spec)
			})

			It("exits with status code of 0", func() {
				sess := RunCFDot("create-task", taskArg)
				Eventually(sess).Should(gexec.Exit(0))
			})

			Context("when timeout flag is present", func() {
				var sess *gexec.Session

				BeforeEach(func() {
					sess = RunCFDot("create-task", "--timeout", "1", taskArg)
				})

				Context("when request exceeds timeout", func() {
					BeforeEach(func() {
						serverTimeout = 2
					})

					It("exits with code 4 and a timeout message", func() {
						Eventually(sess, 2).Should(gexec.Exit(4))
						Expect(sess.Err).To(gbytes.Say(`Timeout exceeded`))
					})
				})

				Context("when request is within the timeout", func() {
					It("exits with status code of 0", func() {
						Eventually(sess).Should(gexec.Exit(0))
					})
				})
			})
		})

		Context("as a file", func() {
			var taskArg string

			BeforeEach(func() {
				f, err := os.CreateTemp(os.TempDir(), "desired_task_spec")
				Expect(err).NotTo(HaveOccurred())
				defer f.Close()
				Expect(json.NewEncoder(f).Encode(task)).To(Succeed())

				taskArg = "@" + f.Name()
			})

			It("exits with status code 0", func() {
				sess := RunCFDot("create-task", taskArg)
				Eventually(sess).Should(gexec.Exit(0))
			})
		})

		Context("empty spec", func() {
			It("exits with status code of 3", func() {
				sess := RunCFDot("create-task")
				Eventually(sess).Should(gexec.Exit(3))
			})
		})

		Context("invalid spec", func() {
			It("exits with status code of 3 and prints the error", func() {
				sess := RunCFDot("create-task", "foo")
				Eventually(sess).Should(gexec.Exit(3))
				Expect(sess.Err).To(gbytes.Say("Invalid JSON:"))
			})
		})

		Context("non-existing spec file", func() {
			It("exits with status 3 and prints the error", func() {
				sess := RunCFDot("create-task", "@/path/to/non/existing/file")
				Eventually(sess).Should(gexec.Exit(3))
				Expect(sess.Err).To(gbytes.Say("no such file"))
			})
		})
	})

	Context("when bbs responds with non-200 status code", func() {
		JustBeforeEach(func() {
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/tasks/desire.r2"),
					ghttp.RespondWithProto(500, &models.TaskLifecycleResponse{
						Error: &models.Error{
							Type:    models.Error_Deadlock,
							Message: "deadlock detected",
						},
					}),
				),
			)
		})

		It("exits with status code 4 and prints the error", func() {
			sess := RunCFDot("create-task", "{}")
			Eventually(sess).Should(gexec.Exit(4))
			Expect(sess.Err).To(gbytes.Say("deadlock"))
		})
	})
})
