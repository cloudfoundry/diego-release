package vizzini_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/onsi/gomega/ghttp"

	"code.cloudfoundry.org/bbs/models"
	. "code.cloudfoundry.org/vizzini/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tasks", func() {
	var task *models.TaskDefinition

	BeforeEach(func() {
		task = Task()
	})

	Describe("Creating Tasks", func() {
		Context("When the task is well formed (the happy path)", func() {
			BeforeEach(func() {
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
			})

			It("runs the task", func() {
				Eventually(TaskGetter(logger, guid)).Should(HaveTaskState(models.Task_Completed))

				task, err := bbsClient.TaskByGuid(logger, traceID, guid)
				Expect(err).NotTo(HaveOccurred())
				Expect(task.TaskGuid).To(Equal(guid))

				Expect(task.Failed).To(BeFalse())
				Expect(task.Result).To(ContainSubstring("some output"))

				Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
				Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
			})
		})

		Context("when the task guid is malformed", func() {
			It("should fail to create", func() {
				var badGuid string

				badGuid = "abc def"
				err := bbsClient.DesireTask(logger, traceID, badGuid, domain, task)
				Expect(models.ConvertError(err).Type).To(Equal(models.Error_InvalidRequest))

				badGuid = "abc/def"
				Expect(bbsClient.DesireTask(logger, traceID, badGuid, domain, task)).NotTo(Succeed())

				badGuid = "abc,def"
				Expect(bbsClient.DesireTask(logger, traceID, badGuid, domain, task)).NotTo(Succeed())

				badGuid = "abc.def"
				Expect(bbsClient.DesireTask(logger, traceID, badGuid, domain, task)).NotTo(Succeed())

				badGuid = "abc∆def"
				Expect(bbsClient.DesireTask(logger, traceID, badGuid, domain, task)).NotTo(Succeed())
			})
		})

		Context("when the task guid is not unique", func() {
			It("should fail to create", func() {
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
				err := bbsClient.DesireTask(logger, traceID, guid, domain, task)
				Expect(models.ConvertError(err).Type).To(Equal(models.Error_ResourceExists))

				By("even when the domain is different")
				Expect(bbsClient.DesireTask(logger, traceID, guid, otherDomain, task)).NotTo(Succeed())

				Eventually(TaskGetter(logger, guid)).Should(HaveTaskState(models.Task_Completed))
				Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
				Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
			})
		})

		Context("when required fields are missing", func() {
			It("should fail", func() {
				By("not having TaskGuid")
				Expect(bbsClient.DesireTask(logger, traceID, "", domain, task)).NotTo(Succeed())

				By("not having a domain")
				Expect(bbsClient.DesireTask(logger, traceID, guid, "", task)).NotTo(Succeed())

				By("not having any actions")
				invalidTask := Task()
				invalidTask.Action = nil
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, invalidTask)).NotTo(Succeed())

				By("not having a rootfs")
				invalidTask = Task()
				invalidTask.RootFs = ""
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, invalidTask)).NotTo(Succeed())

				By("having a malformed rootfs")
				invalidTask = Task()
				invalidTask.RootFs = "ploop"
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, invalidTask)).NotTo(Succeed())
			})
		})

		Context("when the annotation is too large", func() {
			It("should fail", func() {
				task.Annotation = strings.Repeat("7", 1024*10+1)
				err := bbsClient.DesireTask(logger, traceID, guid, domain, task)
				Expect(models.ConvertError(err).Type).To(Equal(models.Error_InvalidRequest))
			})
		})

		Context("Upon failure", func() {
			BeforeEach(func() {
				task.Action = models.WrapAction(&models.RunAction{
					Path: "bash",
					Args: []string{"-c", "echo 'some output' > /tmp/bar && exit 1"},
					User: "vcap",
				})
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
			})

			It("should be marked as failed and should not return the result file", func() {
				Eventually(TaskGetter(logger, guid)).Should(HaveTaskState(models.Task_Completed))

				task, err := bbsClient.TaskByGuid(logger, traceID, guid)
				Expect(err).NotTo(HaveOccurred())
				Expect(task.TaskGuid).To(Equal(guid))
				Expect(task.Failed).To(BeTrue())

				Expect(task.Result).To(BeEmpty())

				Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
				Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
			})
		})
	})

	Describe("Specifying environment variables", func() {
		BeforeEach(func() {
			task.EnvironmentVariables = []*models.EnvironmentVariable{
				{Name: "CONTAINER_LEVEL", Value: "A"},
				{Name: "OVERRIDE", Value: "B"},
			}
			task.Action = models.WrapAction(&models.RunAction{
				Path: "bash",
				Args: []string{"-c", "env > /tmp/bar"},
				User: "vcap",
				Env: []*models.EnvironmentVariable{
					{Name: "ACTION_LEVEL", Value: "C"},
					{Name: "OVERRIDE", Value: "D"},
				},
			})
		})

		It("should be possible to specify environment variables on both the Task and the RunAction", func() {
			Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
			Eventually(TaskGetter(logger, guid)).Should(HaveTaskState(models.Task_Completed))

			task, err := bbsClient.TaskByGuid(logger, traceID, guid)
			Expect(err).NotTo(HaveOccurred())

			Expect(task.Result).To(ContainSubstring("CONTAINER_LEVEL=A"))
			Expect(task.Result).To(ContainSubstring("ACTION_LEVEL=C"))
			Expect(task.Result).To(ContainSubstring("OVERRIDE=D"))
			Expect(task.Result).NotTo(ContainSubstring("OVERRIDE=B"))

			Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
			Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
		})
	})

	Describe("{DOCKER} Creating a Docker-based Task", func() {
		BeforeEach(func() {
			task.RootFs = "docker:///cloudfoundry/busybox-alice"
			task.Action = models.WrapAction(&models.RunAction{
				Path: "sh",
				Args: []string{"-c", "echo 'down-the-rabbit-hole' > payload && chmod 0400 payload"},
				User: "alice",
			})
			task.ResultFile = "/home/alice/payload"
		})
		JustBeforeEach(func() {
			Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
		})

		It("should succeed", func() {
			Eventually(TaskGetter(logger, guid), 120).Should(HaveTaskState(models.Task_Completed), "Docker can be quite slow to spin up....")

			task, err := bbsClient.TaskByGuid(logger, traceID, guid)
			Expect(err).NotTo(HaveOccurred())
			Expect(task.Failed).To(BeFalse())
			Expect(task.Result).To(ContainSubstring("down-the-rabbit-hole"))

			Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
			Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
		})
		Context("with an OCI image", func() {
			BeforeEach(func() {
				task.RootFs = config.DiegoDockerOCIImageURL
				task.Action = models.WrapAction(&models.RunAction{
					Path: "sh",
					Args: []string{"-c", "echo 'down-the-rabbit-hole' > /payload && chmod 0400 /payload"},
					User: "root",
				})
				task.ResultFile = "/payload"
			})
			It("should succeed", func() {
				Eventually(TaskGetter(logger, guid), 120).Should(HaveTaskState(models.Task_Completed), "Docker can be quite slow to spin up....")

				task, err := bbsClient.TaskByGuid(logger, traceID, guid)
				Expect(err).NotTo(HaveOccurred())
				Expect(task.Failed).To(BeFalse())
				Expect(task.Result).To(ContainSubstring("down-the-rabbit-hole"))

				Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
				Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
			})
		})
	})

	Describe("Cancelling tasks", func() {
		Context("when the task exists", func() {
			var lrpGuid string

			BeforeEach(func() {
				lrpGuid = NewGuid()

				lrp := DesiredLRPWithGuid(lrpGuid)
				Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
				Eventually(EndpointCurler("http://" + RouteForGuid(lrpGuid) + "/env")).Should(Equal(http.StatusOK))

				incrementCounterRoute := "http://" + RouteForGuid(lrpGuid) + "/counter"

				task.EgressRules = []*models.SecurityGroupRule{
					{
						Protocol:     models.AllProtocol,
						Destinations: []string{"0.0.0.0/0"},
					},
				}
				task.Action = models.WrapAction(&models.RunAction{
					Path: "bash",
					Args: []string{"-c", fmt.Sprintf("while true; do curl %s -X POST -d 'body'; sleep 0.05; done", incrementCounterRoute)},
					User: "vcap",
				})

				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
			})

			It("should cancel the task immediately", func() {
				Eventually(TaskGetter(logger, guid)).Should(HaveTaskState(models.Task_Running))

				By("verifying the counter is being incremented")
				Eventually(GraceCounterGetter(lrpGuid)).Should(BeNumerically(">", 2))

				Expect(bbsClient.CancelTask(logger, traceID, guid)).To(Succeed())

				By("marking the task as completed")
				task, err := bbsClient.TaskByGuid(logger, traceID, guid)
				Expect(err).NotTo(HaveOccurred())
				Expect(task.State).To(Equal(models.Task_Completed))
				Expect(task.Failed).To(BeTrue())
				Expect(task.FailureReason).To(Equal("task was cancelled"))

				By("actually shutting down the container immediately, it should stop incrementing the counter")
				counterAfterCancel, err := GraceCounterGetter(lrpGuid)()
				Expect(err).NotTo(HaveOccurred())

				time.Sleep(2 * time.Second)

				counterAfterSomeTime, err := GraceCounterGetter(lrpGuid)()
				Expect(err).NotTo(HaveOccurred())
				Expect(counterAfterSomeTime).To(BeNumerically("<", counterAfterCancel+20))

				Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
				Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
			})
		})

		Context("when the task does not exist", func() {
			It("should fail", func() {
				Expect(bbsClient.CancelTask(logger, traceID, "floobeedoo")).NotTo(Succeed())
			})
		})

		Context("when the task is already completed", func() {
			BeforeEach(func() {
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
				Eventually(TaskGetter(logger, guid)).Should(HaveTaskState(models.Task_Completed))
			})

			It("should fail", func() {
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).NotTo(Succeed())

				Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
				Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
			})
		})
	})

	Describe("Getting a task", func() {
		Context("when the task exists", func() {
			BeforeEach(func() {
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
			})

			It("should succeed", func() {
				Eventually(TaskGetter(logger, guid)).ShouldNot(BeZero())
				task, err := bbsClient.TaskByGuid(logger, traceID, guid)
				Expect(err).NotTo(HaveOccurred())
				Expect(task.TaskGuid).To(Equal(guid))

				Eventually(TaskGetter(logger, guid)).Should(HaveTaskState(models.Task_Completed))
				Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
				Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
			})
		})

		Context("when the task does not exist", func() {
			It("should error", func() {
				task, err := bbsClient.TaskByGuid(logger, traceID, "floobeedoo")
				Expect(task).To(BeZero())
				Expect(models.ConvertError(err).Type).To(Equal(models.Error_ResourceNotFound))
			})
		})
	})

	Describe("Getting All Tasks and Getting Tasks by Domain", func() {
		var otherGuids []string

		BeforeEach(func() {
			Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
			Eventually(TaskGetter(logger, guid)).Should(HaveTaskState(models.Task_Completed))

			otherGuids = []string{NewGuid(), NewGuid()}
			for _, otherGuid := range otherGuids {
				otherTask := Task()
				Expect(bbsClient.DesireTask(logger, traceID, otherGuid, otherDomain, otherTask)).To(Succeed())
				Eventually(TaskGetter(logger, otherGuid)).Should(HaveTaskState(models.Task_Completed))
			}
		})

		AfterEach(func() {
			Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
			Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
			for _, otherGuid := range otherGuids {
				Expect(bbsClient.ResolvingTask(logger, traceID, otherGuid)).To(Succeed())
				Expect(bbsClient.DeleteTask(logger, traceID, otherGuid)).To(Succeed())
			}
		})

		It("should fetch tasks in the given domain", func() {
			tasksInDomain, err := bbsClient.TasksByDomain(logger, traceID, domain)
			Expect(err).NotTo(HaveOccurred())

			tasksInOtherDomain, err := bbsClient.TasksByDomain(logger, traceID, otherDomain)
			Expect(err).NotTo(HaveOccurred())

			Expect(tasksInDomain).To(HaveLen(1))
			Expect(tasksInOtherDomain).To(HaveLen(2))
			Expect([]string{tasksInOtherDomain[0].TaskGuid, tasksInOtherDomain[1].TaskGuid}).To(ConsistOf(otherGuids))
		})

		It("should not error if a domain is empty", func() {
			tasks, err := bbsClient.TasksByDomain(logger, traceID, "farfignoogan")
			Expect(err).NotTo(HaveOccurred())
			Expect(tasks).To(BeEmpty())
		})

		It("should fetch all tasks", func() {
			allTasks, err := bbsClient.Tasks(logger, traceID)
			Expect(err).NotTo(HaveOccurred())

			//if we're running in parallel there may be more than 3 things here!
			Expect(len(allTasks)).To(BeNumerically(">=", 3))
			taskGuids := []string{}
			for _, task := range allTasks {
				taskGuids = append(taskGuids, task.TaskGuid)
			}
			Expect(taskGuids).To(ContainElement(guid))
			Expect(taskGuids).To(ContainElement(otherGuids[0]))
			Expect(taskGuids).To(ContainElement(otherGuids[1]))
		})
	})

	Describe("Deleting Tasks", func() {
		Context("when the task is in the completed state", func() {
			It("should be deleted", func() {
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
				Eventually(TaskGetter(logger, guid)).Should(HaveTaskState(models.Task_Completed))

				Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
				Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
				_, err := bbsClient.TaskByGuid(logger, traceID, guid)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the task is not in the completed state", func() {
			It("should not be deleted, and should error", func() {
				task.Action = models.WrapAction(&models.RunAction{
					Path: "bash",
					Args: []string{"-c", "sleep 2; echo 'some output' > /tmp/bar"},
					User: "vcap",
				})
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
				Eventually(TaskGetter(logger, guid)).Should(HaveTaskState(models.Task_Running))
				err := bbsClient.ResolvingTask(logger, traceID, guid)
				Expect(models.ConvertError(err).Type).To(Equal(models.Error_InvalidStateTransition))

				_, err = bbsClient.TasksByDomain(logger, traceID, domain)
				Expect(err).NotTo(HaveOccurred())

				Eventually(TaskGetter(logger, guid)).Should(HaveTaskState(models.Task_Completed))
				Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
				Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
			})
		})

		Context("when the task does not exist", func() {
			It("should not be deleted, and should error", func() {
				err := bbsClient.ResolvingTask(logger, traceID, "floobeedoobee")
				Expect(models.ConvertError(err).Type).To(Equal(models.Error_ResourceNotFound))
			})
		})
	})

	Describe("Registering Completion Callbacks", func() {
		var server *ghttp.Server
		var port string
		var status int
		var done chan struct{}

		BeforeEach(func() {
			status = http.StatusOK

			server = ghttp.NewUnstartedServer()
			l, err := net.Listen("tcp", "0.0.0.0:0")
			Expect(err).NotTo(HaveOccurred())
			server.HTTPTestServer.Listener = l
			server.HTTPTestServer.Start()

			re := regexp.MustCompile(`:(\d+)$`)
			port = re.FindStringSubmatch(server.URL())[1]
			Expect(port).NotTo(BeZero())

			done = make(chan struct{})
			server.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/endpoint"),
				ghttp.RespondWithPtr(&status, nil),
				func(w http.ResponseWriter, req *http.Request) {
					var receivedTask models.Task
					json.NewDecoder(req.Body).Decode(&receivedTask)
					Expect(receivedTask.TaskGuid).To(Equal(guid))
					close(done)
				},
			))

			task.CompletionCallbackUrl = "http://" + config.HostAddress + ":" + port + "/endpoint"
		})

		AfterEach(func() {
			server.Close()
		})

		Context("when the server responds succesfully", func() {
			BeforeEach(func() {
				status = http.StatusOK
			})

			It("cleans up the task", func() {
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
				Eventually(done).Should(BeClosed())
				Eventually(func() bool {
					_, err := bbsClient.TaskByGuid(logger, traceID, guid)
					return err == nil
				}).Should(BeFalse(), "Eventually, the task should be resolved")
			})
		})

		Context("when the server responds in the 4XX range", func() {
			BeforeEach(func() {
				status = http.StatusNotFound
			})

			It("nonetheless, cleans up the task", func() {
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
				Eventually(done).Should(BeClosed())
				Eventually(func() bool {
					_, err := bbsClient.TaskByGuid(logger, traceID, guid)
					return err == nil
				}).Should(BeFalse(), "Eventually, the task should be resolved")
			})
		})

		Context("when the server responds with 503 repeatedly", func() {
			var secondDone chan struct{}

			BeforeEach(func() {
				status = http.StatusServiceUnavailable

				secondDone = make(chan struct{})
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/endpoint"),
					ghttp.RespondWith(http.StatusOK, nil),
					func(w http.ResponseWriter, req *http.Request) {
						var receivedTask models.Task
						json.NewDecoder(req.Body).Decode(&receivedTask)
						Expect(receivedTask.TaskGuid).To(Equal(guid))
						close(secondDone)
					},
				))
			})

			It("should retry", func() {
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
				Eventually(done).Should(BeClosed())
				Eventually(secondDone).Should(BeClosed())
				Eventually(func() bool {
					_, err := bbsClient.TaskByGuid(logger, traceID, guid)
					return err == nil
				}).Should(BeFalse(), "Eventually, the task should be resolved")
			})
		})

		Context("[Regression: #84595244] when there's no room for the Task", func() {
			BeforeEach(func() {
				task.MemoryMb = 1024 * 1024
			})

			It("should hit the callback", func() {
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
				Eventually(done, taskFailureTimeout).Should(BeClosed())

				Eventually(func() bool {
					_, err := bbsClient.TaskByGuid(logger, traceID, guid)
					return err == nil
				}).Should(BeFalse(), "Eventually, the task should be resolved")
			})
		})
	})

	Describe("when the Task cannot be allocated", func() {
		Context("because it's too large", func() {
			BeforeEach(func() {
				task.MemoryMb = 1024 * 1024
			})

			It("should allow creation of the task but should mark the task as failed", func() {
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
				Eventually(TaskGetter(logger, guid), taskFailureTimeout).Should(HaveTaskState(models.Task_Completed))

				retreivedTask, err := bbsClient.TaskByGuid(logger, traceID, guid)
				Expect(err).NotTo(HaveOccurred())

				Expect(retreivedTask.Failed).To(BeTrue())
				Expect(retreivedTask.FailureReason).To(ContainSubstring("insufficient resources"))

				Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
				Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
			})
		})

		Context("because of a stack mismatch", func() {
			BeforeEach(func() {
				task.RootFs = models.PreloadedRootFS("fruitfs")
			})

			It("should allow creation of the task but should mark the task as failed", func() {
				Expect(bbsClient.DesireTask(logger, traceID, guid, domain, task)).To(Succeed())
				Eventually(TaskGetter(logger, guid), taskFailureTimeout).Should(HaveTaskState(models.Task_Completed))

				retreivedTask, err := bbsClient.TaskByGuid(logger, traceID, guid)
				Expect(err).NotTo(HaveOccurred())

				Expect(retreivedTask.Failed).To(BeTrue())
				Expect(retreivedTask.FailureReason).To(ContainSubstring("found no compatible cell"))

				Expect(bbsClient.ResolvingTask(logger, traceID, guid)).To(Succeed())
				Expect(bbsClient.DeleteTask(logger, traceID, guid)).To(Succeed())
			})
		})
	})
})
