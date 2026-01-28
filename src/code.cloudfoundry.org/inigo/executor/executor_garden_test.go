package executor_test

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/gardenhealth"
	executorinit "code.cloudfoundry.org/executor/initializer"
	"code.cloudfoundry.org/executor/initializer/configuration"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/inigo/world"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/tedsuo/ifrit/grouper"
)

var _ = Describe("Executor/Garden", func() {
	const pruningInterval = durationjson.Duration(500 * time.Millisecond)
	var (
		executorClient          executor.Client
		process                 ifrit.Process
		runner                  ifrit.Runner
		gardenCapacity          garden.Capacity
		cachePath               string
		config                  executorinit.ExecutorConfig
		logger                  lager.Logger
		ownerName               string
		gardenHealthcheckRootFS string

		expectedDiskCapacityMB int
	)

	BeforeEach(func() {
		gardenHealthcheckRootFS = ""

		randomName, err := uuid.NewV4()
		Expect(err).NotTo(HaveOccurred())

		config = executorinit.ExecutorConfig{
			GardenNetwork:                      "unix",
			GardenAddr:                         "/tmp/garden.sock",
			MemoryMB:                           configuration.Automatic,
			DiskMB:                             configuration.Automatic,
			TempDir:                            "/tmp",
			ReservedExpirationTime:             durationjson.Duration(time.Minute),
			ContainerReapInterval:              durationjson.Duration(time.Minute),
			ContainerInodeLimit:                200000,
			ContainerMaxCpuShares:              0,
			CachePath:                          "/tmp/cache",
			EnableDeclarativeHealthcheck:       false,
			MaxCacheSizeInBytes:                10 * 1024 * 1024 * 1024,
			SkipCertVerify:                     false,
			HealthyMonitoringInterval:          durationjson.Duration(30 * time.Second),
			UnhealthyMonitoringInterval:        durationjson.Duration(500 * time.Millisecond),
			ContainerOwnerName:                 "executor",
			HealthCheckContainerOwnerName:      "executor-health-check",
			CreateWorkPoolSize:                 32,
			DeleteWorkPoolSize:                 32,
			ReadWorkPoolSize:                   64,
			MetricsWorkPoolSize:                8,
			HealthCheckWorkPoolSize:            64,
			MaxConcurrentDownloads:             5,
			GardenHealthcheckInterval:          durationjson.Duration(10 * time.Minute),
			GardenHealthcheckEmissionInterval:  durationjson.Duration(30 * time.Second),
			GardenHealthcheckTimeout:           durationjson.Duration(10 * time.Minute),
			GardenHealthcheckCommandRetryPause: durationjson.Duration(time.Second),
			GardenHealthcheckProcessArgs:       []string{},
			GardenHealthcheckProcessEnv:        []string{},
			GracefulShutdownInterval:           durationjson.Duration(10 * time.Second),
			ContainerMetricsReportInterval:     durationjson.Duration(15 * time.Second),
			EnvoyConfigRefreshDelay:            durationjson.Duration(time.Second),
			EnvoyDrainTimeout:                  durationjson.Duration(15 * time.Minute),
		}
		config.GardenNetwork = "tcp"
		config.GardenAddr = componentMaker.Addresses().Garden
		config.ReservedExpirationTime = pruningInterval
		config.ContainerReapInterval = pruningInterval
		config.HealthyMonitoringInterval = durationjson.Duration(time.Second)
		config.UnhealthyMonitoringInterval = durationjson.Duration(100 * time.Millisecond)
		config.GardenHealthcheckProcessPath = "/bin/sh"
		config.GardenHealthcheckProcessArgs = []string{"-c", "echo", "checking health"}
		config.GardenHealthcheckProcessUser = "vcap"
		config.MaxCacheSizeInBytes = 1 * 1024 * 1024
		config.EnableUnproxiedPortMappings = true

		cachePath = world.TempDir("executor-tmp")

		ownerName = "executor" + randomName.String()

		config.CachePath = cachePath
		config.ContainerOwnerName = ownerName
	})

	JustBeforeEach(func() {
		var err error

		logger = lagertest.NewTestLogger("test")
		var executorMembers grouper.Members
		metronClient, err := loggingclient.NewIngressClient(loggingclient.Config{})
		Expect(err).NotTo(HaveOccurred())

		rootFSes := map[string]string{"somestack": gardenHealthcheckRootFS}

		executorClient, _, executorMembers, err = executorinit.Initialize(logger, config, "", "", rootFSes, gardenHealthcheckRootFS, metronClient, clock.NewClock())
		Expect(err).NotTo(HaveOccurred())
		runner = grouper.NewParallel(os.Kill, executorMembers)

		gardenCapacity, err = gardenClient.Capacity()
		Expect(err).NotTo(HaveOccurred())

		expectedDiskCapacityMB = int((gardenCapacity.DiskInBytes - config.MaxCacheSizeInBytes) / 1024 / 1024)
	})

	AfterEach(func() {
		if process != nil {
			ginkgomon.Interrupt(process)
		}

		if executorClient != nil {
			executorClient.Cleanup(logger)
		}

		os.RemoveAll(cachePath)
	})

	generateGuid := func() string {
		id, err := uuid.NewV4()
		Expect(err).NotTo(HaveOccurred())

		return id.String()
	}

	allocNewContainer := func(container executor.Container) string {
		container.Guid = generateGuid()

		request := executor.NewAllocationRequest(container.Guid, &container.Resource, false, container.Tags)
		failures := executorClient.AllocateContainers(logger, "", []executor.AllocationRequest{request})
		Expect(failures).To(BeEmpty())

		return request.Guid
	}

	getContainer := func(guid string) executor.Container {
		container, err := executorClient.GetContainer(logger, guid)
		Expect(err).NotTo(HaveOccurred())

		return container
	}

	containerStatePoller := func(guid string) func() executor.State {
		return func() executor.State {
			return getContainer(guid).State
		}
	}

	containerEventPoller := func(eventSource executor.EventSource, event *executor.Event) func() executor.EventType {
		return func() executor.EventType {
			var err error
			*event, err = eventSource.Next()
			Expect(err).NotTo(HaveOccurred())
			return (*event).EventType()
		}
	}

	findGardenContainer := func(handle string) garden.Container {
		var container garden.Container

		Eventually(func() error {
			var err error

			container, err = gardenClient.Lookup(handle)
			return err
		}).ShouldNot(HaveOccurred())

		return container
	}

	removeHealthcheckContainers := func(containers []executor.Container) []executor.Container {
		newContainers := []executor.Container{}
		for i := range containers {
			if !containers[i].HasTags(executor.Tags{gardenhealth.HealthcheckTag: gardenhealth.HealthcheckTagValue}) {
				newContainers = append(newContainers, containers[i])
			}
		}

		return newContainers
	}

	Describe("Starting up", func() {
		BeforeEach(func() {
			os.RemoveAll(cachePath)
		})

		JustBeforeEach(func() {
			process = ginkgomon.Invoke(runner)
		})

		Context("when the cache directory doesn't exist", func() {
			It("creates a new cache directory", func() {
				Eventually(func() bool {
					dirInfo, err := os.Stat(cachePath)
					if err != nil {
						return false
					}

					return dirInfo.IsDir()
				}, 10*time.Second).Should(BeTrue())
			})
		})

		Context("when there are containers that are owned by the executor", func() {
			var container1, container2 garden.Container

			BeforeEach(func() {
				var err error

				container1, err = gardenClient.Create(garden.ContainerSpec{
					Properties: garden.Properties{
						"executor:owner": ownerName,
					},
				})
				Expect(err).NotTo(HaveOccurred())

				container2, err = gardenClient.Create(garden.ContainerSpec{
					Properties: garden.Properties{
						"executor:owner": ownerName,
					},
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("deletes those containers (and only those containers)", func() {
				Eventually(func() error {
					_, err := gardenClient.Lookup(container1.Handle())
					return err
				}).Should(HaveOccurred())

				Eventually(func() error {
					_, err := gardenClient.Lookup(container2.Handle())
					return err
				}).Should(HaveOccurred())
			})
		})
	})

	Describe("Failing start up", func() {
		Context("when a blank garden healthcheck RootFS is given", func() {
			BeforeEach(func() {
				gardenHealthcheckRootFS = ""
			})

			Context("when garden is started with a default RootFS", func() {
				It("starts without an error", func() {
					process = ifrit.Background(runner)
					processExit := process.Wait()

					Consistently(processExit).ShouldNot(Receive())
				})
			})
		})
	})

	Describe("Running", func() {
		JustBeforeEach(func() {
			process = ginkgomon.Invoke(runner)
		})

		Describe("Checking garden's health", func() {
			Context("container creation succeeds", func() {
				It("reports healthy", func() {
					Expect(executorClient.Healthy(logger)).Should(BeTrue())
				})
			})

			Context("garden fails then resolves the problem", func() {
				BeforeEach(func() {
					config.GardenHealthcheckInterval = durationjson.Duration(5 * time.Millisecond)
				})

				It("reports correctly", func() {
					Expect(executorClient.Healthy(logger)).Should(BeTrue())

					isHealthy := func() bool { return executorClient.Healthy(logger) }

					ginkgomon.Interrupt(gardenProcess)
					Eventually(isHealthy).Should(BeFalse())

					gardenProcess = ginkgomon.Invoke(componentMaker.Garden())
					Eventually(isHealthy).Should(BeTrue())
				})
			})
		})

		Describe("pinging the server", func() {
			var pingErr error

			Context("when Garden responds to ping", func() {
				JustBeforeEach(func() {
					pingErr = executorClient.Ping(logger)
				})

				It("does not return an error", func() {
					Expect(pingErr).NotTo(HaveOccurred())
				})
			})

			Context("when Garden returns an error", func() {
				JustBeforeEach(func() {
					ginkgomon.Interrupt(gardenProcess)
					pingErr = executorClient.Ping(logger)
				})

				AfterEach(func() {
					gardenProcess = ginkgomon.Invoke(componentMaker.Garden())
				})

				It("should return an error", func() {
					Expect(pingErr).To(HaveOccurred())
					Expect(pingErr.Error()).To(ContainSubstring("connection refused"))
				})
			})
		})

		Describe("getting the total resources", func() {
			var resources executor.ExecutorResources
			var resourceErr error

			JustBeforeEach(func() {
				resources, resourceErr = executorClient.TotalResources(logger)
			})

			It("not return an error", func() {
				Expect(resourceErr).NotTo(HaveOccurred())
			})

			It("returns the preset capacity", func() {
				expectedResources := executor.ExecutorResources{
					MemoryMB:   int(gardenCapacity.MemoryInBytes / 1024 / 1024),
					DiskMB:     expectedDiskCapacityMB,
					Containers: int(gardenCapacity.MaxContainers) - 1,
				}
				Expect(resources).To(Equal(expectedResources))
			})
		})

		Describe("allocating a container", func() {
			var (
				allocationRequest executor.AllocationRequest

				guid string

				allocationFailures []executor.AllocationFailure
			)

			BeforeEach(func() {
				guid = generateGuid()
				tags := executor.Tags{"some-tag": "some-value"}
				allocationRequest = executor.NewAllocationRequest(guid, &executor.Resource{}, false, tags)
			})

			JustBeforeEach(func() {
				allocationFailures = executorClient.AllocateContainers(logger, "", []executor.AllocationRequest{allocationRequest})
			})

			It("returns an empty error map", func() {
				Expect(allocationFailures).To(BeEmpty())
			})

			It("shows up in the container list", func() {
				containers, err := executorClient.ListContainers(logger)
				Expect(err).NotTo(HaveOccurred())
				containers = removeHealthcheckContainers(containers)

				Expect(containers).To(HaveLen(1))

				Expect(containers[0].State).To(Equal(executor.StateReserved))
				Expect(containers[0].Guid).To(Equal(guid))
				Expect(containers[0].MemoryMB).To(Equal(0))
				Expect(containers[0].DiskMB).To(Equal(0))
				Expect(containers[0].Tags).To(Equal(executor.Tags{"some-tag": "some-value"}))
				Expect(containers[0].State).To(Equal(executor.StateReserved))
				Expect(containers[0].AllocatedAt).To(BeNumerically("~", time.Now().UnixNano(), time.Second))
			})

			Context("when allocated with memory and disk limits", func() {
				BeforeEach(func() {
					allocationRequest.Resource.MemoryMB = 256
					allocationRequest.Resource.DiskMB = 256
				})

				It("returns the limits on the container", func() {
					containers, err := executorClient.ListContainers(logger)
					Expect(err).NotTo(HaveOccurred())
					containers = removeHealthcheckContainers(containers)

					Expect(containers).To(HaveLen(1))
					Expect(containers[0].MemoryMB).To(Equal(256))
					Expect(containers[0].DiskMB).To(Equal(256))
				})

				It("reduces the capacity by the amount reserved", func() {
					Expect(executorClient.RemainingResources(logger)).To(Equal(executor.ExecutorResources{
						MemoryMB:   int(gardenCapacity.MemoryInBytes/1024/1024) - 256,
						DiskMB:     expectedDiskCapacityMB - 256,
						Containers: int(gardenCapacity.MaxContainers) - 2,
					}))
				})
			})

			Context("when the guid is already taken", func() {
				JustBeforeEach(func() {
					allocationFailures = executorClient.AllocateContainers(logger, "", []executor.AllocationRequest{allocationRequest})
				})

				It("returns an error", func() {
					Expect(allocationFailures).To(HaveLen(1))
					Expect(allocationFailures[0].Error()).To(Equal(executor.ErrContainerGuidNotAvailable.Error()))
				})
			})

			Context("when a guid is not specified", func() {
				BeforeEach(func() {
					allocationRequest.Guid = ""
				})

				It("returns an error", func() {
					Expect(allocationFailures).To(HaveLen(1))
					Expect(allocationFailures[0].Error()).To(Equal(executor.ErrGuidNotSpecified.Error()))
				})
			})

			Context("when there is no room", func() {
				BeforeEach(func() {
					allocationRequest.Resource.MemoryMB = 999999999999999
					allocationRequest.Resource.DiskMB = 999999999999999
				})

				It("returns an error", func() {
					Expect(allocationFailures).To(HaveLen(1))
					Expect(allocationFailures[0].Error()).To(Equal(executor.ErrInsufficientResourcesAvailable.Error()))
				})
			})

			Describe("running it", func() {
				var runErr error
				var runReq executor.RunRequest
				var runInfo executor.RunInfo
				var eventSource executor.EventSource

				BeforeEach(func() {
					runInfo = executor.RunInfo{
						Env: []executor.EnvironmentVariable{
							{Name: "ENV1", Value: "val1"},
							{Name: "ENV2", Value: "val2"},
						},

						Action: models.WrapAction(&models.RunAction{
							Path: "true",
							User: "vcap",
							Env: []*models.EnvironmentVariable{
								{Name: "RUN_ENV1", Value: "run_val1"},
								{Name: "RUN_ENV2", Value: "run_val2"},
							},
						}),
					}
				})

				JustBeforeEach(func() {
					var err error

					eventSource, err = executorClient.SubscribeToEvents(logger)
					Expect(err).NotTo(HaveOccurred())

					runReq = executor.NewRunRequest(guid, &runInfo, executor.Tags{})
					runErr = executorClient.RunContainer(logger, "", &runReq)
				})

				AfterEach(func() {
					eventSource.Close()
				})

				Context("when the container can be created", func() {
					var gardenContainer garden.Container

					JustBeforeEach(func() {
						gardenContainer = findGardenContainer(guid)
					})

					It("returns no error", func() {
						Expect(runErr).NotTo(HaveOccurred())
					})

					It("creates it with the configured owner", func() {
						info, err := gardenContainer.Info()
						Expect(err).NotTo(HaveOccurred())

						Expect(info.Properties["executor:owner"]).To(Equal(ownerName))
					})

					It("sets global environment variables on the container", func() {
						output := gbytes.NewBuffer()

						process, err := gardenContainer.Run(garden.ProcessSpec{
							Path: "env",
							User: "vcap",
						}, garden.ProcessIO{
							Stdout: output,
						})
						Expect(err).NotTo(HaveOccurred())
						Expect(process.Wait()).To(Equal(0))

						Expect(output.Contents()).To(ContainSubstring("ENV1=val1"))
						Expect(output.Contents()).To(ContainSubstring("ENV2=val2"))
					})

					It("saves the succeeded run result", func() {
						Eventually(containerStatePoller(guid)).Should(Equal(executor.StateCompleted))

						container := getContainer(guid)
						Expect(container.RunResult.Failed).To(BeFalse())
						Expect(container.RunResult.FailureReason).To(BeEmpty())
					})

					Context("when created without a monitor action", func() {
						BeforeEach(func() {
							runInfo.Action = models.WrapAction(&models.RunAction{
								Path: "sh",
								User: "vcap",
								Args: []string{"-c", "while true; do sleep 1; done"},
							})
						})

						It("reports the state as 'running'", func() {
							Eventually(containerStatePoller(guid)).Should(Equal(executor.StateRunning))
							Consistently(containerStatePoller(guid)).Should(Equal(executor.StateRunning))
						})
					})

					Context("when created with a monitor action", func() {
						itFailsOnlyIfMonitoringSucceedsAndThenFails := func() {
							Context("when monitoring succeeds", func() {
								BeforeEach(func() {
									runInfo.Monitor = models.WrapAction(&models.RunAction{
										User: "vcap",
										Path: "true",
									})
								})

								It("emits a running container event", func() {
									var event executor.Event
									Eventually(containerEventPoller(eventSource, &event), 5).Should(Equal(executor.EventTypeContainerRunning))
								})

								It("reports the state as 'running'", func() {
									Eventually(containerStatePoller(guid)).Should(Equal(executor.StateRunning))
									Consistently(containerStatePoller(guid)).Should(Equal(executor.StateRunning))
								})

								It("does not stop the container", func() {
									Consistently(containerStatePoller(guid)).ShouldNot(Equal(executor.StateCompleted))
								})
							})

							Context("when monitoring persistently fails", func() {
								BeforeEach(func() {
									runInfo.Monitor = models.WrapAction(&models.RunAction{
										User: "vcap",
										Path: "false",
									})
								})

								It("reports the state as 'created'", func() {
									Eventually(containerStatePoller(guid)).Should(Equal(executor.StateCreated))
									Consistently(containerStatePoller(guid)).Should(Equal(executor.StateCreated))
								})
							})

							Context("when monitoring succeeds and then fails", func() {
								BeforeEach(func() {
									runInfo.Monitor = models.WrapAction(&models.RunAction{
										User: "vcap",
										Path: "sh",
										Args: []string{
											"-c",
											`
													if [ -f already_ran ]; then
														exit 1
													else
														touch already_ran
													fi
												`,
										},
									})
								})

								It("reports the container as 'running' and then as 'completed'", func() {
									Eventually(containerStatePoller(guid)).Should(Equal(executor.StateRunning))
									Eventually(containerStatePoller(guid)).Should(Equal(executor.StateCompleted))
								})
							})
						}

						Context("when the action succeeds and exits immediately (daemonization)", func() {
							BeforeEach(func() {
								runInfo.Action = models.WrapAction(&models.RunAction{
									Path: "true",
									User: "vcap",
								})
							})

							itFailsOnlyIfMonitoringSucceedsAndThenFails()
						})

						Context("while the action does not stop running", func() {
							BeforeEach(func() {
								runInfo.Action = models.WrapAction(&models.RunAction{
									Path: "sh",
									User: "vcap",
									Args: []string{"-c", "while true; do sleep 1; done"},
								})
							})

							itFailsOnlyIfMonitoringSucceedsAndThenFails()
						})

						Context("when the action fails", func() {
							BeforeEach(func() {
								runInfo.Action = models.WrapAction(&models.RunAction{
									User: "vcap",
									Path: "false",
								})
							})

							Context("even if the monitoring succeeds", func() {
								BeforeEach(func() {
									runInfo.Monitor = models.WrapAction(&models.RunAction{
										User: "vcap",
										Path: "true",
									})
								})

								It("stops the container", func() {
									Eventually(containerStatePoller(guid)).Should(Equal(executor.StateCompleted))
								})
							})
						})
					})

					Context("after running succeeds", func() {
						Describe("deleting the container", func() {
							It("works", func(ctx context.Context) {
								done := make(chan interface{})
								defer GinkgoRecover()
								go func() {
									Eventually(containerStatePoller(guid)).Should(Equal(executor.StateCompleted))

									err := executorClient.DeleteContainer(logger, "", guid)
									Expect(err).NotTo(HaveOccurred())
									close(done)
								}()
								Eventually(done, 5*time.Second).Should(BeClosed())
							})
						})
					})

					Context("when running fails", func() {
						BeforeEach(func() {
							runInfo.Action = models.WrapAction(&models.RunAction{
								User: "vcap",
								Path: "false",
							})
						})

						It("saves the failed result and reason", func() {
							Eventually(containerStatePoller(guid)).Should(Equal(executor.StateCompleted))

							container := getContainer(guid)
							Expect(container.RunResult.Failed).To(BeTrue())
							Expect(container.RunResult.FailureReason).To(ContainSubstring("Exited with status 1"))
						})

						Context("when listening for events", func() {
							It("emits a completed container event", func() {
								var event executor.Event
								Eventually(containerEventPoller(eventSource, &event), 5).Should(Equal(executor.EventTypeContainerComplete))

								completeEvent := event.(executor.ContainerCompleteEvent)
								Expect(completeEvent.Container().State).To(Equal(executor.StateCompleted))
								Expect(completeEvent.Container().RunResult.Failed).To(BeTrue())
								Expect(completeEvent.Container().RunResult.FailureReason).To(ContainSubstring("Exited with status 1"))
							})
						})
					})
				})

				Context("when the container cannot be created", func() {
					BeforeEach(func() {
						runInfo.RootFSPath = "gopher://example.com"
					})

					It("does not immediately return an error", func() {
						Expect(runErr).NotTo(HaveOccurred())
					})

					Context("when listening for events", func() {
						It("eventually completes with failure", func() {
							Eventually(containerStatePoller(guid)).Should(Equal(executor.StateCompleted))

							container := getContainer(guid)
							Expect(container.RunResult.Failed).To(BeTrue())
							Expect(container.RunResult.FailureReason).To(ContainSubstring("failed to create container"))
						})
					})
				})
			})
		})

		Describe("running a bogus guid", func() {
			It("returns an error", func() {
				runReq := executor.NewRunRequest("bogus", &executor.RunInfo{}, executor.Tags{})
				err := executorClient.RunContainer(logger, "", &runReq)
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})

		Context("when the container has been allocated", func() {
			var guid string

			JustBeforeEach(func() {
				guid = allocNewContainer(executor.Container{
					Resource: executor.Resource{
						MemoryMB: 1024,
						DiskMB:   1024,
					},
				})
			})

			Describe("deleting it", func() {
				It("makes the previously allocated resources available again", func() {
					err := executorClient.DeleteContainer(logger, "", guid)
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() (executor.ExecutorResources, error) { return executorClient.RemainingResources(logger) }).Should(Equal(executor.ExecutorResources{
						MemoryMB:   int(gardenCapacity.MemoryInBytes / 1024 / 1024),
						DiskMB:     expectedDiskCapacityMB,
						Containers: int(gardenCapacity.MaxContainers) - 1,
					}))
				})
			})

			Describe("listing containers", func() {
				It("shows up in the container list in reserved state", func() {
					containers, err := executorClient.ListContainers(logger)
					Expect(err).NotTo(HaveOccurred())
					containers = removeHealthcheckContainers(containers)

					Expect(containers).To(HaveLen(1))
					Expect(containers[0].Guid).To(Equal(guid))
					Expect(containers[0].State).To(Equal(executor.StateReserved))
				})
			})
		})

		Context("while it is running", func() {
			var guid string

			JustBeforeEach(func() {
				guid = allocNewContainer(executor.Container{
					Resource: executor.Resource{
						MemoryMB: 64,
						DiskMB:   64,
					},
				})

				runRequest := executor.NewRunRequest(guid, &executor.RunInfo{
					Action: models.WrapAction(&models.RunAction{
						Path: "sh",
						User: "vcap",
						Args: []string{"-c", "while true; do sleep 1; done"},
					}),
				}, executor.Tags{})

				err := executorClient.RunContainer(logger, "", &runRequest)
				Expect(err).NotTo(HaveOccurred())

				Eventually(containerStatePoller(guid)).Should(Equal(executor.StateRunning))
			})

			Describe("StopContainer", func() {
				It("does not return an error", func() {
					err := executorClient.StopContainer(logger, "", guid)
					Expect(err).NotTo(HaveOccurred())
				})

				It("stops the container but does not delete it", func() {
					err := executorClient.StopContainer(logger, "", guid)
					Expect(err).NotTo(HaveOccurred())

					var container executor.Container
					Eventually(func() executor.State {
						container, err = executorClient.GetContainer(logger, guid)
						Expect(err).NotTo(HaveOccurred())
						return container.State
					}).Should(Equal(executor.StateCompleted))

					Expect(container.RunResult.Stopped).To(BeTrue())

					_, err = gardenClient.Lookup(guid)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Describe("DeleteContainer", func() {
				It("deletes the container", func() {
					err := executorClient.DeleteContainer(logger, "", guid)
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() error {
						_, err := gardenClient.Lookup(guid)
						return err
					}).Should(HaveOccurred())
				})
			})

			Describe("listing containers", func() {
				It("shows up in the container list in running state", func() {
					containers, err := executorClient.ListContainers(logger)
					Expect(err).NotTo(HaveOccurred())
					containers = removeHealthcheckContainers(containers)

					Expect(containers).To(HaveLen(1))
					Expect(containers[0].Guid).To(Equal(guid))
					Expect(containers[0].State).To(Equal(executor.StateRunning))
				})
			})

			Describe("remaining resources", func() {
				It("has the container's reservation subtracted", func() {
					remaining, err := executorClient.RemainingResources(logger)
					Expect(err).NotTo(HaveOccurred())

					Expect(remaining.MemoryMB).To(Equal(int(gardenCapacity.MemoryInBytes/1024/1024) - 64))
					Expect(remaining.DiskMB).To(Equal(expectedDiskCapacityMB - 64))
				})

				Context("when the container disappears", func() {
					It("eventually goes back to the total resources", func() {
						// wait for the container to be present
						findGardenContainer(guid)

						// delete it
						err := executorClient.DeleteContainer(logger, "", guid)
						Expect(err).NotTo(HaveOccurred())

						Eventually(func() (executor.ExecutorResources, error) { return executorClient.RemainingResources(logger) }).Should(
							Equal(executor.ExecutorResources{
								MemoryMB:   int(gardenCapacity.MemoryInBytes / 1024 / 1024),
								DiskMB:     expectedDiskCapacityMB,
								Containers: int(gardenCapacity.MaxContainers) - 1,
							}))
					})
				})
			})

			Describe("removing the container from garden", func() {
				It("transistions the container into the completed state", func() {
					findGardenContainer(guid)

					// delete it
					err := gardenClient.Destroy(guid)
					Expect(err).NotTo(HaveOccurred())

					Eventually(containerStatePoller(guid)).Should(Equal(executor.StateCompleted))
					container := getContainer(guid)
					Expect(container.RunResult.Failed).To(BeTrue())
				})
			})
		})

		Describe("getting files from a container", func() {
			var (
				guid string

				stream    io.ReadCloser
				streamErr error
			)

			Context("when the container hasn't been initialized", func() {
				JustBeforeEach(func() {
					guid = allocNewContainer(executor.Container{
						Resource: executor.Resource{
							MemoryMB: 1024,
							DiskMB:   1024,
						},
					})

					stream, streamErr = executorClient.GetFiles(logger, guid, "some/path")
				})

				It("returns an error", func() {
					Expect(streamErr).To(HaveOccurred())
				})
			})

			Context("when the container is running", func() {
				var container garden.Container

				JustBeforeEach(func() {
					guid = allocNewContainer(executor.Container{})

					runRequest := executor.NewRunRequest(guid, &executor.RunInfo{
						Action: models.WrapAction(&models.RunAction{
							Path: "sh",
							User: "vcap",
							Args: []string{
								"-c", `while true; do	sleep 1; done`,
							},
						}),
					}, executor.Tags{})

					err := executorClient.RunContainer(logger, "", &runRequest)
					Expect(err).NotTo(HaveOccurred())

					container = findGardenContainer(guid)

					process, err := container.Run(garden.ProcessSpec{
						Path: "sh",
						Args: []string{"-c", "mkdir some; echo hello > some/path"},
						User: "vcap",
					}, garden.ProcessIO{})
					Expect(err).NotTo(HaveOccurred())
					Expect(process.Wait()).To(Equal(0))

					stream, streamErr = executorClient.GetFiles(logger, guid, "/home/vcap/some/path")
				})

				It("does not error", func() {
					Expect(streamErr).NotTo(HaveOccurred())
				})

				It("returns a stream of the contents of the file", func() {
					tarReader := tar.NewReader(stream)

					header, err := tarReader.Next()
					Expect(err).NotTo(HaveOccurred())

					Expect(header.FileInfo().Name()).To(Equal("path"))
					Expect(io.ReadAll(tarReader)).To(Equal([]byte("hello\n")))
				})
			})
		})

		Describe("pruning the registry", func() {
			It("continously prunes the registry", func() {
				failures := executorClient.AllocateContainers(logger, "", []executor.AllocationRequest{
					{
						Guid: "some-handle",
						Resource: executor.Resource{
							MemoryMB: 1024,
							DiskMB:   1024,
						},
					},
				})
				Expect(failures).To(BeEmpty())

				containers, err := executorClient.ListContainers(logger)
				Expect(err).NotTo(HaveOccurred())
				containers = removeHealthcheckContainers(containers)
				Expect(containers).To(HaveLen(1))

				Eventually(func() executor.State {
					container, err := executorClient.GetContainer(logger, "some-handle")
					Expect(err).NotTo(HaveOccurred())

					return container.State
				}, pruningInterval*3).Should(Equal(executor.StateCompleted))
			})
		})

		Describe("listing containers", func() {
			Context("with no containers", func() {
				It("returns an empty set of containers", func() {
					containers, err := executorClient.ListContainers(logger)
					Expect(err).NotTo(HaveOccurred())
					containers = removeHealthcheckContainers(containers)
					Expect(containers).To(BeEmpty())
				})
			})

			Context("when a container has been allocated", func() {
				var (
					container executor.Container

					guid string
				)

				JustBeforeEach(func() {
					guid = allocNewContainer(container)
				})

				It("includes the allocated container", func() {
					containers, err := executorClient.ListContainers(logger)
					Expect(err).NotTo(HaveOccurred())
					containers = removeHealthcheckContainers(containers)
					Expect(containers).To(HaveLen(1))
					Expect(containers[0].Guid).To(Equal(guid))
				})
			})
		})

		Describe("container networking", func() {
			Context("when a container listens on the local end of CF_INSTANCE_ADDR", func() {
				var guid string
				var containerResponse []byte
				var externalAddr string

				JustBeforeEach(func() {
					guid = allocNewContainer(executor.Container{})
					runRequest := executor.NewRunRequest(guid, &executor.RunInfo{
						Ports: []executor.PortMapping{
							{ContainerPort: 8080},
						},

						Action: models.WrapAction(&models.RunAction{
							User: "vcap",
							Path: "sh",
							Args: []string{"-c", "echo -n .$CF_INSTANCE_ADDR. | nc -l -N -p 8080"},
						}),
					}, executor.Tags{})

					err := executorClient.RunContainer(logger, "", &runRequest)
					Expect(err).NotTo(HaveOccurred())

					Eventually(containerStatePoller(guid)).Should(Equal(executor.StateRunning))

					container := getContainer(guid)

					externalAddr = fmt.Sprintf("%s:%d", container.ExternalIP, container.Ports[0].HostPort)

					var conn net.Conn
					Eventually(func() error {
						var err error
						dialer := net.Dialer{Timeout: time.Second * 5}
						conn, err = dialer.Dial("tcp", externalAddr)
						return err
					}).ShouldNot(HaveOccurred())

					containerResponse, err = io.ReadAll(conn)
					Expect(err).NotTo(HaveOccurred())
				})

				It("echoes back the correct CF_INSTANCE_ADDR", func() {
					Expect(string(containerResponse)).To(Equal("." + externalAddr + "."))
				})
			})
		})
	})
})
