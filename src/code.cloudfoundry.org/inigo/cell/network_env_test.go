package cell_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"code.cloudfoundry.org/archiver/extractor/test_helper"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/inigo/fixtures"
	"code.cloudfoundry.org/inigo/helpers"
	repconfig "code.cloudfoundry.org/rep/cmd/rep/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/tedsuo/ifrit/grouper"
)

var _ = Describe("Network Environment Variables", func() {
	var (
		guid                string
		modifyRepConfig     func(*repconfig.RepConfig)
		fileServerStaticDir string
		fileServer          ifrit.Runner
		ifritRuntime        ifrit.Process
	)

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip(" not yet working on windows")
		}
		fileServer, fileServerStaticDir = componentMaker.FileServer()
		modifyRepConfig = func(*repconfig.RepConfig) {}
		guid = helpers.GenerateGuid()
	})

	JustBeforeEach(func() {
		ifritRuntime = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{Name: "rep", Runner: componentMaker.Rep(modifyRepConfig)},
			{Name: "auctioneer", Runner: componentMaker.Auctioneer()},
			{Name: "router", Runner: componentMaker.Router()},
			{Name: "route-emitter", Runner: componentMaker.RouteEmitter()},
			{Name: "file-server", Runner: fileServer},
		}))
	})

	AfterEach(func() {
		helpers.StopProcesses(ifritRuntime)
	})

	Describe("tasks", func() {
		var task *models.Task

		JustBeforeEach(func() {
			taskToDesire := helpers.TaskCreateRequest(
				guid,
				&models.RunAction{
					User: "vcap",
					Path: "sh",
					Args: []string{"-c", "/usr/bin/env | grep 'CF_INSTANCE' > /home/vcap/env"},
				},
			)
			taskToDesire.TaskDefinition.ResultFile = "/home/vcap/env"

			err := bbsClient.DesireTask(lgr, "", taskToDesire.TaskGuid, taskToDesire.Domain, taskToDesire.TaskDefinition)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() interface{} {
				var err error
				task, err = bbsClient.TaskByGuid(lgr, "", guid)
				Expect(err).ShouldNot(HaveOccurred())

				return task.State
			}).Should(Equal(models.Task_Completed))
		})

		It("sets the networking environment variables", func() {
			Expect(task.Result).To(ContainSubstring("CF_INSTANCE_ADDR=\n"))
			Expect(task.Result).To(ContainSubstring("CF_INSTANCE_PORT=\n"))
			Expect(task.Result).To(ContainSubstring("CF_INSTANCE_PORTS=[]\n"))
			Expect(task.Result).To(ContainSubstring(fmt.Sprintf("CF_INSTANCE_IP=%s\n", os.Getenv("EXTERNAL_ADDRESS"))))
			Expect(task.Result).To(ContainSubstring("CF_INSTANCE_INTERNAL_IP="))
		})
	})

	Describe("LRPs", func() {
		var response []byte
		var actualLRP *models.ActualLRP

		BeforeEach(func() {
			test_helper.CreateZipArchive(
				filepath.Join(fileServerStaticDir, "lrp.zip"),
				fixtures.GoServerApp(),
			)
		})

		JustBeforeEach(func() {
			lrp := helpers.DefaultLRPCreateRequest(componentMaker.Addresses(), guid, guid, 1)
			lrp.Setup = models.WrapAction(&models.DownloadAction{
				User: "vcap",
				From: fmt.Sprintf("http://%s/v1/static/%s", componentMaker.Addresses().FileServer, "lrp.zip"),
				To:   "/tmp/diego",
			})
			lrp.Action = models.WrapAction(&models.RunAction{
				User: "vcap",
				Path: "/tmp/diego/go-server",
				Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
			})

			err := bbsClient.DesireLRP(lgr, "", lrp)
			Expect(err).NotTo(HaveOccurred())

			Eventually(helpers.LRPStatePoller(lgr, bbsClient, guid, nil)).Should(Equal(models.ActualLRPStateRunning))
			Eventually(helpers.ResponseCodeFromHostPoller(componentMaker.Addresses().Router, helpers.DefaultHost)).Should(Equal(http.StatusOK))

			lrps, err := bbsClient.ActualLRPs(lgr, "", models.ActualLRPFilter{ProcessGuid: guid})
			Expect(err).NotTo(HaveOccurred())

			Expect(lrps).To(HaveLen(1))
			actualLRP = lrps[0]

			var status int
			response, status, err = helpers.ResponseBodyAndStatusCodeFromHost(componentMaker.Addresses().Router, helpers.DefaultHost, "env")
			Expect(status).To(Equal(http.StatusOK))
			Expect(err).NotTo(HaveOccurred())
		})

		It("sets the networking environment variables", func() {
			netInfo := actualLRP.ActualLrpNetInfo
			Expect(response).To(ContainSubstring(fmt.Sprintf("CF_INSTANCE_ADDR=%s:%d\n", netInfo.Address, netInfo.Ports[0].HostPort)))
			Expect(response).To(ContainSubstring(fmt.Sprintf("CF_INSTANCE_IP=%s\n", os.Getenv("EXTERNAL_ADDRESS"))))
			Expect(response).To(ContainSubstring(fmt.Sprintf("CF_INSTANCE_INTERNAL_IP=%s\n", netInfo.InstanceAddress)))
			Expect(response).To(ContainSubstring(fmt.Sprintf("CF_INSTANCE_PORT=%d\n", netInfo.Ports[0].HostPort)))

			type portMapping struct {
				External uint32 `json:"external"`
				Internal uint32 `json:"internal"`
			}
			ports := []portMapping{}

			buf := bytes.NewBuffer(response)
			for {
				line, err := buf.ReadString('\n')
				if err != nil {
					break
				}
				if strings.HasPrefix(line, "CF_INSTANCE_PORTS=") {
					err := json.Unmarshal([]byte(strings.TrimPrefix(line, "CF_INSTANCE_PORTS=")), &ports)
					Expect(err).NotTo(HaveOccurred())
					break
				}
			}

			Expect(ports).To(Equal([]portMapping{
				{External: netInfo.Ports[0].HostPort, Internal: netInfo.Ports[0].ContainerPort},
			}))
		})
	})
})
