package cell_test

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"code.cloudfoundry.org/archiver/extractor/test_helper"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/inigo/fixtures"
	"code.cloudfoundry.org/inigo/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/tedsuo/ifrit/grouper"
)

var _ = Describe("Privileges", func() {
	var ifritRuntime ifrit.Process

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip(" not yet working on windows")
		}
		fileServer, fileServerStaticDir := componentMaker.FileServer()
		ifritRuntime = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{Name: "router", Runner: componentMaker.Router()},
			{Name: "file-server", Runner: fileServer},
			{Name: "rep", Runner: componentMaker.Rep()},
			{Name: "auctioneer", Runner: componentMaker.Auctioneer()},
			{Name: "route-emitter", Runner: componentMaker.RouteEmitter()},
		}))

		test_helper.CreateZipArchive(
			filepath.Join(fileServerStaticDir, "lrp.zip"),
			fixtures.GoServerApp(),
		)
	})

	AfterEach(func() {
		helpers.StopProcesses(ifritRuntime)
	})

	Context("when a task that tries to do privileged things is requested", func() {
		var taskToDesire *models.Task

		BeforeEach(func() {
			taskToDesire = helpers.TaskCreateRequest(
				helpers.GenerateGuid(),
				&models.RunAction{
					Path: "sh",
					// always run as root; tests change task-level privileged
					User: "root",
					Args: []string{
						"-c",
						// writing to /proc/sysrq-trigger requires full privileges;
						// h is a safe thing to write
						"echo h > /proc/sysrq-trigger",
					},
				},
			)
		})

		JustBeforeEach(func() {
			err := bbsClient.DesireTask(lgr, "", taskToDesire.TaskGuid, taskToDesire.Domain, taskToDesire.TaskDefinition)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the task is privileged", func() {
			BeforeEach(func() {
				taskToDesire.TaskDefinition.Privileged = true
			})

			It("succeeds", func() {
				var task models.Task
				Eventually(helpers.TaskStatePoller(lgr, bbsClient, taskToDesire.TaskGuid, &task)).Should(Equal(models.Task_Completed))
				Expect(task.Failed).To(BeFalse())
			})
		})

		Context("when the task is not privileged", func() {
			BeforeEach(func() {
				taskToDesire.TaskDefinition.Privileged = false
			})

			It("fails", func() {
				var task models.Task
				Eventually(helpers.TaskStatePoller(lgr, bbsClient, taskToDesire.TaskGuid, &task)).Should(Equal(models.Task_Completed))
				Expect(task.Failed).To(BeTrue())
			})
		})
	})

	Context("when a LRP that tries to do privileged things is requested", func() {
		var lrpRequest *models.DesiredLRP

		BeforeEach(func() {
			lrpRequest = helpers.DefaultLRPCreateRequest(componentMaker.Addresses(), helpers.GenerateGuid(), "log-guid", 1)
			lrpRequest.Action = models.WrapAction(&models.RunAction{
				User: "root",
				Path: "/tmp/diego/go-server",
				Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
			})
		})

		JustBeforeEach(func() {
			err := bbsClient.DesireLRP(lgr, "", lrpRequest)
			Expect(err).NotTo(HaveOccurred())
			Eventually(helpers.LRPStatePoller(lgr, bbsClient, lrpRequest.ProcessGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
		})

		Context("when the LRP is privileged", func() {
			BeforeEach(func() {
				lrpRequest.Privileged = true
			})

			It("succeeds", func() {
				Eventually(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses().Router, helpers.DefaultHost, "privileged")).Should(Equal(http.StatusOK))
			})
		})

		Context("when the LRP is not privileged", func() {
			BeforeEach(func() {
				lrpRequest.Privileged = false
			})

			It("fails", func() {
				Eventually(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses().Router, helpers.DefaultHost, "privileged")).Should(Equal(http.StatusInternalServerError))
			})
		})
	})
})
