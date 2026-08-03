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

var _ = Describe("tasks", func() {
	itValidatesBBSFlags("tasks")

	Context("when the server responds for tasks", func() {
		task := models.Task{
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
					ghttp.VerifyRequest("POST", "/v1/tasks/list.r3"),
					func(w http.ResponseWriter, req *http.Request) {
						time.Sleep(time.Duration(serverTimeout) * time.Second)
					},
					ghttp.RespondWithProto(200, &models.TasksResponse{Tasks: []*models.Task{&task}}),
				),
			)
		})

		It("should return the tasks as a stream of json objects", func() {
			sess := RunCFDot("tasks")
			Eventually(sess).Should(gexec.Exit(0))

			Expect(bbsServer.ReceivedRequests()).To(HaveLen(1))

			expectedOutput, err := json.Marshal(&task)
			Expect(err).NotTo(HaveOccurred())

			Expect(sess.Out).To(gbytes.Say(string(expectedOutput)))
		})

		Context("when timeout flag is present", func() {
			Context("when request exceeds timeout", func() {
				BeforeEach(func() {
					serverTimeout = 2
				})

				It("exits with code 4 and a timeout message", func() {
					sess := RunCFDot("tasks", "--timeout", "1")
					Eventually(sess, 2).Should(gexec.Exit(4))
					Expect(sess.Err).To(gbytes.Say(`Timeout exceeded`))
				})
			})

			Context("when request is within the timeout", func() {
				It("exits with status code of 0", func() {
					sess := RunCFDot("tasks", "--timeout", "1")
					Eventually(sess).Should(gexec.Exit(0))

					Expect(bbsServer.ReceivedRequests()).To(HaveLen(1))

					expectedOutput, err := json.Marshal(&task)
					Expect(err).NotTo(HaveOccurred())

					Expect(sess.Out).To(gbytes.Say(string(expectedOutput)))
				})
			})
		})
	})

	Context("when there are filters for tasks", func() {
		Context("with domain filters", func() {
			BeforeEach(func() {
				bbsServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/tasks/list.r3"),
						ghttp.VerifyProtoRepresenting(&models.TasksRequest{
							Domain: "domain",
						}),
						ghttp.RespondWithProto(200, &models.TaskResponse{
							Task: &models.Task{
								TaskGuid: "task-guid",
							},
						}),
					),
				)
			})

			Context("with -d", func() {
				It("should exit with status code 0", func() {
					sess := RunCFDot("tasks", "-d", "domain")
					Eventually(sess).Should(gexec.Exit(0))
				})
			})

			Context("with --domain", func() {
				It("should exit with status code 0", func() {
					sess := RunCFDot("tasks", "--domain", "domain")
					Eventually(sess).Should(gexec.Exit(0))
				})
			})

			Context("with both --domain and -d", func() {
				It("should exit with a status code of 3", func() {
					sess := RunCFDot("tasks", "--domain", "domain", "-d", "domain")
					Eventually(sess).Should(gexec.Exit(3))
				})
			})
		})

		Context("with cell filters", func() {
			BeforeEach(func() {
				bbsServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/tasks/list.r3"),
						ghttp.VerifyProtoRepresenting(&models.TasksRequest{
							CellId: "cell-id",
						}),
						ghttp.RespondWithProto(200, &models.TaskResponse{
							Task: &models.Task{
								TaskGuid: "task-guid",
							},
						}),
					),
				)
			})

			Context("with -c", func() {
				It("should exit with status code 0", func() {
					sess := RunCFDot("tasks", "-c", "cell-id")
					Eventually(sess).Should(gexec.Exit(0))
				})
			})

			Context("with --cell-id", func() {
				It("should exit with status code 0", func() {
					sess := RunCFDot("tasks", "--cell-id", "cell-id")
					Eventually(sess).Should(gexec.Exit(0))
				})
			})

			Context("with both --cell-id and -c", func() {
				It("should exit with a status code of 3", func() {
					sess := RunCFDot("tasks", "--cell-id", "cell-id", "-c", "cell-id")
					Eventually(sess).Should(gexec.Exit(3))
				})
			})
		})

		Context("with cell and domain filters", func() {
			BeforeEach(func() {
				bbsServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/v1/tasks/list.r3"),
						ghttp.VerifyProtoRepresenting(&models.TasksRequest{
							Domain: "domain",
							CellId: "cell-id",
						}),
						ghttp.RespondWithProto(200, &models.TaskResponse{
							Task: &models.Task{
								TaskGuid: "task-guid",
							},
						}),
					),
				)
			})

			It("should exit with status code 0", func() {
				sess := RunCFDot("tasks", "-c", "cell-id", "-d", "domain")
				Eventually(sess).Should(gexec.Exit(0))
			})
		})
	})

	Context("when extra args are given", func() {
		It("returns an error and exits with status 3", func() {
			sess := RunCFDot("tasks", "garbage")
			Eventually(sess).Should(gexec.Exit(3))
			Expect(sess.Err).To(gbytes.Say("Too many arguments specified"))
		})
	})

	Context("when the bbs returns an error", func() {
		It("returns an error and exits with status 4", func() {
			bbsServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/v1/tasks/list.r3"),
					ghttp.RespondWithProto(200, &models.TasksResponse{Error: models.ErrUnknownError}),
				),
			)

			sess := RunCFDot("tasks")
			Eventually(sess).Should(gexec.Exit(4))
			Expect(sess.Err).To(gbytes.Say("the request failed for an unknown reason"))
		})
	})
})
