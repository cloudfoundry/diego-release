package generator_test

import (
	"errors"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/executor"
	efakes "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/operationq"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context/fake_evacuation_context"
	"code.cloudfoundry.org/rep/generator"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
)

type BogusEvent struct{}

func (BogusEvent) EventType() executor.EventType {
	return executor.EventTypeInvalid
}

var _ = Describe("Generator", func() {
	var (
		cellID             string
		availabilityZone   string
		fakeExecutorClient *efakes.FakeClient

		opGenerator generator.Generator
	)

	BeforeEach(func() {
		cellID = "some-cell-id"
		availabilityZone = "some-zone"
		fakeExecutorClient = new(efakes.FakeClient)
		fakeEvacuationReporter := &fake_evacuation_context.FakeEvacuationReporter{}
		opGenerator = generator.New(cellID, availabilityZone, rep.StackPathMap{}, "", fakeBBS, fakeExecutorClient, nil, fakeEvacuationReporter)
	})

	Describe("BatchOperations", func() {
		const sessionName = "test.batch-operations"

		var (
			batch    map[string]operationq.Operation
			batchErr error
		)

		JustBeforeEach(func() {
			batch, batchErr = opGenerator.BatchOperations(logger)
		})

		It("logs its lifecycle", func() {
			Expect(logger).To(Say(sessionName + ".started"))
		})

		It("retrieves all actual lrps for its cell id", func() {
			Expect(fakeBBS.ActualLRPsCallCount()).To(Equal(1))
			_, traceID, actualFilter := fakeBBS.ActualLRPsArgsForCall(0)
			Expect(traceID).To(BeEmpty())
			Expect(actualFilter.CellID).To(Equal(cellID))
		})

		It("retrieves all tasks for its cell id", func() {
			Expect(fakeBBS.TasksByCellIDCallCount()).To(Equal(1))
			_, traceID, actualCellID := fakeBBS.TasksByCellIDArgsForCall(0)
			Expect(traceID).To(BeEmpty())
			Expect(actualCellID).To(Equal(cellID))
		})

		It("lists all containers from the executor", func() {
			Expect(fakeExecutorClient.ListContainersCallCount()).To(Equal(1))
		})

		Context("when retrieving container and BBS data succeeds", func() {
			const (
				instanceGuidContainerOnly                 = "guid-container-only"
				instanceGuidContainerForInstanceLRP       = "guid-container-for-instance-lrp"
				instanceGuidContainerForEvacuatingLRP     = "guid-container-for-evacuating-lrp"
				guidContainerForTask                      = "guid-container-for-task"
				instanceGuidInstanceLRPOnly               = "guid-instance-lrp-only"
				instanceGuidEvacuatingLRPOnly             = "guid-evacuating-lrp-only"
				instanceGuidInstanceAndEvacuatingLRPsOnly = "guid-instance-and-evacuating-lrps-only"
				guidTaskOnly                              = "guid-task-only"

				processGuid = "process-guid"
			)

			BeforeEach(func() {
				containers := []executor.Container{
					{Guid: rep.LRPContainerGuid(processGuid, instanceGuidContainerOnly)},
					{Guid: rep.LRPContainerGuid(processGuid, instanceGuidContainerForInstanceLRP)},
					{Guid: rep.LRPContainerGuid(processGuid, instanceGuidContainerForEvacuatingLRP)},
					{Guid: guidContainerForTask},
				}

				actualLRPKey := models.ActualLRPKey{ProcessGuid: processGuid}

				containerOnlyLRP := models.ActualLRP{ActualLRPKey: actualLRPKey, ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuidContainerForInstanceLRP, cellID)}
				instanceOnlyLRP := models.ActualLRP{ActualLRPKey: actualLRPKey, ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuidInstanceLRPOnly, cellID)}

				containerForEvacuatingLRP := models.ActualLRP{ActualLRPKey: actualLRPKey, ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuidContainerForEvacuatingLRP, cellID), Presence: models.ActualLRP_Evacuating}
				evacuatingOnlyLRP := models.ActualLRP{ActualLRPKey: actualLRPKey, ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuidEvacuatingLRPOnly, cellID), Presence: models.ActualLRP_Evacuating}

				instanceAndEvacuatingLRP1 := models.ActualLRP{ActualLRPKey: actualLRPKey, ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuidInstanceAndEvacuatingLRPsOnly, cellID)}
				instanceAndEvacuatingLRP2 := models.ActualLRP{ActualLRPKey: actualLRPKey, ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuidInstanceAndEvacuatingLRPsOnly, cellID), Presence: models.ActualLRP_Evacuating}

				lrps := []*models.ActualLRP{
					&containerOnlyLRP,
					&instanceOnlyLRP,
					&instanceAndEvacuatingLRP1,
					&instanceAndEvacuatingLRP2,
					&containerForEvacuatingLRP,
					&evacuatingOnlyLRP,
				}

				tasks := []*models.Task{
					{TaskGuid: guidContainerForTask},
					{TaskGuid: guidTaskOnly},
				}

				fakeExecutorClient.ListContainersReturns(containers, nil)

				fakeBBS.ActualLRPsReturns(lrps, nil)
				fakeBBS.TasksByCellIDReturns(tasks, nil)
			})

			It("does not return an error", func() {
				Expect(batchErr).NotTo(HaveOccurred())
			})

			It("logs success", func() {
				Expect(logger).To(Say(sessionName + ".succeeded"))
			})

			It("returns a batch of the correct size", func() {
				Expect(batch).To(HaveLen(8))
			})

			batchHasAContainerOperationForGuid := func(guid string, batch map[string]operationq.Operation) {
				Expect(batch).To(HaveKey(guid))
				Expect(batch[guid]).To(BeAssignableToTypeOf(new(generator.ContainerOperation)))
			}

			It("returns a container operation for a container with an instance lrp", func() {
				batchHasAContainerOperationForGuid(rep.LRPContainerGuid(processGuid, instanceGuidContainerForInstanceLRP), batch)
			})

			It("returns a container operation for a container with an evacuating lrp", func() {
				batchHasAContainerOperationForGuid(rep.LRPContainerGuid(processGuid, instanceGuidContainerForEvacuatingLRP), batch)
			})

			It("returns a container operation for a container with a task", func() {
				batchHasAContainerOperationForGuid(guidContainerForTask, batch)
			})

			It("returns a container operation for a container with nothing in bbs", func() {
				batchHasAContainerOperationForGuid(rep.LRPContainerGuid(processGuid, instanceGuidContainerOnly), batch)
			})

			It("returns a residual instance lrp operation for a guid with an instance lrp but no container", func() {
				guid := rep.LRPContainerGuid(processGuid, instanceGuidInstanceLRPOnly)
				Expect(batch).To(HaveKey(guid))
				Expect(batch[guid]).To(BeAssignableToTypeOf(new(generator.ResidualInstanceLRPOperation)))
			})

			It("returns a residual evacuating lrp operation for a guid with an evacuating lrp but no container", func() {
				guid := rep.LRPContainerGuid(processGuid, instanceGuidEvacuatingLRPOnly)
				Expect(batch).To(HaveKey(guid))
				Expect(batch[guid]).To(BeAssignableToTypeOf(new(generator.ResidualEvacuatingLRPOperation)))
			})

			It("returns a residual joint lrp operation for a guid with both an instance and an evacuating lrp but no container", func() {
				guid := rep.LRPContainerGuid(processGuid, instanceGuidInstanceAndEvacuatingLRPsOnly)
				Expect(batch).To(HaveKey(guid))
				Expect(batch[guid]).To(BeAssignableToTypeOf(new(generator.ResidualJointLRPOperation)))
			})

			It("returns a residual task operation for a task with no container", func() {
				guid := guidTaskOnly
				Expect(batch).To(HaveKey(guid))
				Expect(batch[guid]).To(BeAssignableToTypeOf(new(generator.ResidualTaskOperation)))
			})

		})

		Context("when retrieving data fails", func() {
			Context("when retrieving the containers fails", func() {
				BeforeEach(func() {
					fakeExecutorClient.ListContainersReturns(nil, errors.New("oh no, no container!"))
				})

				It("returns an error", func() {
					Expect(batchErr).To(HaveOccurred())
					Expect(batchErr).To(MatchError(ContainSubstring("oh no, no container!")))
				})

				It("logs the failure", func() {
					Expect(logger).To(Say(sessionName + ".failed-to-list-containers"))
				})
			})

			Context("when retrieving the tasks fails", func() {
				BeforeEach(func() {
					fakeBBS.TasksByCellIDReturns(nil, errors.New("oh no, no task!"))
				})

				It("returns an error", func() {
					Expect(batchErr).To(HaveOccurred())
					Expect(batchErr).To(MatchError(ContainSubstring("oh no, no task!")))
				})

				It("logs the failure", func() {
					Expect(logger).To(Say(sessionName + ".failed-to-retrieve-tasks"))
				})
			})

			Context("when retrieving the LRP groups fails", func() {
				BeforeEach(func() {
					fakeBBS.ActualLRPsReturns(nil, errors.New("oh no, no lrp!"))
				})

				It("returns an error", func() {
					Expect(batchErr).To(HaveOccurred())
					Expect(batchErr).To(MatchError(ContainSubstring("oh no, no lrp!")))
				})

				It("logs the failure", func() {
					Expect(logger).To(Say(sessionName + ".failed-to-retrieve-lrps"))
				})
			})
		})
	})

	Describe("OperationStream", func() {
		const sessionPrefix = "test.operation-stream."

		var (
			stream    <-chan operationq.Operation
			streamErr error
		)

		JustBeforeEach(func() {
			stream, streamErr = opGenerator.OperationStream(logger)
		})

		Context("when subscribing to the executor succeeds", func() {
			var receivedEvents chan<- executor.Event

			BeforeEach(func() {
				events := make(chan executor.Event, 1)
				receivedEvents = events

				fakeExecutorSource := new(efakes.FakeEventSource)
				fakeExecutorSource.NextStub = func() (executor.Event, error) {
					ev, ok := <-events
					if !ok {
						return nil, errors.New("nope")
					}

					return ev, nil
				}

				fakeExecutorClient.SubscribeToEventsReturns(fakeExecutorSource, nil)
			})

			It("logs that it succeeded", func() {
				Expect(logger).To(Say(sessionPrefix + "subscribing"))
				Expect(logger).To(Say(sessionPrefix + "succeeded-subscribing"))
			})

			Context("when the event stream closes", func() {
				BeforeEach(func() {
					close(receivedEvents)
				})

				It("closes the operation stream", func() {
					Eventually(stream).Should(BeClosed())
				})

				It("logs the closure", func() {
					Eventually(logger).Should(Say(sessionPrefix + "event-stream-closed"))
				})
			})

			Context("when an executor event appears", func() {
				AfterEach(func() {
					close(receivedEvents)
				})

				Context("when the event is a lifecycle event", func() {
					var container executor.Container

					BeforeEach(func() {
						container = executor.Container{
							Guid: "some-instance-guid",

							State: executor.StateCompleted,

							Tags: executor.Tags{
								rep.ProcessGuidTag:  "some-process-guid",
								rep.DomainTag:       "some-domain",
								rep.ProcessIndexTag: "1",
							},
						}
					})

					JustBeforeEach(func() {
						receivedEvents <- executor.NewContainerCompleteEvent(container, "some-trace-id")
					})

					Context("when the lifecycle is LRP", func() {
						BeforeEach(func() {
							container.Tags[rep.LifecycleTag] = rep.LRPLifecycle
						})

						It("yields an operation for that container", func() {
							var operation operationq.Operation
							Eventually(stream).Should(Receive(&operation))
							Expect(operation.Key()).To(Equal(container.Guid))
						})
					})

					Context("when the lifecycle is Task", func() {
						var task *models.Task

						BeforeEach(func() {
							container.Tags[rep.LifecycleTag] = rep.TaskLifecycle

							task = &models.Task{
								TaskGuid: "some-instance-guid",
								State:    models.Task_Running,
							}

							fakeBBS.TaskByGuidReturns(task, nil)
						})

						It("yields an operation for that container", func() {
							var operation operationq.Operation
							Eventually(stream).Should(Receive(&operation))
							Expect(operation.Key()).To(Equal(container.Guid))
						})
					})
				})

				Context("when the event is not a lifecycle event", func() {
					BeforeEach(func() {
						receivedEvents <- BogusEvent{}
					})

					It("does not yield an operation", func() {
						Consistently(stream).ShouldNot(Receive())
					})

					It("logs the non-lifecycle event", func() {
						Eventually(logger).Should(Say(sessionPrefix + "received-non-lifecycle-event"))
					})
				})
			})
		})

		Context("when subscribing to the executor fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeExecutorClient.SubscribeToEventsReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(streamErr).To(Equal(disaster))
			})

			It("logs the failure", func() {
				Expect(logger).To(Say(sessionPrefix + "failed-subscribing"))
			})
		})
	})
})
