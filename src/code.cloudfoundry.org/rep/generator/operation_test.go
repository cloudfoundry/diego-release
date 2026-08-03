package generator_test

import (
	"errors"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/generator"
	"code.cloudfoundry.org/rep/generator/internal"
	"code.cloudfoundry.org/rep/generator/internal/fake_internal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
)

var _ = Describe("Operation", func() {
	Describe("ResidualInstanceLRPOperation", func() {
		var (
			containerDelegate    *fake_internal.FakeContainerDelegate
			residualLRPOperation *generator.ResidualInstanceLRPOperation
			lrpKey               models.ActualLRPKey
			instanceKey          models.ActualLRPInstanceKey

			expectedContainerGuid string
		)

		BeforeEach(func() {
			lrpKey = models.NewActualLRPKey("the-process-guid", 0, "the-domain")
			instanceKey = models.NewActualLRPInstanceKey("the-instance-guid", "the-cell-id")
			containerDelegate = new(fake_internal.FakeContainerDelegate)
			residualLRPOperation = generator.NewResidualInstanceLRPOperation(logger, "some-trace-id", fakeBBS, containerDelegate, lrpKey, instanceKey)

			expectedContainerGuid = rep.LRPContainerGuid(lrpKey.GetProcessGuid(), instanceKey.GetInstanceGuid())
		})

		Describe("Key", func() {
			It("returns the InstanceGuid", func() {
				Expect(residualLRPOperation.Key()).To(Equal("the-instance-guid"))
			})
		})

		Describe("Execute", func() {
			const sessionName = "test.executing-residual-instance-lrp-operation"

			JustBeforeEach(func() {
				residualLRPOperation.Execute()
			})

			It("checks whether the container exists", func() {
				Expect(containerDelegate.GetContainerCallCount()).To(Equal(1))
				containerDelegateLogger, containerGuid := containerDelegate.GetContainerArgsForCall(0)
				Expect(containerGuid).To(Equal(expectedContainerGuid))
				Expect(containerDelegateLogger.SessionName()).To(Equal(sessionName))
			})

			It("logs its execution lifecycle", func() {
				Expect(logger).To(Say(sessionName + ".starting"))
				Expect(logger).To(Say(sessionName + ".finished"))
			})

			Context("when the container does not exist", func() {
				BeforeEach(func() {
					containerDelegate.GetContainerReturns(executor.Container{}, false)
				})

				It("removes the actualLRP", func() {
					Expect(fakeBBS.RemoveActualLRPCallCount()).To(Equal(1))
					_, traceID, actualLRPKey, actualInstanceKey := fakeBBS.RemoveActualLRPArgsForCall(0)

					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualLRPKey.ProcessGuid).To(Equal(lrpKey.ProcessGuid))
					Expect(actualLRPKey.Index).To(Equal(lrpKey.Index))
					Expect(*actualInstanceKey).To(Equal(instanceKey))
				})
			})

			Context("when the container exists", func() {
				BeforeEach(func() {
					containerDelegate.GetContainerReturns(executor.Container{}, true)
				})

				It("does not remove the actualLRP", func() {
					Expect(fakeBBS.RemoveActualLRPCallCount()).To(Equal(0))
				})

				It("logs that it skipped the operation because the container was found", func() {
					Expect(logger).To(Say(sessionName + ".skipped-because-container-exists"))
				})
			})
		})
	})

	Describe("ResidualEvacuatingLRPOperation", func() {
		var (
			containerDelegate              *fake_internal.FakeContainerDelegate
			residualEvacuatingLRPOperation *generator.ResidualEvacuatingLRPOperation
			instanceGuid                   string
			lrpKey                         models.ActualLRPKey
			instanceKey                    models.ActualLRPInstanceKey

			expectedContainerGuid string
		)

		BeforeEach(func() {
			instanceGuid = "the-instance-guid"
			lrpKey = models.NewActualLRPKey("the-process-guid", 0, "the-domain")
			instanceKey = models.NewActualLRPInstanceKey(instanceGuid, "the-cell-id")
			containerDelegate = new(fake_internal.FakeContainerDelegate)
			residualEvacuatingLRPOperation = generator.NewResidualEvacuatingLRPOperation(logger, "some-trace-id", fakeBBS, containerDelegate, lrpKey, instanceKey)

			expectedContainerGuid = rep.LRPContainerGuid(lrpKey.GetProcessGuid(), instanceKey.GetInstanceGuid())
		})

		Describe("Key", func() {
			It("returns the InstanceGuid", func() {
				Expect(residualEvacuatingLRPOperation.Key()).To(Equal(instanceGuid))
			})
		})

		Describe("Execute", func() {
			const sessionName = "test.executing-residual-evacuating-lrp-operation"

			JustBeforeEach(func() {
				residualEvacuatingLRPOperation.Execute()
			})

			It("checks whether the container exists", func() {
				Expect(containerDelegate.GetContainerCallCount()).To(Equal(1))
				containerDelegateLogger, containerGuid := containerDelegate.GetContainerArgsForCall(0)
				Expect(containerGuid).To(Equal(expectedContainerGuid))
				Expect(containerDelegateLogger.SessionName()).To(Equal(sessionName))
			})

			It("logs its execution lifecycle", func() {
				Expect(logger).To(Say(sessionName + ".starting"))
				Expect(logger).To(Say(sessionName + ".finished"))
			})

			Context("when the container does not exist", func() {
				BeforeEach(func() {
					containerDelegate.GetContainerReturns(executor.Container{}, false)
				})

				It("removes the actualLRP", func() {
					Expect(fakeBBS.RemoveEvacuatingActualLRPCallCount()).To(Equal(1))
					_, traceID, actualLRPKey, actualLRPContainerKey := fakeBBS.RemoveEvacuatingActualLRPArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(*actualLRPKey).To(Equal(lrpKey))
					Expect(*actualLRPContainerKey).To(Equal(instanceKey))
				})
			})

			Context("when the container exists", func() {
				BeforeEach(func() {
					containerDelegate.GetContainerReturns(executor.Container{}, true)
				})

				It("does not remove the actualLRP", func() {
					Expect(fakeBBS.RemoveEvacuatingActualLRPCallCount()).To(Equal(0))
				})

				It("logs that it skipped the operation because the container was found", func() {
					Expect(logger).To(Say(sessionName + ".skipped-because-container-exists"))
				})
			})
		})
	})

	Describe("ResidualJointLRPOperation", func() {
		var (
			containerDelegate         *fake_internal.FakeContainerDelegate
			residualJointLRPOperation *generator.ResidualJointLRPOperation
			instanceGuid              string
			lrpKey                    models.ActualLRPKey
			instanceKey               models.ActualLRPInstanceKey

			expectedContainerGuid string
		)

		BeforeEach(func() {
			instanceGuid = "the-instance-guid"
			lrpKey = models.NewActualLRPKey("the-process-guid", 0, "the-domain")
			instanceKey = models.NewActualLRPInstanceKey(instanceGuid, "the-cell-id")
			containerDelegate = new(fake_internal.FakeContainerDelegate)
			residualJointLRPOperation = generator.NewResidualJointLRPOperation(logger, "some-trace-id", fakeBBS, containerDelegate, lrpKey, instanceKey)

			expectedContainerGuid = rep.LRPContainerGuid(lrpKey.GetProcessGuid(), instanceKey.GetInstanceGuid())
		})

		Describe("Key", func() {
			It("returns the InstanceGuid", func() {
				Expect(residualJointLRPOperation.Key()).To(Equal(instanceGuid))
			})
		})

		Describe("Execute", func() {
			const sessionName = "test.executing-residual-joint-lrp-operation"

			JustBeforeEach(func() {
				residualJointLRPOperation.Execute()
			})

			It("checks whether the container exists", func() {
				Expect(containerDelegate.GetContainerCallCount()).To(Equal(1))
				containerDelegateLogger, containerGuid := containerDelegate.GetContainerArgsForCall(0)
				Expect(containerGuid).To(Equal(expectedContainerGuid))
				Expect(containerDelegateLogger.SessionName()).To(Equal(sessionName))
			})

			It("logs its execution lifecycle", func() {
				Expect(logger).To(Say(sessionName + ".starting"))
				Expect(logger).To(Say(sessionName + ".finished"))
			})

			Context("when the container does not exist", func() {
				BeforeEach(func() {
					containerDelegate.GetContainerReturns(executor.Container{}, false)
				})

				It("removes the instance actualLRP", func() {
					Expect(fakeBBS.RemoveActualLRPCallCount()).To(Equal(1))
					_, traceID, actualLRPKey, actualInstanceKey := fakeBBS.RemoveActualLRPArgsForCall(0)

					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualLRPKey.ProcessGuid).To(Equal(lrpKey.ProcessGuid))
					Expect(actualLRPKey.Index).To(Equal(lrpKey.Index))
					Expect(*actualInstanceKey).To(Equal(instanceKey))
				})

				It("removes the evacuating actualLRP", func() {
					Expect(fakeBBS.RemoveEvacuatingActualLRPCallCount()).To(Equal(1))
					_, traceID, actualLRPKey, actualLRPContainerKey := fakeBBS.RemoveEvacuatingActualLRPArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(*actualLRPKey).To(Equal(lrpKey))
					Expect(*actualLRPContainerKey).To(Equal(instanceKey))
				})
			})

			Context("when the container exists", func() {
				BeforeEach(func() {
					containerDelegate.GetContainerReturns(executor.Container{}, true)
				})

				It("does not remove either actualLRP", func() {
					Expect(fakeBBS.RemoveActualLRPCallCount()).To(Equal(0))
					Expect(fakeBBS.RemoveEvacuatingActualLRPCallCount()).To(Equal(0))
				})

				It("logs that it skipped the operation because the container was found", func() {
					Expect(logger).To(Say(sessionName + ".skipped-because-container-exists"))
				})
			})
		})
	})

	Describe("ResidualTaskOperation", func() {
		var (
			containerDelegate     *fake_internal.FakeContainerDelegate
			residualTaskOperation *generator.ResidualTaskOperation
			taskGuid, cellId      string
		)

		BeforeEach(func() {
			taskGuid = "the-task-guid"
			cellId = "the-cell-id"
			containerDelegate = new(fake_internal.FakeContainerDelegate)
			residualTaskOperation = generator.NewResidualTaskOperation(logger, "some-trace-id", taskGuid, cellId, fakeBBS, containerDelegate)
		})

		Describe("Key", func() {
			It("returns the TaskGuid", func() {
				Expect(residualTaskOperation.Key()).To(Equal("the-task-guid"))
			})
		})

		Describe("Execute", func() {
			const sessionName = "test.executing-residual-task-operation"

			JustBeforeEach(func() {
				residualTaskOperation.Execute()
			})

			It("checks whether the container exists", func() {
				Expect(containerDelegate.GetContainerCallCount()).To(Equal(1))
				containerDelegateLogger, containerGuid := containerDelegate.GetContainerArgsForCall(0)
				Expect(containerGuid).To(Equal("the-task-guid"))
				Expect(containerDelegateLogger.SessionName()).To(Equal(sessionName))
			})

			It("logs its execution lifecycle", func() {
				Expect(logger).To(Say(sessionName + ".starting"))
				Expect(logger).To(Say(sessionName + ".finished"))
			})

			Context("when the container does not exist", func() {
				BeforeEach(func() {
					containerDelegate.GetContainerReturns(executor.Container{}, false)
				})

				It("completes the task with failure", func() {
					Expect(fakeBBS.CompleteTaskCallCount()).To(Equal(1))
					_, traceID, actualTaskGuid, actualCellId, failed, actualFailureReason, _ := fakeBBS.CompleteTaskArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualTaskGuid).To(Equal(taskGuid))
					Expect(actualCellId).To(Equal(cellId))
					Expect(failed).To(BeTrue())
					Expect(actualFailureReason).To(Equal(internal.TaskCompletionReasonMissingContainer))
				})

				Context("when completing the task fails", func() {
					BeforeEach(func() {
						fakeBBS.CompleteTaskReturns(errors.New("failed"))
					})

					It("logs the failure", func() {
						Expect(logger).To(Say(sessionName + ".failed-to-complete-task"))
					})
				})
			})

			Context("when the container exists", func() {
				BeforeEach(func() {
					containerDelegate.GetContainerReturns(executor.Container{}, true)
				})

				It("does not complete the task", func() {
					Expect(fakeBBS.CompleteTaskCallCount()).To(Equal(0))
				})

				It("logs that it skipped the operation because the container was found", func() {
					Expect(logger).To(Say(sessionName + ".skipped-because-container-exists"))
				})
			})
		})
	})

	Describe("ContainerOperation", func() {
		var (
			containerDelegate  *fake_internal.FakeContainerDelegate
			lrpProcessor       *fake_internal.FakeLRPProcessor
			taskProcessor      *fake_internal.FakeTaskProcessor
			containerOperation *generator.ContainerOperation
			guid               string
		)

		BeforeEach(func() {
			containerDelegate = new(fake_internal.FakeContainerDelegate)
			lrpProcessor = new(fake_internal.FakeLRPProcessor)
			taskProcessor = new(fake_internal.FakeTaskProcessor)
			guid = "the-guid"
			containerOperation = generator.NewContainerOperation(logger, "some-trace-id", lrpProcessor, taskProcessor, containerDelegate, guid)
		})

		Describe("Key", func() {
			It("returns the Guid", func() {
				Expect(containerOperation.Key()).To(Equal("the-guid"))
			})
		})

		Describe("Execute", func() {
			const sessionName = "test.executing-container-operation"

			JustBeforeEach(func() {
				containerOperation.Execute()
			})

			It("checks whether the container exists", func() {
				Expect(containerDelegate.GetContainerCallCount()).To(Equal(1))
				containerDelegateLogger, containerGuid := containerDelegate.GetContainerArgsForCall(0)
				Expect(containerGuid).To(Equal(guid))
				Expect(containerDelegateLogger.SessionName()).To(Equal(sessionName))
			})

			It("logs its execution lifecycle", func() {
				Expect(logger).To(Say(sessionName + ".starting"))
				Expect(logger).To(Say(sessionName + ".finished"))
			})

			Context("when the container does not exist", func() {
				BeforeEach(func() {
					containerDelegate.GetContainerReturns(executor.Container{}, false)
				})

				It("logs that it skipped the operation because the container was found", func() {
					Expect(logger).To(Say(sessionName + ".skipped-because-container-does-not-exist"))
				})

				It("does not farm the container out to any processor", func() {
					Expect(lrpProcessor.ProcessCallCount()).To(Equal(0))
					Expect(taskProcessor.ProcessCallCount()).To(Equal(0))
				})
			})

			Context("when the container exists", func() {
				var (
					container executor.Container
				)

				BeforeEach(func() {
					containerDelegate.GetContainerReturns(executor.Container{}, true)
				})

				Context("when the container has an LRP lifecycle tag", func() {
					BeforeEach(func() {
						container = executor.Container{
							Tags: executor.Tags{
								rep.LifecycleTag: rep.LRPLifecycle,
							},
						}
						containerDelegate.GetContainerReturns(container, true)
					})

					It("farms the container out to only the lrp processor", func() {
						Expect(lrpProcessor.ProcessCallCount()).To(Equal(1))
						Expect(taskProcessor.ProcessCallCount()).To(Equal(0))
						actualLogger, traceID, actualContainer := lrpProcessor.ProcessArgsForCall(0)
						Expect(traceID).To(Equal("some-trace-id"))
						Expect(actualLogger.SessionName()).To(Equal(sessionName))
						Expect(actualContainer).To(Equal(container))
					})
				})

				Context("when the container has a Task lifecycle tag", func() {
					BeforeEach(func() {
						container = executor.Container{
							Tags: executor.Tags{
								rep.LifecycleTag: rep.TaskLifecycle,
							},
						}
						containerDelegate.GetContainerReturns(container, true)
					})

					It("farms the container out to only the task processor", func() {
						Expect(taskProcessor.ProcessCallCount()).To(Equal(1))
						Expect(lrpProcessor.ProcessCallCount()).To(Equal(0))
						actualLogger, traceID, actualContainer := taskProcessor.ProcessArgsForCall(0)
						Expect(traceID).To(Equal("some-trace-id"))
						Expect(actualLogger.SessionName()).To(Equal(sessionName))
						Expect(actualContainer).To(Equal(container))
					})
				})

				Context("when the container has an unknown lifecycle tag", func() {
					BeforeEach(func() {
						container = executor.Container{
							Tags: executor.Tags{
								rep.LifecycleTag: "some-other-tag",
							},
						}
						containerDelegate.GetContainerReturns(container, true)
					})

					It("does not farm the container out to any processor", func() {
						Expect(lrpProcessor.ProcessCallCount()).To(Equal(0))
						Expect(taskProcessor.ProcessCallCount()).To(Equal(0))
					})

					It("logs the unknown lifecycle", func() {
						Expect(logger).To(Say(sessionName + ".failed-to-process-container-with-unknown-lifecycle"))
					})
				})
			})
		})
	})
})
