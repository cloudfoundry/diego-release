package voldocker_test

import (
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/dockerdriver"
	"code.cloudfoundry.org/dockerdriver/dockerdriverfakes"
	"code.cloudfoundry.org/volman/voldocker"

	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/volman"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("DockerDriverMounter", func() {
	var (
		volumeId         string
		logger           *lagertest.TestLogger
		dockerPlugin     volman.Plugin
		fakeDockerDriver *dockerdriverfakes.FakeDriver
	)

	BeforeEach(func() {
		volumeId = "fake-volume"
		logger = lagertest.NewTestLogger("docker-mounter-test")
		fakeDockerDriver = &dockerdriverfakes.FakeDriver{}
		dockerPlugin = voldocker.NewVolmanPluginWithDockerDriver(fakeDockerDriver, volman.PluginSpec{})
	})

	Describe("Mount", func() {
		Context("when given a driver", func() {

			Context("mount", func() {

				BeforeEach(func() {
					mountResponse := dockerdriver.MountResponse{Mountpoint: "/var/vcap/data/mounts/" + volumeId}
					fakeDockerDriver.MountReturns(mountResponse)
				})

				It("should be able to mount without warning", func() {
					mountPath, err := dockerPlugin.Mount(logger, volumeId, map[string]interface{}{"volume_id": volumeId})
					Expect(err).NotTo(HaveOccurred())
					Expect(mountPath).NotTo(Equal(""))
					Expect(logger.Buffer()).NotTo(gbytes.Say("Invalid or dangerous mountpath"))
				})

				It("should not be able to mount if mount fails", func() {
					mountResponse := dockerdriver.MountResponse{Err: "an error"}
					fakeDockerDriver.MountReturns(mountResponse)
					_, err := dockerPlugin.Mount(logger, volumeId, map[string]interface{}{"volume_id": volumeId})
					Expect(err).To(HaveOccurred())
				})

				Context("with bad mount path", func() {
					var err error
					BeforeEach(func() {
						mountResponse := dockerdriver.MountResponse{Mountpoint: "/var/tmp"}
						fakeDockerDriver.MountReturns(mountResponse)
					})

					JustBeforeEach(func() {
						_, err = dockerPlugin.Mount(logger, volumeId, map[string]interface{}{"volume_id": volumeId})
					})

					It("should return a warning in the log", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(logger.Buffer()).To(gbytes.Say("Invalid or dangerous mountpath"))
					})
				})

				Context("with safe error", func() {
					var (
						err         error
						safeError   dockerdriver.SafeError
						unsafeError error
						errString   string
					)

					JustBeforeEach(func() {

						mountResponse := dockerdriver.MountResponse{Err: errString}
						fakeDockerDriver.MountReturns(mountResponse)
						_, err = dockerPlugin.Mount(logger, volumeId, map[string]interface{}{"volume_id": volumeId})
					})

					Context("with safe error msg", func() {
						BeforeEach(func() {
							safeError = dockerdriver.SafeError{SafeDescription: "safe-badness"}
							errBytes, err := json.Marshal(safeError)
							Expect(err).NotTo(HaveOccurred())
							errString = string(errBytes[:])
						})

						It("should return a safe error", func() {
							Expect(err).To(HaveOccurred())
							_, ok := err.(dockerdriver.SafeError)
							Expect(ok).To(Equal(true))
							Expect(err.Error()).To(Equal("safe-badness"))
						})
					})

					Context("with unsafe error msg", func() {
						BeforeEach(func() {
							unsafeError = errors.New("unsafe-badness")
							errString = unsafeError.Error()
						})

						It("should return regular error", func() {
							Expect(err).To(HaveOccurred())
							_, ok := err.(dockerdriver.SafeError)
							Expect(ok).To(Equal(false))
							Expect(err.Error()).To(Equal("unsafe-badness"))
						})

					})

					Context("with a really unsafe error msg", func() {
						BeforeEach(func() {
							errString = "{ badness"
						})

						It("should return regular error", func() {
							Expect(err).To(HaveOccurred())
							_, ok := err.(dockerdriver.SafeError)
							Expect(ok).To(Equal(false))
							Expect(err.Error()).To(Equal("{ badness"))
						})
					})
				})

			})
		})
	})

	Describe("Unmount", func() {
		It("should be able to unmount", func() {
			err := dockerPlugin.Unmount(logger, volumeId)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeDockerDriver.UnmountCallCount()).To(Equal(1))
			Expect(fakeDockerDriver.RemoveCallCount()).To(Equal(0))
		})

		It("should not be able to unmount when driver unmount fails", func() {
			fakeDockerDriver.UnmountReturns(dockerdriver.ErrorResponse{Err: "unmount failure"})
			err := dockerPlugin.Unmount(logger, volumeId)
			Expect(err).To(HaveOccurred())
		})

		Context("with safe error", func() {
			var (
				err         error
				safeError   dockerdriver.SafeError
				unsafeError error
				errString   string
			)

			JustBeforeEach(func() {
				fakeDockerDriver.UnmountReturns(dockerdriver.ErrorResponse{Err: errString})
				err = dockerPlugin.Unmount(logger, volumeId)
			})

			Context("with safe error msg", func() {
				BeforeEach(func() {
					safeError = dockerdriver.SafeError{SafeDescription: "safe-badness"}
					errBytes, err := json.Marshal(safeError)
					Expect(err).NotTo(HaveOccurred())
					errString = string(errBytes[:])
				})

				It("should return a safe error", func() {
					Expect(err).To(HaveOccurred())
					_, ok := err.(dockerdriver.SafeError)
					Expect(ok).To(Equal(true))
					Expect(err.Error()).To(Equal("safe-badness"))
				})
			})

			Context("with unsafe error msg", func() {
				BeforeEach(func() {
					unsafeError = errors.New("unsafe-badness")
					errString = unsafeError.Error()
				})

				It("should return regular error", func() {
					Expect(err).To(HaveOccurred())
					_, ok := err.(dockerdriver.SafeError)
					Expect(ok).To(Equal(false))
					Expect(err.Error()).To(Equal("unsafe-badness"))
				})

			})

			Context("with a really unsafe error msg", func() {
				BeforeEach(func() {
					errString = "{ badness"
				})

				It("should return regular error", func() {
					Expect(err).To(HaveOccurred())
					_, ok := err.(dockerdriver.SafeError)
					Expect(ok).To(Equal(false))
					Expect(err.Error()).To(Equal("{ badness"))
				})
			})
		})

	})

	Describe("ListVolumes", func() {
		var (
			volumes []string
			err     error
		)
		BeforeEach(func() {
			listResponse := dockerdriver.ListResponse{Volumes: []dockerdriver.VolumeInfo{
				{Name: "fake_volume_1"},
				{Name: "fake_volume_2"},
			}}
			fakeDockerDriver.ListReturns(listResponse)
		})

		JustBeforeEach(func() {
			volumes, err = dockerPlugin.ListVolumes(logger)
		})

		It("should be able list volumes", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(volumes).To(ContainElement("fake_volume_1"))
			Expect(volumes).To(ContainElement("fake_volume_2"))
		})

		Context("when the driver returns an err response", func() {
			BeforeEach(func() {
				listResponse := dockerdriver.ListResponse{Volumes: []dockerdriver.VolumeInfo{
					{Name: "fake_volume_1"},
					{Name: "fake_volume_2"},
				}, Err: "badness"}
				fakeDockerDriver.ListReturns(listResponse)
			})
			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

})
