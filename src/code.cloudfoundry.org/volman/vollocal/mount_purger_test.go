package vollocal_test

import (
	"code.cloudfoundry.org/volman/voldiscoverers"
	"code.cloudfoundry.org/volman/vollocal"

	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/dockerdriver"
	"code.cloudfoundry.org/dockerdriver/dockerdriverfakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/volman"
	"code.cloudfoundry.org/volman/volmanfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
)

var _ = Describe("MountPurger", func() {

	var (
		logger *lagertest.TestLogger

		driverRegistry         volman.PluginRegistry
		dockerDriverDiscoverer volman.Discoverer
		purger                 vollocal.MountPurger

		fakeDriverFactory *volmanfakes.FakeDockerDriverFactory
		fakeDriver        *dockerdriverfakes.FakeDriver
		fakeClock         clock.Clock

		scanInterval time.Duration

		process ifrit.Process

		err error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("mount-purger")

		driverRegistry = vollocal.NewPluginRegistry()
	})

	JustBeforeEach(func() {
		purger = vollocal.NewMountPurger(logger, driverRegistry)
		err = purger.PurgeMounts(logger)
	})

	It("should succeed when there are no drivers", func() {
		//err := purger.PurgeMounts(logger)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when there is a non-dockerdriver plugin", func() {
		BeforeEach(func() {
			driverRegistry.Set(map[string]volman.Plugin{"not-a-dockerdriver": new(volmanfakes.FakePlugin)})
		})

		It("should succeed", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when there is a driver", func() {
		BeforeEach(func() {
			err := dockerdriver.WriteDriverSpec(logger, defaultPluginsDirectory, "fakedriver", "spec", []byte("http://0.0.0.0:8080"))
			Expect(err).NotTo(HaveOccurred())

			fakeDriverFactory = new(volmanfakes.FakeDockerDriverFactory)

			fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))

			scanInterval = 1 * time.Second

			dockerDriverDiscoverer = voldiscoverers.NewDockerDriverDiscovererWithDriverFactory(logger, driverRegistry, []string{defaultPluginsDirectory}, fakeDriverFactory)
			client = vollocal.NewLocalClient(logger, driverRegistry, nil, fakeClock)
			syncer := vollocal.NewSyncer(logger, driverRegistry, []volman.Discoverer{dockerDriverDiscoverer}, scanInterval, fakeClock)
			fakeDriver = new(dockerdriverfakes.FakeDriver)
			fakeDriverFactory.DockerDriverReturns(fakeDriver, nil)

			fakeDriver.ActivateReturns(dockerdriver.ActivateResponse{Implements: []string{"VolumeDriver"}})

			process = ginkgomon.Invoke(syncer.Runner())
		})

		AfterEach(func() {
			ginkgomon.Kill(process)
		})

		It("should succeed when there are no mounts", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there is a mount", func() {
			BeforeEach(func() {
				fakeDriver.ListReturns(dockerdriver.ListResponse{Volumes: []dockerdriver.VolumeInfo{
					{
						Name:       "a-volume",
						Mountpoint: "foo",
					},
				}})
			})

			It("should unmount the volume", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeDriver.UnmountCallCount()).To(Equal(1))
			})

			Context("when the unmount fails", func() {
				BeforeEach(func() {
					fakeDriver.UnmountReturns(dockerdriver.ErrorResponse{Err: "badness"})
				})

				It("should log but not fail", func() {
					Expect(err).NotTo(HaveOccurred())

					Expect(logger.TestSink.LogMessages()).To(ContainElement("mount-purger.purge-mounts.failed-unmounting-volume-mount a-volume"))
				})
			})
		})
	})
})
