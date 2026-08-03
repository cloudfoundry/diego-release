package gardenhealth_test

import (
	"errors"

	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/executor/gardenhealth"
	"code.cloudfoundry.org/executor/guidgen/fakeguidgen"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager/v3/lagertest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Checker", func() {
	const (
		retryCount         = 3
		rootfsPath         = "test-rootfs-path"
		containerOwnerName = "container-owner"
	)
	var (
		gardenChecker   gardenhealth.Checker
		gardenClient    *gardenfakes.FakeClient
		healthcheckSpec garden.ProcessSpec
		logger          *lagertest.TestLogger
	)

	BeforeEach(func() {
		healthcheckSpec = garden.ProcessSpec{
			Path: "/bin/sh",
			Args: []string{"-c", "echo", "hello"},
			User: "vcap",
		}
		logger = lagertest.NewTestLogger("test")
		gardenClient = &gardenfakes.FakeClient{}
		guidGenerator := &fakeguidgen.FakeGenerator{}
		guidGenerator.GuidReturns("abc-123")
		gardenChecker = gardenhealth.NewChecker(rootfsPath, containerOwnerName, 0, healthcheckSpec, gardenClient, guidGenerator)
	})

	Describe("Healthcheck", func() {
		var fakeContainer *gardenfakes.FakeContainer
		var oldContainer *gardenfakes.FakeContainer
		var fakeProcess *gardenfakes.FakeProcess

		BeforeEach(func() {
			fakeContainer = &gardenfakes.FakeContainer{}
			oldContainer = &gardenfakes.FakeContainer{}
			oldContainer.HandleReturns("old-guid")
			fakeProcess = &gardenfakes.FakeProcess{}
		})

		Context("When garden is healthy", func() {
			BeforeEach(func() {
				gardenClient.CreateReturns(fakeContainer, nil)
				gardenClient.ContainersReturns([]garden.Container{oldContainer}, nil)
				fakeContainer.RunReturns(fakeProcess, nil)
				fakeProcess.WaitReturns(0, nil)
			})

			It("drives a container lifecycle", func() {
				err := gardenChecker.Healthcheck(logger)

				By("Fetching any pre-existing healthcheck containers")
				Expect(gardenClient.ContainersCallCount()).To(Equal(1))

				By("Deleting all pre-existing-containers")
				//call count is two because we also expect to destroy the container we create
				Expect(gardenClient.DestroyCallCount()).To(Equal(2))
				guid := gardenClient.DestroyArgsForCall(0)
				Expect(guid).To(Equal("old-guid"))

				By("Creates the container")
				Expect(gardenClient.CreateCallCount()).To(Equal(1))
				containerSpec := gardenClient.CreateArgsForCall(0)
				Expect(containerSpec).To(Equal(garden.ContainerSpec{
					Handle:     "check-abc-123",
					RootFSPath: rootfsPath,
					Properties: garden.Properties{
						executor.ContainerOwnerProperty: containerOwnerName,
						gardenhealth.HealthcheckTag:     gardenhealth.HealthcheckTagValue,
						"network.healthcheck":           "true",
					},
				}))

				By("Runs the process")
				Expect(fakeContainer.RunCallCount()).To(Equal(1))

				procSpec, procIO := fakeContainer.RunArgsForCall(0)
				Expect(procSpec).To(Equal(healthcheckSpec))
				Expect(procIO).To(Equal(garden.ProcessIO{}))

				By("Waits for the process to finish")
				Expect(fakeProcess.WaitCallCount()).To(Equal(1))

				By("Destroys the container")
				guid = gardenClient.DestroyArgsForCall(1)
				Expect(guid).To(Equal("check-abc-123"))

				By("Returns success")
				Expect(err).Should(BeNil())
			})
		})

		Context("when list containers fails", func() {
			var listErr = errors.New("boom")
			BeforeEach(func() {
				gardenClient.ContainersReturns(nil, listErr)
			})

			It("returns the list error", func() {
				err := gardenChecker.Healthcheck(logger)

				By("Retries the failing list command")
				Expect(gardenClient.ContainersCallCount()).To(Equal(retryCount))

				By("Returns the error")
				Expect(err).To(Equal(listErr))
			})
		})

		Context("when deleting old containers fails", func() {
			var destroyErr = errors.New("boom")
			BeforeEach(func() {
				gardenClient.ContainersReturns([]garden.Container{fakeContainer}, nil)
				gardenClient.DestroyReturns(destroyErr)
			})

			It("returns the destroy error", func() {
				err := gardenChecker.Healthcheck(logger)

				By("Retries the destroy container command")
				Expect(gardenClient.DestroyCallCount()).To(Equal(retryCount))

				By("Returns the error")
				Expect(err).To(Equal(destroyErr))
			})
		})

		Context("when create fails", func() {
			var createErr = errors.New("nope")

			BeforeEach(func() {
				gardenClient.CreateReturns(nil, createErr)
			})

			It("sends back the creation error", func() {
				err := gardenChecker.Healthcheck(logger)

				By("Retries the failing create")
				Expect(gardenClient.CreateCallCount()).To(Equal(retryCount))

				By("Does not attempt to destroy the container")
				Expect(gardenClient.DestroyCallCount()).To(Equal(0))

				By("Returns the error")
				Expect(err).To(Equal(createErr))
			})
		})

		Context("when run fails", func() {
			var runErr = errors.New("nope")

			BeforeEach(func() {
				gardenClient.CreateReturns(fakeContainer, nil)
				fakeContainer.RunReturns(nil, runErr)
			})

			It("sends back the run error", func() {
				err := gardenChecker.Healthcheck(logger)

				By("Sending the result back")
				Expect(err).To(Equal(runErr))

				By("Retries the failing run command")
				Expect(fakeContainer.RunCallCount()).To(Equal(retryCount))

				By("Destroys the container")
				Expect(gardenClient.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when wait returns an error", func() {
			var waitErr = errors.New("no waiting!")

			BeforeEach(func() {
				gardenClient.CreateReturns(fakeContainer, nil)
				fakeContainer.RunReturns(fakeProcess, nil)
				fakeProcess.WaitReturns(0, waitErr)
			})

			It("sends back the wait error", func() {
				err := gardenChecker.Healthcheck(logger)

				By("Retries the failing wait command")
				Expect(fakeProcess.WaitCallCount()).To(Equal(retryCount))

				By("Returns the error")
				Expect(err).To(Equal(waitErr))
			})
		})

		Context("when the health check process returns with a non-zero exit code", func() {
			BeforeEach(func() {
				gardenClient.CreateReturns(fakeContainer, nil)
				fakeContainer.RunReturns(fakeProcess, nil)
				fakeProcess.WaitReturns(1, nil)
			})

			It("sends back HealthcheckFailedError", func() {
				err := gardenChecker.Healthcheck(logger)
				Expect(err).To(Equal(gardenhealth.HealthcheckFailedError(1)))
			})
		})

		Context("when destroying fails", func() {
			var destroyErr = garden.ContainerNotFoundError{}

			BeforeEach(func() {
				gardenClient.CreateReturns(fakeContainer, nil)
				fakeContainer.RunReturns(fakeProcess, nil)
				fakeProcess.WaitReturns(0, nil)
				gardenClient.DestroyReturns(destroyErr)
			})

			It("retries and returns the error", func() {
				err := gardenChecker.Healthcheck(logger)

				By("Retries the failing destroy command")
				Expect(gardenClient.DestroyCallCount()).To(Equal(retryCount))
				By("Returns the error")
				Expect(err).To(Equal(destroyErr))
			})
		})
	})

	Describe("Cancel", func() {
		var oldContainer *gardenfakes.FakeContainer

		BeforeEach(func() {
			oldContainer = &gardenfakes.FakeContainer{}
			oldContainer.HandleReturns("old-guid")
			gardenClient.ContainersReturns([]garden.Container{oldContainer}, nil)
		})

		It("destroys any containers with the healthcheck tag", func() {
			gardenChecker.Cancel(logger)

			By("Fetching any pre-existing healthcheck containers")
			Expect(gardenClient.ContainersCallCount()).To(Equal(1))
			properties := gardenClient.ContainersArgsForCall(0)
			Expect(properties).To(Equal(garden.Properties{
				gardenhealth.HealthcheckTag: gardenhealth.HealthcheckTagValue,
			}))

			By("Deleting all pre-existing-containers")
			Expect(gardenClient.DestroyCallCount()).To(Equal(1))
			guid := gardenClient.DestroyArgsForCall(0)
			Expect(guid).To(Equal("old-guid"))
		})

		Context("when listing the containers fails", func() {
			BeforeEach(func() {
				gardenClient.ContainersReturns(nil, errors.New("boom!"))
			})

			It("logs the error", func() {
				gardenChecker.Cancel(logger)
				Expect(logger).To(gbytes.Say("failed-to-list-containers"))
			})
		})

		Context("when destroying the container fails", func() {
			BeforeEach(func() {
				gardenClient.DestroyReturns(errors.New("boom!"))
			})

			It("logs the error", func() {
				gardenChecker.Cancel(logger)
				Expect(logger).To(gbytes.Say("failed-to-destroy-containers"))
			})
		})
	})
})
