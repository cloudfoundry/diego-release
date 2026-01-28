package cell_test

import (
	"os"
	"runtime"

	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/tedsuo/ifrit/grouper"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/inigo/helpers"
	repconfig "code.cloudfoundry.org/rep/cmd/rep/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tasks as specific user", func() {
	var cellProcess ifrit.Process

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip(" not yet working on windows")
		}
		var fileServerRunner ifrit.Runner

		fileServerRunner, _ = componentMaker.FileServer()

		cellGroup := grouper.Members{
			{Name: "file-server", Runner: fileServerRunner},
			{Name: "rep", Runner: componentMaker.Rep(func(config *repconfig.RepConfig) { config.MemoryMB = "1024" })},
			{Name: "auctioneer", Runner: componentMaker.Auctioneer()},
		}
		cellProcess = ginkgomon.Invoke(grouper.NewParallel(os.Interrupt, cellGroup))

		Eventually(func() (models.CellSet, error) { return bbsServiceClient.Cells(lgr) }).Should(HaveLen(1))
	})

	AfterEach(func() {
		helpers.StopProcesses(cellProcess)
	})

	Describe("Running a task", func() {
		var guid string

		BeforeEach(func() {
			guid = helpers.GenerateGuid()
		})

		It("runs the command as a specific user", func() {
			expectedTask := helpers.TaskCreateRequest(
				guid,
				&models.RunAction{
					User: "testuser",
					Path: "sh",
					Args: []string{"-c", `[ $(whoami) = testuser ]`},
				},
			)
			expectedTask.TaskDefinition.Privileged = true
			err := bbsClient.DesireTask(lgr, "", expectedTask.TaskGuid, expectedTask.Domain, expectedTask.TaskDefinition)
			Expect(err).NotTo(HaveOccurred())

			var task *models.Task

			Eventually(func() interface{} {
				var err error

				task, err = bbsClient.TaskByGuid(lgr, "", guid)
				Expect(err).NotTo(HaveOccurred())

				return task.State
			}).Should(Equal(models.Task_Completed))
			Expect(task.Failed).To(BeFalse())
		})
	})
})
