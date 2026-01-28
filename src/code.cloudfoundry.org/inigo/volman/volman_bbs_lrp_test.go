package volman_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/inigo/helpers"
	"code.cloudfoundry.org/lager/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/tedsuo/ifrit/grouper"

	archive_helper "code.cloudfoundry.org/archiver/extractor/test_helper"
	"code.cloudfoundry.org/inigo/fixtures"
)

var _ = Describe("LRPs with volume mounts", func() {
	var (
		cellProcess         ifrit.Process
		fileServerStaticDir string
		plumbing            ifrit.Process
		logger              lager.Logger
		bbsClient           bbs.InternalClient
		processGuid         string
		archiveFiles        []archive_helper.ArchiveFile
	)

	BeforeEach(func() {
		var fileServerRunner ifrit.Runner
		fileServerRunner, fileServerStaticDir = componentMaker.FileServer()
		plumbing = ginkgomon.Invoke(grouper.NewOrdered(os.Kill, grouper.Members{
			{Name: "initial-services", Runner: grouper.NewParallel(os.Kill, grouper.Members{
				{Name: "sql", Runner: componentMaker.SQL()},
				{Name: "nats", Runner: componentMaker.NATS()},
			})},
			{Name: "locket", Runner: componentMaker.Locket()},
			{Name: "bbs", Runner: componentMaker.BBS()},
		}))

		logger = lager.NewLogger("test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))

		cellProcess = ginkgomon.Invoke(grouper.NewParallel(os.Kill, grouper.Members{
			{Name: "router", Runner: componentMaker.Router()},
			{Name: "file-server", Runner: fileServerRunner},
			{Name: "rep", Runner: componentMaker.Rep()},
			{Name: "auctioneer", Runner: componentMaker.Auctioneer()},
			{Name: "route-emitter", Runner: componentMaker.RouteEmitter()},
		}))

		bbsServiceClient := componentMaker.BBSServiceClient(logger)
		bbsClient = componentMaker.BBSClient()
		archiveFiles = fixtures.GoServerApp()

		Eventually(func() (models.CellSet, error) { return bbsServiceClient.Cells(logger) }).Should(HaveLen(1))
	})

	JustBeforeEach(func() {
		archive_helper.CreateZipArchive(
			filepath.Join(fileServerStaticDir, "lrp.zip"),
			archiveFiles,
		)
	})

	AfterEach(func() {
		destroyContainerErrors := helpers.CleanupGarden(gardenClient)
		helpers.StopProcesses(plumbing, cellProcess)
		Expect(destroyContainerErrors).To(
			BeEmpty(),
			"%d containers failed to be destroyed!",
			len(destroyContainerErrors),
		)
	})

	Describe("desiring with volume mount", func() {
		var lrp *models.DesiredLRP

		BeforeEach(func() {
			processGuid = helpers.GenerateGuid()

			volumeId := fmt.Sprintf("some-volumeID-%d", time.Now().UnixNano())
			someConfig := map[string]interface{}{"volume_id": volumeId}
			jsonSomeConfig, err := json.Marshal(someConfig)
			Expect(err).NotTo(HaveOccurred())

			lrp = helpers.DefaultLRPCreateRequest(componentMaker.Addresses(), processGuid, "log-guid", 1)

			lrp.Setup = nil

			lrp.VolumeMounts = []*models.VolumeMount{
				&models.VolumeMount{
					Driver:       "localdriver",
					ContainerDir: "/testmount",
					Mode:         "rw",
					Shared: &models.SharedDevice{
						VolumeId:    volumeId,
						MountConfig: string(jsonSomeConfig),
					},
				},
			}

			lrp.CachedDependencies = []*models.CachedDependency{{
				From:      fmt.Sprintf("http://%s/v1/static/%s", componentMaker.Addresses().FileServer, "lrp.zip"),
				To:        "/tmp/diego",
				Name:      "lrp bits",
				CacheKey:  "lrp-cache-key",
				LogSource: "APP",
			}}

			lrp.Privileged = true

			lrp.Action = models.WrapAction(&models.RunAction{
				User: "vcap",
				Path: "/tmp/diego/go-server",
				Env: []*models.EnvironmentVariable{
					{Name: "PORT", Value: "8080"},
					{Name: "MOUNT_POINT_DIR", Value: "/testmount"},
				},
			})
		})

		JustBeforeEach(func() {
			err := bbsClient.DesireLRP(logger, "", lrp)
			Expect(err).NotTo(HaveOccurred())
		})

		It("can write to a file on the mounted volume", func() {
			Eventually(helpers.LRPStatePoller(logger, bbsClient, processGuid, nil)).Should(Equal(models.ActualLRPStateRunning))
			Eventually(helpers.HelloWorldInstancePoller(componentMaker.Addresses().Router, helpers.DefaultHost)).Should(ConsistOf([]string{"0"}))
			body, statusCode, err := helpers.ResponseBodyAndStatusCodeFromHost(componentMaker.Addresses().Router, helpers.DefaultHost, "write")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal("Hello Persistent World!\n"))
			Expect(statusCode).To(Equal(200))
		})

		Context("when driver required not available on any cell", func() {
			BeforeEach(func() {
				lrp.VolumeMounts = []*models.VolumeMount{
					generateVolumeObject("non-existent-driver"),
				}
			})

			It("should error placing the lrp", func() {
				var actualLRP *models.ActualLRP
				Eventually(func() interface{} {
					index := int32(0)
					lrps, err := bbsClient.ActualLRPs(logger, "", models.ActualLRPFilter{ProcessGuid: processGuid, Index: &index})
					Expect(err).NotTo(HaveOccurred())
					Expect(len(lrps)).To(Equal(1))
					Expect(lrps[0].Presence).NotTo(Equal(models.ActualLRP_Evacuating))
					actualLRP = lrps[0]

					return actualLRP.PlacementError
				}).Should(Equal(auctiontypes.ErrorVolumeDriverMismatch.Error()))
			})
		})

		Context("when one of the drivers required is not available on any cell", func() {
			BeforeEach(func() {
				lrp.VolumeMounts = []*models.VolumeMount{
					generateVolumeObject("non-existent-driver"),
					generateVolumeObject("localdriver"),
				}
			})

			It("should error placing the task", func() {
				var actualLRP *models.ActualLRP
				Eventually(func() interface{} {
					index := int32(0)
					lrps, err := bbsClient.ActualLRPs(logger, "", models.ActualLRPFilter{ProcessGuid: processGuid, Index: &index})
					Expect(err).NotTo(HaveOccurred())
					Expect(len(lrps)).To(Equal(1))
					Expect(lrps[0].Presence).NotTo(Equal(models.ActualLRP_Evacuating))
					actualLRP = lrps[0]

					return actualLRP.PlacementError
				}).Should(Equal(auctiontypes.ErrorVolumeDriverMismatch.Error()))
			})
		})

		Context("when one of the drivers required is on a cell, but not running", func() {
			BeforeEach(func() {
				lrp.VolumeMounts = []*models.VolumeMount{
					generateVolumeObject("deaddriver"),
					generateVolumeObject("localdriver"),
				}
			})

			It("should error placing the task", func() {
				var actualLRP *models.ActualLRP
				Eventually(func() interface{} {
					index := int32(0)
					lrps, err := bbsClient.ActualLRPs(logger, "", models.ActualLRPFilter{ProcessGuid: processGuid, Index: &index})
					Expect(err).NotTo(HaveOccurred())
					Expect(lrps[0].Presence).NotTo(Equal(models.ActualLRP_Evacuating))
					actualLRP = lrps[0]

					return actualLRP.PlacementError
				}).Should(Equal(auctiontypes.ErrorVolumeDriverMismatch.Error()))
			})
		})
	})
})
