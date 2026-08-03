package steps_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/depot/log_streamer/fake_log_streamer"
	"code.cloudfoundry.org/executor/depot/steps"
	"code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("RunAction", func() {
	var (
		step ifrit.Runner

		runAction                            models.RunAction
		fakeStreamer                         *fake_log_streamer.FakeLogStreamer
		gardenClient                         *fakes.FakeGardenClient
		logger                               *lagertest.TestLogger
		fileDescriptorLimit, processesLimit  uint64
		externalIP, internalIP, internalIPv6 string
		portMappings                         []executor.PortMapping
		fakeClock                            *fakeclock.FakeClock
		suppressExitStatusCode               bool

		spawnedProcess           *gardenfakes.FakeProcess
		runError                 error
		testLogSource            string
		sidecar                  steps.Sidecar
		privileged               bool
		gracefulShutdownInterval = 5 * time.Second
	)

	BeforeEach(func() {
		fileDescriptorLimit = 17
		processesLimit = 1024
		suppressExitStatusCode = false
		testLogSource = "testlogsource"
		sidecar = steps.Sidecar{}

		rl := models.ResourceLimits{}
		rl.SetNofile(fileDescriptorLimit)
		rl.SetNproc(processesLimit)

		runAction = models.RunAction{
			Path: "sudo",
			Args: []string{"reboot"},
			Dir:  "/some-dir",
			Env: []*models.EnvironmentVariable{
				{Name: "A", Value: "1"},
				{Name: "B", Value: "2"},
			},
			ResourceLimits: &rl,
			User:           "notroot",
		}

		fakeStreamer = new(fake_log_streamer.FakeLogStreamer)
		fakeStreamer.StdoutReturns(gbytes.NewBuffer())
		fakeStreamer.StderrReturns(gbytes.NewBuffer())
		fakeStreamer.SourceNameReturns(testLogSource)
		gardenClient = fakes.NewGardenClient()

		logger = lagertest.NewTestLogger("test")

		spawnedProcess = new(gardenfakes.FakeProcess)
		runError = nil

		gardenClient.Connection.RunStub = func(string, garden.ProcessSpec, garden.ProcessIO) (garden.Process, error) {
			return spawnedProcess, runError
		}

		externalIP = "external-ip"
		internalIP = "internal-ip"
		internalIPv6 = "internal-ipv6"
		portMappings = nil
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
	})

	handle := "some-container-handle"

	JustBeforeEach(func() {
		gardenClient.Connection.CreateReturns(handle, nil)

		container, err := gardenClient.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		step = steps.NewRunWithSidecar(
			container,
			runAction,
			fakeStreamer,
			logger,
			externalIP,
			internalIP,
			internalIPv6,
			portMappings,
			fakeClock,
			gracefulShutdownInterval,
			suppressExitStatusCode,
			sidecar,
			privileged,
		)
	})

	Describe("Run", func() {
		var process ifrit.Process

		JustBeforeEach(func() {
			process = ifrit.Background(step)
			Eventually(gardenClient.Connection.RunCallCount).Should(Equal(1))
		})

		Context("when the Garden process succeeds", func() {
			BeforeEach(func() {
				gardenClient.Connection.RunStub = func(string, garden.ProcessSpec, garden.ProcessIO) (garden.Process, error) {
					fakeClock.Increment(time.Minute)
					return spawnedProcess, runError
				}
				spawnedProcess.WaitReturns(0, nil)
			})

			It("does not return an error", func() {
				Eventually(process.Wait()).Should(Receive(BeNil()))
			})

			It("executed the command in the passed-in container", func() {
				ranHandle, spec, _ := gardenClient.Connection.RunArgsForCall(0)
				Expect(ranHandle).To(Equal(handle))
				Expect(spec.ID).To(BeEmpty())
				Expect(spec.Path).To(Equal("sudo"))
				Expect(spec.Args).To(Equal([]string{"reboot"}))
				Expect(spec.Dir).To(Equal("/some-dir"))
				Expect(*spec.Limits.Nofile).To(BeNumerically("==", fileDescriptorLimit))
				Expect(spec.Limits.Nproc).To(BeNil())
				Expect(spec.Env).To(ContainElement("A=1"))
				Expect(spec.Env).To(ContainElement("B=2"))
				Expect(spec.User).To(Equal("notroot"))
			})

			It("logs the step", func() {
				Eventually(logger.TestSink.LogMessages).Should(ConsistOf([]string{
					"test.run-step.running",
					"test.run-step.creating-process",
					"test.run-step.successful-process-create",
					"test.run-step.process-exit",
				}))
			})

			It("logs the duration for process creation", func() {
				Eventually(logger).Should(gbytes.Say("test.run-step.successful-process-create.+\"duration\":%d", time.Minute))
			})

			Context("when a sidecar container is specified", func() {
				var (
					imageRef   garden.ImageRef
					bindMounts []garden.BindMount
				)
				BeforeEach(func() {
					privileged = false
					imageRef = garden.ImageRef{URI: "sandwich"}
					bindMounts = []garden.BindMount{
						{
							SrcPath: "/etc/something",
							DstPath: "/container/something",
						},
					}
					sidecar = steps.Sidecar{
						Image:      imageRef,
						BindMounts: bindMounts,
						Name:       "sandwich",
					}
				})

				It("sets the side car's container handle to its name", func() {
					_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
					Expect(spec.ID).To(Equal(sidecar.Name))
				})

				It("runs the process using that image", func() {
					_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
					Expect(spec.Image).To(Equal(imageRef))
				})

				It("runs the process with the BindMounts", func() {
					_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
					Expect(spec.BindMounts).To(Equal(bindMounts))
				})

				It("runs the process in the same resource namespaces", func() {
					_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
					Expect(spec.OverrideContainerLimits).To(BeNil())
				})

				Context("when requesting a new resource namespace", func() {
					BeforeEach(func() {
						sidecar.OverrideContainerLimits = &garden.ProcessLimits{}
					})

					It("runs the process in a new resource namespace", func() {
						_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
						Expect(spec.OverrideContainerLimits).NotTo(BeNil())
					})
				})
			})

			Context("when the container is privileged", func() {
				BeforeEach(func() {
					privileged = true
				})

				It("does not specify image", func() {
					_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
					Expect(spec.Image).To(BeZero())
				})

				It("does not specify bind mounts", func() {
					_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
					Expect(spec.BindMounts).To(BeZero())
				})
			})
		})

		Context("when the the Garden process errors", func() {
			var waitErr error

			BeforeEach(func() {
				waitErr = errors.New("wait-error")
				spawnedProcess.WaitReturns(0, waitErr)
			})

			Context("when logs are suppressed", func() {
				BeforeEach(func() {
					runAction.SuppressLogOutput = true
				})

				It("returns an error", func() {
					Eventually(process.Wait()).Should(Receive(MatchError(waitErr)))
				})

				It("logs the step", func() {
					Eventually(logger.TestSink.LogMessages).Should(ConsistOf([]string{
						"test.run-step.running",
						"test.run-step.creating-process",
						"test.run-step.successful-process-create",
						"test.run-step.running-error",
					}))

				})
			})

			Context("when logs are not suppressed", func() {
				BeforeEach(func() {
					runAction.SuppressLogOutput = false
				})

				It("returns an error", func() {
					Eventually(process.Wait()).Should(Receive(MatchError(waitErr)))
				})

				It("logs the step", func() {
					Eventually(logger.TestSink.LogMessages).Should(ConsistOf([]string{
						"test.run-step.running",
						"test.run-step.creating-process",
						"test.run-step.successful-process-create",
						"test.run-step.running-error",
					}))

				})
			})

			Context("when Garden Run errors out", func() {
				BeforeEach(func() {
					gardenClient.Connection.RunStub = func(string, garden.ProcessSpec, garden.ProcessIO) (garden.Process, error) {
						return nil, errors.New("some-error-from-garden")
					}
				})

				It("it receives an error from the streamer", func() {
					Expect(fakeStreamer.Stdout()).To(gbytes.Say("failed-creating-process: some-error-from-garden"))
				})
			})
		})

		Context("CF_INSTANCE_* networking env vars", func() {
			It("sets CF_INSTANCE_IP on the container", func() {
				_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
				Expect(spec.Env).To(ContainElement("CF_INSTANCE_IP=external-ip"))
			})

			It("sets CF_INSTANCE_INTERNAL_IP and CF_INSTANCE_INTERNAL_IPV6 on the container", func() {
				_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
				Expect(spec.Env).To(ContainElement("CF_INSTANCE_INTERNAL_IP=internal-ip"))
				Expect(spec.Env).To(ContainElement("CF_INSTANCE_INTERNAL_IPV6=internal-ipv6"))
			})

			Context("when the container has no internal IPv6", func() {
				BeforeEach(func() {
					internalIPv6 = ""
				})

				It("does not set CF_INSTANCE_INTERNAL_IPV6 on the container", func() {
					_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
					Expect(spec.Env).NotTo(ContainElement("CF_INSTANCE_INTERNAL_IPV6="))
				})
			})

			Context("when the container has port mappings configured", func() {
				BeforeEach(func() {
					portMappings = []executor.PortMapping{
						{HostPort: 1, ContainerPort: 2},
						{HostPort: 3, ContainerPort: 4},
					}
				})

				It("sets CF_INSTANCE_* networking env vars", func() {
					_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
					Expect(spec.Env).To(ContainElement("CF_INSTANCE_PORT=1"))
					Expect(spec.Env).To(ContainElement("CF_INSTANCE_ADDR=external-ip:1"))

					var cfPortsValue string
					for _, env := range spec.Env {
						if strings.HasPrefix(env, "CF_INSTANCE_PORTS=") {
							cfPortsValue = strings.Split(env, "=")[1]
							break
						}
					}
					Expect(cfPortsValue).To(MatchJSON("[{\"internal\":2,\"external\":1},{\"internal\":4,\"external\":3}]"))
				})

				Context("and a container proxy is enabled", func() {
					BeforeEach(func() {
						portMappings = []executor.PortMapping{
							{HostPort: 1, ContainerPort: 2, ContainerTLSProxyPort: 5, HostTLSProxyPort: 6},
							{HostPort: 3, ContainerPort: 4, ContainerTLSProxyPort: 7, HostTLSProxyPort: 8},
						}
					})

					It("includes the tls_proxy_ports in CF_INSTANCE_PORTS", func() {
						_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
						var cfPortsValue string
						for _, env := range spec.Env {
							if strings.HasPrefix(env, "CF_INSTANCE_PORTS=") {
								cfPortsValue = strings.Split(env, "=")[1]
								break
							}
						}

						expectedPortsValue := `[{
							"internal":2,
							"external":1,
							"internal_tls_proxy":5,
							"external_tls_proxy":6
						},
						{
							"internal":4,
							"external":3,
							"internal_tls_proxy":7,
							"external_tls_proxy":8
					  }]`

						var aval []interface{}
						var eval []interface{}

						json.Unmarshal([]byte(cfPortsValue), &aval)
						json.Unmarshal([]byte(expectedPortsValue), &eval)
						Expect(eval).To(ContainElement(aval[0]))
						Expect(eval).To(ContainElement(aval[1]))
					})

					Context("and unproxied ports are disabled", func() {
						BeforeEach(func() {
							portMappings = []executor.PortMapping{
								{HostPort: 0, ContainerPort: 2, ContainerTLSProxyPort: 5, HostTLSProxyPort: 6},
							}
						})

						It("does not set CF_INSTANCE_ADDR or CF_INSTANCE_PORT", func() {
							_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
							Expect(spec.Env).NotTo(ContainElement(MatchRegexp("^CF_INSTANCE_PORT=")))
							Expect(spec.Env).NotTo(ContainElement(MatchRegexp("^CF_INSTANCE_ADDR=")))
						})

						It("does not set an 'external' port in CF_INSTANCE_PORTS", func() {
							_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
							var cfPortsValue string
							for _, env := range spec.Env {
								if strings.HasPrefix(env, "CF_INSTANCE_PORTS=") {
									cfPortsValue = strings.Split(env, "=")[1]
									break
								}
							}

							expectedPortsValue := `[{
								"internal":2,
								"internal_tls_proxy":5,
								"external_tls_proxy":6
							}]`

							var actual []interface{}
							var expected []interface{}
							json.Unmarshal([]byte(cfPortsValue), &actual)
							json.Unmarshal([]byte(expectedPortsValue), &expected)

							Expect(actual).To(Equal(expected))
						})
					})
				})
			})

			Context("when the container does not have any port mappings configured", func() {
				BeforeEach(func() {
					portMappings = []executor.PortMapping{}
				})

				It("sets all port-related env vars to the empty string", func() {
					_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
					Expect(spec.Env).To(ContainElement("CF_INSTANCE_PORT="))
					Expect(spec.Env).To(ContainElement("CF_INSTANCE_ADDR="))
					Expect(spec.Env).To(ContainElement("CF_INSTANCE_PORTS=[]"))
				})
			})
		})

		Context("when resource limits are not configured", func() {
			BeforeEach(func() {
				runAction.ResourceLimits = nil
				spawnedProcess.WaitReturns(0, nil)
			})

			It("does not enforce a file descriptor limit on the process", func() {
				_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
				Expect(spec.Limits.Nofile).To(BeNil())
			})

			It("does not enforce a process limit on the process", func() {
				_, spec, _ := gardenClient.Connection.RunArgsForCall(0)
				Expect(spec.Limits.Nproc).To(BeNil())
			})
		})

		Context("when the Garden process has a non-zero exit code", func() {
			BeforeEach(func() {
				spawnedProcess.WaitReturns(19, nil)
			})

			Context("when logs are not suppressed", func() {
				BeforeEach(func() {
					runAction.SuppressLogOutput = false
				})

				It("should return an emittable error with the exit code", func() {
					errMsg := fmt.Sprintf("%s: Exited with status 19", testLogSource)
					Eventually(process.Wait()).Should(Receive(MatchError(steps.NewEmittableError(nil, "%s", errMsg))))
				})
			})

			Context("when logs are suppressed", func() {
				BeforeEach(func() {
					runAction.SuppressLogOutput = true
				})

				It("should return an emittable error with the exit code", func() {
					errMsg := fmt.Sprintf("%s: Exited with status 19", testLogSource)
					Eventually(process.Wait()).Should(Receive(MatchError(steps.NewEmittableError(nil, "%s", errMsg))))
				})
			})
		})

		Context("readiness", func() {
			var waitCh chan struct{}

			BeforeEach(func() {
				waitCh = make(chan struct{})
				gardenClient.Connection.RunStub = func(string, garden.ProcessSpec, garden.ProcessIO) (garden.Process, error) {
					<-waitCh
					return spawnedProcess, nil
				}
			})

			It("becomes ready once the garden process is started", func() {
				Consistently(process.Ready()).ShouldNot(BeClosed())
				close(waitCh)
				Eventually(process.Ready()).Should(BeClosed())
			})
		})

		Context("when Garden errors creating the process", func() {
			disaster := errors.New("I, like, tried but failed")

			BeforeEach(func() {
				runError = disaster
			})

			It("returns the error", func() {
				Eventually(process.Wait()).Should(Receive(MatchError(disaster)))
			})

			It("logs the step", func() {
				Eventually(logger.TestSink.LogMessages).Should(ConsistOf([]string{
					"test.run-step.running",
					"test.run-step.creating-process",
					"test.run-step.failed-creating-process",
				}))
			})

			It("never becomes ready", func() {
				Expect(process.Ready()).ToNot(BeClosed())
			})

			Context("", func() {
				BeforeEach(func() {
					gardenClient.Connection.RunStub = func(string, garden.ProcessSpec, garden.ProcessIO) (garden.Process, error) {
						fakeClock.Increment(time.Minute)
						return spawnedProcess, runError
					}
				})
				It("logs the duration for process creation", func() {
					Eventually(logger).Should(gbytes.Say("test.run-step.failed-creating-process.+\"duration\":%d", time.Minute))
				})
			})
		})

		// Garden-RunC capitalizes out the O in out of memory whereas Garden-linux does not
		Context("regardless of status code, when an Out of memory event has occured", func() {
			BeforeEach(func() {
				gardenClient.Connection.InfoReturns(
					garden.ContainerInfo{
						Events: []string{"happy land", "Out of memory", "another event"},
					},
					nil,
				)

				spawnedProcess.WaitReturns(19, nil)
			})

			It("returns an emittable error", func() {
				errMsg := fmt.Sprintf("%s: Exited with status 19 (out of memory)", testLogSource)
				Eventually(process.Wait()).Should(Receive(MatchError(steps.NewEmittableError(nil, "%s", errMsg))))
			})
		})

		Context("regardless of status code, when an out of memory event has occured", func() {
			BeforeEach(func() {
				gardenClient.Connection.InfoReturns(
					garden.ContainerInfo{
						Events: []string{"happy land", "out of memory", "another event"},
					},
					nil,
				)

				spawnedProcess.WaitReturns(19, nil)
			})

			It("returns an emittable error", func() {
				errMsg := fmt.Sprintf("%s: Exited with status 19 (out of memory)", testLogSource)
				Eventually(process.Wait()).Should(Receive(MatchError(steps.NewEmittableError(nil, "%s", errMsg))))
			})
		})

		Context("when container info cannot be retrieved", func() {
			BeforeEach(func() {
				gardenClient.Connection.InfoReturns(garden.ContainerInfo{}, errors.New("info-error"))
				spawnedProcess.WaitReturns(19, nil)
			})

			It("logs the step", func() {
				Eventually(logger.TestSink.LogMessages).Should(ConsistOf([]string{
					"test.run-step.running",
					"test.run-step.creating-process",
					"test.run-step.successful-process-create",
					"test.run-step.process-exit",
					"test.run-step.failed-to-get-info",
					"test.run-step.run-step-failed-with-nonzero-status-code",
				}))

			})
		})

		Describe("emitting logs", func() {
			BeforeEach(func() {
				spawnedProcess.WaitStub = func() (int, error) {
					_, _, io := gardenClient.Connection.RunArgsForCall(0)

					_, err := io.Stdout.Write([]byte("hi out"))
					Expect(err).NotTo(HaveOccurred())

					_, err = io.Stderr.Write([]byte("hi err"))
					Expect(err).NotTo(HaveOccurred())

					return 34, nil
				}
			})

			JustBeforeEach(func() {
				Eventually(spawnedProcess.WaitCallCount).Should(Equal(1))
			})

			Context("when logs are not suppressed", func() {
				It("emits the output chunks as they come in", func() {
					Expect(fakeStreamer.Stdout()).To(gbytes.Say("hi out"))
					Expect(fakeStreamer.Stderr()).To(gbytes.Say("hi err"))
				})

				It("should flush the output when the code exits", func() {
					Expect(fakeStreamer.FlushCallCount()).To(Equal(1))
				})

				It("emits the exit status code", func() {
					Expect(fakeStreamer.Stdout()).To(gbytes.Say("Exit status 34$"))
				})

				Context("when exit code suppressed for healthcheck", func() {
					BeforeEach(func() {
						suppressExitStatusCode = true
					})

					It("does not emits the exit status code", func() {
						Expect(fakeStreamer.Stdout()).NotTo(gbytes.Say("Exit status 34"))
					})
				})

				Context("when out of memory", func() {
					BeforeEach(func() {
						gardenClient.Connection.InfoReturns(
							garden.ContainerInfo{
								Events: []string{"happy land", "Out of memory", "another event"},
							},
							nil,
						)

						spawnedProcess.WaitReturns(34, nil)
					})

					It("emits the exit status code", func() {
						Eventually(fakeStreamer.Stdout()).Should(gbytes.Say(`Exit status 34 \(out of memory\)`))
					})

					Context("when there are multiple out of memory events", func() {
						BeforeEach(func() {
							gardenClient.Connection.InfoReturns(
								garden.ContainerInfo{
									Events: []string{"Out of memory", "Out of memory", "another event"},
								},
								nil,
							)

							spawnedProcess.WaitReturns(34, nil)
						})

						It("emits only one out of memory error", func() {
							Expect(fakeStreamer.Stdout()).To(gbytes.Say("Exit status 34"))
							Expect(fakeStreamer.Stdout()).To(gbytes.Say(`(out of memory)`))
							Expect(fakeStreamer.Stdout()).ToNot(gbytes.Say(`(out of memory)`))
						})
					})

					Context("when exit code suppressed for healthcheck", func() {
						BeforeEach(func() {
							suppressExitStatusCode = true
						})

						It("does not emits the exit status code", func() {
							Expect(fakeStreamer.Stdout()).ToNot(gbytes.Say("Exit status 34"))
							Expect(fakeStreamer.Stdout()).To(gbytes.Say(`(out of memory)`))
						})
					})
				})
			})

			Context("when logs are suppressed", func() {
				BeforeEach(func() {
					runAction.SuppressLogOutput = true
				})

				It("does not emit the output chunks as they come in", func() {
					Expect(fakeStreamer.Stdout()).ToNot(gbytes.Say("hi out"))
					Expect(fakeStreamer.Stderr()).ToNot(gbytes.Say("hi err"))
				})

				It("does not emit the exit status code", func() {
					Expect(fakeStreamer.Stdout()).ToNot(gbytes.Say("Exit status 34"))
				})
			})
		})
	})

	Describe("Signalling", func() {
		var (
			waiting    chan struct{}
			waitExited chan int
		)

		BeforeEach(func() {
			waitingCh := make(chan struct{})
			waiting = waitingCh

			waitExitedCh := make(chan int, 1)
			waitExited = waitExitedCh

			spawnedProcess.WaitStub = func() (int, error) {
				close(waitingCh)
				return <-waitExitedCh, nil
			}
		})

		Context("when signalling after running the process", func() {
			var process ifrit.Process

			JustBeforeEach(func() {
				process = ifrit.Background(step)
				Eventually(waiting).Should(BeClosed())
				process.Signal(os.Interrupt)
			})

			AfterEach(func() {
				close(waitExited)
				Eventually(process.Wait()).Should(Receive())
			})

			It("sends an interrupt to the process", func() {
				Eventually(spawnedProcess.SignalCallCount).Should(Equal(1))
				Expect(spawnedProcess.SignalArgsForCall(0)).To(Equal(garden.SignalTerminate))
			})

			Context("when the process exits", func() {
				It("completes the run without having sent kill", func() {
					Eventually(spawnedProcess.SignalCallCount).Should(Equal(1))

					waitExited <- (128 + 15)

					Eventually(process.Wait()).Should(Receive(Equal(new(steps.CancelledError))))

					Expect(spawnedProcess.SignalCallCount()).To(Equal(1))
					Expect(spawnedProcess.SignalArgsForCall(0)).To(Equal(garden.SignalTerminate))
				})
			})

			Context("when the process does not exit after the graceful shutdown interval expires", func() {
				JustBeforeEach(func() {
					Eventually(spawnedProcess.SignalCallCount).Should(Equal(1))

					fakeClock.WaitForWatcherAndIncrement(gracefulShutdownInterval + 1*time.Second)

					Eventually(spawnedProcess.SignalCallCount).Should(Equal(2))
				})

				It("sends a kill signal to the process", func() {
					Expect(spawnedProcess.SignalArgsForCall(1)).To(Equal(garden.SignalKill))

					waitExited <- (128 + 9)

					Eventually(process.Wait()).Should(Receive(Equal(new(steps.ExceededGracefulShutdownIntervalError))))

					Expect(logger.TestSink.LogMessages()).To(ContainElement(
						ContainSubstring("graceful-shutdown-timeout-exceeded"),
					))
				})

				It("logs that the process was killed", func() {
					waitExited <- (128 + 9)
					Eventually(fakeStreamer.StdoutCallCount).Should(Equal(2))
					Expect(fakeStreamer.Stdout()).To(gbytes.Say("Exit status 137 \\(exceeded 5s graceful shutdown interval\\)"))
				})

				Context("when the process *still* does not exit after 1m", func() {
					It("finishes running with failure", func() {
						fakeClock.WaitForWatcherAndIncrement(steps.ExitTimeout / 2)

						Consistently(process.Wait()).ShouldNot(Receive())

						fakeClock.WaitForWatcherAndIncrement(steps.ExitTimeout / 2)

						Eventually(process.Wait()).Should(Receive(Equal(new(steps.ExitTimeoutError))))

						Expect(logger.TestSink.LogMessages()).To(ContainElement(
							ContainSubstring("process-did-not-exit"),
						))
					})
				})
			})
		})

		Context("when Garden hangs on spawning a process", func() {
			var (
				process  ifrit.Process
				hangChan chan struct{}
			)

			BeforeEach(func() {
				hangChan = make(chan struct{})
				gardenClient.Connection.RunStub = func(string, garden.ProcessSpec, garden.ProcessIO) (garden.Process, error) {
					//hang until test is finished
					<-hangChan
					return nil, nil
				}
			})

			JustBeforeEach(func() {
				process = ifrit.Background(step)
				Eventually(gardenClient.Connection.RunCallCount).Should(Equal(1))
				process.Signal(os.Interrupt)
			})

			AfterEach(func() {
				close(hangChan)
				Eventually(process.Wait()).Should(Receive())
			})

			It("finishes running with failure", func() {
				Eventually(process.Wait()).Should(Receive(Equal(new(steps.CancelledError))))
			})
		})

		Context("when signalling before running", func() {
			It("does not start the process or become ready", func() {
				runErr := make(chan error)
				ready := make(chan struct{})
				signals := make(chan os.Signal, 1)
				signals <- os.Interrupt
				go func() {
					runErr <- step.Run(signals, ready)
				}()

				Eventually(runErr).Should(Receive(MatchError(new(steps.CancelledError))))
				Expect(gardenClient.Connection.RunCallCount()).To(Equal(0))
				Consistently(ready).ShouldNot(BeClosed())
			})
		})
	})
})
