package containerstore_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/containerstore"
	"code.cloudfoundry.org/executor/depot/containerstore/containerstorefakes"
	eventfakes "code.cloudfoundry.org/executor/depot/event/fakes"
	"code.cloudfoundry.org/executor/depot/log_streamer/fake_log_streamer"
	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/executor/depot/transformer/faketransformer"
	"code.cloudfoundry.org/executor/initializer/configuration/configurationfakes"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/garden/server"
	loggregator "code.cloudfoundry.org/go-loggregator/v9"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/volman"
	"code.cloudfoundry.org/volman/volmanfakes"
	multierror "github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	fake_runner "github.com/tedsuo/ifrit/fake_runner_v2"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/tedsuo/ifrit/grouper"
)

var _ = Describe("Container Store", func() {
	const cellID = "93ae1d34-4f95-4fb1-a283-4bbccac4d0f2"
	const logRateUnlimitedBytesPerSecond = -1

	var (
		containerConfig containerstore.ContainerConfig
		containerStore  containerstore.ContainerStore

		iNodeLimit                            uint64
		maxCPUShares                          uint64
		ownerName                             string
		totalCapacity                         executor.ExecutorResources
		advertisePreferenceForInstanceAddress bool

		containerGuid string

		metricMap     map[string]struct{}
		metricMapLock sync.RWMutex

		gardenClient        *gardenfakes.FakeClient
		gardenClientFactory *containerstorefakes.FakeGardenClientFactory
		gardenContainer     *gardenfakes.FakeContainer
		megatron            *faketransformer.FakeTransformer
		dependencyManager   *containerstorefakes.FakeDependencyManager
		credManager         *containerstorefakes.FakeCredManager
		proxyManager        *containerstorefakes.FakeProxyManager
		volumeManager       *volmanfakes.FakeManager
		logManager          *containerstorefakes.FakeLogManager
		logStreamer         *fake_log_streamer.FakeLogStreamer

		clock                     *fakeclock.FakeClock
		eventEmitter              *eventfakes.FakeHub
		metronClient              *mfakes.FakeIngressClient
		rootFSSizer               *configurationfakes.FakeRootFSSizer
		volumeMountedFilesHandler *containerstorefakes.FakeVolumeMountedFilesImplementor
	)

	var containerState = func(guid string) func() executor.State {
		return func() executor.State {
			container, err := containerStore.Get(logger, guid)
			Expect(err).NotTo(HaveOccurred())
			return container.State
		}
	}

	getMetrics := func() map[string]struct{} {
		metricMapLock.Lock()
		defer metricMapLock.Unlock()
		m := make(map[string]struct{}, len(metricMap))
		for k, v := range metricMap {
			m[k] = v
		}
		return m
	}

	BeforeEach(func() {
		// avoid race detector warnings
		metricMapLock.Lock()
		defer metricMapLock.Unlock()
		metricMap = map[string]struct{}{}

		gardenContainer = &gardenfakes.FakeContainer{}
		gardenClientFactory = &containerstorefakes.FakeGardenClientFactory{}
		gardenClient = &gardenfakes.FakeClient{}
		gardenClientFactory.NewGardenClientReturns(gardenClient)
		dependencyManager = &containerstorefakes.FakeDependencyManager{}
		credManager = &containerstorefakes.FakeCredManager{}
		logManager = &containerstorefakes.FakeLogManager{}
		proxyManager = &containerstorefakes.FakeProxyManager{}
		volumeManager = &volmanfakes.FakeManager{}
		clock = fakeclock.NewFakeClock(time.Now())
		eventEmitter = &eventfakes.FakeHub{}
		rootFSSizer = new(configurationfakes.FakeRootFSSizer)
		volumeMountedFilesHandler = &containerstorefakes.FakeVolumeMountedFilesImplementor{}

		credManager.RunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)
			<-signals
			return nil
		}))

		logStreamer = fake_log_streamer.NewFakeLogStreamer()
		logManager.NewLogStreamerReturns(logStreamer)

		iNodeLimit = 64
		maxCPUShares = 100
		ownerName = "test-owner"
		totalCapacity = executor.NewExecutorResources(1024*10, 1024*10, 10)
		advertisePreferenceForInstanceAddress = false

		containerGuid = "container-guid"

		megatron = &faketransformer.FakeTransformer{}

		metronClient = new(mfakes.FakeIngressClient)

		containerConfig = containerstore.ContainerConfig{
			OwnerName:              ownerName,
			INodeLimit:             iNodeLimit,
			MaxCPUShares:           maxCPUShares,
			ReapInterval:           20 * time.Millisecond,
			ReservedExpirationTime: 20 * time.Millisecond,
			MetricReportInterval:   30 * time.Second,
		}

		containerStore = containerstore.New(
			containerConfig,
			&totalCapacity,
			gardenClientFactory,
			dependencyManager,
			volumeManager,
			credManager,
			logManager,
			clock,
			eventEmitter,
			megatron,
			"/var/vcap/data/cf-system-trusted-certs",
			metronClient,
			rootFSSizer,
			"/var/vcap/packages/healthcheck",
			proxyManager,
			cellID,
			true,
			advertisePreferenceForInstanceAddress,
			volumeMountedFilesHandler,
			json.Marshal,
		)

		metronClient.SendDurationStub = func(name string, value time.Duration, opts ...loggregator.EmitGaugeOption) error {
			metricMapLock.Lock()
			defer metricMapLock.Unlock()
			metricMap[name] = struct{}{}
			return nil
		}
	})

	Describe("Reserve", func() {
		var (
			containerTags     executor.Tags
			containerResource executor.Resource
			req               *executor.AllocationRequest
		)

		BeforeEach(func() {
			containerTags = executor.Tags{
				"Foo": "bar",
			}
			containerResource = executor.Resource{
				MemoryMB: 1024,
				DiskMB:   1024,
			}

			req = &executor.AllocationRequest{
				Guid:               containerGuid,
				Tags:               containerTags,
				GenerateLogMetrics: true,
				Resource:           containerResource,
			}
		})

		It("returns a populated container", func() {
			container, err := containerStore.Reserve(logger, "some-trace-id", req)
			Expect(err).NotTo(HaveOccurred())

			Expect(container.Guid).To(Equal(containerGuid))
			Expect(container.Tags).To(Equal(containerTags))
			Expect(container.Resource).To(Equal(containerResource))
			Expect(container.State).To(Equal(executor.StateReserved))
			Expect(container.AllocatedAt).To(Equal(clock.Now().UnixNano()))
			Expect(container.AdvertisePreferenceForInstanceAddress).To(Equal(advertisePreferenceForInstanceAddress))
		})

		It("tracks the container", func() {
			container, err := containerStore.Reserve(logger, "some-trace-id", req)
			Expect(err).NotTo(HaveOccurred())

			found, err := containerStore.Get(logger, container.Guid)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(Equal(container))
		})

		It("emits a reserved container event", func() {
			container, err := containerStore.Reserve(logger, "some-trace-id", req)
			Expect(err).NotTo(HaveOccurred())

			Eventually(eventEmitter.EmitCallCount).Should(Equal(1))

			event := eventEmitter.EmitArgsForCall(0)
			Expect(event).To(Equal(executor.NewContainerReservedEvent(container, "some-trace-id")))
		})

		It("decrements the remaining capacity", func() {
			_, err := containerStore.Reserve(logger, "some-trace-id", req)
			Expect(err).NotTo(HaveOccurred())

			remainingCapacity := containerStore.RemainingResources(logger)
			Expect(remainingCapacity.MemoryMB).To(Equal(totalCapacity.MemoryMB - req.MemoryMB))
			Expect(remainingCapacity.DiskMB).To(Equal(totalCapacity.DiskMB - req.DiskMB))
			Expect(remainingCapacity.Containers).To(Equal(totalCapacity.Containers - 1))
		})

		Context("when the container guid is already reserved", func() {
			BeforeEach(func() {
				_, err := containerStore.Reserve(logger, "some-trace-id", req)
				Expect(err).NotTo(HaveOccurred())
			})

			It("fails with container guid not available", func() {
				_, err := containerStore.Reserve(logger, "some-trace-id", req)
				Expect(err).To(Equal(executor.ErrContainerGuidNotAvailable))
			})
		})

		Context("when there are not enough remaining resources available", func() {
			BeforeEach(func() {
				req.Resource.MemoryMB = totalCapacity.MemoryMB + 1
			})

			It("returns an error", func() {
				_, err := containerStore.Reserve(logger, "some-trace-id", req)
				Expect(err).To(Equal(executor.ErrInsufficientResourcesAvailable))
			})
		})
	})

	Describe("Initialize", func() {
		var (
			req     *executor.RunRequest
			runInfo executor.RunInfo
			runTags executor.Tags
		)

		BeforeEach(func() {
			runInfo = executor.RunInfo{
				CPUWeight:      2,
				StartTimeoutMs: 50000,
				Privileged:     true,
			}

			runTags = executor.Tags{
				"Beep": "Boop",
			}

			req = &executor.RunRequest{
				Guid:    containerGuid,
				RunInfo: runInfo,
				Tags:    runTags,
			}
		})

		Context("when the container has not been reserved", func() {
			It("returns a container not found error", func() {
				err := containerStore.Initialize(logger, req)
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})

		Context("when the conatiner is reserved", func() {
			BeforeEach(func() {
				allocationReq := &executor.AllocationRequest{
					Guid: containerGuid,
					Tags: executor.Tags{},
				}

				_, err := containerStore.Reserve(logger, "some-trace-id", allocationReq)
				Expect(err).NotTo(HaveOccurred())
			})

			It("populates the container with info from the run request", func() {
				err := containerStore.Initialize(logger, req)
				Expect(err).NotTo(HaveOccurred())

				container, err := containerStore.Get(logger, req.Guid)
				Expect(err).NotTo(HaveOccurred())
				Expect(container.State).To(Equal(executor.StateInitializing))
				Expect(container.RunInfo).To(Equal(runInfo))
				Expect(container.Tags).To(Equal(runTags))
			})
		})

		Context("when the container exists but is not reserved", func() {
			BeforeEach(func() {
				allocationReq := &executor.AllocationRequest{
					Guid: containerGuid,
					Tags: executor.Tags{},
				}

				_, err := containerStore.Reserve(logger, "some-trace-id", allocationReq)
				Expect(err).NotTo(HaveOccurred())

				err = containerStore.Initialize(logger, req)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an invalid state tranistion error", func() {
				err := containerStore.Initialize(logger, req)
				Expect(err).To(Equal(executor.ErrInvalidTransition))
			})
		})
	})

	Describe("Create", func() {
		var (
			resource      executor.Resource
			allocationReq *executor.AllocationRequest
			runInfo       executor.RunInfo
		)

		BeforeEach(func() {
			resource = executor.Resource{
				MemoryMB: 1024,
				DiskMB:   1024,
				MaxPids:  1024,
			}

			allocationReq = &executor.AllocationRequest{
				Guid: containerGuid,
				Tags: executor.Tags{
					"Foo": "Bar",
				},
				Resource: resource,
			}
		})

		Context("when the container is initializing", func() {
			var (
				externalIP, internalIP string
				runReq                 *executor.RunRequest
				logGuid                string
			)

			BeforeEach(func() {
				externalIP = "6.6.6.6"
				internalIP = "7.7.7.7"
				env := []executor.EnvironmentVariable{
					{Name: "foo", Value: "bar"},
					{Name: "beep", Value: "booop"},
				}

				volumeMountedFiles := []executor.VolumeMountedFiles{
					{Path: "/redis/username", Content: "redis_username"},
				}

				logGuid = "log-guid-foo"
				runInfo = executor.RunInfo{
					RootFSPath:     "/foo/bar",
					Privileged:     true,
					CPUWeight:      50,
					StartTimeoutMs: 99000,
					CachedDependencies: []executor.CachedDependency{
						{Name: "artifact", From: "https://example.com", To: "/etc/foo", CacheKey: "abc", LogSource: "source"},
					},
					LogConfig: models.LogConfig{
						Guid:       logGuid,
						Index:      1,
						SourceName: "test-source",
					},
					MetricsConfig: executor.MetricsConfig{
						Guid:  "metric-guid",
						Index: 1,
					},
					Env:                           env,
					TrustedSystemCertificatesPath: "",
					Network: &executor.Network{
						Properties: map[string]string{
							"some-key":       "some-value",
							"some-other-key": "some-other-value",
						},
					},
					EnableContainerProxy:       true,
					LogRateLimitBytesPerSecond: logRateUnlimitedBytesPerSecond,
					VolumeMountedFiles:         volumeMountedFiles,
				}

				runReq = &executor.RunRequest{
					Guid:    containerGuid,
					RunInfo: runInfo,
				}

				gardenContainer.InfoReturns(garden.ContainerInfo{ExternalIP: externalIP, ContainerIP: internalIP}, nil)
				gardenClient.CreateReturns(gardenContainer, nil)

				rootFSSizer.RootFSSizeFromPathReturns(1000)
			})

			JustBeforeEach(func() {
				_, err := containerStore.Reserve(logger, "some-trace-id", allocationReq)
				Expect(err).NotTo(HaveOccurred())

				err = containerStore.Initialize(logger, runReq)
				Expect(err).NotTo(HaveOccurred())
			})

			It("sets the container state to created", func() {
				_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())

				container, err := containerStore.Get(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(container.State).To(Equal(executor.StateCreated))
			})

			It("creates the container in garden with correct image parameters", func() {
				_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClient.CreateCallCount()).To(Equal(1))
				containerSpec := gardenClient.CreateArgsForCall(0)
				Expect(containerSpec.Handle).To(Equal(containerGuid))
				Expect(containerSpec.Image.URI).To(Equal(runInfo.RootFSPath))
				Expect(containerSpec.Privileged).To(Equal(true))
			})

			Context("when setting image credentials", func() {
				BeforeEach(func() {
					runReq.RunInfo.ImageUsername = "some-username"
					runReq.RunInfo.ImagePassword = "some-password"
				})

				It("creates the container in garden with correct image credentials", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(gardenClient.CreateCallCount()).To(Equal(1))
					containerSpec := gardenClient.CreateArgsForCall(0)
					Expect(containerSpec.Image.URI).To(Equal(runInfo.RootFSPath))
					Expect(containerSpec.Image.Username).To(Equal("some-username"))
					Expect(containerSpec.Image.Password).To(Equal("some-password"))
				})
			})

			Context("when SetCPUWeight is set", func() {
				BeforeEach(func() {
					containerConfig.SetCPUWeight = true

					containerStore = containerstore.New(
						containerConfig,
						&totalCapacity,
						gardenClientFactory,
						dependencyManager,
						volumeManager,
						credManager,
						logManager,
						clock,
						eventEmitter,
						megatron,
						"/var/vcap/data/cf-system-trusted-certs",
						metronClient,
						rootFSSizer,
						"/var/vcap/packages/healthcheck",
						proxyManager,
						cellID,
						true,
						advertisePreferenceForInstanceAddress,
						volumeMountedFilesHandler,
						json.Marshal,
					)
				})

				It("creates the container in garden with the correct limits", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(gardenClient.CreateCallCount()).To(Equal(1))
					containerSpec := gardenClient.CreateArgsForCall(0)
					Expect(containerSpec.Limits.Memory.LimitInBytes).To(BeEquivalentTo(resource.MemoryMB * 1024 * 1024))

					Expect(containerSpec.Limits.Disk.Scope).To(Equal(garden.DiskLimitScopeTotal))
					Expect(containerSpec.Limits.Disk.ByteHard).To(BeEquivalentTo((resource.DiskMB * 1024 * 1024) + 1000))
					Expect(containerSpec.Limits.Disk.InodeHard).To(Equal(iNodeLimit))

					Expect(int(containerSpec.Limits.Pid.Max)).To(Equal(resource.MaxPids))

					expectedCPUShares := uint64(float64(maxCPUShares) * float64(runReq.CPUWeight) / 100.0)
					Expect(containerSpec.Limits.CPU.LimitInShares).To(Equal(expectedCPUShares))

					expectedCPUWeight := uint64(resource.MemoryMB)
					Expect(containerSpec.Limits.CPU.Weight).To(Equal(expectedCPUWeight))
				})
			})

			It("creates the container in garden with the correct limits", func() {
				_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClient.CreateCallCount()).To(Equal(1))
				containerSpec := gardenClient.CreateArgsForCall(0)
				Expect(containerSpec.Limits.Memory.LimitInBytes).To(BeEquivalentTo(resource.MemoryMB * 1024 * 1024))

				Expect(containerSpec.Limits.Disk.Scope).To(Equal(garden.DiskLimitScopeTotal))
				Expect(containerSpec.Limits.Disk.ByteHard).To(BeEquivalentTo((resource.DiskMB * 1024 * 1024) + 1000))
				Expect(containerSpec.Limits.Disk.InodeHard).To(Equal(iNodeLimit))

				Expect(int(containerSpec.Limits.Pid.Max)).To(Equal(resource.MaxPids))

				expectedCPUShares := uint64(float64(maxCPUShares) * float64(runReq.CPUWeight) / 100.0)
				Expect(containerSpec.Limits.CPU.LimitInShares).To(Equal(expectedCPUShares))
				Expect(containerSpec.Limits.CPU.Weight).To(BeZero())
			})

			Context("when the requested disk limit is 0", func() {
				BeforeEach(func() {
					allocationReq.Resource.DiskMB = 0
				})

				It("creates the container in garden with a 0 disk limit", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(gardenClient.CreateCallCount()).To(Equal(1))
					containerSpec := gardenClient.CreateArgsForCall(0)

					Expect(containerSpec.Limits.Disk.Scope).To(Equal(garden.DiskLimitScopeTotal))
					Expect(containerSpec.Limits.Disk.ByteHard).To(BeEquivalentTo(0))
					Expect(containerSpec.Limits.Disk.InodeHard).To(Equal(iNodeLimit))
				})
			})

			It("downloads the correct cache dependencies", func() {
				_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(dependencyManager.DownloadCachedDependenciesCallCount()).To(Equal(1))
				_, mounts, _, _ := dependencyManager.DownloadCachedDependenciesArgsForCall(0)
				Expect(mounts).To(Equal(runReq.CachedDependencies))
			})

			It("creates the container in garden with the correct bind mounts", func() {
				expectedMount := garden.BindMount{
					SrcPath: "foo",
					DstPath: "/etc/foo",
					Mode:    garden.BindMountModeRO,
					Origin:  garden.BindMountOriginHost,
				}
				dependencyManager.DownloadCachedDependenciesReturns(containerstore.BindMounts{
					GardenBindMounts: []garden.BindMount{expectedMount},
				}, nil)

				_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClient.CreateCallCount()).To(Equal(1))
				containerSpec := gardenClient.CreateArgsForCall(0)
				Expect(containerSpec.BindMounts).To(ContainElement(expectedMount))
			})

			It("creates the container with the correct properties", func() {
				_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClient.CreateCallCount()).To(Equal(1))
				containerSpec := gardenClient.CreateArgsForCall(0)

				Expect(containerSpec.Properties).To(Equal(garden.Properties{
					executor.ContainerOwnerProperty: ownerName,
					"network.some-key":              "some-value",
					"network.some-other-key":        "some-other-value",
					"log_config":                    "{\"guid\":\"log-guid-foo\",\"index\":1,\"source_name\":\"test-source\",\"tags\":null}",
				}))
			})

			Context("if the network is not set", func() {
				BeforeEach(func() {
					runReq.RunInfo.Network = nil
				})
				It("sets the owner property", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())

					containerSpec := gardenClient.CreateArgsForCall(0)

					Expect(containerSpec.Properties).To(Equal(garden.Properties{
						executor.ContainerOwnerProperty: ownerName,
						"log_config":                    "{\"guid\":\"log-guid-foo\",\"index\":1,\"source_name\":\"test-source\",\"tags\":null}",
					}))
				})
			})

			Context("if the RootFSPath is not a known preloaded rootfs", func() {
				BeforeEach(func() {
					runReq.RunInfo.RootFSPath = "docker://some/repo"
					rootFSSizer.RootFSSizeFromPathReturns(0)
				})

				It("sets the correct disk limit", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(gardenClient.CreateCallCount()).To(Equal(1))
					containerSpec := gardenClient.CreateArgsForCall(0)

					Expect(containerSpec.Limits.Disk.Scope).To(Equal(garden.DiskLimitScopeTotal))
					Expect(containerSpec.Limits.Disk.ByteHard).To(BeEquivalentTo(resource.DiskMB * 1024 * 1024))
				})
			})

			It("creates the container with the correct environment", func() {
				_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClient.CreateCallCount()).To(Equal(1))
				containerSpec := gardenClient.CreateArgsForCall(0)

				expectedEnv := []string{}
				for _, envVar := range runReq.Env {
					expectedEnv = append(expectedEnv, envVar.Name+"="+envVar.Value)
				}
				Expect(containerSpec.Env).To(Equal(expectedEnv))
			})

			It("creates container with service binding root", func() {
				expectedMount := garden.BindMount{
					SrcPath: "/var/vcap/data/rep/shared/garden/service_binding_root",
					DstPath: "/etc/cf-instance-binding",
					Mode:    garden.BindMountModeRO,
					Origin:  garden.BindMountOriginHost,
				}

				dependencyManager.DownloadCachedDependenciesReturns(containerstore.BindMounts{
					GardenBindMounts: []garden.BindMount{expectedMount},
				}, nil)

				_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClient.CreateCallCount()).To(Equal(1))
				containerSpec := gardenClient.CreateArgsForCall(0)
				Expect(containerSpec.BindMounts).To(ContainElement(expectedMount))
			})

			It("sets the correct external and internal ip", func() {
				container, err := containerStore.Create(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(container.ExternalIP).To(Equal(externalIP))
				Expect(container.InternalIP).To(Equal(internalIP))
			})

			It("emits metrics after creating the container", func() {
				_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Eventually(getMetrics).Should(HaveKey(containerstore.GardenContainerCreationSucceededDuration))
			})

			It("sends a log after creating the container", func() {
				_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Eventually(metronClient.SendAppLogCallCount()).Should(Equal(2))
				Eventually(func() string {
					msg, _, _ := metronClient.SendAppLogArgsForCall(1)
					return msg
				}).Should(ContainSubstring(fmt.Sprintf("Cell %s successfully created container for instance %s", cellID, containerGuid)))
			})

			It("generates container credential directory", func() {
				_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(credManager.CreateCredDirCallCount()).To(Equal(1))
				_, container := credManager.CreateCredDirArgsForCall(0)
				Expect(container.Guid).To(Equal(containerGuid))
			})

			It("does not bind mount the healthcheck", func() {
				_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClient.CreateCallCount()).To(Equal(1))
				containerSpec := gardenClient.CreateArgsForCall(0)
				Expect(containerSpec.BindMounts).To(ContainElement(garden.BindMount{
					SrcPath: "/var/vcap/packages/healthcheck",
					DstPath: "/etc/cf-assets/healthcheck",
					Mode:    garden.BindMountModeRO,
					Origin:  garden.BindMountOriginHost,
				}))
			})

			Context("when declarative healthchecks are enabled", func() {
				BeforeEach(func() {
					containerStore = containerstore.New(
						containerConfig,
						&totalCapacity,
						gardenClientFactory,
						dependencyManager,
						volumeManager,
						credManager,
						logManager,
						clock,
						eventEmitter,
						megatron,
						"/var/vcap/data/cf-system-trusted-certs",
						metronClient,
						rootFSSizer,
						"/var/vcap/packages/healthcheck",
						proxyManager,
						cellID,
						true,
						advertisePreferenceForInstanceAddress,
						volumeMountedFilesHandler,
						json.Marshal,
					)
				})

				It("bind mounts the healthcheck", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(gardenClient.CreateCallCount()).To(Equal(1))
					containerSpec := gardenClient.CreateArgsForCall(0)
					Expect(containerSpec.BindMounts).To(ContainElement(garden.BindMount{
						SrcPath: "/var/vcap/packages/healthcheck",
						DstPath: "/etc/cf-assets/healthcheck",
						Mode:    garden.BindMountModeRO,
						Origin:  garden.BindMountOriginHost,
					}))
				})
			})

			Context("when credential mounts are configured", func() {
				var (
					expectedBindMount garden.BindMount
				)

				BeforeEach(func() {
					expectedBindMount = garden.BindMount{SrcPath: "hpath1", DstPath: "cpath1", Mode: garden.BindMountModeRO, Origin: garden.BindMountOriginHost}
					envVariables := []executor.EnvironmentVariable{
						{Name: "CF_INSTANCE_CERT", Value: "some-cert"},
					}
					credManager.CreateCredDirReturns([]garden.BindMount{expectedBindMount}, envVariables, nil)
				})

				It("mounts the credential directory into the container", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(gardenClient.CreateCallCount()).To(Equal(1))
					Expect(gardenClient.CreateArgsForCall(0).BindMounts).To(ContainElement(expectedBindMount))
				})

				It("add the instance identity environment variables to the container", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(gardenClient.CreateCallCount()).To(Equal(1))
					Expect(gardenClient.CreateArgsForCall(0).Env).To(ContainElement("CF_INSTANCE_CERT=some-cert"))
				})

				Context("when failing to create credential directory on host", func() {
					BeforeEach(func() {
						credManager.CreateCredDirReturns(nil, nil, errors.New("failed to create dir"))
					})

					It("fails fast and completes the container", func() {
						_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
						Expect(err).To(HaveOccurred())

						container, err := containerStore.Get(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())
						Expect(container.State).To(Equal(executor.StateCompleted))
						Expect(container.RunResult.Failed).To(BeTrue())
						Expect(container.RunResult.FailureReason).To(Equal(containerstore.CredDirFailed))
						Expect(container.RunResult.Retryable).To(BeTrue())
					})
				})
			})

			Context("when there are volume mounts configured", func() {
				BeforeEach(func() {
					someConfig := map[string]interface{}{"some-config": "interface"}
					runReq.RunInfo.VolumeMounts = []executor.VolumeMount{
						executor.VolumeMount{ContainerPath: "cpath1", Mode: executor.BindMountModeRW, Driver: "some-driver", VolumeId: "some-volume", Config: someConfig},
						executor.VolumeMount{ContainerPath: "cpath2", Mode: executor.BindMountModeRO, Driver: "some-other-driver", VolumeId: "some-other-volume", Config: someConfig},
					}

					count := 0
					volumeManager.MountStub = // first call mounts at a different point than second call
						func(lager.Logger, string, string, string, map[string]interface{}) (volman.MountResponse, error) {
							defer func() { count = count + 1 }()
							if count == 0 {
								return volman.MountResponse{Path: "hpath1"}, nil
							}
							return volman.MountResponse{Path: "hpath2"}, nil
						}
				})

				It("mounts the correct volumes via the volume manager", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(volumeManager.MountCallCount()).To(Equal(2))

					_, driverName, volumeId, containerId, config := volumeManager.MountArgsForCall(0)
					Expect(driverName).To(Equal(runReq.VolumeMounts[0].Driver))
					Expect(volumeId).To(Equal(runReq.VolumeMounts[0].VolumeId))
					Expect(containerId).To(Equal(containerGuid))
					Expect(config).To(Equal(runReq.VolumeMounts[0].Config))

					_, driverName, volumeId, containerId, config = volumeManager.MountArgsForCall(1)
					Expect(driverName).To(Equal(runReq.VolumeMounts[1].Driver))
					Expect(volumeId).To(Equal(runReq.VolumeMounts[1].VolumeId))
					Expect(containerId).To(Equal(containerGuid))
					Expect(config).To(Equal(runReq.VolumeMounts[1].Config))
				})

				It("correctly maps container and host directories in garden", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(gardenClient.CreateCallCount()).To(Equal(1))

					expectedBindMount1 := garden.BindMount{SrcPath: "hpath1", DstPath: "cpath1", Mode: garden.BindMountModeRW}
					Expect(gardenClient.CreateArgsForCall(0).BindMounts).To(ContainElement(expectedBindMount1))
					expectedBindMount2 := garden.BindMount{SrcPath: "hpath2", DstPath: "cpath2", Mode: garden.BindMountModeRO}
					Expect(gardenClient.CreateArgsForCall(0).BindMounts).To(ContainElement(expectedBindMount2))
				})

				Context("when it errors on mount", func() {
					Context("when the driver returns a safeError", func() {
						BeforeEach(func() {
							volumeManager.MountReturns(volman.MountResponse{Path: "host-path"}, volman.SafeError{SafeDescription: "some-error"})
						})

						It("fails fast and completes the container", func() {
							_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
							Expect(err).To(HaveOccurred())
							Expect(volumeManager.MountCallCount()).To(Equal(1))

							container, err := containerStore.Get(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())
							Expect(container.State).To(Equal(executor.StateCompleted))
							Expect(container.RunResult.Failed).To(BeTrue())
							Expect(container.RunResult.FailureReason).To(Equal(fmt.Sprintf("%s, errors: some-error", containerstore.VolmanMountFailed)))
							Expect(container.RunResult.Retryable).To(BeTrue())
						})

					})

					Context("when the driver returns a unsafeError", func() {
						BeforeEach(func() {
							volumeManager.MountReturns(volman.MountResponse{Path: "host-path"}, errors.New("some-error"))
						})

						It("fails fast and completes the container", func() {
							_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
							Expect(err).To(HaveOccurred())
							Expect(volumeManager.MountCallCount()).To(Equal(1))

							container, err := containerStore.Get(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())
							Expect(container.State).To(Equal(executor.StateCompleted))
							Expect(container.RunResult.Failed).To(BeTrue())
							Expect(container.RunResult.FailureReason).To(Equal(containerstore.VolmanMountFailed))
							Expect(container.RunResult.Retryable).To(BeTrue())
						})
					})

				})
			})

			Context("when there are trusted system certificates", func() {
				Context("and the desired LRP has a certificates path", func() {
					var mounts []garden.BindMount

					BeforeEach(func() {
						runReq.RunInfo.TrustedSystemCertificatesPath = "/etc/cf-system-certificates"
						mounts = []garden.BindMount{
							{
								SrcPath: "/var/vcap/data/cf-system-trusted-certs",
								DstPath: runReq.RunInfo.TrustedSystemCertificatesPath,
								Mode:    garden.BindMountModeRO,
								Origin:  garden.BindMountOriginHost,
							},
						}
					})

					It("creates a bind mount", func() {
						_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
						Expect(err).NotTo(HaveOccurred())

						Expect(gardenClient.CreateCallCount()).To(Equal(1))
						gardenContainerSpec := gardenClient.CreateArgsForCall(0)
						Expect(gardenContainerSpec.BindMounts).To(ContainElement(mounts[0]))
					})

					It("creates a CF_SYSTEM_CERT_PATH env var", func() {
						_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
						Expect(err).NotTo(HaveOccurred())

						containerSpec := gardenClient.CreateArgsForCall(0)
						Expect(containerSpec.Env).To(ContainElement("CF_SYSTEM_CERT_PATH=" + runReq.RunInfo.TrustedSystemCertificatesPath))
					})
				})

				Context("and the desired LRP does not have a certificates path", func() {
					It("does not create a bind mount", func() {
						_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
						Expect(err).NotTo(HaveOccurred())

						Expect(gardenClient.CreateCallCount()).To(Equal(1))
						gardenContainerSpec := gardenClient.CreateArgsForCall(0)
						Expect(gardenContainerSpec.BindMounts).NotTo(ContainElement(garden.BindMount{
							SrcPath: "/var/vcap/data/cf-system-trusted-certs",
							DstPath: runReq.RunInfo.TrustedSystemCertificatesPath,
							Mode:    garden.BindMountModeRO,
							Origin:  garden.BindMountOriginHost,
						}))
					})

					It("does not create the CF_SYSTEM_CERT_PATH env var", func() {
						_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
						Expect(err).NotTo(HaveOccurred())
						containerSpec := gardenClient.CreateArgsForCall(0)
						Expect(containerSpec.Env).NotTo(ContainElement(ContainSubstring("CF_SYSTEM_CERT_PATH")))
					})
				})
			})

			Context("when downloading bind mounts fails", func() {
				BeforeEach(func() {
					dependencyManager.DownloadCachedDependenciesReturns(containerstore.BindMounts{}, errors.New("no"))
				})

				It("transitions to a completed state", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).To(HaveOccurred())

					container, err := containerStore.Get(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(container.State).To(Equal(executor.StateCompleted))
					Expect(container.RunResult.Failed).To(BeTrue())
					Expect(container.RunResult.FailureReason).To(Equal(containerstore.DownloadCachedDependenciesFailed))
					Expect(container.RunResult.Retryable).To(BeTrue())
				})
			})

			Context("when egress rules are requested", func() {
				BeforeEach(func() {
					egressRules := []*models.SecurityGroupRule{
						{
							Protocol:     "icmp",
							Destinations: []string{"1.1.1.1"},
							IcmpInfo: &models.ICMPInfo{
								Type: 2,
								Code: 10,
							},
						},
						{
							Protocol:     "icmp",
							Destinations: []string{"1.1.1.2"},
							IcmpInfo: &models.ICMPInfo{
								Type: 3,
								Code: 10,
							},
						},
						{
							Protocol:     "icmpv6",
							Destinations: []string{"2100::1"},
							IcmpInfo: &models.ICMPInfo{
								Type: 1,
								Code: 10,
							},
						},
					}
					runReq.EgressRules = egressRules
				})

				It("calls NetOut for each egress rule", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(gardenClient.CreateCallCount()).To(Equal(1))
					containerSpec := gardenClient.CreateArgsForCall(0)
					Expect(containerSpec.NetOut).To(HaveLen(3))
					icmpCode := garden.ICMPCode(10)
					Expect(containerSpec.NetOut).To(ContainElement(garden.NetOutRule{
						Protocol: garden.Protocol(3),
						Networks: []garden.IPRange{
							{
								Start: net.ParseIP("1.1.1.1"),
								End:   net.ParseIP("1.1.1.1"),
							},
						},
						ICMPs: &garden.ICMPControl{
							Type: 2,
							Code: &icmpCode,
						},
					}))
					Expect(containerSpec.NetOut).To(ContainElement(garden.NetOutRule{
						Protocol: garden.Protocol(3),
						Networks: []garden.IPRange{
							{
								Start: net.ParseIP("1.1.1.2"),
								End:   net.ParseIP("1.1.1.2"),
							},
						},
						ICMPs: &garden.ICMPControl{
							Type: 3,
							Code: &icmpCode,
						},
					}))
					Expect(containerSpec.NetOut).To(ContainElement(garden.NetOutRule{
						Protocol: garden.Protocol(4),
						Networks: []garden.IPRange{
							{
								Start: net.ParseIP("2100::1"),
								End:   net.ParseIP("2100::1"),
							},
						},
						ICMPs: &garden.ICMPControl{
							Type: 1,
							Code: &icmpCode,
						},
					}))
				})

				Context("when a destination is a comma-delimited list", func() {
					BeforeEach(func() {
						egressRules := []*models.SecurityGroupRule{
							{
								Protocol:     "icmp",
								Destinations: []string{"1.1.1.1,2.2.2.2"},
								IcmpInfo: &models.ICMPInfo{
									Type: 2,
									Code: 17,
								},
							},
							{
								Protocol:     "icmpv6",
								Destinations: []string{"2100::1,2100::2"},
								IcmpInfo: &models.ICMPInfo{
									Type: 3,
									Code: 17,
								},
							},
						}
						runReq.EgressRules = egressRules
					})

					It("calls NetOut for each rule in a comma-delimited list", func() {
						_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
						Expect(err).NotTo(HaveOccurred())

						Expect(gardenClient.CreateCallCount()).To(Equal(1))
						containerSpec := gardenClient.CreateArgsForCall(0)
						Expect(containerSpec.NetOut).To(HaveLen(4))
						icmpCode := garden.ICMPCode(17)
						Expect(containerSpec.NetOut).To(ContainElement(garden.NetOutRule{
							Protocol: garden.Protocol(3),
							Networks: []garden.IPRange{
								{
									Start: net.ParseIP("1.1.1.1"),
									End:   net.ParseIP("1.1.1.1"),
								},
							},
							ICMPs: &garden.ICMPControl{
								Type: 2,
								Code: &icmpCode,
							},
						}))
						Expect(containerSpec.NetOut).To(ContainElement(garden.NetOutRule{
							Protocol: garden.Protocol(3),
							Networks: []garden.IPRange{
								{
									Start: net.ParseIP("2.2.2.2"),
									End:   net.ParseIP("2.2.2.2"),
								},
							},
							ICMPs: &garden.ICMPControl{
								Type: 2,
								Code: &icmpCode,
							},
						}))
						Expect(containerSpec.NetOut).To(ContainElement(garden.NetOutRule{
							Protocol: garden.Protocol(4),
							Networks: []garden.IPRange{
								{
									Start: net.ParseIP("2100::1"),
									End:   net.ParseIP("2100::1"),
								},
							},
							ICMPs: &garden.ICMPControl{
								Type: 3,
								Code: &icmpCode,
							},
						}))
						Expect(containerSpec.NetOut).To(ContainElement(garden.NetOutRule{
							Protocol: garden.Protocol(4),
							Networks: []garden.IPRange{
								{
									Start: net.ParseIP("2100::2"),
									End:   net.ParseIP("2100::2"),
								},
							},
							ICMPs: &garden.ICMPControl{
								Type: 3,
								Code: &icmpCode,
							},
						}))
					})
				})

				Context("when a egress rule is not valid", func() {
					BeforeEach(func() {
						egressRule := &models.SecurityGroupRule{
							Protocol: "tcp",
						}
						runReq.EgressRules = append(runReq.EgressRules, egressRule)
					})

					It("returns an error", func() {
						_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
						Expect(err).To(HaveOccurred())

						Expect(gardenClient.CreateCallCount()).To(Equal(0))
					})
				})
			})

			Context("when containerProxy is disabled", func() {
				BeforeEach(func() {
					portMapping := []executor.PortMapping{
						{ContainerPort: 8080},
						{ContainerPort: 9090},
					}
					runReq.Ports = portMapping

					gardenClient.CreateStub = func(spec garden.ContainerSpec) (garden.Container, error) {
						gardenContainer.InfoStub = func() (garden.ContainerInfo, error) {
							info := garden.ContainerInfo{}
							info.MappedPorts = []garden.PortMapping{}
							for _, netIn := range spec.NetIn {
								switch netIn.ContainerPort {
								case 8080:
									info.MappedPorts = append(info.MappedPorts, garden.PortMapping{HostPort: 16000, ContainerPort: 8080})
								case 9090:
									info.MappedPorts = append(info.MappedPorts, garden.PortMapping{HostPort: 32000, ContainerPort: 9090})
								default:
									return info, errors.New("failed-net-in")
								}
							}
							return info, nil
						}
						return gardenContainer, nil
					}
				})

				It("passes all port mappings to NetIn on container creation", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())

					containerSpec := gardenClient.CreateArgsForCall(0)
					Expect(containerSpec.NetIn).To(HaveLen(2))
					Expect(containerSpec.NetIn).To(ContainElement(garden.NetIn{
						HostPort: 0, ContainerPort: 8080,
					}))
					Expect(containerSpec.NetIn).To(ContainElement(garden.NetIn{
						HostPort: 0, ContainerPort: 9090,
					}))
				})

				Context("when the app has duplicate port exposed", func() {
					BeforeEach(func() {
						runReq.Ports = append(runReq.Ports, executor.PortMapping{ContainerPort: 8080})
					})

					It("de duplicate the exposed ports", func() {
						container, err := containerStore.Create(logger, "some-trace-id", containerGuid)
						Expect(err).NotTo(HaveOccurred())

						Expect(container.Ports).To(ConsistOf(executor.PortMapping{
							ContainerPort: 8080,
							HostPort:      16000,
						}, executor.PortMapping{
							ContainerPort: 9090,
							HostPort:      32000,
						}))

						fetchedContainer, err := containerStore.Get(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())
						Expect(fetchedContainer).To(Equal(container))
					})
				})

				It("saves the actual port mappings on the container", func() {
					container, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(container.Ports).To(ConsistOf(executor.PortMapping{
						ContainerPort: 8080,
						HostPort:      16000,
					}, executor.PortMapping{
						ContainerPort: 9090,
						HostPort:      32000,
					}))

					fetchedContainer, err := containerStore.Get(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(fetchedContainer).To(Equal(container))
				})

				It("preserves port orders", func() {
					Consistently(func() uint16 {
						containerGUID := "new-container-guid"
						allocationReq = &executor.AllocationRequest{
							Guid: containerGUID,
							Tags: executor.Tags{
								"Foo": "Bar",
							},
						}

						_, err := containerStore.Reserve(logger, "some-trace-id", allocationReq)
						Expect(err).NotTo(HaveOccurred())

						defer containerStore.Destroy(logger, "some-trace-id", containerGUID)

						runReq = &executor.RunRequest{
							Guid:    containerGUID,
							RunInfo: runInfo,
						}
						runReq.Ports = []executor.PortMapping{
							{ContainerPort: 8080},
							{ContainerPort: 9090},
						}
						err = containerStore.Initialize(logger, runReq)
						Expect(err).NotTo(HaveOccurred())

						container, err := containerStore.Create(logger, "some-trace-id", containerGUID)
						Expect(err).NotTo(HaveOccurred())
						return container.Ports[0].ContainerPort
					}).Should(Equal(uint16(8080)))
				})
			})

			Context("when containerProxy is enabled", func() {
				var (
					envoySourceDir string
					envoyConfigDir string
					err            error
				)

				BeforeEach(func() {
					envoySourceDir, err = os.MkdirTemp("", "envoy_dir")
					Expect(err).NotTo(HaveOccurred())
					envoyConfigDir, err = os.MkdirTemp("", "envoy_config_dir")
					Expect(err).NotTo(HaveOccurred())

					bindMounts := []garden.BindMount{
						{
							Origin:  garden.BindMountOriginHost,
							SrcPath: envoySourceDir,
							DstPath: "/etc/cf-assets/envoy",
						},
						{
							Origin:  garden.BindMountOriginHost,
							SrcPath: filepath.Join(envoyConfigDir, containerGuid),
							DstPath: "/etc/cf-assets/envoy_config",
						},
					}
					credManager.CreateCredDirReturns(bindMounts, nil, nil)

					proxyManager.ProxyPortsReturns([]executor.ProxyPortMapping{
						{
							AppPort:   8080,
							ProxyPort: 61001,
						},
						{
							AppPort:   8080,
							ProxyPort: containerstore.C2CTLSPort,
						},
						{
							AppPort:   9090,
							ProxyPort: 61002,
						},
					}, []uint16{
						61001,
						61002,
						containerstore.C2CTLSPort,
					}, nil)

					containerStore = containerstore.New(
						containerConfig,
						&totalCapacity,
						gardenClientFactory,
						dependencyManager,
						volumeManager,
						credManager,
						logManager,
						clock,
						eventEmitter,
						megatron,
						"/var/vcap/data/cf-system-trusted-certs",
						metronClient,
						rootFSSizer,
						"/var/vcap/packages/healthcheck",
						proxyManager,
						cellID,
						true,
						advertisePreferenceForInstanceAddress,
						volumeMountedFilesHandler,
						json.Marshal,
					)

					portMapping := []executor.PortMapping{
						{ContainerPort: 8080},
						{ContainerPort: 9090},
					}
					runReq.Ports = portMapping

					gardenClient.CreateStub = func(spec garden.ContainerSpec) (garden.Container, error) {
						gardenContainer.InfoStub = func() (garden.ContainerInfo, error) {
							info := garden.ContainerInfo{}
							info.MappedPorts = []garden.PortMapping{}
							for _, netIn := range spec.NetIn {
								switch netIn.ContainerPort {
								case 8080:
									info.MappedPorts = append(info.MappedPorts, garden.PortMapping{HostPort: 16000, ContainerPort: 8080})
								case 9090:
									info.MappedPorts = append(info.MappedPorts, garden.PortMapping{HostPort: 16004, ContainerPort: 9090})
								case 61001:
									info.MappedPorts = append(info.MappedPorts, garden.PortMapping{HostPort: 16001, ContainerPort: 61001})
								case 61002:
									info.MappedPorts = append(info.MappedPorts, garden.PortMapping{HostPort: 16002, ContainerPort: 61002})
								case 61003:
									info.MappedPorts = append(info.MappedPorts, garden.PortMapping{HostPort: 16003, ContainerPort: 61003})
								case containerstore.C2CTLSPort:
									info.MappedPorts = append(info.MappedPorts, garden.PortMapping{HostPort: 16004, ContainerPort: containerstore.C2CTLSPort})
								default:
									return info, errors.New("failed-net-in")
								}
							}
							return info, nil
						}
						return gardenContainer, nil
					}
				})

				AfterEach(func() {
					err := os.RemoveAll(envoySourceDir)
					Expect(err).NotTo(HaveOccurred())

					err = os.RemoveAll(envoyConfigDir)
					Expect(err).NotTo(HaveOccurred())
				})

				It("passes all port mappings to NetIn on container creation", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())

					containerSpec := gardenClient.CreateArgsForCall(0)
					Expect(containerSpec.NetIn).To(HaveLen(4))
					Expect(containerSpec.NetIn).To(ContainElement(garden.NetIn{
						HostPort: 0, ContainerPort: 8080,
					}))
					Expect(containerSpec.NetIn).To(ContainElement(garden.NetIn{
						HostPort: 0, ContainerPort: 9090,
					}))
					Expect(containerSpec.NetIn).To(ContainElement(garden.NetIn{
						HostPort: 0, ContainerPort: 61001,
					}))
					Expect(containerSpec.NetIn).To(ContainElement(garden.NetIn{
						HostPort: 0, ContainerPort: 61002,
					}))
					Expect(containerSpec.NetIn).ToNot(ContainElement(garden.NetIn{
						HostPort: 0, ContainerPort: containerstore.C2CTLSPort,
					}))
				})

				Context("when disabling unproxied port mappings", func() {
					BeforeEach(func() {
						containerStore = containerstore.New(
							containerConfig,
							&totalCapacity,
							gardenClientFactory,
							dependencyManager,
							volumeManager,
							credManager,
							logManager,
							clock,
							eventEmitter,
							megatron,
							"/var/vcap/data/cf-system-trusted-certs",
							metronClient,
							rootFSSizer,
							"/var/vcap/packages/healthcheck",
							proxyManager,
							cellID,
							false,
							advertisePreferenceForInstanceAddress,
							volumeMountedFilesHandler,
							json.Marshal,
						)
					})

					It("passes only proxied port mappings to NetIn on container creation", func() {
						_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
						Expect(err).NotTo(HaveOccurred())

						containerSpec := gardenClient.CreateArgsForCall(0)
						Expect(containerSpec.NetIn).To(HaveLen(2))
						Expect(containerSpec.NetIn).To(ContainElement(garden.NetIn{
							HostPort: 0, ContainerPort: 61001,
						}))
						Expect(containerSpec.NetIn).To(ContainElement(garden.NetIn{
							HostPort: 0, ContainerPort: 61002,
						}))
						Expect(containerSpec.NetIn).ToNot(ContainElement(garden.NetIn{
							HostPort: 0, ContainerPort: containerstore.C2CTLSPort,
						}))
					})

					It("unproxied host ports are set to 0", func() {
						container, err := containerStore.Create(logger, "some-trace-id", containerGuid)
						Expect(err).NotTo(HaveOccurred())

						Expect(container.Ports).To(ConsistOf(executor.PortMapping{
							ContainerPort:         8080,
							HostPort:              0,
							ContainerTLSProxyPort: 61001,
							HostTLSProxyPort:      16001,
						}, executor.PortMapping{
							ContainerPort:         8080,
							HostPort:              0,
							ContainerTLSProxyPort: containerstore.C2CTLSPort,
							HostTLSProxyPort:      0,
						}, executor.PortMapping{
							ContainerPort:         9090,
							HostPort:              0,
							ContainerTLSProxyPort: 61002,
							HostTLSProxyPort:      16002,
						}))
					})
				})

				It("each port gets an equivalent extra proxy port", func() {
					container, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(container.Ports).To(ConsistOf(executor.PortMapping{
						ContainerPort:         8080,
						HostPort:              16000,
						ContainerTLSProxyPort: 61001,
						HostTLSProxyPort:      16001,
					}, executor.PortMapping{
						ContainerPort:         8080,
						HostPort:              16000,
						ContainerTLSProxyPort: containerstore.C2CTLSPort,
						HostTLSProxyPort:      0,
					}, executor.PortMapping{
						ContainerPort:         9090,
						HostPort:              16004,
						ContainerTLSProxyPort: 61002,
						HostTLSProxyPort:      16002,
					}))
				})

				It("passes the proxy ports in the config", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())
					megatron.StepsRunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
						return nil
					}), nil, nil)
					Expect(containerStore.Run(logger, "some-trace-id", containerGuid)).NotTo(HaveOccurred())
					Eventually(megatron.StepsRunnerCallCount).Should(Equal(1))
					_, _, _, _, cfg := megatron.StepsRunnerArgsForCall(0)
					Expect(cfg.ProxyTLSPorts).To(ConsistOf(uint16(61001), uint16(61002), uint16(containerstore.C2CTLSPort)))
				})

				It("bind mounts envoy", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(gardenClient.CreateCallCount()).To(Equal(1))
					containerSpec := gardenClient.CreateArgsForCall(0)
					Expect(containerSpec.BindMounts).To(ContainElement(garden.BindMount{
						SrcPath: envoySourceDir,
						DstPath: "/etc/cf-assets/envoy",
						Mode:    garden.BindMountModeRO,
						Origin:  garden.BindMountOriginHost,
					}))

					Expect(containerSpec.BindMounts).To(ContainElement(garden.BindMount{
						SrcPath: filepath.Join(envoyConfigDir, containerGuid),
						DstPath: "/etc/cf-assets/envoy_config",
						Mode:    garden.BindMountModeRO,
						Origin:  garden.BindMountOriginHost,
					}))
				})
			})

			Context("when creating the container fails", func() {
				BeforeEach(func() {
					gardenClient.CreateStub = func(garden.ContainerSpec) (garden.Container, error) {
						clock.Increment(1 * time.Second)
						return nil, errors.New("boom!")
					}
				})

				It("returns an error", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).To(Equal(errors.New("boom!")))
				})

				It("transitions to a completed state", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).To(Equal(errors.New("boom!")))

					container, err := containerStore.Get(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(container.State).To(Equal(executor.StateCompleted))
					Expect(container.RunResult.Failed).To(BeTrue())
					Expect(container.RunResult.FailureReason).To(ContainSubstring(containerstore.ContainerCreationFailedMessage))
					Expect(container.RunResult.FailureReason).To(ContainSubstring("boom!"))
					Expect(container.RunResult.Retryable).To(BeTrue())
				})

				It("emits a metric after failing to create the container", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).To(HaveOccurred())
					Eventually(getMetrics).Should(HaveKey(containerstore.GardenContainerCreationFailedDuration))
				})

				It("logs that the reason the container failed to create", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).To(HaveOccurred())

					Expect(metronClient.SendAppErrorLogCallCount()).To(Equal(1))
					msg, sourceType, tags := metronClient.SendAppErrorLogArgsForCall(0)
					Expect(msg).To(Equal(fmt.Sprintf("Cell %s failed to create container for instance %s: boom!", cellID, containerGuid)))
					Expect(tags["source_id"]).To(Equal(logGuid))
					Expect(sourceType).To(Equal("test-source"))
					Expect(tags["instance_id"]).To(Equal("1"))
				})

				It("logs the total time it took to create the container before it failed", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).To(HaveOccurred())
					Eventually(logger).Should(gbytes.Say("container-setup-failed.*duration.*1000000000"))
				})

				It("emits metric on the total time it took to create the container before it failed", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).To(HaveOccurred())
					Eventually(getMetrics).Should(HaveKey(steps.ContainerSetupFailedDuration))
				})
			})

			Context("when requesting the container info for the created container fails", func() {
				BeforeEach(func() {
					gardenContainer.InfoStub = func() (garden.ContainerInfo, error) {
						return garden.ContainerInfo{}, errors.New("could not obtain info")
					}
				})

				It("returns an error", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).To(HaveOccurred())

					Expect(gardenClient.DestroyCallCount()).To(Equal(1))
					Expect(gardenClient.DestroyArgsForCall(0)).To(Equal(containerGuid))
				})

				It("transitions to a completed state", func() {
					_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
					Expect(err).To(HaveOccurred())

					container, err := containerStore.Get(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(container.State).To(Equal(executor.StateCompleted))
					Expect(container.RunResult.Failed).To(BeTrue())
					Expect(container.RunResult.FailureReason).To(ContainSubstring(containerstore.ContainerCreationFailedMessage))
					Expect(container.RunResult.FailureReason).To(ContainSubstring("could not obtain info"))
					Expect(container.RunResult.Retryable).To(BeTrue())
				})
			})
		})

		Context("when the container does not exist", func() {
			It("returns a conatiner not found error", func() {
				_, err := containerStore.Create(logger, "some-trace-id", "bogus-guid")
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})

		Context("when the container is not initializing", func() {
			BeforeEach(func() {
				_, err := containerStore.Reserve(logger, "some-trace-id", allocationReq)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an invalid state transition error", func() {
				_, err := containerStore.Create(logger, "some-trace-id", containerGuid)
				Expect(err).To(Equal(executor.ErrInvalidTransition))
			})
		})
	})

	Describe("Run", func() {
		var (
			allocationReq *executor.AllocationRequest
			runProcess    *gardenfakes.FakeProcess
		)

		BeforeEach(func() {
			allocationReq = &executor.AllocationRequest{
				Guid:               containerGuid,
				GenerateLogMetrics: true,
			}

			runProcess = &gardenfakes.FakeProcess{}

			gardenContainer.RunReturns(runProcess, nil)
			gardenClient.CreateReturns(gardenContainer, nil)
			gardenClient.LookupReturns(gardenContainer, nil)
		})

		Context("when it is in the created state", func() {
			var (
				runReq *executor.RunRequest
			)

			BeforeEach(func() {
				runAction := &models.Action{
					RunAction: &models.RunAction{
						Path: "/foo/bar",
					},
				}

				runReq = &executor.RunRequest{
					Guid: containerGuid,
					RunInfo: executor.RunInfo{
						Action: runAction,
						LogConfig: models.LogConfig{
							Guid: containerGuid,
						},
						LogRateLimitBytesPerSecond: logRateUnlimitedBytesPerSecond,
					},
				}

				var err error
				_, err = os.MkdirTemp("", "envoy_dir")
				Expect(err).NotTo(HaveOccurred())
				_, err = os.MkdirTemp("", "envoy_config_dir")
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				_, err := containerStore.Reserve(logger, "some-trace-id", allocationReq)
				Expect(err).NotTo(HaveOccurred())

				err = containerStore.Initialize(logger, runReq)
				Expect(err).NotTo(HaveOccurred())

				_, err = containerStore.Create(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("while the cred manager is still setting up", func() {
				var (
					cmFinishSetup            chan struct{}
					containerRunnerCalled    chan struct{}
					credManagerBindMount     []garden.BindMount
					credManagerSignalled     chan struct{}
					containerRunnerSignalled chan struct{}
				)

				BeforeEach(func() {
					cmFinishSetup = make(chan struct{}, 1)
					containerRunnerCalled = make(chan struct{})

					credManagerSignalled = make(chan struct{}, 1)
					containerRunnerSignalled = make(chan struct{}, 1)

					credManagerBindMount = []garden.BindMount{
						{
							SrcPath: "hpath1",
							DstPath: "cpath1",
							Mode:    garden.BindMountModeRO,
							Origin:  garden.BindMountOriginHost,
						},
					}
					envVariables := []executor.EnvironmentVariable{
						{Name: "CF_INSTANCE_CERT", Value: "some-cert"},
					}
					credManager.CreateCredDirReturns(credManagerBindMount, envVariables, nil)

					credManager.RunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
						<-cmFinishSetup
						close(ready)
						<-signals
						credManagerSignalled <- struct{}{}
						return nil
					}))

					megatron.StepsRunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
						close(containerRunnerCalled)
						<-signals
						containerRunnerSignalled <- struct{}{}
						return nil
					}), nil, nil)
				})

				AfterEach(func() {
					close(cmFinishSetup)
					Expect(containerStore.Destroy(logger, "some-trace-id", containerGuid)).To(Succeed())
				})

				It("does not start the container while cred manager is setting up", func() {
					go containerStore.Run(logger, "some-trace-id", containerGuid)
					Consistently(containerRunnerCalled).ShouldNot(BeClosed())
				})

				Context("when cred manager is finished setting up", func() {
					BeforeEach(func() {
						cmFinishSetup <- struct{}{}
					})

					It("starts the container", func() {
						go containerStore.Run(logger, "some-trace-id", containerGuid)
						Eventually(containerRunnerCalled).Should(BeClosed())
					})
				})
			})

			Context("when the runner fails the initial credential generation", func() {
				BeforeEach(func() {
					credManager.RunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
						return errors.New("BOOOM")
					}))
				})

				It("destroys the container and returns an error", func() {
					err := containerStore.Run(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() executor.State {
						container, err := containerStore.Get(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())
						return container.State
					}).Should(Equal(executor.StateCompleted))
					container, _ := containerStore.Get(logger, containerGuid)
					Expect(container.RunResult.Failed).To(BeTrue())
					// make sure the error message is at the end so that
					// FailureReasonSanitizer can properly map the error messages
					Expect(container.RunResult.FailureReason).To(MatchRegexp("cred-manager-runner exited: BOOOM$"))
					Expect(container.RunResult.Retryable).To(BeFalse())
				})

				It("tranistions immediately to Completed state", func() {
					err := containerStore.Run(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() executor.State {
						container, err := containerStore.Get(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())
						return container.State
					}).Should(Equal(executor.StateCompleted))

					Eventually(func() []string {
						var events []string
						for i := 0; i < eventEmitter.EmitCallCount(); i++ {
							event := eventEmitter.EmitArgsForCall(i)
							events = append(events, string(event.EventType()))
						}
						return events
					}).Should(ConsistOf("container_reserved", "container_complete"))
				})
			})

			Context("when instance credential is ready", func() {
				var (
					containerRunnerCalled   chan struct{}
					credManagerRunnerCalled chan struct{}
				)

				BeforeEach(func() {
					credManagerRunnerCalled = make(chan struct{})
					cmCalled := credManagerRunnerCalled
					credManager.RunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
						close(ready)
						close(cmCalled)
						<-signals
						return nil
					}))

					containerRunnerCalled = make(chan struct{})

					megatron.StepsRunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
						close(containerRunnerCalled)
						close(ready)
						<-signals
						return nil
					}), nil, nil)
				})

				AfterEach(func() {
					containerStore.Destroy(logger, "some-trace-id", containerGuid)
				})

				Context("when the runner fails subsequent credential generation", func() {
					BeforeEach(func() {
						credManager.RunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
							close(ready)
							select {
							case <-containerRunnerCalled:
								return errors.New("BOOOM")
							case <-signals:
								return nil
							}
						}))
					})

					It("destroys the container and returns an error", func() {
						err := containerStore.Run(logger, "some-trace-id", containerGuid)
						Expect(err).NotTo(HaveOccurred())

						Eventually(func() executor.State {
							container, err := containerStore.Get(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())
							return container.State
						}).Should(Equal(executor.StateCompleted))
						container, _ := containerStore.Get(logger, containerGuid)
						Expect(container.RunResult.Failed).To(BeTrue())
						// make sure the error message is at the end so that
						// FailureReasonSanitizer can properly map the error messages
						Expect(container.RunResult.FailureReason).To(MatchRegexp("BOOOM$"))
						Expect(container.RunResult.Retryable).To(BeFalse())
					})
				})

				Context("when the action runs indefinitely", func() {
					var readyChan chan struct{}
					var readinessCheckChan chan steps.ReadinessState

					BeforeEach(func() {
						readyChan = make(chan struct{})
						readinessCheckChan = nil
					})

					JustBeforeEach(func() {
						var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
							readyChan <- struct{}{}
							close(ready)
							<-signals
							return nil
						}
						megatron.StepsRunnerReturns(testRunner, readinessCheckChan, nil)
					})

					It("performs the step", func() {
						err := containerStore.Run(logger, "some-trace-id", containerGuid)
						Expect(err).NotTo(HaveOccurred())

						Expect(megatron.StepsRunnerCallCount()).To(Equal(1))
						Eventually(readyChan).Should(Receive())
					})

					It("sets the metric reporting interval", func() {
						err := containerStore.Run(logger, "some-trace-id", containerGuid)
						Expect(err).NotTo(HaveOccurred())

						Eventually(readyChan).Should(Receive())

						Expect(logManager.NewLogStreamerCallCount()).To(Equal(1))
						_, _, _, _, metricReportInterval := logManager.NewLogStreamerArgsForCall(0)
						Expect(metricReportInterval).To(Equal(time.Duration(30 * time.Second)))
					})

					Context("when GenerateLogMetrics is false", func() {
						BeforeEach(func() {
							allocationReq.GenerateLogMetrics = false
						})

						It("sets the metricReportInterval to zero", func() {
							err := containerStore.Run(logger, "some-trace-id", containerGuid)
							Expect(err).NotTo(HaveOccurred())

							Eventually(readyChan).Should(Receive())

							Expect(logManager.NewLogStreamerCallCount()).To(Equal(1))
							_, _, _, _, metricReportInterval := logManager.NewLogStreamerArgsForCall(0)
							Expect(metricReportInterval).To(Equal(time.Duration(0 * time.Second)))
						})
					})

					It("sets the container state to running once the healthcheck passes, and emits a running event", func() {
						err := containerStore.Run(logger, "some-trace-id", containerGuid)
						Expect(err).NotTo(HaveOccurred())

						container, err := containerStore.Get(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())
						Expect(container.State).To(Equal(executor.StateCreated))

						Eventually(readyChan).Should(Receive())

						Eventually(func() executor.State {
							container, err := containerStore.Get(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())
							return container.State
						}).Should(Equal(executor.StateRunning))

						container, err = containerStore.Get(logger, containerGuid)
						Expect(err).NotTo(HaveOccurred())

						Eventually(eventEmitter.EmitCallCount).Should(Equal(2))
						event := eventEmitter.EmitArgsForCall(1)
						Expect(event).To(Equal(executor.NewContainerRunningEvent(container, "some-trace-id")))
					})

					Context("when there is no readiness check defined", func() {
						BeforeEach(func() {
							runReq.RunInfo.CheckDefinition = nil
						})

						It("initializes routable as true", func() {
							err := containerStore.Run(logger, "some-trace-id", containerGuid)
							Expect(err).NotTo(HaveOccurred())

							Eventually(readyChan).Should(Receive())

							Eventually(func() executor.State {
								container, err := containerStore.Get(logger, containerGuid)
								Expect(err).NotTo(HaveOccurred())
								return container.State
							}).Should(Equal(executor.StateRunning))

							container, err := containerStore.Get(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())
							Expect(container.Routable).To(BeTrue())

							Eventually(eventEmitter.EmitCallCount).Should(Equal(2))
							event := eventEmitter.EmitArgsForCall(0)
							Expect(event.EventType()).To(Equal(executor.EventTypeContainerReserved))
							event = eventEmitter.EmitArgsForCall(1)
							Expect(event.EventType()).To(Equal(executor.EventTypeContainerRunning))
							Expect(event).To(Equal(executor.NewContainerRunningEvent(container, "some-trace-id")))
						})
					})

					Context("when there is a readiness check defined", func() {
						JustBeforeEach(func() {
							runReq.RunInfo.CheckDefinition = &models.CheckDefinition{
								ReadinessChecks: []*models.Check{
									{
										HttpCheck: &models.HTTPCheck{
											Port: 8989,
										},
									},
								},
							}
						})

						Context("when the container gets a message from the readinessChan", func() {
							BeforeEach(func() {
								readinessCheckChan = make(chan steps.ReadinessState)
							})

							JustBeforeEach(func() {
								err := containerStore.Run(logger, "some-trace-id", containerGuid)
								Expect(err).NotTo(HaveOccurred())

								Expect(megatron.StepsRunnerCallCount()).To(Equal(1))
								Eventually(readyChan).Should(Receive())

								Eventually(logger).Should(gbytes.Say("healthcheck-passed"))
							})

							Context("when the message is IsNotReady", func() {
								JustBeforeEach(func() {
									readinessCheckChan <- steps.IsNotReady
								})

								It("logs", func() {
									Eventually(logger).Should(gbytes.Say("readiness-healthcheck-failed"))
								})

								It("emits a container not ready event", func() {
									Eventually(eventEmitter.EmitCallCount).Should(Equal(3))
									event := eventEmitter.EmitArgsForCall(0)
									Expect(event.EventType()).To(Equal(executor.EventTypeContainerReserved))
									event = eventEmitter.EmitArgsForCall(1)
									Expect(event.EventType()).To(Equal(executor.EventTypeContainerRunning))

									container, err := containerStore.Get(logger, containerGuid)
									Expect(err).NotTo(HaveOccurred())
									Expect(container.Routable).To(BeFalse())

									Expect(event).To(Equal(executor.NewContainerRunningEvent(container, "some-trace-id")))

									event = eventEmitter.EmitArgsForCall(2)
									Expect(event.EventType()).To(Equal(executor.EventTypeContainerRunning))
									Expect(event).To(Equal(executor.NewContainerRunningEvent(container, "some-trace-id")))
								})
							})

							Context("when the message is IsReady", func() {
								JustBeforeEach(func() {
									readinessCheckChan <- steps.IsReady
								})

								It("Logs", func() {
									Eventually(logger).Should(gbytes.Say("readiness-healthcheck-passed"))
								})

								It("does not emit an event", func() {
									Eventually(eventEmitter.EmitCallCount).Should(Equal(3))
									event := eventEmitter.EmitArgsForCall(0)
									Expect(event.EventType()).To(Equal(executor.EventTypeContainerReserved))
									event = eventEmitter.EmitArgsForCall(1)
									Expect(event.EventType()).To(Equal(executor.EventTypeContainerRunning))

									container, err := containerStore.Get(logger, containerGuid)
									Expect(err).NotTo(HaveOccurred())
									Expect(container.Routable).To(BeTrue())

									firstContainerEvent := container.Copy()
									firstContainerEvent.Routable = false

									Expect(event).To(Equal(executor.NewContainerRunningEvent(firstContainerEvent, "some-trace-id")))

									event = eventEmitter.EmitArgsForCall(2)
									Expect(event.EventType()).To(Equal(executor.EventTypeContainerRunning))
									Expect(event).To(Equal(executor.NewContainerRunningEvent(container, "some-trace-id")))
								})
							})

							Context("when the message is IsReady followed by IsNotReady", func() {
								JustBeforeEach(func() {
									readinessCheckChan <- steps.IsReady
									readinessCheckChan <- steps.IsNotReady
								})

								It("Logs both (listener does not exit the channel)", func() {
									Eventually(logger).Should(gbytes.Say("readiness-healthcheck-passed"))
									Eventually(logger).Should(gbytes.Say("readiness-healthcheck-failed"))
								})

								It("emits a container not ready event", func() {
									container, err := containerStore.Get(logger, containerGuid)
									Expect(err).NotTo(HaveOccurred())
									Expect(container.Routable).To(Equal(false))

									Eventually(eventEmitter.EmitCallCount).Should(Equal(4))
									event := eventEmitter.EmitArgsForCall(3)
									Expect(event.EventType()).To(Equal(executor.EventTypeContainerRunning))
								})

							})
						})
					})
				})

				Context("when the action exits", func() {
					var (
						completeChan chan struct{}
					)

					BeforeEach(func() {
						completeChan = make(chan struct{})
					})

					Context("successfully", func() {
						BeforeEach(func() {
							var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
								close(ready)
								<-completeChan
								return nil
							}
							megatron.StepsRunnerReturns(testRunner, nil, nil)
						})

						It("sets its state to completed", func() {
							err := containerStore.Run(logger, "some-trace-id", containerGuid)
							Expect(err).NotTo(HaveOccurred())

							close(completeChan)

							Eventually(containerState(containerGuid)).Should(Equal(executor.StateCompleted))

							container, err := containerStore.Get(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())
							Expect(container.State).To(Equal(executor.StateCompleted))
						})

						It("emits a container completed event", func() {
							err := containerStore.Run(logger, "some-trace-id", containerGuid)
							Expect(err).NotTo(HaveOccurred())

							Eventually(containerState(containerGuid)).Should(Equal(executor.StateRunning))
							close(completeChan)
							Eventually(containerState(containerGuid)).Should(Equal(executor.StateCompleted))

							Expect(eventEmitter.EmitCallCount()).To(Equal(3))

							container, err := containerStore.Get(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())

							emittedEvents := []executor.Event{}
							for i := 0; i < eventEmitter.EmitCallCount(); i++ {
								emittedEvents = append(emittedEvents, eventEmitter.EmitArgsForCall(i))
							}

							Expect(emittedEvents).To(ContainElement(executor.NewContainerCompleteEvent(container, "some-trace-id")))
						})

						It("sets the result on the container", func() {
							err := containerStore.Run(logger, "some-trace-id", containerGuid)
							Expect(err).NotTo(HaveOccurred())

							close(completeChan)

							Eventually(containerState(containerGuid)).Should(Equal(executor.StateCompleted))

							container, err := containerStore.Get(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())
							Expect(container.RunResult.Failed).To(Equal(false))
							Expect(container.RunResult.Stopped).To(Equal(false))
							Expect(container.RunResult.Retryable).To(BeFalse())
						})

						It("increments the ContainerCompletedCount metric", func() {
							err := containerStore.Run(logger, "some-trace-id", containerGuid)
							Expect(err).NotTo(HaveOccurred())

							close(completeChan)

							Eventually(containerState(containerGuid)).Should(Equal(executor.StateCompleted))
							Expect(metronClient.IncrementCounterCallCount()).To(Equal(1))

							Expect(metronClient.IncrementCounterArgsForCall(0)).To(Equal(containerstore.ContainerCompletedCount))
						})
					})

					Context("unsuccessfully", func() {
						BeforeEach(func() {
							var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
								close(ready)
								return errors.New("BOOOOM!!!!")
							}
							megatron.StepsRunnerReturns(testRunner, nil, nil)
						})

						It("sets the run result on the container", func() {
							err := containerStore.Run(logger, "some-trace-id", containerGuid)
							Expect(err).NotTo(HaveOccurred())

							Eventually(containerState(containerGuid)).Should(Equal(executor.StateCompleted))

							container, err := containerStore.Get(logger, containerGuid)
							Expect(err).NotTo(HaveOccurred())
							Expect(container.RunResult.Failed).To(Equal(true))
							// make sure the error message is at the end so that
							// FailureReasonSanitizer can properly map the error messages
							Expect(container.RunResult.FailureReason).To(MatchRegexp("BOOOOM!!!!$"))
							Expect(container.RunResult.Stopped).To(Equal(false))
							Expect(container.RunResult.Retryable).To(BeFalse())
						})

						It("increments the ContainerCompletedCount metric", func() {
							err := containerStore.Run(logger, "some-trace-id", containerGuid)
							Expect(err).NotTo(HaveOccurred())

							Eventually(containerState(containerGuid)).Should(Equal(executor.StateCompleted))
							Expect(metronClient.IncrementCounterCallCount()).To(Equal(1))

							Expect(metronClient.IncrementCounterArgsForCall(0)).To(Equal(containerstore.ContainerCompletedCount))
						})

						Context("when run fails with ErrExceededGracefulShutdownInterval", func() {
							BeforeEach(func() {
								var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
									close(ready)
									return new(steps.ExceededGracefulShutdownIntervalError)
								}
								megatron.StepsRunnerReturns(testRunner, nil, nil)
							})

							It("increments the ContainerExitedOnTimeoutCount and ContainerCompletedCount metric", func() {
								err := containerStore.Run(logger, "some-trace-id", containerGuid)
								Expect(err).NotTo(HaveOccurred())

								Eventually(containerState(containerGuid)).Should(Equal(executor.StateCompleted))
								Expect(metronClient.IncrementCounterCallCount()).To(Equal(2))

								Expect(metronClient.IncrementCounterArgsForCall(0)).To(Equal(containerstore.ContainerExitedOnTimeoutCount))
								Expect(metronClient.IncrementCounterArgsForCall(1)).To(Equal(containerstore.ContainerCompletedCount))
							})
						})

						Context("when run fails with ErrExitTimeout", func() {
							BeforeEach(func() {
								var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
									close(ready)
									return new(steps.ExitTimeoutError)
								}
								megatron.StepsRunnerReturns(testRunner, nil, nil)
							})

							It("increments the ContainerExitedOnTimeoutCount and ContainerCompletedCount metric", func() {
								err := containerStore.Run(logger, "some-trace-id", containerGuid)
								Expect(err).NotTo(HaveOccurred())

								Eventually(containerState(containerGuid)).Should(Equal(executor.StateCompleted))
								Expect(metronClient.IncrementCounterCallCount()).To(Equal(2))

								Expect(metronClient.IncrementCounterArgsForCall(0)).To(Equal(containerstore.ContainerExitedOnTimeoutCount))
								Expect(metronClient.IncrementCounterArgsForCall(1)).To(Equal(containerstore.ContainerCompletedCount))
							})
						})

						Context("when run fails with a complex error that contains a graceful shutdown error", func() {
							BeforeEach(func() {
								aggregate := &multierror.Error{}
								aggregate = multierror.Append(aggregate, errors.New("some-other-error"))
								aggregate = multierror.Append(aggregate, &steps.ExceededGracefulShutdownIntervalError{})
								var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
									close(ready)
									return grouper.ErrorTrace{
										{
											Member: grouper.Member{Name: "foo"}, Err: errors.New("foo"),
										},
										{
											Member: grouper.Member{Name: "runner"}, Err: aggregate,
										},
									}
								}
								megatron.StepsRunnerReturns(testRunner, nil, nil)
							})

							It("increments the ContainerExitedOnTimeoutCount and ContainerCompletedCount metric", func() {
								err := containerStore.Run(logger, "some-trace-id", containerGuid)
								Expect(err).NotTo(HaveOccurred())

								Eventually(containerState(containerGuid)).Should(Equal(executor.StateCompleted))
								Expect(metronClient.IncrementCounterCallCount()).To(Equal(2))

								Expect(metronClient.IncrementCounterArgsForCall(0)).To(Equal(containerstore.ContainerExitedOnTimeoutCount))
								Expect(metronClient.IncrementCounterArgsForCall(1)).To(Equal(containerstore.ContainerCompletedCount))
							})

							Context("when there are multiple graceful shutdown exceeded errors", func() {
								BeforeEach(func() {
									var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
										close(ready)
										return grouper.ErrorTrace{
											{
												Member: grouper.Member{Name: "runner"}, Err: &multierror.Error{
													Errors: []error{errors.New("some-other-error"), new(steps.ExitTimeoutError), new(steps.ExceededGracefulShutdownIntervalError)},
												},
											},
										}
									}
									megatron.StepsRunnerReturns(testRunner, nil, nil)
								})

								It("only increments the graceful shutdown exceeded metric once", func() {
									err := containerStore.Run(logger, "some-trace-id", containerGuid)
									Expect(err).NotTo(HaveOccurred())

									Eventually(containerState(containerGuid)).Should(Equal(executor.StateCompleted))
									Expect(metronClient.IncrementCounterCallCount()).To(Equal(2))
								})
							})
						})
					})
				})

				Context("when the transformer fails to generate steps", func() {
					BeforeEach(func() {
						megatron.StepsRunnerReturns(nil, nil, errors.New("defeated by the auto bots"))
					})

					It("returns an error", func() {
						err := containerStore.Run(logger, "some-trace-id", containerGuid)
						Expect(err).To(HaveOccurred())
					})
				})
			})
		})

		Context("when the container does not exist", func() {
			It("returns an ErrContainerNotFound error", func() {
				err := containerStore.Run(logger, "some-trace-id", containerGuid)
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})

		Context("When the container is not in the created state", func() {
			JustBeforeEach(func() {
				_, err := containerStore.Reserve(logger, "some-trace-id", allocationReq)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a transition error", func() {
				err := containerStore.Run(logger, "some-trace-id", containerGuid)
				Expect(err).To(Equal(executor.ErrInvalidTransition))
			})
		})
	})

	Context("when the container creation is hung", func() {
		var blockCh chan struct{}

		BeforeEach(func() {
			blockCh = make(chan struct{})

			// avoid race detector warnings
			ch := blockCh
			gardenClient.CreateStub = func(garden.ContainerSpec) (garden.Container, error) {
				<-ch
				return &gardenfakes.FakeContainer{}, nil
			}
		})

		JustBeforeEach(func() {
			_, err := containerStore.Reserve(logger, "some-trace-id", &executor.AllocationRequest{Guid: containerGuid})
			Expect(err).NotTo(HaveOccurred())

			runInfo := executor.RunInfo{
				LogConfig: models.LogConfig{
					Guid:       containerGuid,
					Index:      1,
					SourceName: "test-source",
				},
				LogRateLimitBytesPerSecond: logRateUnlimitedBytesPerSecond,
			}
			runReq := &executor.RunRequest{Guid: containerGuid, RunInfo: runInfo}
			err = containerStore.Initialize(logger, runReq)
			Expect(err).NotTo(HaveOccurred())

			go containerStore.Create(logger, "some-trace-id", containerGuid)
			Eventually(gardenClient.CreateCallCount).Should(Equal(1))
		})

		AfterEach(func() {
			close(blockCh)
		})

		Context("when Destroy has been called", func() {
			JustBeforeEach(func() {
				blockCh := make(chan error)
				go func() {
					blockCh <- containerStore.Destroy(logger, "some-trace-id", containerGuid)
				}()
				Consistently(blockCh, time.Second).ShouldNot(Receive())
			})

			It("should return immediately", func() {
				errCh := make(chan error)
				go func() {
					errCh <- containerStore.Destroy(logger, "some-trace-id", containerGuid)
				}()
				Eventually(errCh).Should(Receive(BeNil()))
			})
		})

		Context("when Stop has been called and is hanging", func() {
			JustBeforeEach(func() {
				blockCh := make(chan error)
				go func() {
					blockCh <- containerStore.Stop(logger, "some-trace-id", containerGuid)
				}()
				Consistently(blockCh, time.Second).ShouldNot(Receive())
			})

			It("should return immediately", func() {
				errCh := make(chan error)
				go func() {
					errCh <- containerStore.Stop(logger, "some-trace-id", containerGuid)
				}()
				Eventually(errCh).Should(Receive(BeNil()))
			})
		})
	})

	Describe("Update", func() {
		var (
			runReq         *executor.RunRequest
			updateReq      *executor.UpdateRequest
			internalRoutes models.InternalRoutes
			metricTags     map[string]string
		)

		BeforeEach(func() {
			var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				<-signals
				return nil
			}
			runInfo := executor.RunInfo{
				LogConfig: models.LogConfig{
					Guid:       containerGuid,
					Index:      1,
					SourceName: "test-source",
					Tags: map[string]string{
						"old-log-tag": "old-value",
					},
				},
				MetricsConfig: executor.MetricsConfig{
					Guid:  containerGuid,
					Index: 1,
					Tags: map[string]string{
						"old-metrics-tag": "old-value",
					},
				},
				LogRateLimitBytesPerSecond: logRateUnlimitedBytesPerSecond,
				InternalRoutes: models.InternalRoutes{
					{Hostname: "old.apps.internal"},
				},
			}
			runReq = &executor.RunRequest{Guid: containerGuid, RunInfo: runInfo}
			gardenClient.CreateReturns(gardenContainer, nil)
			megatron.StepsRunnerReturns(testRunner, nil, nil)
			internalRoutes = models.InternalRoutes{
				{Hostname: "a.apps.internal"},
				{Hostname: "b.apps.internal"},
			}
			metricTags = map[string]string{"some-tag": "some-value"}
			updateReq = &executor.UpdateRequest{Guid: containerGuid, InternalRoutes: internalRoutes, MetricTags: metricTags}
		})

		JustBeforeEach(func() {
			_, err := containerStore.Reserve(logger, "some-trace-id", &executor.AllocationRequest{Guid: containerGuid})
			Expect(err).NotTo(HaveOccurred())

			err = containerStore.Initialize(logger, runReq)
			Expect(err).NotTo(HaveOccurred())

			_, err = containerStore.Create(logger, "some-trace-id", containerGuid)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the container exists", func() {
			JustBeforeEach(func() {

				err := containerStore.Run(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates container internal routes", func() {
				err := containerStore.Update(logger, updateReq)
				Expect(err).NotTo(HaveOccurred())

				container, err := containerStore.Get(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(container.InternalRoutes).To(Equal(internalRoutes))
			})

			It("sends a signal to the credmanager to regenerate certs", func() {
				_, _, regenerateCertsCh := credManager.RunnerArgsForCall(0)

				go func() {
					err := containerStore.Update(logger, updateReq)
					Expect(err).NotTo(HaveOccurred())
				}()
				Eventually(regenerateCertsCh).Should(Receive(Equal(struct{}{})))
			})

			It("updates metric tags", func() {
				err := containerStore.Update(logger, updateReq)
				Expect(err).NotTo(HaveOccurred())

				container, err := containerStore.Get(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(container.MetricsConfig.Tags).To(Equal(metricTags))
			})

			It("updates log tags and the logStreamer", func() {
				err := containerStore.Update(logger, updateReq)
				Expect(err).NotTo(HaveOccurred())

				container, err := containerStore.Get(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(container.LogConfig.Tags).To(Equal(metricTags))

				Expect(logStreamer.UpdateTagsArgsForCall(0)).To(Equal(metricTags))
			})

			It("updates the log config property on the container", func() {
				err := containerStore.Update(logger, updateReq)
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenContainer.SetPropertyCallCount()).To(Equal(1))
				name, value := gardenContainer.SetPropertyArgsForCall(0)
				Expect(name).To(Equal("log_config"))

				var parsedValue map[string]interface{}
				err = json.Unmarshal([]byte(value), &parsedValue)
				Expect(err).NotTo(HaveOccurred())
				Expect(parsedValue["guid"]).To(Equal("container-guid"))
				Expect(parsedValue["index"]).To(Equal(1.0))
				Expect(parsedValue["source_name"]).To(Equal("test-source"))
				Expect(parsedValue["tags"]).To(Equal(map[string]interface{}{"some-tag": "some-value"}))
			})

			Context("when Update is called concurrently", func() {
				JustBeforeEach(func() {
					updateReq.InternalRoutes = nil
				})
				It("locks access to the container info", func() {
					var wg sync.WaitGroup
					wg.Add(2)
					go func() {
						err := containerStore.Update(logger, updateReq)
						Expect(err).NotTo(HaveOccurred())
						wg.Done()
					}()
					go func() {
						err := containerStore.Update(logger, updateReq)
						Expect(err).NotTo(HaveOccurred())
						wg.Done()
					}()
					wg.Wait()
				})
			})

			Context("when the log config cannot be serialized", func() {
				var fm failingMarshaller

				BeforeEach(func() {
					containerStore = containerstore.New(
						containerConfig,
						&totalCapacity,
						gardenClientFactory,
						dependencyManager,
						volumeManager,
						credManager,
						logManager,
						clock,
						eventEmitter,
						megatron,
						"/var/vcap/data/cf-system-trusted-certs",
						metronClient,
						rootFSSizer,
						"/var/vcap/packages/healthcheck",
						proxyManager,
						cellID,
						true,
						advertisePreferenceForInstanceAddress,
						volumeMountedFilesHandler,
						fm.Marshal,
					)
				})

				It("releases the info lock", func() {
					fm.fail = true
					err := containerStore.Update(logger, updateReq)
					Expect(err).To(HaveOccurred())

					fm.fail = false
					err = containerStore.Update(logger, updateReq)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when there is an error updating the log config property on the container", func() {
				BeforeEach(func() {
					gardenContainer.SetPropertyReturns(errors.New("some-error"))
				})
				It("logs the error", func() {
					err := containerStore.Update(logger, updateReq)
					Expect(err).To(HaveOccurred())

					Expect(gardenContainer.SetPropertyCallCount()).To(Equal(1))
					Expect(logger).To(gbytes.Say("failed-to-set-log-config-property"))
					Expect(logger).To(gbytes.Say(updateReq.Guid))
					Expect(logger).To(gbytes.Say("some-error"))
				})
			})

			Context("when internal routes are not provided", func() {
				BeforeEach(func() {
					updateReq.InternalRoutes = nil
				})
				It("does not update container internal routes", func() {
					err := containerStore.Update(logger, updateReq)
					Expect(err).NotTo(HaveOccurred())

					container, err := containerStore.Get(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(container.InternalRoutes).To(Equal(models.InternalRoutes{
						{Hostname: "old.apps.internal"},
					}))
				})
				It("does not send a signal to the credmanager to regenerate certs", func() {
					_, _, regenerateCertsCh := credManager.RunnerArgsForCall(0)

					go func() {
						err := containerStore.Update(logger, updateReq)
						Expect(err).NotTo(HaveOccurred())
					}()
					Consistently(regenerateCertsCh, 100*time.Millisecond).ShouldNot(Receive(Equal(struct{}{})))
				})
			})

			Context("when metric tags are not provided", func() {
				BeforeEach(func() {
					updateReq.MetricTags = nil
				})

				It("does not update container metric tags", func() {
					err := containerStore.Update(logger, updateReq)
					Expect(err).NotTo(HaveOccurred())

					container, err := containerStore.Get(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(container.LogConfig.Tags).To(Equal(map[string]string{
						"old-log-tag": "old-value",
					}))
					Expect(container.MetricsConfig.Tags).To(Equal(map[string]string{
						"old-metrics-tag": "old-value",
					}))

					Expect(logStreamer.UpdateTagsCallCount()).To(Equal(0))
				})

				It("does not update the log config property on the container", func() {
					err := containerStore.Update(logger, updateReq)
					Expect(err).NotTo(HaveOccurred())

					Expect(gardenContainer.SetPropertyCallCount()).To(Equal(0))
				})
			})

		})

		Context("when the container does not exist", func() {
			It("returns an ErrContainerNotFound", func() {
				updateReq.Guid = "unknown-guid"
				err := containerStore.Update(logger, updateReq)
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})
	})

	Describe("Stop", func() {
		var (
			runReq *executor.RunRequest
		)

		BeforeEach(func() {
			var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				<-signals
				return nil
			}
			runInfo := executor.RunInfo{
				LogConfig: models.LogConfig{
					Guid:       containerGuid,
					Index:      1,
					SourceName: "test-source",
				},
				LogRateLimitBytesPerSecond: logRateUnlimitedBytesPerSecond,
			}
			runReq = &executor.RunRequest{Guid: containerGuid, RunInfo: runInfo}
			gardenClient.CreateReturns(gardenContainer, nil)
			megatron.StepsRunnerReturns(testRunner, nil, nil)
		})

		JustBeforeEach(func() {
			_, err := containerStore.Reserve(logger, "some-trace-id", &executor.AllocationRequest{Guid: containerGuid})
			Expect(err).NotTo(HaveOccurred())

			err = containerStore.Initialize(logger, runReq)
			Expect(err).NotTo(HaveOccurred())

			_, err = containerStore.Create(logger, "some-trace-id", containerGuid)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the container has processes associated with it", func() {
			JustBeforeEach(func() {
				err := containerStore.Run(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())
			})

			It("sets stopped to true on the run result", func() {
				err := containerStore.Stop(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())

				container, err := containerStore.Get(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(container.RunResult.Stopped).To(BeTrue())
				Expect(container.RunResult.Retryable).To(BeFalse())
			})

			It("logs that the container is stopping", func() {
				err := containerStore.Stop(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(metronClient.SendAppLogCallCount()).To(Equal(3))
				msg, sourceType, tags := metronClient.SendAppLogArgsForCall(2)
				Expect(tags["source_id"]).To(Equal(containerGuid))
				Expect(sourceType).To(Equal("test-source"))
				Expect(msg).To(Equal(fmt.Sprintf("Cell %s stopping instance %s", cellID, containerGuid)))
				Expect(tags["instance_id"]).To(Equal("1"))
			})
		})

		Context("when the container does not have processes associated with it", func() {
			It("transitions to the completed state", func() {
				err := containerStore.Stop(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())

				container, err := containerStore.Get(logger, containerGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(container.RunResult.Stopped).To(BeTrue())
				Expect(container.State).To(Equal(executor.StateCompleted))
				Expect(container.RunResult.Retryable).To(BeFalse())
			})
		})

		Context("when the container does not exist", func() {
			It("returns an ErrContainerNotFound", func() {
				err := containerStore.Stop(logger, "some-trace-id", "")
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})
	})

	Describe("Destroy", func() {
		var resource executor.Resource
		var expectedMounts containerstore.BindMounts
		var runReq *executor.RunRequest

		BeforeEach(func() {
			runInfo := executor.RunInfo{
				LogConfig: models.LogConfig{
					Guid:       containerGuid,
					Index:      1,
					SourceName: "test-source",
				},
				LogRateLimitBytesPerSecond: logRateUnlimitedBytesPerSecond,
			}
			runReq = &executor.RunRequest{Guid: containerGuid, RunInfo: runInfo}
			gardenClient.CreateReturns(gardenContainer, nil)
			resource = executor.NewResource(1024, 2048, 1024)
			expectedMounts = containerstore.BindMounts{
				CacheKeys: []containerstore.BindMountCacheKey{
					{CacheKey: "cache-key", Dir: "foo"},
				},
			}
			dependencyManager.DownloadCachedDependenciesReturns(expectedMounts, nil)
		})

		JustBeforeEach(func() {
			_, err := containerStore.Reserve(logger, "some-trace-id", &executor.AllocationRequest{Guid: containerGuid, Resource: resource})
			Expect(err).NotTo(HaveOccurred())

			err = containerStore.Initialize(logger, runReq)
			Expect(err).NotTo(HaveOccurred())

			_, err = containerStore.Create(logger, "some-trace-id", containerGuid)
			Expect(err).NotTo(HaveOccurred())
		})

		It("cleans up the credentials dir", func() {
			err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
			Expect(err).NotTo(HaveOccurred())
			Expect(credManager.RemoveCredDirCallCount()).To(Equal(1))
		})

		Context("when there are volumes mounted", func() {
			BeforeEach(func() {
				someConfig := map[string]interface{}{"some-config": "interface"}
				runReq.RunInfo.VolumeMounts = []executor.VolumeMount{
					executor.VolumeMount{ContainerPath: "cpath1", Driver: "some-driver", VolumeId: "some-volume", Config: someConfig},
					executor.VolumeMount{ContainerPath: "cpath2", Driver: "some-other-driver", VolumeId: "some-other-volume", Config: someConfig},
				}
				count := 0
				volumeManager.MountStub = // first call mounts at a different point than second call
					func(lager.Logger, string, string, string, map[string]interface{}) (volman.MountResponse, error) {
						defer func() { count = count + 1 }()
						if count == 0 {
							return volman.MountResponse{Path: "hpath1"}, nil
						}
						return volman.MountResponse{Path: "hpath2"}, nil
					}
			})

			It("removes mounted volumes on the host machine", func() {
				err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(volumeManager.UnmountCallCount()).To(Equal(2))

				_, driverId, volumeId, containerId := volumeManager.UnmountArgsForCall(0)
				Expect(driverId).To(Equal(runReq.RunInfo.VolumeMounts[0].Driver))
				Expect(volumeId).To(Equal(runReq.RunInfo.VolumeMounts[0].VolumeId))
				Expect(containerId).To(Equal(containerGuid))

				_, driverId, volumeId, containerId = volumeManager.UnmountArgsForCall(1)
				Expect(driverId).To(Equal(runReq.RunInfo.VolumeMounts[1].Driver))
				Expect(volumeId).To(Equal(runReq.RunInfo.VolumeMounts[1].VolumeId))
				Expect(containerId).To(Equal(containerGuid))
			})

			Context("when we fail to release cache dependencies", func() {
				BeforeEach(func() {
					dependencyManager.ReleaseCachedDependenciesReturns(errors.New("oh noes!"))
				})
				It("still attempts to unmount our volumes", func() {
					err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
					Expect(err).To(HaveOccurred())
					Expect(volumeManager.UnmountCallCount()).To(Equal(2))

					_, driverId, volumeId, containerId := volumeManager.UnmountArgsForCall(0)
					Expect(driverId).To(Equal(runReq.RunInfo.VolumeMounts[0].Driver))
					Expect(volumeId).To(Equal(runReq.RunInfo.VolumeMounts[0].VolumeId))
					Expect(containerId).To(Equal(containerGuid))

					_, driverId, volumeId, containerId = volumeManager.UnmountArgsForCall(1)
					Expect(driverId).To(Equal(runReq.RunInfo.VolumeMounts[1].Driver))
					Expect(volumeId).To(Equal(runReq.RunInfo.VolumeMounts[1].VolumeId))
					Expect(containerId).To(Equal(containerGuid))
				})
			})
			Context("when an volume unmount fails", func() {
				BeforeEach(func() {
					volumeManager.UnmountReturns(errors.New("oh noes!"))
				})

				It("still attempts to unmount the remaining volumes", func() {
					err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(volumeManager.UnmountCallCount()).To(Equal(2))
				})
			})
			Context("when garden fails to destroy the container", func() {
				BeforeEach(func() {
					gardenClient.DestroyReturns(errors.New("destroy failed"))
				})
				It("should still unmount the volumes", func() {
					err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
					Expect(err).To(MatchError("destroy failed"))
					Expect(volumeManager.UnmountCallCount()).To(Equal(2))
				})
			})
		})

		It("removes downloader cache references", func() {
			err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
			Expect(err).NotTo(HaveOccurred())
			Expect(dependencyManager.ReleaseCachedDependenciesCallCount()).To(Equal(1))
			_, keys := dependencyManager.ReleaseCachedDependenciesArgsForCall(0)
			Expect(keys).To(Equal(expectedMounts.CacheKeys))
		})

		It("destroys the container", func() {
			err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
			Expect(err).NotTo(HaveOccurred())

			Expect(gardenClient.DestroyCallCount()).To(Equal(1))
			Expect(gardenClient.DestroyArgsForCall(0)).To(Equal(containerGuid))

			_, err = containerStore.Get(logger, containerGuid)
			Expect(err).To(Equal(executor.ErrContainerNotFound))
		})

		It("emits a metric after destroying the container", func() {
			err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
			Expect(err).NotTo(HaveOccurred())

			Eventually(getMetrics).Should(HaveKey(containerstore.GardenContainerDestructionSucceededDuration))
		})

		It("frees the containers resources", func() {
			err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
			Expect(err).NotTo(HaveOccurred())

			remainingResources := containerStore.RemainingResources(logger)
			Expect(remainingResources).To(Equal(totalCapacity))
		})

		Context("when destroying the garden container fails", func() {
			var destroyErr error
			BeforeEach(func() {
				destroyErr = errors.New("failed to destroy garden container")
			})

			JustBeforeEach(func() {
				gardenClient.DestroyReturns(destroyErr)
			})

			Context("because the garden container does not exist", func() {
				BeforeEach(func() {
					destroyErr = garden.ContainerNotFoundError{}
				})

				It("does not return an error", func() {
					err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("because the garden container is already being destroyed", func() {
				BeforeEach(func() {
					destroyErr = server.ErrConcurrentDestroy
				})

				It("does not return an error", func() {
					err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())
				})

				It("logs the container is destroyed", func() {
					err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(metronClient.SendAppLogCallCount()).To(Equal(4))
					msg, sourceType, tags := metronClient.SendAppLogArgsForCall(2)
					Expect(tags["source_id"]).To(Equal(containerGuid))
					Expect(sourceType).To(Equal("test-source"))
					Expect(msg).To(Equal(fmt.Sprintf("Cell %s destroying container for instance %s", cellID, containerGuid)))
					Expect(tags["instance_id"]).To(Equal("1"))

					msg, sourceType, tags = metronClient.SendAppLogArgsForCall(3)
					Expect(tags["source_id"]).To(Equal(containerGuid))
					Expect(sourceType).To(Equal("test-source"))
					Expect(msg).To(Equal(fmt.Sprintf("Cell %s successfully destroyed container for instance %s", cellID, containerGuid)))
					Expect(tags["instance_id"]).To(Equal("1"))
				})
			})

			Context("for unknown reason", func() {
				It("returns an error", func() {
					err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
					Expect(err).To(Equal(destroyErr))
				})

				It("emits a metric after failing to destroy the container", func() {
					err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
					Expect(err).To(Equal(destroyErr))
					Eventually(getMetrics).Should(HaveKey(containerstore.GardenContainerDestructionFailedDuration))
				})

				It("does not remove the container from the container store", func() {
					err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
					Expect(err).To(Equal(destroyErr))

					Expect(gardenClient.DestroyCallCount()).To(Equal(1))
					Expect(gardenClient.DestroyArgsForCall(0)).To(Equal(containerGuid))

					container, err := containerStore.Get(logger, containerGuid)
					Expect(err).ToNot(HaveOccurred())
					Expect(container.Guid).To(Equal(containerGuid))
				})

				It("does not free up the containers resources", func() {
					err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
					Expect(err).To(Equal(destroyErr))

					resources := containerStore.RemainingResources(logger)
					expectedResources := totalCapacity.Copy()
					expectedResources.Subtract(&resource)
					Expect(resources).To(Equal(expectedResources))
				})
			})
		})

		Context("when the container does not exist", func() {
			It("returns a ErrContainerNotFound", func() {
				err := containerStore.Destroy(logger, "some-trace-id", "")
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})

		Context("when there is a stopped process associated with the container", func() {
			var (
				finishRun                  chan struct{}
				credManagerRunnerSignalled chan struct{}
				destroyed                  chan struct{}
			)

			BeforeEach(func() {
				credManagerRunnerSignalled = make(chan struct{})
				finishRun = make(chan struct{})
				finishRunIfrit := finishRun
				var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
					close(ready)
					<-signals
					<-finishRunIfrit
					return nil
				}

				signalled := credManagerRunnerSignalled
				megatron.StepsRunnerReturns(testRunner, nil, nil)
				credManager.RunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
					close(ready)
					<-signals
					close(signalled)
					return nil
				}))
			})

			JustBeforeEach(func() {
				err := containerStore.Run(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Eventually(containerState(containerGuid)).Should(Equal(executor.StateRunning))
				err = containerStore.Stop(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())
				destroyed = make(chan struct{})
				go func(ch chan struct{}) {
					containerStore.Destroy(logger, "some-trace-id", containerGuid)
					close(ch)
				}(destroyed)
			})

			It("cancels the process", func() {
				Consistently(destroyed).ShouldNot(Receive())
				close(finishRun)
				Eventually(destroyed).Should(BeClosed())
			})

			It("logs that the container is stopping once", func() {
				close(finishRun)
				Eventually(destroyed).Should(BeClosed())

				Expect(metronClient.SendAppLogCallCount()).To(Equal(5))
				msg, sourceType, tags := metronClient.SendAppLogArgsForCall(2)
				Expect(tags["source_id"]).To(Equal(containerGuid))
				Expect(sourceType).To(Equal("test-source"))
				Expect(msg).To(Equal(fmt.Sprintf("Cell %s stopping instance %s", cellID, containerGuid)))
				Expect(tags["instance_id"]).To(Equal("1"))
			})
		})

		Context("when there is a running process associated with the container", func() {
			var (
				finishRun                  chan struct{}
				credManagerRunnerSignalled chan struct{}
				destroyed                  chan struct{}
			)

			BeforeEach(func() {
				credManagerRunnerSignalled = make(chan struct{})
				finishRun = make(chan struct{})
				finishRunIfrit := finishRun
				var testRunner ifrit.RunFunc = func(signals <-chan os.Signal, ready chan<- struct{}) error {
					close(ready)
					<-signals
					<-finishRunIfrit
					return nil
				}

				signalled := credManagerRunnerSignalled
				megatron.StepsRunnerReturns(testRunner, nil, nil)
				credManager.RunnerReturns(ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
					close(ready)
					<-signals
					close(signalled)
					return nil
				}))
			})

			JustBeforeEach(func() {
				err := containerStore.Run(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())
				Eventually(containerState(containerGuid)).Should(Equal(executor.StateRunning))
				destroyed = make(chan struct{})
				go func(ch chan struct{}) {
					containerStore.Destroy(logger, "some-trace-id", containerGuid)
					close(ch)
				}(destroyed)
			})

			It("cancels the process", func() {
				Consistently(destroyed).ShouldNot(Receive())
				close(finishRun)
				Eventually(destroyed).Should(BeClosed())
			})

			It("signals the cred manager runner", func() {
				close(finishRun)
				Eventually(credManagerRunnerSignalled).Should(BeClosed())
			})

			It("logs that the container is stopping", func() {
				close(finishRun)
				Eventually(destroyed).Should(BeClosed())
				Expect(metronClient.SendAppLogCallCount()).To(Equal(5))
				msg, sourceType, tags := metronClient.SendAppLogArgsForCall(2)
				Expect(tags["source_id"]).To(Equal(containerGuid))
				Expect(sourceType).To(Equal("test-source"))
				Expect(msg).To(Equal(fmt.Sprintf("Cell %s stopping instance %s", cellID, containerGuid)))
				Expect(tags["instance_id"]).To(Equal("1"))
			})

			Context("when the container credManager config process is running in the container", func() {
				var (
					credManagerRunnerSignalled chan struct{}
				)

				BeforeEach(func() {
					credManagerRunnerSignalled = make(chan struct{})

					containerStore = containerstore.New(
						containerConfig,
						&totalCapacity,
						gardenClientFactory,
						dependencyManager,
						volumeManager,
						credManager,
						logManager,
						clock,
						eventEmitter,
						megatron,
						"/var/vcap/data/cf-system-trusted-certs",
						metronClient,
						rootFSSizer,
						"/var/vcap/packages/healthcheck",
						proxyManager,
						cellID,
						true,
						advertisePreferenceForInstanceAddress,
						volumeMountedFilesHandler,
						json.Marshal,
					)

					signalled := credManagerRunnerSignalled
					runner := &fake_runner.FakeRunner{}
					runner.RunStub = ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
						close(ready)
						<-signals
						close(signalled)
						return nil
					})
					credManager.RunnerReturns(runner)
				})

				It("signals the credManager config runner", func() {
					close(finishRun)
					Eventually(credManagerRunnerSignalled).Should(BeClosed())
				})
			})
		})

		Context("when there are no process associated with the container", func() {
			Context("when container is in completed state", func() {
				JustBeforeEach(func() {
					destroyErr := errors.New("failed-to-destroy-container")
					gardenClient.DestroyReturns(destroyErr)
					eventEmitter.EmitStub = func(e executor.Event) {
						// processing completed container
						defer GinkgoRecover()
						if e.EventType() == executor.EventTypeContainerComplete {
							// sleeping to avoid atomic destroying state
							time.Sleep(time.Second)
							err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
							Expect(err).To(HaveOccurred())
						}
					}
				})

				It("does not emit the completed event", func() {
					// completed event will trigger Destroy for failed completed container and that will cause an infinite loop
					Eventually(eventEmitter.EmitCallCount).Should(Equal(1))
					event1 := eventEmitter.EmitArgsForCall(0)
					Expect(event1.EventType()).To(Equal(executor.EventTypeContainerReserved))

					err := containerStore.Destroy(logger, "some-trace-id", containerGuid)
					Expect(err).To(HaveOccurred())

					container, err := containerStore.Get(logger, containerGuid)
					Expect(err).NotTo(HaveOccurred())

					Eventually(eventEmitter.EmitCallCount).Should(Equal(2))
					event2 := eventEmitter.EmitArgsForCall(1)
					Expect(event2).To(Equal(executor.NewContainerCompleteEvent(container, "some-trace-id")))

					Consistently(eventEmitter.EmitCallCount).Should(Equal(2))
				})
			})
		})
	})

	Describe("Get", func() {
		BeforeEach(func() {
			_, err := containerStore.Reserve(logger, "some-trace-id", &executor.AllocationRequest{Guid: containerGuid})
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the specified container", func() {
			container, err := containerStore.Get(logger, containerGuid)
			Expect(err).NotTo(HaveOccurred())

			Expect(container.Guid).To(Equal(containerGuid))
		})

		Context("when the container does not exist", func() {
			It("returns an ErrContainerNotFound", func() {
				_, err := containerStore.Get(logger, "")
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})
	})

	Describe("List", func() {
		var container1, container2 executor.Container

		BeforeEach(func() {
			_, err := containerStore.Reserve(logger, "some-trace-id", &executor.AllocationRequest{Guid: containerGuid})

			Expect(err).NotTo(HaveOccurred())

			err = containerStore.Initialize(logger, &executor.RunRequest{
				Guid: containerGuid,
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = containerStore.Reserve(logger, "some-trace-id", &executor.AllocationRequest{
				Guid: containerGuid + "2",
			})
			Expect(err).NotTo(HaveOccurred())

			container1, err = containerStore.Get(logger, containerGuid)
			Expect(err).NotTo(HaveOccurred())

			container2, err = containerStore.Get(logger, containerGuid+"2")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the list of known containers", func() {
			containers := containerStore.List(logger)
			Expect(containers).To(HaveLen(2))
			Expect(containers).To(ContainElement(container1))
			Expect(containers).To(ContainElement(container2))
		})
	})

	reserveContainer := func(guid string) {
		resource := executor.Resource{
			MemoryMB: 10,
			DiskMB:   10,
		}
		tags := executor.Tags{}
		_, err := containerStore.Reserve(logger, "some-trace-id", &executor.AllocationRequest{Guid: guid, Tags: tags, Resource: resource})
		Expect(err).NotTo(HaveOccurred())
	}

	initializeContainer := func(guid string, rootFSPath string) {
		runInfo := executor.RunInfo{
			RootFSPath:         rootFSPath,
			CPUWeight:          2,
			StartTimeoutMs:     50000,
			Privileged:         true,
			CachedDependencies: []executor.CachedDependency{},
			LogConfig: models.LogConfig{
				Guid:       "log-guid",
				Index:      1,
				SourceName: "test-source",
			},
			MetricsConfig: executor.MetricsConfig{
				Guid:  "metric-guid",
				Index: 1,
			},
			Env:                           []executor.EnvironmentVariable{},
			TrustedSystemCertificatesPath: "",
			Network: &executor.Network{
				Properties: map[string]string{},
			},
		}

		req := &executor.RunRequest{
			Guid:    guid,
			RunInfo: runInfo,
			Tags:    executor.Tags{},
		}

		err := containerStore.Initialize(logger, req)
		Expect(err).ToNot(HaveOccurred())
	}

	Describe("Metrics", func() {
		var (
			containerGuid1, containerGuid2, containerGuid3, containerGuid4 string
			containerGuid5, containerGuid6                                 string
		)

		BeforeEach(func() {
			rootFSSizer.RootFSSizeFromPathStub = func(path string) uint64 {
				switch path {
				case "/foo/bar":
					return 1000
				default:
					return 0
				}
			}

			containerGuid1 = "container-guid-1"
			containerGuid2 = "container-guid-2"
			containerGuid3 = "container-guid-3"
			containerGuid4 = "container-guid-4"
			containerGuid5 = "container-guid-5"
			containerGuid6 = "container-guid-6"

			reserveContainer(containerGuid1)
			reserveContainer(containerGuid2)
			reserveContainer(containerGuid3)
			reserveContainer(containerGuid4)
			reserveContainer(containerGuid5)
			reserveContainer(containerGuid6)

			initializeContainer(containerGuid1, "/foo/bar")
			initializeContainer(containerGuid2, "docker://repo")
			initializeContainer(containerGuid3, "/foo/bar")
			initializeContainer(containerGuid4, "/foo/bar")

			gardenContainer.InfoReturns(garden.ContainerInfo{ExternalIP: "6.6.6.6"}, nil)
			gardenClient.CreateReturns(gardenContainer, nil)
			_, err := containerStore.Create(logger, "some-trace-id", containerGuid1)
			Expect(err).NotTo(HaveOccurred())
			_, err = containerStore.Create(logger, "some-trace-id", containerGuid2)
			Expect(err).ToNot(HaveOccurred())
			_, err = containerStore.Create(logger, "some-trace-id", containerGuid3)
			Expect(err).ToNot(HaveOccurred())
			_, err = containerStore.Create(logger, "some-trace-id", containerGuid4)
			Expect(err).ToNot(HaveOccurred())

			networkStat1 := garden.ContainerNetworkStat{
				RxBytes: uint64(42),
				TxBytes: uint64(43),
			}
			bulkMetrics := map[string]garden.ContainerMetricsEntry{
				containerGuid1: garden.ContainerMetricsEntry{
					Metrics: garden.Metrics{
						MemoryStat: garden.ContainerMemoryStat{
							TotalUsageTowardLimit: 1024,
						},
						DiskStat: garden.ContainerDiskStat{
							TotalBytesUsed: uint64(1000 + 2048),
						},
						CPUStat: garden.ContainerCPUStat{
							Usage: 5000000000,
						},
						Age:            1000000000,
						CPUEntitlement: 100,
						NetworkStat:    &networkStat1,
					},
				},
				containerGuid2: garden.ContainerMetricsEntry{
					Metrics: garden.Metrics{
						MemoryStat: garden.ContainerMemoryStat{
							TotalUsageTowardLimit: 512,
						},
						DiskStat: garden.ContainerDiskStat{
							TotalBytesUsed: 256,
						},
						CPUStat: garden.ContainerCPUStat{
							Usage: 1000000,
						},
						Age:            2000000000,
						CPUEntitlement: 200,
						NetworkStat:    nil,
					},
				},
				containerGuid4: garden.ContainerMetricsEntry{
					Err: &garden.Error{Err: errors.New("no-metrics-here")},
					Metrics: garden.Metrics{
						MemoryStat: garden.ContainerMemoryStat{
							TotalUsageTowardLimit: 512,
						},
						DiskStat: garden.ContainerDiskStat{
							TotalBytesUsed: uint64(1000 + 128),
						},
						CPUStat: garden.ContainerCPUStat{
							Usage: 1000000,
						},
						Age:            4000000000,
						CPUEntitlement: 300,
					},
				},
				"BOGUS-GUID": garden.ContainerMetricsEntry{},
			}
			gardenClient.BulkMetricsReturns(bulkMetrics, nil)
		})

		It("returns metrics for all known containers in the running and created state", func() {
			metrics, err := containerStore.Metrics(logger)
			Expect(err).NotTo(HaveOccurred())
			containerSpec1 := gardenClient.CreateArgsForCall(0)
			containerSpec2 := gardenClient.CreateArgsForCall(1)

			Expect(gardenClient.BulkMetricsCallCount()).To(Equal(1))
			Expect(gardenClient.BulkMetricsArgsForCall(0)).To(ConsistOf(
				containerGuid1, containerGuid2, containerGuid3, containerGuid4,
			))

			Expect(metrics).To(HaveLen(2))

			container1Metrics, ok := metrics[containerGuid1]
			Expect(ok).To(BeTrue())
			Expect(container1Metrics.MemoryUsageInBytes).To(BeEquivalentTo(1024))
			Expect(container1Metrics.DiskUsageInBytes).To(BeEquivalentTo(2048))
			Expect(container1Metrics.MemoryLimitInBytes).To(BeEquivalentTo(containerSpec1.Limits.Memory.LimitInBytes))
			Expect(container1Metrics.DiskLimitInBytes).To(BeEquivalentTo(10 * 1024 * 1024))
			Expect(container1Metrics.TimeSpentInCPU).To(Equal(5 * time.Second))
			Expect(container1Metrics.ContainerAgeInNanoseconds).To(Equal(uint64(1000000000)))
			Expect(container1Metrics.AbsoluteCPUEntitlementInNanoseconds).To(Equal(uint64(100)))
			Expect(*container1Metrics.RxInBytes).To(Equal(uint64(42)))
			Expect(*container1Metrics.TxInBytes).To(Equal(uint64(43)))

			container2Metrics, ok := metrics[containerGuid2]
			Expect(ok).To(BeTrue())
			Expect(container2Metrics.MemoryUsageInBytes).To(BeEquivalentTo(512))
			Expect(container2Metrics.DiskUsageInBytes).To(BeEquivalentTo(256))
			Expect(container2Metrics.MemoryLimitInBytes).To(BeEquivalentTo(containerSpec2.Limits.Memory.LimitInBytes))
			Expect(container2Metrics.DiskLimitInBytes).To(BeEquivalentTo(10 * 1024 * 1024))
			Expect(container2Metrics.TimeSpentInCPU).To(Equal(1 * time.Millisecond))
			Expect(container2Metrics.ContainerAgeInNanoseconds).To(Equal(uint64(2000000000)))
			Expect(container2Metrics.AbsoluteCPUEntitlementInNanoseconds).To(Equal(uint64(200)))
			Expect(container2Metrics.RxInBytes).To(BeNil())
			Expect(container2Metrics.TxInBytes).To(BeNil())
		})

		Context("when fetching bulk metrics fails", func() {
			BeforeEach(func() {
				gardenClient.BulkMetricsReturns(nil, errors.New("failed-bulk-metrics"))
			})

			It("returns an error", func() {
				_, err := containerStore.Metrics(logger)
				Expect(err).To(Equal(errors.New("failed-bulk-metrics")))
			})
		})
	})

	Describe("GetFiles", func() {
		BeforeEach(func() {
			gardenClient.CreateReturns(gardenContainer, nil)
			gardenContainer.StreamOutReturns(io.NopCloser(bytes.NewReader([]byte("this is the stream"))), nil)
		})

		JustBeforeEach(func() {
			_, err := containerStore.Reserve(logger, "some-trace-id", &executor.AllocationRequest{Guid: containerGuid})
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the container has a corresponding garden container", func() {
			JustBeforeEach(func() {
				err := containerStore.Initialize(logger, &executor.RunRequest{Guid: containerGuid})
				Expect(err).NotTo(HaveOccurred())

				_, err = containerStore.Create(logger, "some-trace-id", containerGuid)
				Expect(err).NotTo(HaveOccurred())
			})

			It("calls streamout on the garden client", func() {
				stream, err := containerStore.GetFiles(logger, containerGuid, "/path/to/file")
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenContainer.StreamOutCallCount()).To(Equal(1))
				streamOutSpec := gardenContainer.StreamOutArgsForCall(0)
				Expect(streamOutSpec.Path).To(Equal("/path/to/file"))
				Expect(streamOutSpec.User).To(Equal("root"))

				output := make([]byte, len("this is the stream"))
				_, err = stream.Read(output)
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(Equal([]byte("this is the stream")))
			})
		})

		Context("when the container does not have a corresponding garden container", func() {
			It("returns an error", func() {
				_, err := containerStore.GetFiles(logger, containerGuid, "/path")
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})

		Context("when the container does not exist", func() {
			It("returns ErrContainerNotFound", func() {
				_, err := containerStore.GetFiles(logger, "", "/stuff")
				Expect(err).To(Equal(executor.ErrContainerNotFound))
			})
		})
	})

	Describe("RegistryPruner", func() {
		var (
			expirationTime time.Duration
			process        ifrit.Process
			resource       executor.Resource
		)

		BeforeEach(func() {
			resource = executor.NewResource(512, 512, 1024)
			req := executor.NewAllocationRequest("forever-reserved", &resource, true, nil)

			_, err := containerStore.Reserve(logger, "some-trace-id", &req)
			Expect(err).NotTo(HaveOccurred())

			resource = executor.NewResource(512, 512, 1024)
			req = executor.NewAllocationRequest("eventually-initialized", &resource, true, nil)

			_, err = containerStore.Reserve(logger, "some-trace-id", &req)
			Expect(err).NotTo(HaveOccurred())

			runReq := executor.NewRunRequest("eventually-initialized", &executor.RunInfo{}, executor.Tags{})
			err = containerStore.Initialize(logger, &runReq)
			Expect(err).NotTo(HaveOccurred())

			expirationTime = 20 * time.Millisecond

			pruner := containerStore.NewRegistryPruner(logger)
			process = ginkgomon.Invoke(pruner)
		})

		AfterEach(func() {
			ginkgomon.Interrupt(process)
		})

		Context("when the elapsed time is less than expiration period", func() {
			BeforeEach(func() {
				clock.Increment(expirationTime / 2)
			})

			It("still has all the containers in the list", func() {
				Consistently(func() []executor.Container {
					return containerStore.List(logger)
				}).Should(HaveLen(2))

				resources := containerStore.RemainingResources(logger)
				expectedResources := totalCapacity.Copy()
				expectedResources.Subtract(&resource)
				expectedResources.Subtract(&resource)
				Expect(resources).To(Equal(expectedResources))
			})
		})

		Context("when the elapsed time is more than expiration period", func() {
			BeforeEach(func() {
				clock.Increment(2 * expirationTime)
			})

			It("completes only RESERVED containers from the list", func() {
				Eventually(func() executor.State {
					container, err := containerStore.Get(logger, "forever-reserved")
					Expect(err).NotTo(HaveOccurred())
					return container.State
				}).Should(Equal(executor.StateCompleted))

				Consistently(func() executor.State {
					container, err := containerStore.Get(logger, "eventually-initialized")
					Expect(err).NotTo(HaveOccurred())
					return container.State
				}).ShouldNot(Equal(executor.StateCompleted))
			})
		})
	})

	Describe("ContainerReaper", func() {
		var (
			containerGuid1, containerGuid2, containerGuid3 string
			containerGuid4, containerGuid5, containerGuid6 string
			process                                        ifrit.Process
			extraGardenContainer                           *gardenfakes.FakeContainer
		)

		BeforeEach(func() {
			gardenClient.CreateReturns(gardenContainer, nil)

			containerGuid1 = "container-guid-1"
			containerGuid2 = "container-guid-2"
			containerGuid3 = "container-guid-3"
			containerGuid4 = "container-guid-4"
			containerGuid5 = "container-guid-5"
			containerGuid6 = "container-guid-6"

			// Reserve
			_, err := containerStore.Reserve(logger, "some-trace-id", &executor.AllocationRequest{Guid: containerGuid1})
			Expect(err).NotTo(HaveOccurred())
			_, err = containerStore.Reserve(logger, "some-trace-id", &executor.AllocationRequest{Guid: containerGuid2})
			Expect(err).NotTo(HaveOccurred())
			_, err = containerStore.Reserve(logger, "some-trace-id", &executor.AllocationRequest{Guid: containerGuid3})
			Expect(err).NotTo(HaveOccurred())
			_, err = containerStore.Reserve(logger, "some-trace-id", &executor.AllocationRequest{Guid: containerGuid4})
			Expect(err).NotTo(HaveOccurred())
			_, err = containerStore.Reserve(logger, "some-trace-id", &executor.AllocationRequest{Guid: containerGuid5})
			Expect(err).NotTo(HaveOccurred())
			_, err = containerStore.Reserve(logger, "some-trace-id", &executor.AllocationRequest{Guid: containerGuid6})
			Expect(err).NotTo(HaveOccurred())

			// Initialize
			err = containerStore.Initialize(logger, &executor.RunRequest{Guid: containerGuid2})
			Expect(err).NotTo(HaveOccurred())
			err = containerStore.Initialize(logger, &executor.RunRequest{Guid: containerGuid3})
			Expect(err).NotTo(HaveOccurred())
			err = containerStore.Initialize(logger, &executor.RunRequest{Guid: containerGuid4})
			Expect(err).NotTo(HaveOccurred())
			err = containerStore.Initialize(logger, &executor.RunRequest{Guid: containerGuid5})
			Expect(err).NotTo(HaveOccurred())
			err = containerStore.Initialize(logger, &executor.RunRequest{Guid: containerGuid6})
			Expect(err).NotTo(HaveOccurred())

			// Create Containers
			_, err = containerStore.Create(logger, "some-trace-id", containerGuid3)
			Expect(err).NotTo(HaveOccurred())
			_, err = containerStore.Create(logger, "some-trace-id", containerGuid4)
			Expect(err).NotTo(HaveOccurred())
			_, err = containerStore.Create(logger, "some-trace-id", containerGuid5)
			Expect(err).NotTo(HaveOccurred())

			// Stop One of the containers
			err = containerStore.Stop(logger, "some-trace-id", containerGuid6)
			Expect(err).NotTo(HaveOccurred())

			Eventually(eventEmitter.EmitCallCount).Should(Equal(7))

			extraGardenContainer = &gardenfakes.FakeContainer{}
			extraGardenContainer.HandleReturns("foobar")
			gardenContainer.HandleReturns(containerGuid3)
			gardenContainers := []garden.Container{gardenContainer, extraGardenContainer}
			gardenClient.ContainersReturns(gardenContainers, nil)
		})

		JustBeforeEach(func() {
			reaper := containerStore.NewContainerReaper(logger)
			process = ginkgomon.Invoke(reaper)
		})

		AfterEach(func() {
			ginkgomon.Interrupt(process)
		})

		It("marks containers completed that no longer have corresponding garden containers", func() {
			initialEmitCallCount := eventEmitter.EmitCallCount()

			clock.WaitForWatcherAndIncrement(30 * time.Millisecond)

			Eventually(func() executor.State {
				container, err := containerStore.Get(logger, containerGuid4)
				Expect(err).NotTo(HaveOccurred())
				return container.State
			}).Should(Equal(executor.StateCompleted))

			Eventually(func() executor.State {
				container, err := containerStore.Get(logger, containerGuid5)
				Expect(err).NotTo(HaveOccurred())
				return container.State
			}).Should(Equal(executor.StateCompleted))

			Eventually(eventEmitter.EmitCallCount).Should(Equal(initialEmitCallCount + 2))

			container4, err := containerStore.Get(logger, containerGuid4)
			Expect(err).NotTo(HaveOccurred())
			container5, err := containerStore.Get(logger, containerGuid5)
			Expect(err).NotTo(HaveOccurred())

			var events []executor.Event
			events = append(events, eventEmitter.EmitArgsForCall(initialEmitCallCount))
			events = append(events, eventEmitter.EmitArgsForCall(initialEmitCallCount+1))

			Expect(events).To(ContainElement(executor.ContainerCompleteEvent{RawContainer: container4}))
			Expect(events).To(ContainElement(executor.ContainerCompleteEvent{RawContainer: container5}))

			Expect(gardenClient.ContainersCallCount()).To(Equal(2))

			properties := gardenClient.ContainersArgsForCall(0)
			Expect(properties[executor.ContainerOwnerProperty]).To(Equal(ownerName))
			properties = gardenClient.ContainersArgsForCall(1)
			Expect(properties[executor.ContainerOwnerProperty]).To(Equal(ownerName))

			clock.WaitForWatcherAndIncrement(30 * time.Millisecond)

			Eventually(gardenClient.ContainersCallCount).Should(Equal(4))

			Eventually(logger).Should(gbytes.Say("reaped-missing-container"))
		})

		Context("when garden's list of containers is stale (ie. a container was created since obtaining the list from garden)", func() {
			var syncCh chan struct{}

			BeforeEach(func() {
				syncCh = make(chan struct{})
				gardenClient.ContainersStub = func(garden.Properties) ([]garden.Container, error) {
					<-syncCh
					return []garden.Container{gardenContainer, extraGardenContainer}, nil
				}
			})

			getContainerState := func(guid string) func() executor.State {
				return func() executor.State {
					container, err := containerStore.Get(logger, guid)
					Expect(err).NotTo(HaveOccurred())
					return container.State
				}
			}

			It("does not reap the newly created container", func() {
				clock.WaitForWatcherAndIncrement(30 * time.Millisecond)

				// reap extra garden containers first
				syncCh <- struct{}{}

				// wait for reap missing containers to be called
				Eventually(gardenClient.ContainersCallCount).Should(Equal(2))

				newContainerGuid := "new-container-guid"
				_, err := containerStore.Reserve(logger, "some-trace-id", &executor.AllocationRequest{Guid: newContainerGuid})
				Expect(err).NotTo(HaveOccurred())
				err = containerStore.Initialize(logger, &executor.RunRequest{Guid: newContainerGuid})
				Expect(err).NotTo(HaveOccurred())
				_, err = containerStore.Create(logger, "some-trace-id", newContainerGuid)
				Expect(err).NotTo(HaveOccurred())

				Eventually(getContainerState(newContainerGuid)).Should(Equal(executor.StateCreated))

				// continue reaping missing garden containers
				syncCh <- struct{}{}

				// new container should not be reaped despite garden's list of containers not containing the newly created container
				Consistently(getContainerState(newContainerGuid)).ShouldNot(Equal(executor.StateCompleted))
			})
		})

		Context("when listing containers in garden fails", func() {
			BeforeEach(func() {
				gardenClient.ContainersReturns([]garden.Container{}, errors.New("failed-to-list"))
			})

			It("logs the failure and continues", func() {
				clock.Increment(30 * time.Millisecond)
				Eventually(logger).Should(gbytes.Say("failed-to-fetch-containers"))

				Consistently(func() []executor.Container {
					return containerStore.List(logger)
				}).Should(HaveLen(6))
			})
		})

		Context("when destroying the extra container fails", func() {
			BeforeEach(func() {
				gardenClient.DestroyReturns(errors.New("failed-to-destroy"))
			})

			It("logs the error and continues", func() {
				clock.Increment(30 * time.Millisecond)
				Eventually(logger).Should(gbytes.Say("failed-to-destroy-container"))
			})
		})
	})
})

type failingMarshaller struct {
	fail bool
}

func (m *failingMarshaller) Marshal(v any) ([]byte, error) {
	if m.fail {
		return []byte{}, errors.New("marshalling failed")
	}
	return json.Marshal(v)
}
