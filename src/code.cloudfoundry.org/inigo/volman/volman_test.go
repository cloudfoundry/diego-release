package volman_test

import (
	"os"
	"path"

	dockerdriverutils "code.cloudfoundry.org/dockerdriver/utils"
	"code.cloudfoundry.org/inigo/helpers"
	"code.cloudfoundry.org/localdriver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
)

var _ = Describe("Given volman and localdriver", func() {
	var (
		driverId     string
		volumeId     string
		containerId  string
		volumeConfig map[string]interface{}
	)

	BeforeEach(func() {
		driverId = "localdriver"
		volumeId = "test-volume"
		containerId = "test-container"
		volumeConfig = map[string]interface{}{}
	})

	It("should mount a volume", func() {
		var err error
		mountPointResponse, err := volmanClient.Mount(logger, driverId, volumeId, containerId, volumeConfig)
		Expect(err).NotTo(HaveOccurred())
		Expect(mountPointResponse.Path).NotTo(BeEmpty())

		volmanClient.Unmount(logger, driverId, volumeId, containerId)
	})

	Context("and a mounted volman", func() {
		BeforeEach(func() {
			var err error
			mountPointResponse, err := volmanClient.Mount(logger, driverId, volumeId, containerId, volumeConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(mountPointResponse.Path).NotTo(BeEmpty())
		})

		It("should be able to unmount the volume", func() {
			err := volmanClient.Unmount(logger, driverId, volumeId, containerId)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when volman client restarted", func() {
		var (
			expectedMountPath string
		)
		BeforeEach(func() {
			uniqueVolumeId := dockerdriverutils.NewVolumeId(volumeId, containerId)
			expectedMountPath = path.Join(componentMaker.VolmanDriverConfigDir(), localdriver.MountsRootDir, uniqueVolumeId.GetUniqueId())
		})

		JustBeforeEach(func() {
			_, err := volmanClient.Mount(logger, "localdriver", volumeId, containerId, volumeConfig)
			Expect(err).NotTo(HaveOccurred())
			helpers.StopProcesses(driverSyncerProcess)
		})

		It("should purge existing mount points", func() {
			Eventually(func() bool {
				_, err := os.Stat(expectedMountPath)
				return os.IsNotExist(err)
			}).Should(Equal(false))

			volmanClient, driverSyncer = componentMaker.VolmanClient(logger)
			driverSyncerProcess = ginkgomon.Invoke(driverSyncer)

			Eventually(func() bool {
				_, err := os.Stat(expectedMountPath)
				return os.IsNotExist(err)
			}).Should(Equal(true))
		})
	})
})
