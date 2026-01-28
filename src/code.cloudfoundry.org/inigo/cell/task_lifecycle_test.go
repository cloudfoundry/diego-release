package cell_test

import (
	"fmt"
	"os"
	"runtime"
	"time"

	bbsconfig "code.cloudfoundry.org/bbs/cmd/bbs/config"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/inigo/helpers"
	"code.cloudfoundry.org/inigo/inigo_announcement_server"

	repconfig "code.cloudfoundry.org/rep/cmd/rep/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/tedsuo/ifrit/grouper"
)

func overrideKickTaskDuration(config *bbsconfig.BBSConfig) {
	config.KickTaskDuration = durationjson.Duration(time.Second)
}

func overrideExpirePendingTaskDuration(config *bbsconfig.BBSConfig) {
	config.ExpirePendingTaskDuration = durationjson.Duration(time.Second)
}

var _ = Describe("Task Lifecycle", func() {
	var (
		auctioneerProcess ifrit.Process
		cellProcess       ifrit.Process
	)

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip(" not yet working on windows")
		}
		auctioneerProcess = nil
		cellProcess = nil
	})

	AfterEach(func() {
		helpers.StopProcesses(
			auctioneerProcess,
			cellProcess,
		)
	})

	Context("when a rep, and auctioneer are running", func() {
		BeforeEach(func() {

			cellProcess = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
				{Name: "rep", Runner: componentMaker.Rep(func(config *repconfig.RepConfig) {
					config.MemoryMB = "1024"
				})},
			}))

			auctioneerProcess = ginkgomon.Invoke(componentMaker.Auctioneer())
		})

		Context("and a standard Task is desired", func() {
			var taskGuid string
			var taskSleepSeconds int

			var taskToCreate *models.Task

			BeforeEach(func() {
				taskSleepSeconds = 5
				taskGuid = helpers.GenerateGuid()

				taskToCreate = helpers.TaskCreateRequestWithMemory(
					taskGuid,
					&models.RunAction{
						User: "vcap",
						Path: "sh",
						Args: []string{
							"-c",
							// sleep a bit so that we can make assertions around behavior as it's running
							fmt.Sprintf(`
								kill_sleep() {
									kill -15 $child
									exit
								}

								trap kill_sleep 15 9

								curl %s
								sleep %d &

								child=$!
								wait $child
							`, inigo_announcement_server.AnnounceURL(taskGuid), taskSleepSeconds),
						},
					},
					512,
				)
			})

			theFailureReason := func() string {
				taskAfterCancel, err := bbsClient.TaskByGuid(lgr, "", taskGuid)
				if err != nil {
					return ""
				}

				if taskAfterCancel.State != models.Task_Completed {
					return ""
				}

				if taskAfterCancel.Failed != true {
					return ""
				}

				return taskAfterCancel.FailureReason
			}

			JustBeforeEach(func() {
				err := bbsClient.DesireTask(lgr, "", taskToCreate.TaskGuid, taskToCreate.Domain, taskToCreate.TaskDefinition)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when there is a matching rootfs", func() {
				It("eventually runs the Task", func() {
					Eventually(inigo_announcement_server.Announcements).Should(ContainElement(taskGuid))
				})
			})

			Context("when there is no matching rootfs", func() {
				BeforeEach(func() {
					taskToCreate = helpers.TaskCreateRequestWithRootFS(
						taskGuid,
						helpers.BogusPreloadedRootFS,
						&models.RunAction{
							User: "vcap",
							Path: "true",
						},
					)
				})

				It("marks the task as complete, failed and cancelled", func() {
					Eventually(theFailureReason).Should(ContainSubstring("found no compatible cell"))
				})
			})

			Context("when there is not enough resources", func() {
				BeforeEach(func() {
					taskToCreate = helpers.TaskCreateRequestWithMemory(
						taskGuid,
						&models.RunAction{
							User: "vcap",
							Path: "sh",
							Args: []string{
								"-c",
								// sleep a bit so that we can make assertions around behavior as it's running
								fmt.Sprintf("curl %s; sleep %d", inigo_announcement_server.AnnounceURL(taskGuid), taskSleepSeconds),
							},
						},
						2048,
					)
				})

				It("marks the task as complete, failed and cancelled", func() {
					Eventually(theFailureReason).Should(Equal("insufficient resources: memory"))
				})
			})

			Context("and then the task is cancelled", func() {
				BeforeEach(func() {
					taskSleepSeconds = 9999 // ensure task never completes on its own
				})

				JustBeforeEach(func() {
					Eventually(inigo_announcement_server.Announcements).Should(ContainElement(taskGuid))

					err := bbsClient.CancelTask(lgr, "", taskGuid)
					Expect(err).NotTo(HaveOccurred())
				})

				It("marks the task as complete, failed and cancelled", func() {
					taskAfterCancel, err := bbsClient.TaskByGuid(lgr, "", taskGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(taskAfterCancel.State).To(Equal(models.Task_Completed))
					Expect(taskAfterCancel.Failed).To(BeTrue())
					Expect(taskAfterCancel.FailureReason).To(Equal("task was cancelled"))
				})
			})

			Context("when a converger is running", func() {
				BeforeEach(func() {
					By("restarting the bbs with smaller convergeRepeatInterval")
					ginkgomon.Interrupt(bbsProcess)
					bbsProcess = ginkgomon.Invoke(componentMaker.BBS(
						overrideConvergenceRepeatInterval,
						overrideKickTaskDuration,
					))
				})

				Context("after the task starts", func() {
					JustBeforeEach(func() {
						Eventually(inigo_announcement_server.Announcements).Should(ContainElement(taskGuid))
					})

					Context("when the cellProcess disappears", func() {
						JustBeforeEach(func() {
							helpers.StopProcesses(cellProcess)
						})

						It("eventually marks the task as failed", func() {
							// time is primarily influenced by rep's heartbeat interval
							var completedTask *models.Task
							Eventually(func() interface{} {
								var err error

								completedTask, err = bbsClient.TaskByGuid(lgr, "", taskGuid)
								Expect(err).NotTo(HaveOccurred())

								return completedTask.State
							}).Should(Equal(models.Task_Completed))

							Expect(completedTask.Failed).To(BeTrue())
						})
					})
				})
			})
		})

		Context("Egress Rules", func() {
			var (
				taskGuid     string
				taskToCreate *models.Task
			)

			BeforeEach(func() {
				taskGuid = helpers.GenerateGuid()
				taskToCreate = helpers.TaskCreateRequest(
					taskGuid,
					&models.RunAction{
						User: "vcap",
						Path: "sh",
						Args: []string{
							"-c",
							`
curl -s --connect-timeout 5 http://www.example.com -o /dev/null
echo $? >> /tmp/result
exit 0
					`,
						},
					},
				)
				taskToCreate.TaskDefinition.ResultFile = "/tmp/result"
			})

			JustBeforeEach(func() {
				err := bbsClient.DesireTask(lgr, "", taskToCreate.TaskGuid, taskToCreate.Domain, taskToCreate.TaskDefinition)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("default networking", func() {
				It("rejects outbound tcp traffic", func() {
					// Failed to connect to host
					pollTaskStatus(taskGuid, "7\n")
				})
			})

			Context("with appropriate security group setting", func() {
				BeforeEach(func() {
					taskToCreate.TaskDefinition.EgressRules = []*models.SecurityGroupRule{
						{
							Protocol:     models.TCPProtocol,
							Destinations: []string{"9.0.0.0-89.255.255.255", "90.0.0.0-94.0.0.0"},
							Ports:        []uint32{80, 443},
						},
						{
							Protocol:     models.UDPProtocol,
							Destinations: []string{"0.0.0.0/0"},
							PortRange: &models.PortRange{
								Start: 53,
								End:   53,
							},
						},
					}
				})

				It("allows outbound tcp traffic", func() {
					pollTaskStatus(taskGuid, "0\n")
				})
			})
		})
	})

	Context("when an auctioneer is not running", func() {
		BeforeEach(func() {
			By("restarting the bbs with smaller convergeRepeatInterval")
			ginkgomon.Interrupt(bbsProcess)
			bbsProcess = ginkgomon.Invoke(componentMaker.BBS(
				overrideConvergenceRepeatInterval,
				overrideKickTaskDuration,
			))

			cellProcess = ginkgomon.Invoke(grouper.NewParallel(os.Interrupt, grouper.Members{
				{Name: "rep", Runner: componentMaker.Rep()},
			}))
		})

		Context("and a task is desired", func() {
			var taskGuid string

			BeforeEach(func() {
				taskGuid = helpers.GenerateGuid()

				taskToDesire := helpers.TaskCreateRequest(
					taskGuid,
					&models.RunAction{
						User: "vcap",
						Path: "curl",
						Args: []string{inigo_announcement_server.AnnounceURL(taskGuid)},
					},
				)
				err := bbsClient.DesireTask(lgr, "", taskToDesire.TaskGuid, taskToDesire.Domain, taskToDesire.TaskDefinition)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("and then an auctioneer come up", func() {
				BeforeEach(func() {
					auctioneerProcess = ginkgomon.Invoke(componentMaker.Auctioneer())
				})

				AfterEach(func() {
					helpers.StopProcesses(cellProcess)
				})

				It("eventually runs the Task", func() {
					Eventually(inigo_announcement_server.Announcements).Should(ContainElement(taskGuid))
				})
			})
		})
	})

	Context("when a very impatient converger is running", func() {
		BeforeEach(func() {
			By("restarting the bbs with smaller convergeRepeatInterval")
			ginkgomon.Interrupt(bbsProcess)
			bbsProcess = ginkgomon.Invoke(componentMaker.BBS(
				overrideConvergenceRepeatInterval,
				overrideExpirePendingTaskDuration,
			))
		})

		Context("and a task is desired", func() {
			var taskGuid string

			BeforeEach(func() {
				taskGuid = helpers.GenerateGuid()

				taskToDesire := helpers.TaskCreateRequest(
					taskGuid,
					&models.RunAction{
						User: "vcap",
						Path: "curl",
						Args: []string{inigo_announcement_server.AnnounceURL(taskGuid)},
					},
				)

				err := bbsClient.DesireTask(lgr, "", taskToDesire.TaskGuid, taskToDesire.Domain, taskToDesire.TaskDefinition)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should be marked as failed after the expire duration", func() {
				var completedTask *models.Task
				Eventually(func() interface{} {
					var err error

					completedTask, err = bbsClient.TaskByGuid(lgr, "", taskGuid)
					Expect(err).NotTo(HaveOccurred())

					return completedTask.State
				}).Should(Equal(models.Task_Completed))

				Expect(completedTask.Failed).To(BeTrue())
				Expect(completedTask.FailureReason).To(ContainSubstring("not started within time limit"))

				Expect(inigo_announcement_server.Announcements()).To(BeEmpty())
			})
		})
	})
})

func pollTaskStatus(taskGuid string, result string) {
	var completedTask *models.Task
	Eventually(func() interface{} {
		var err error

		completedTask, err = bbsClient.TaskByGuid(lgr, "", taskGuid)
		Expect(err).NotTo(HaveOccurred())

		return completedTask.State
	}).Should(Equal(models.Task_Completed))

	Expect(completedTask.Result).To(Equal(result))
}
