package internal_test

import (
	"errors"
	"fmt"
	"strconv"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/executor"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context/fake_evacuation_context"
	"code.cloudfoundry.org/rep/generator/internal"
	"code.cloudfoundry.org/rep/generator/internal/fake_internal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
)

var _ = Describe("EvacuationLrpProcessor", func() {
	Describe("Process", func() {
		const (
			localCellID           = "cell-α"
			localAvailabilityZone = "some-zone"
		)

		var (
			logger                 *lagertest.TestLogger
			fakeBBS                *fake_bbs.FakeInternalClient
			fakeContainerDelegate  *fake_internal.FakeContainerDelegate
			fakeEvacuationReporter *fake_evacuation_context.FakeEvacuationReporter
			fakeMetronClient       *mfakes.FakeIngressClient

			lrpProcessor internal.LRPProcessor

			processGuid         string
			desiredLRP          models.DesiredLRP
			index               int
			container           executor.Container
			instanceGuid        string
			logGuid, sourceName string

			lrpKey         models.ActualLRPKey
			lrpInstanceKey models.ActualLRPInstanceKey
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")

			fakeBBS = new(fake_bbs.FakeInternalClient)

			fakeContainerDelegate = &fake_internal.FakeContainerDelegate{}
			fakeEvacuationReporter = &fake_evacuation_context.FakeEvacuationReporter{}
			fakeEvacuationReporter.EvacuatingReturns(true)

			fakeMetronClient = new(mfakes.FakeIngressClient)

			lrpProcessor = internal.NewLRPProcessor(fakeBBS, fakeContainerDelegate, fakeMetronClient, localCellID, localAvailabilityZone, rep.StackPathMap{}, "", fakeEvacuationReporter)

			processGuid = "process-guid"
			desiredLRP = models.DesiredLRP{
				Domain:      "domain",
				ProcessGuid: processGuid,
				Instances:   1,
				RootFs:      "some-rootfs",
				Action: models.WrapAction(&models.RunAction{
					Path: "/bin/true",
				}),
			}

			instanceGuid = "instance-guid"
			logGuid = "some-log-guid"
			sourceName = "some-source-name"
			index = 0

			container = executor.Container{
				Guid:    rep.LRPContainerGuid(desiredLRP.ProcessGuid, instanceGuid),
				RunInfo: executor.RunInfo{LogConfig: models.LogConfig{Guid: logGuid, SourceName: sourceName, Index: index}},
				Tags: executor.Tags{
					rep.LifecycleTag:    rep.LRPLifecycle,
					rep.DomainTag:       desiredLRP.Domain,
					rep.ProcessGuidTag:  desiredLRP.ProcessGuid,
					rep.InstanceGuidTag: instanceGuid,
					rep.ProcessIndexTag: strconv.Itoa(index),
				},
			}

			lrpKey = models.NewActualLRPKey(processGuid, int32(index), desiredLRP.Domain)
			lrpInstanceKey = models.NewActualLRPInstanceKey(instanceGuid, localCellID)
		})

		JustBeforeEach(func() {
			lrpProcessor.Process(logger, "some-trace-id", container)
		})

		Context("when the container is Reserved", func() {
			BeforeEach(func() {
				container.State = executor.StateReserved
			})

			It("evacuates the lrp", func() {
				Expect(fakeBBS.EvacuateClaimedActualLRPCallCount()).To(Equal(1))
				_, traceID, actualLRPKey, actualLRPContainerKey := fakeBBS.EvacuateClaimedActualLRPArgsForCall(0)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(*actualLRPKey).To(Equal(lrpKey))
				Expect(*actualLRPContainerKey).To(Equal(lrpInstanceKey))
			})

			Context("when the evacuation returns successfully", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateClaimedActualLRPReturns(false, nil)
				})

				It("deletes the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, traceID, actualContainerGuid := fakeContainerDelegate.DeleteContainerArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualContainerGuid).To(Equal(container.Guid))
				})
			})

			Context("when the evacuation returns that it failed to unclaim the LRP", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateClaimedActualLRPReturns(false, models.ErrActualLRPCannotBeUnclaimed)
				})

				It("deletes the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, traceID, actualContainerGuid := fakeContainerDelegate.DeleteContainerArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualContainerGuid).To(Equal(container.Guid))
				})
			})

			Context("when the evacuation returns some other error", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateClaimedActualLRPReturns(false, errors.New("whoops"))
				})

				It("deletes the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, traceID, actualContainerGuid := fakeContainerDelegate.DeleteContainerArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualContainerGuid).To(Equal(container.Guid))
				})
			})
		})

		Context("when the container is Initializing", func() {
			BeforeEach(func() {
				container.State = executor.StateInitializing
			})

			It("evacuates the lrp", func() {
				Expect(fakeBBS.EvacuateClaimedActualLRPCallCount()).To(Equal(1))
				_, traceID, actualLRPKey, actualLRPContainerKey := fakeBBS.EvacuateClaimedActualLRPArgsForCall(0)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(*actualLRPKey).To(Equal(lrpKey))
				Expect(*actualLRPContainerKey).To(Equal(lrpInstanceKey))
			})

			Context("when the evacuation returns successfully", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateClaimedActualLRPReturns(false, nil)
				})

				It("deletes the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, traceID, actualContainerGuid := fakeContainerDelegate.DeleteContainerArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualContainerGuid).To(Equal(container.Guid))
				})
			})

			Context("when the evacuation returns that it failed to unclaim the LRP", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateClaimedActualLRPReturns(false, models.ErrActualLRPCannotBeUnclaimed)
				})

				It("deletes the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, traceID, actualContainerGuid := fakeContainerDelegate.DeleteContainerArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualContainerGuid).To(Equal(container.Guid))
				})
			})

			Context("when the evacuation returns some other error", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateClaimedActualLRPReturns(false, errors.New("whoops"))
				})

				It("deletes the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, traceID, actualContainerGuid := fakeContainerDelegate.DeleteContainerArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualContainerGuid).To(Equal(container.Guid))
				})
			})
		})

		Context("when the container is Created", func() {
			BeforeEach(func() {
				container.State = executor.StateCreated
			})

			It("evacuates the lrp", func() {
				Expect(fakeBBS.EvacuateClaimedActualLRPCallCount()).To(Equal(1))
				_, traceID, actualLRPKey, actualLRPContainerKey := fakeBBS.EvacuateClaimedActualLRPArgsForCall(0)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(*actualLRPKey).To(Equal(lrpKey))
				Expect(*actualLRPContainerKey).To(Equal(lrpInstanceKey))
			})

			Context("when the evacuation returns successfully", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateClaimedActualLRPReturns(false, nil)
				})

				It("deletes the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, traceID, actualContainerGuid := fakeContainerDelegate.DeleteContainerArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualContainerGuid).To(Equal(container.Guid))
				})
			})

			Context("when the evacuation returns that it failed to unclaim the LRP", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateClaimedActualLRPReturns(false, models.ErrActualLRPCannotBeUnclaimed)
				})

				It("deletes the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, traceID, actualContainerGuid := fakeContainerDelegate.DeleteContainerArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualContainerGuid).To(Equal(container.Guid))
				})
			})

			Context("when the evacuation returns some other error", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateClaimedActualLRPReturns(false, errors.New("whoops"))
				})

				It("deletes the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, traceID, actualContainerGuid := fakeContainerDelegate.DeleteContainerArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualContainerGuid).To(Equal(container.Guid))
				})
			})
		})

		Context("when the container is Running", func() {
			var lrpNetInfo models.ActualLRPNetInfo

			BeforeEach(func() {
				container.State = executor.StateRunning
				externalIP := "executor-ip"
				internalIP := "container-ip"
				container.ExternalIP = externalIP
				container.InternalIP = internalIP
				container.AdvertisePreferenceForInstanceAddress = false
				container.Ports = []executor.PortMapping{{ContainerPort: 1357, HostPort: 8642}}
				container.InternalRoutes = models.InternalRoutes{{Hostname: "some-internal-route.apps.internal"}, {Hostname: "some-other-internal-route"}}
				lrpNetInfo = models.NewActualLRPNetInfo(externalIP, internalIP, models.ActualLRPNetInfo_PreferredAddressHost, models.NewPortMapping(8642, 1357))
				container.MetricsConfig.Tags = map[string]string{"app_name": "some-application"}
				container.Routable = true
			})

			It("evacuates the lrp", func() {
				Expect(fakeBBS.EvacuateRunningActualLRPCallCount()).To(Equal(1))
				_, traceID, actualLRPKey, actualLRPContainerKey, actualLRPNetInfo, internalRoutes, metricTags, routable, availabilityZone := fakeBBS.EvacuateRunningActualLRPArgsForCall(0)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(*actualLRPKey).To(Equal(lrpKey))
				Expect(*actualLRPContainerKey).To(Equal(lrpInstanceKey))
				Expect(*actualLRPNetInfo).To(Equal(lrpNetInfo))
				Expect(internalRoutes).To(Equal([]*models.ActualLRPInternalRoute{{Hostname: "some-internal-route.apps.internal"}, {Hostname: "some-other-internal-route"}}))
				Expect(metricTags).To(Equal(map[string]string{"app_name": "some-application"}))
				Expect(routable).To(Equal(true))
				Expect(availabilityZone).To(Equal(localAvailabilityZone))

				Eventually(logger).Should(Say(
					fmt.Sprintf(
						`"net_info":\{"address":"%s","ports":\[\{"container_port":%d,"host_port":%d,"container_tls_proxy_port":0,"host_tls_proxy_port":0\}\],"instance_address":"%s","preferred_address":"%s"\}`,
						lrpNetInfo.Address,
						lrpNetInfo.Ports[0].ContainerPort,
						lrpNetInfo.Ports[0].HostPort,
						lrpNetInfo.InstanceAddress,
						lrpNetInfo.PreferredAddress,
					),
				))
			})

			It("emits single log line indicating replacement request for instance", func() {
				Eventually(fakeMetronClient.SendAppLogCallCount).Should(Equal(1))
				msg, containerSource, tags := fakeMetronClient.SendAppLogArgsForCall(0)
				Expect(tags["source_id"]).To(Equal(logGuid))
				Expect(containerSource).To(Equal(sourceName))
				Expect(tags["instance_id"]).To(Equal(strconv.Itoa(index)))
				Expect(msg).To(Equal(fmt.Sprintf("Cell %s requesting replacement for instance %s", localCellID, instanceGuid)))

				lrpProcessor.Process(logger, "some-trace-id", container)
				Consistently(fakeMetronClient.SendAppLogCallCount).Should(Equal(1))
			})

			Context("when the evacuation returns successfully", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateRunningActualLRPReturns(true, nil)
				})

				It("does not delete the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(0))
				})
			})

			Context("when the evacuation returns that it failed to evacuate the LRP", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateRunningActualLRPReturns(false, models.ErrUnknownError)
				})

				It("deletes the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, traceID, actualContainerGuid := fakeContainerDelegate.DeleteContainerArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualContainerGuid).To(Equal(container.Guid))
				})
			})

			Context("when the evacuation returns some other error", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateRunningActualLRPReturns(true, errors.New("whoops"))
				})

				It("does not delete the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(0))
				})
			})
		})

		Context("when the container is COMPLETED (shutdown)", func() {
			BeforeEach(func() {
				container.State = executor.StateCompleted
				container.RunResult.Stopped = true
			})

			It("evacuates the lrp", func() {
				Expect(fakeBBS.EvacuateStoppedActualLRPCallCount()).To(Equal(1))
				_, traceID, actualLRPKey, actualLRPContainerKey := fakeBBS.EvacuateStoppedActualLRPArgsForCall(0)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(*actualLRPKey).To(Equal(lrpKey))
				Expect(*actualLRPContainerKey).To(Equal(lrpInstanceKey))
			})

			Context("when the evacuation returns successfully", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateStoppedActualLRPReturns(false, nil)
				})

				It("deletes the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, traceID, actualContainerGuid := fakeContainerDelegate.DeleteContainerArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualContainerGuid).To(Equal(container.Guid))
				})
			})

			Context("when the evacuation returns that it failed to remove the LRP", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateStoppedActualLRPReturns(false, models.ErrActualLRPCannotBeRemoved)
				})

				It("deletes the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, traceID, actualContainerGuid := fakeContainerDelegate.DeleteContainerArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualContainerGuid).To(Equal(container.Guid))
				})
			})

			Context("when the evacuation returns some other error", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateStoppedActualLRPReturns(false, errors.New("whoops"))
				})

				It("deletes the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, traceID, actualContainerGuid := fakeContainerDelegate.DeleteContainerArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualContainerGuid).To(Equal(container.Guid))
				})
			})
		})

		Context("when the container is COMPLETED (crashed)", func() {
			BeforeEach(func() {
				container.State = executor.StateCompleted
				container.RunResult.Stopped = false
				container.RunResult.FailureReason = "crashed"
			})

			It("evacuates the lrp", func() {
				Expect(fakeBBS.EvacuateCrashedActualLRPCallCount()).To(Equal(1))
				_, traceID, actualLRPKey, actualLRPContainerKey, reason := fakeBBS.EvacuateCrashedActualLRPArgsForCall(0)
				Expect(traceID).To(Equal("some-trace-id"))
				Expect(*actualLRPKey).To(Equal(lrpKey))
				Expect(*actualLRPContainerKey).To(Equal(lrpInstanceKey))
				Expect(reason).To(Equal("crashed"))
			})

			Context("when the evacuation returns successfully", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateCrashedActualLRPReturns(false, nil)
				})

				It("deletes the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, traceID, actualContainerGuid := fakeContainerDelegate.DeleteContainerArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualContainerGuid).To(Equal(container.Guid))
				})
			})

			Context("when the evacuation returns that it failed to remove the LRP", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateCrashedActualLRPReturns(false, models.ErrActualLRPCannotBeCrashed)
				})

				It("deletes the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, traceID, actualContainerGuid := fakeContainerDelegate.DeleteContainerArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualContainerGuid).To(Equal(container.Guid))
				})
			})

			Context("when the evacuation returns some other error", func() {
				BeforeEach(func() {
					fakeBBS.EvacuateCrashedActualLRPReturns(false, errors.New("whoops"))
				})

				It("deletes the container", func() {
					Expect(fakeContainerDelegate.DeleteContainerCallCount()).To(Equal(1))
					_, traceID, actualContainerGuid := fakeContainerDelegate.DeleteContainerArgsForCall(0)
					Expect(traceID).To(Equal("some-trace-id"))
					Expect(actualContainerGuid).To(Equal(container.Guid))
				})
			})
		})
	})
})
