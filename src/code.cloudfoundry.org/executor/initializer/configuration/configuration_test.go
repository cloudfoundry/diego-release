package configuration_test

import (
	"errors"
	"strings"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/executor/gardenhealth"
	"code.cloudfoundry.org/executor/guidgen/fakeguidgen"
	"code.cloudfoundry.org/executor/initializer/configuration"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("configuration", func() {
	var gardenClient *fakes.FakeGardenClient

	BeforeEach(func() {
		gardenClient = fakes.NewGardenClient()
	})

	Describe("ConfigureCapacity", func() {
		var (
			capacity               executor.ExecutorResources
			err                    error
			memLimit, diskLimit    string
			maxCacheSizeInBytes    uint64
			autoDiskMBOverhead     int
			useSchedulableDiskSize bool
		)

		BeforeEach(func() {
			maxCacheSizeInBytes = 0
			autoDiskMBOverhead = 0
			memLimit = ""
			diskLimit = ""
			useSchedulableDiskSize = false
		})

		JustBeforeEach(func() {
			capacity, err = configuration.ConfigureCapacity(gardenClient, memLimit, diskLimit, maxCacheSizeInBytes, autoDiskMBOverhead, useSchedulableDiskSize)
		})

		Context("when getting the capacity fails", func() {
			BeforeEach(func() {
				gardenClient.Connection.CapacityReturns(garden.Capacity{}, errors.New("uh oh"))
			})

			It("returns an error", func() {
				Expect(err).To(Equal(errors.New("uh oh")))
			})
		})

		Context("when getting the capacity succeeds", func() {
			BeforeEach(func() {
				memLimit = "99"
				diskLimit = "99"
				gardenClient.Connection.CapacityReturns(
					garden.Capacity{
						MemoryInBytes: 1024 * 1024 * 3,
						DiskInBytes:   1024 * 1024 * 4,
						MaxContainers: 5,
					},
					nil,
				)
			})

			Describe("Memory Limit", func() {
				Context("when the memory limit flag is 'auto'", func() {
					BeforeEach(func() {
						memLimit = "auto"
					})

					It("does not return an error", func() {
						Expect(err).NotTo(HaveOccurred())
					})

					It("uses the garden server's memory capacity", func() {
						Expect(capacity.MemoryMB).To(Equal(3))
					})
				})

				Context("when the memory limit flag is a positive number", func() {
					BeforeEach(func() {
						memLimit = "2"
					})

					It("does not return an error", func() {
						Expect(err).NotTo(HaveOccurred())
					})

					It("uses that number", func() {
						Expect(capacity.MemoryMB).To(Equal(2))
					})
				})

				Context("when the memory limit flag is not a number", func() {
					BeforeEach(func() {
						memLimit = "stuff"
					})

					It("returns an error", func() {
						Expect(err).To(Equal(configuration.ErrMemoryFlagInvalid))
					})
				})

				Context("when the memory limit flag is not positive", func() {
					BeforeEach(func() {
						memLimit = "0"
					})

					It("returns an error", func() {
						Expect(err).To(Equal(configuration.ErrMemoryFlagInvalid))
					})
				})
			})

			Describe("Disk Limit", func() {
				Context("when the disk limit flag is 'auto'", func() {
					BeforeEach(func() {
						diskLimit = "auto"
					})

					It("uses the garden server's disk capacity", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(capacity.DiskMB).To(Equal(4))
					})

					Context("when the max cache size in bytes is non zero", func() {
						BeforeEach(func() {
							maxCacheSizeInBytes = 1024 * 1024 * 2
						})

						Context("when useSchedulableDiskSize flag is false", func() {
							It("subtracts the cache size from the disk capacity", func() {
								Expect(capacity.DiskMB).To(Equal(2))
							})

							Context("when the max cache size in bytes is larger than the available disk capacity", func() {
								BeforeEach(func() {
									maxCacheSizeInBytes = 1024 * 1024 * 4
								})

								It("returns an error", func() {
									Expect(err).To(HaveOccurred())
								})
							})
						})

						Context("when the auto disk mb overhead property is set", func() {
							BeforeEach(func() {
								autoDiskMBOverhead = 1
							})

							It("subtracts the overhead to the disk capacity", func() {
								Expect(err).NotTo(HaveOccurred())
								Expect(capacity.DiskMB).To(Equal(1))
							})
						})

						Context("when useSchedulableDiskSize flag is true", func() {
							BeforeEach(func() {
								useSchedulableDiskSize = true
							})

							Context("when SchedulableDiskInBytes is 0", func() {
								BeforeEach(func() {
									gardenClient.Connection.CapacityReturns(
										garden.Capacity{
											MemoryInBytes:          1024 * 1024 * 3,
											DiskInBytes:            1024 * 1024 * 4,
											SchedulableDiskInBytes: 0,
											MaxContainers:          5,
										},
										nil,
									)
								})

								It("falls back to using DiskInBytes for backwards compatibility", func() {
									Expect(capacity.DiskMB).To(Equal(2))
								})
							})

							Context("when SchedulableDiskInBytes is greater than 0", func() {
								BeforeEach(func() {
									gardenClient.Connection.CapacityReturns(
										garden.Capacity{
											MemoryInBytes:          1024 * 1024 * 3,
											DiskInBytes:            1024 * 1024 * 4,
											SchedulableDiskInBytes: 1024 * 1024 * 5,
											MaxContainers:          5,
										},
										nil,
									)
								})

								It("uses SchedulableDiskInBytes value", func() {
									Expect(capacity.DiskMB).To(Equal(5))
								})

								Context("when the auto disk mb overhead property is set", func() {
									BeforeEach(func() {
										autoDiskMBOverhead = 1
									})

									It("subtracts the overhead to the disk capacity", func() {
										Expect(err).NotTo(HaveOccurred())
										Expect(capacity.DiskMB).To(Equal(4))
									})
								})
							})
						})
					})
				})

				Context("when the disk limit flag is a positive number", func() {
					BeforeEach(func() {
						diskLimit = "2"
					})

					It("does not return an error", func() {
						Expect(err).NotTo(HaveOccurred())
					})

					It("uses that number", func() {
						Expect(capacity.DiskMB).To(Equal(2))
					})
				})

				Context("when the disk limit flag is not a number", func() {
					BeforeEach(func() {
						diskLimit = "stuff"
					})

					It("returns an error", func() {
						Expect(err).To(Equal(configuration.ErrDiskFlagInvalid))
					})
				})

				Context("when the disk limit flag is not positive", func() {
					BeforeEach(func() {
						diskLimit = "0"
					})

					It("returns an error", func() {
						Expect(err).To(Equal(configuration.ErrDiskFlagInvalid))
					})
				})
			})

			Describe("Containers Limit", func() {
				It("uses the garden server's max containers", func() {
					Expect(capacity.Containers).To(Equal(4))
				})
			})
		})
	})

	Describe("GetRootFSSizes", func() {
		var (
			logger   lager.Logger
			rootFSes map[string]string

			fakeGuidGen  *fakeguidgen.FakeGenerator
			guidToRootFS map[string]string

			rootFSSizes  configuration.RootFSSizer
			getRootFSErr error
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("configuration")
			rootFSes = map[string]string{
				"rootFS1": "/rootFS1/path",
				"rootFS2": "/rootFS2/path",
			}

			fakeGuidGen = new(fakeguidgen.FakeGenerator)
			fakeGuidGen.GuidReturnsOnCall(0, "g1")
			fakeGuidGen.GuidReturnsOnCall(1, "g2")
			guidToRootFS = make(map[string]string)
		})

		JustBeforeEach(func() {
			rootFSSizes, getRootFSErr = configuration.GetRootFSSizes(logger, gardenClient, fakeGuidGen, "some-owner-name", rootFSes)
		})

		Context("when container with provided rootFS is created", func() {
			BeforeEach(func() {
				expectedMetrics := map[string]garden.ContainerMetricsEntry{
					"rootfs-c-g1": garden.ContainerMetricsEntry{Metrics: garden.Metrics{DiskStat: garden.ContainerDiskStat{TotalBytesUsed: uint64(150 * 1024 * 1024), ExclusiveBytesUsed: uint64(50 * 1024 * 1024)}}},
					"rootfs-c-g2": garden.ContainerMetricsEntry{Metrics: garden.Metrics{DiskStat: garden.ContainerDiskStat{TotalBytesUsed: uint64(32 * 1024 * 1024), ExclusiveBytesUsed: uint64(8*1024*1024 - 5)}}},
				}
				gardenClient.Connection.BulkMetricsReturnsOnCall(0, expectedMetrics, nil)
			})

			It("creates containers with provided rootfses", func() {
				Expect(getRootFSErr).NotTo(HaveOccurred())

				Expect(gardenClient.Connection.CreateCallCount()).To(Equal(2))
				spec1 := gardenClient.Connection.CreateArgsForCall(0)
				spec2 := gardenClient.Connection.CreateArgsForCall(1)
				Expect([]string{spec1.Image.URI, spec2.Image.URI}).To(ConsistOf("/rootFS1/path", "/rootFS2/path"))
				Expect([]string{spec1.Properties[gardenhealth.HealthcheckNetworkProperty], spec2.Properties[gardenhealth.HealthcheckNetworkProperty]}).To(ConsistOf("true", "true"))
			})

			It("returned sizer lets us query their size", func() {
				Expect(getRootFSErr).NotTo(HaveOccurred())

				Expect(gardenClient.Connection.CreateCallCount()).To(Equal(2))
				spec1 := gardenClient.Connection.CreateArgsForCall(0)
				spec2 := gardenClient.Connection.CreateArgsForCall(1)

				guidToRootFS[spec1.Handle] = spec1.Image.URI
				guidToRootFS[spec2.Handle] = spec2.Image.URI

				if strings.Contains(guidToRootFS["rootfs-c-g1"], "rootFS1") {
					Expect(rootFSSizes.RootFSSizeFromPath("/rootFS1/path")).To(BeEquivalentTo(100 * 1024 * 1024))
					Expect(rootFSSizes.RootFSSizeFromPath("/rootFS2/path")).To(BeEquivalentTo(24*1024*1024 + 5))
				}
				if strings.Contains(guidToRootFS["rootfs-c-g1"], "rootFS2") {
					Expect(rootFSSizes.RootFSSizeFromPath("/rootFS1/path")).To(BeEquivalentTo(24*1024*1024 + 5))
					Expect(rootFSSizes.RootFSSizeFromPath("/rootFS2/path")).To(BeEquivalentTo(100 * 1024 * 1024))
				}

				Expect(gardenClient.Connection.BulkMetricsCallCount()).To(Equal(1))
				Expect(gardenClient.Connection.BulkMetricsArgsForCall(0)).To(ConsistOf("rootfs-c-g1", "rootfs-c-g2"))

				Expect(gardenClient.Connection.DestroyCallCount()).To(Equal(2))
				c1 := gardenClient.Connection.DestroyArgsForCall(0)
				c2 := gardenClient.Connection.DestroyArgsForCall(1)
				Expect([]string{c1, c2}).To(ConsistOf("rootfs-c-g1", "rootfs-c-g2"))
			})

			Context("when attempting to retrieve the size of a non-preloaded rootfs", func() {
				It("returns a size of 0", func() {
					Expect(getRootFSErr).NotTo(HaveOccurred())
					Expect(rootFSSizes.RootFSSizeFromPath("/some/path/that/wasnt/preloaded")).To(BeEquivalentTo(0))
				})
			})

			Context("when the rootfs URI query string has been augmented with a query/scheme/host/etc.", func() {
				BeforeEach(func() {
					rootFSes = map[string]string{
						"rootFS1": "/rootFS1/path",
					}
				})

				It("correctly returns the size of the preloaded rootfs in question", func() {
					Expect(getRootFSErr).NotTo(HaveOccurred())
					Expect(rootFSSizes.RootFSSizeFromPath("preloaded+layer:/rootFS1/path?query=something")).To(BeEquivalentTo(100 * 1024 * 1024))
				})
			})

			Context("when attempting to retrieve the size from an invalid URI", func() {
				It("returns a size of 0", func() {
					Expect(getRootFSErr).NotTo(HaveOccurred())
					Expect(rootFSSizes.RootFSSizeFromPath("/some/path/that/is/%invalid")).To(BeEquivalentTo(0))
				})
			})

			Context("when a preloaded rootfs URI is not just a simple path", func() {
				BeforeEach(func() {
					rootFSes = map[string]string{
						"rootFS1": "somescheme:///rootFS1/path",
					}
				})

				It("querying the size of the rootfs succeeds as expected", func() {
					Expect(getRootFSErr).NotTo(HaveOccurred())
					Expect(rootFSSizes.RootFSSizeFromPath("somescheme:///rootFS1/path")).To(BeEquivalentTo(100 * 1024 * 1024))
				})
			})

			Context("when a preloaded rootfs URI is an invalid URI", func() {
				BeforeEach(func() {
					rootFSes = map[string]string{
						"rootFS1": "/some/path/that/is/%invalid",
					}
				})

				It("errors", func() {
					Expect(getRootFSErr).To(HaveOccurred())
				})
			})
		})

		Context("when creating the rootFS container(s) fail", func() {
			var expectedErr error
			BeforeEach(func() {
				expectedErr = errors.New("create-error")
				gardenClient.Connection.CreateReturns("", expectedErr)
			})

			It("returns the error", func() {
				Expect(getRootFSErr).To(Equal(expectedErr))
			})
		})

		Context("when deletion of the rootFS container(s) fail", func() {
			var expectedErr error
			BeforeEach(func() {
				expectedErr = errors.New("destroy-error")
				gardenClient.Connection.DestroyReturns(expectedErr)
			})

			It("logs the error", func() {
				Expect(getRootFSErr).To(Succeed())
				testLogger, _ := logger.(*lagertest.TestLogger)
				Eventually(testLogger.Buffer).Should(gbytes.Say("failed to delete container 'rootfs-c-g1'.*destroy-error"))
			})
		})

		Context("when bulkMetrics fail", func() {
			var expectedErr error
			BeforeEach(func() {
				expectedErr = errors.New("bulkmetrics-error")
				gardenClient.Connection.BulkMetricsReturns(nil, expectedErr)
			})

			It("returns the error", func() {
				Expect(getRootFSErr).To(Equal(expectedErr))
			})
		})
	})
})
