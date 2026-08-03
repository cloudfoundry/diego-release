package watcher_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/watcher"
	"code.cloudfoundry.org/route-emitter/watcher/fakes"
	"code.cloudfoundry.org/routing-info/cfroutes"
	"code.cloudfoundry.org/routing-info/tcp_routes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"github.com/vito/go-sse/sse"
)

type EventHolder struct {
	event models.Event
}

var _ = Describe("Watcher", func() {

	getDesiredLRP := func(processGuid, logGuid string,
		containerPort, externalPort uint32) *models.DesiredLRP {
		var desiredLRP models.DesiredLRP
		desiredLRP.ProcessGuid = processGuid
		desiredLRP.Ports = []uint32{containerPort}
		desiredLRP.LogGuid = logGuid
		tcpRoutes := tcp_routes.TCPRoutes{
			tcp_routes.TCPRoute{
				ExternalPort:  externalPort,
				ContainerPort: containerPort,
			},
		}
		desiredLRP.Routes = tcpRoutes.RoutingInfo()
		return &desiredLRP
	}

	getActualLRP := func(processGuid, instanceGuid, hostAddress, instanceAddress string,
		hostPort, containerPort uint32, evacuating bool) *models.ActualLRP {
		lrp := &models.ActualLRP{
			ActualLRPKey:         models.NewActualLRPKey(processGuid, 0, "domain"),
			ActualLRPInstanceKey: models.NewActualLRPInstanceKey(instanceGuid, "cell-id-1"),
			ActualLRPNetInfo: models.NewActualLRPNetInfo(
				hostAddress,
				instanceAddress,
				models.ActualLRPNetInfo_PreferredAddressHost,
				models.NewPortMapping(hostPort, containerPort),
			),
			State: models.ActualLRPStateRunning,
		}

		if evacuating {
			lrp.Presence = models.ActualLRP_Evacuating
		}
		return lrp
	}

	var (
		logger           *lagertest.TestLogger
		eventSource      *eventfakes.FakeEventSource
		bbsClient        *fake_bbs.FakeClient
		routeHandler     *fakes.FakeRouteHandler
		testWatcher      *watcher.Watcher
		clock            *fakeclock.FakeClock
		process          ifrit.Process
		cellID           string
		syncCh           chan struct{}
		emitExternalCh   chan struct{}
		emitInternalCh   chan struct{}
		fakeMetronClient *mfakes.FakeIngressClient
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-watcher")
		eventSource = new(eventfakes.FakeEventSource)
		bbsClient = new(fake_bbs.FakeClient)
		routeHandler = new(fakes.FakeRouteHandler)

		clock = fakeclock.NewFakeClock(time.Now())
		bbsClient.SubscribeToInstanceEventsByCellIDReturns(eventSource, nil)

		syncCh = make(chan struct{})
		emitExternalCh = make(chan struct{})
		emitInternalCh = make(chan struct{})
		cellID = ""
		fakeMetronClient = &mfakes.FakeIngressClient{}
	})

	JustBeforeEach(func() {
		testWatcher = watcher.NewWatcher(
			cellID,
			bbsClient,
			clock,
			routeHandler,
			syncCh,
			emitExternalCh,
			emitInternalCh,
			logger,
			fakeMetronClient,
		)
		process = ifrit.Invoke(testWatcher)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
	})

	Context("event subscriptions", func() {
		Context("when cell id is set", func() {
			BeforeEach(func() {
				cellID = "some-cell-id"
			})

			It("subscribes to events for the current cell", func() {
				Eventually(bbsClient.SubscribeToInstanceEventsByCellIDCallCount).Should(Equal(1))
				_, cellid := bbsClient.SubscribeToInstanceEventsByCellIDArgsForCall(0)
				Expect(cellid).To(Equal("some-cell-id"))
			})
		})

		Context("when the cell id is not set", func() {
			BeforeEach(func() {
				cellID = ""
			})

			It("subscribes to all events", func() {
				Eventually(bbsClient.SubscribeToInstanceEventsByCellIDCallCount).Should(Equal(1))
				_, actualCellID := bbsClient.SubscribeToInstanceEventsByCellIDArgsForCall(0)
				Expect(actualCellID).To(Equal(""))
			})
		})
	})

	Context("handle DesiredLRPCreatedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61000)
			event = models.NewDesiredLRPCreatedEvent(desiredLRP, "some-trace-id")
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleEvent", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, createEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(createEvent).Should(Equal(event))
		})
	})

	Context("handle DesiredLRPChangedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61000)
			afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61001)
			event = models.NewDesiredLRPChangedEvent(beforeLRP, afterLRP, "some-trace-id")
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleEvent", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, changeEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(changeEvent).Should(Equal(event))
		})
	})

	Context("handle DesiredLRPRemovedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", 5222, 61000)
			event = models.NewDesiredLRPRemovedEvent(desiredLRP, "some-trace-id")
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleDesiredDelete", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, deleteEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(deleteEvent).Should(Equal(event))
		})
	})

	Context("handle ActualLRPInstanceRemovedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", "container-ip", 61000, 5222, false)
			event = models.NewActualLRPInstanceRemovedEvent(actualLRP, "some-trace-id")
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleActualCreate", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, createEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(createEvent).Should(Equal(event))
		})
	})

	Context("handle ActualLRPInstanceCreatedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", "container-ip", 61000, 5222, false)
			event = models.NewActualLRPInstanceCreatedEvent(actualLRP, "some-trace-id")
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleActualCreate", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, createEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(createEvent).Should(Equal(event))
		})
	})

	Context("handle ActualLRPInstanceChangedEvent", func() {
		var (
			event models.Event
		)

		BeforeEach(func() {
			beforeLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", "container-ip", 61000, 5222, false)
			afterLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip", "container-ip", 61001, 5222, false)
			event = models.NewActualLRPInstanceChangedEvent(beforeLRP, afterLRP, "some-trace-id")
			eventSource.NextReturns(event, nil)
		})

		It("calls routeHandler HandleActualUpdate", func() {
			Eventually(routeHandler.HandleEventCallCount).Should(BeNumerically(">=", 1))
			_, changeEvent := routeHandler.HandleEventArgsForCall(0)
			Expect(changeEvent).Should(Equal(event))
		})
	})

	Context("when an unrecognized event is received", func() {
		var (
			fakeRawEventSource *eventfakes.FakeRawEventSource
		)
		BeforeEach(func() {
			fakeRawEventSource = new(eventfakes.FakeRawEventSource)
			fakeEventSource := events.NewEventSource(fakeRawEventSource)

			fakeRawEventSource.NextReturns(
				sse.Event{
					ID:   "sup",
					Name: "unrecognized-event-type",
					Data: []byte("c3Nzcw=="),
				},
				nil,
			)

			bbsClient.SubscribeToInstanceEventsByCellIDReturns(fakeEventSource, nil)
		})

		It("should not close the current connection", func() {
			Consistently(fakeRawEventSource.CloseCallCount).Should(Equal(0))
		})
	})

	Context("when eventSource returns error", func() {
		BeforeEach(func() {
			eventSource.NextReturns(nil, errors.New("bazinga..."))
		})

		It("closes the current event source", func() {
			Eventually(eventSource.CloseCallCount).Should(BeNumerically(">=", 1))
		})

		It("resubscribes to SSE from bbs", func() {
			Eventually(bbsClient.SubscribeToInstanceEventsByCellIDCallCount, 5*time.Second, 300*time.Millisecond).Should(BeNumerically(">=", 2))
			Eventually(logger).Should(gbytes.Say("event-source-error"))
		})
	})

	Context("when subscribe to events fails", func() {
		var (
			bbsErrorChannel chan error
		)
		BeforeEach(func() {
			bbsErrorChannel = make(chan error)

			bbsClient.SubscribeToInstanceEventsByCellIDStub = func(logger lager.Logger, cellID string) (events.EventSource, error) {
				err := <-bbsErrorChannel
				if err != nil {
					return nil, err
				}
				return eventSource, nil
			}
		})

		JustBeforeEach(func() {
			bbsErrorChannel <- errors.New("kaboom")
		})

		It("retries to subscribe", func() {
			close(bbsErrorChannel)
			Eventually(bbsClient.SubscribeToInstanceEventsByCellIDCallCount, 5*time.Second, 300*time.Millisecond).Should(Equal(2))
			Eventually(logger).Should(gbytes.Say("kaboom"))
		})
	})

	Describe("emit external event", func() {
		It("emits registrations", func() {
			emitExternalCh <- struct{}{}
			Eventually(routeHandler.EmitExternalCallCount).Should(Equal(1))
		})
	})

	Describe("emit internal event", func() {
		It("emits registrations", func() {
			emitInternalCh <- struct{}{}
			Eventually(routeHandler.EmitInternalCallCount).Should(Equal(1))
		})
	})

	Describe("Sync Events", func() {
		var (
			errCh                                 chan error
			eventCh                               chan EventHolder
			desiredLRP1, desiredLRP2, desiredLRP3 *models.DesiredLRP
			actualLRP1, actualLRP2, actualLRP3    *models.ActualLRP
			currentTag                            *models.ModificationTag
			endpoint1, endpoint2, endpoint3       routingtable.Endpoint
			sendEvent                             func()
		)

		BeforeEach(func() {
			errCh = make(chan error, 10)
			eventCh = make(chan EventHolder, 1)
			// make the variables local to avoid race detection
			nextErr := errCh
			nextEventValue := eventCh

			eventSource.CloseStub = func() error {
				nextErr <- errors.New("closed")
				return nil
			}

			eventSource.NextStub = func() (models.Event, error) {
				t := time.After(10 * time.Millisecond)
				select {
				case err := <-nextErr:
					return nil, err
				case x := <-nextEventValue:
					return x.event, nil
				case <-t:
					return nil, nil
				}
			}

			currentTag = &models.ModificationTag{Epoch: "abc", Index: 1}
			hostname1 := "foo.example.com"
			hostname2 := "bar.example.com"
			hostname3 := "baz.example.com"
			endpoint1 = routingtable.Endpoint{InstanceGUID: "ig-1", Host: "1.1.1.1", Index: 0, Port: 11, ContainerPort: 8080, Presence: models.ActualLRP_Ordinary, ModificationTag: currentTag}
			endpoint2 = routingtable.Endpoint{InstanceGUID: "ig-2", Host: "2.2.2.2", Index: 0, Port: 22, ContainerPort: 8080, Presence: models.ActualLRP_Ordinary, ModificationTag: currentTag}
			endpoint3 = routingtable.Endpoint{InstanceGUID: "ig-3", Host: "2.2.2.2", Index: 1, Port: 23, ContainerPort: 8080, Presence: models.ActualLRP_Ordinary, ModificationTag: currentTag}

			routingInfo1 := cfroutes.CFRoutes{
				cfroutes.CFRoute{
					Hostnames:       []string{hostname1},
					Port:            8080,
					RouteServiceUrl: "https://rs.example.com",
				},
			}.RoutingInfo()
			desiredLRP1 = &models.DesiredLRP{
				ProcessGuid: "pg-1",
				Domain:      "tests",
				LogGuid:     "lg1",
				Routes:      &routingInfo1,
				Instances:   1,
			}

			routingInfo2 := cfroutes.CFRoutes{
				cfroutes.CFRoute{
					Hostnames: []string{hostname2},
					Port:      8080,
				},
			}.RoutingInfo()
			desiredLRP2 = &models.DesiredLRP{
				ProcessGuid: "pg-2",
				Domain:      "tests",
				LogGuid:     "lg2",
				Routes:      &routingInfo2,
				Instances:   1,
			}

			routingInfo3 := cfroutes.CFRoutes{
				cfroutes.CFRoute{
					Hostnames: []string{hostname3},
					Port:      8080,
				},
			}.RoutingInfo()
			desiredLRP3 = &models.DesiredLRP{
				ProcessGuid: "pg-3",
				Domain:      "tests",
				LogGuid:     "lg3",
				Routes:      &routingInfo3,
				Instances:   1,
			}

			actualLRP1 = &models.ActualLRP{
				ActualLRPKey:         models.NewActualLRPKey("pg-1", 0, "domain"),
				ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint1.InstanceGUID, "cell-id"),
				ActualLRPNetInfo:     models.NewActualLRPNetInfo(endpoint1.Host, "container-ip", models.ActualLRPNetInfo_PreferredAddressHost, models.NewPortMapping(endpoint1.Port, endpoint1.ContainerPort)),
				State:                models.ActualLRPStateRunning,
			}

			actualLRP2 = &models.ActualLRP{
				ActualLRPKey:         models.NewActualLRPKey("pg-2", 0, "domain"),
				ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint2.InstanceGUID, "cell-id"),
				ActualLRPNetInfo:     models.NewActualLRPNetInfo(endpoint2.Host, "container-ip", models.ActualLRPNetInfo_PreferredAddressHost, models.NewPortMapping(endpoint2.Port, endpoint2.ContainerPort)),
				State:                models.ActualLRPStateRunning,
			}
			actualLRP2.SetRoutable(true)

			actualLRP3 = &models.ActualLRP{
				ActualLRPKey:         models.NewActualLRPKey("pg-3", 1, "domain"),
				ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint3.InstanceGUID, "cell-id"),
				ActualLRPNetInfo:     models.NewActualLRPNetInfo(endpoint3.Host, "container-ip", models.ActualLRPNetInfo_PreferredAddressHost, models.NewPortMapping(endpoint3.Port, endpoint3.ContainerPort)),
				State:                models.ActualLRPStateRunning,
			}
			actualLRP3.SetRoutable(true)

			sendEvent = func() {
				Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPInstanceRemovedEvent(actualLRP1, "some-trace-id")}))
			}
		})

		JustBeforeEach(func() {
			syncCh <- struct{}{}
		})

		Describe("bbs events", func() {
			BeforeEach(func() {
				bbsClient.ActualLRPsStub = func(lager.Logger, string, models.ActualLRPFilter) ([]*models.ActualLRP, error) {
					defer GinkgoRecover()
					sendEvent()
					Eventually(logger).Should(gbytes.Say("caching-event"))
					return nil, nil
				}
			})

			It("caches events", func() {
				Consistently(routeHandler.HandleEventCallCount).Should(Equal(0))
			})

			It("applies cached events after syncing is complete", func() {
				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				_, _, _, _, event := routeHandler.SyncArgsForCall(0)

				expectedEvent := models.NewActualLRPInstanceRemovedEvent(actualLRP1, "some-trace-id")
				Expect(event[actualLRP1.InstanceGuid]).To(Equal(expectedEvent))
			})

			Context("when an invalid actual lrp created event is cached", func() {
				BeforeEach(func() {
					sendEvent = func() {
						Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPInstanceCreatedEvent(nil, "some-trace-id")}))
					}
				})

				It("an error is logged", func() {
					Eventually(logger).Should(gbytes.Say("nil-actual-lrp"))
				})
			})

			Context("when an invalid actual lrp change event is cached", func() {
				BeforeEach(func() {
					sendEvent = func() {
						Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPInstanceChangedEvent(nil, nil, "some-trace-id")}))
					}
				})

				It("an error is logged", func() {
					Eventually(logger).Should(gbytes.Say("nil-actual-lrp"))
				})
			})
		})

		Context("during sync", func() {
			var (
				unblock chan struct{}
			)

			BeforeEach(func() {
				unblock = make(chan struct{})
				bbsClient.ActualLRPsStub = func(lager.Logger, string, models.ActualLRPFilter) ([]*models.ActualLRP, error) {
					<-unblock
					return nil, nil
				}
			})

			AfterEach(func() {
				close(unblock)
			})

			It("ignores a sync event", func() {
				Eventually(syncCh).Should(BeSent(struct{}{}))
				Eventually(logger).Should(gbytes.Say("sync-already-in-progress"))
			})

			It("can be signaled", func() {
				ginkgomon.Interrupt(process)
			})
		})

		Context("when fetching actuals fails", func() {
			var (
				errCh chan error
			)

			BeforeEach(func() {
				errCh = make(chan error, 1)
				errCh <- errors.New("bam")
				bbsClient.ActualLRPsStub = func(lager.Logger, string, models.ActualLRPFilter) ([]*models.ActualLRP, error) {
					return []*models.ActualLRP{}, <-errCh
				}
			})

			It("should not call sync until the error resolves", func() {
				Eventually(bbsClient.ActualLRPsCallCount).Should(Equal(1))
				Consistently(routeHandler.SyncCallCount).Should(Equal(0))

				// return no errors
				close(errCh)
				syncCh <- struct{}{}

				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				Expect(bbsClient.ActualLRPsCallCount()).To(Equal(2))
			})
		})

		Context("when one of the actual lrps is invalid", func() {
			BeforeEach(func() {
				bbsClient.ActualLRPsStub = func(lager.Logger, string, models.ActualLRPFilter) ([]*models.ActualLRP, error) {
					return []*models.ActualLRP{actualLRP1, &models.ActualLRP{}, actualLRP2}, nil
				}
			})

			It("still syncs all valid Actual LRPs", func() {
				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				_, _, runningActuals, _, _ := routeHandler.SyncArgsForCall(0)
				Expect(runningActuals).To(ConsistOf(actualLRP1, actualLRP2))
			})
		})

		Context("when one of the actual lrps is not routable", func() {
			BeforeEach(func() {
				actualLRP2.SetRoutable(false)
				bbsClient.ActualLRPsStub = func(lager.Logger, string, models.ActualLRPFilter) ([]*models.ActualLRP, error) {
					return []*models.ActualLRP{actualLRP1, actualLRP2, actualLRP3}, nil
				}
			})

			It("syncs all actual LRPs with routable set as true or not set", func() {
				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				_, _, runningActuals, _, _ := routeHandler.SyncArgsForCall(0)
				Expect(runningActuals).To(ConsistOf(actualLRP1, actualLRP3))
			})
		})

		Context("when fetching desireds fails", func() {
			var (
				errCh chan error
			)

			BeforeEach(func() {
				errCh = make(chan error, 1)
				errCh <- errors.New("bam")

				bbsClient.DesiredLRPRoutingInfosStub = func(lager.Logger, string, models.DesiredLRPFilter) ([]*models.DesiredLRP, error) {
					return []*models.DesiredLRP{}, <-errCh
				}
			})

			It("should not call sync until the error resolves", func() {
				Eventually(bbsClient.DesiredLRPRoutingInfosCallCount).Should(Equal(1))
				Consistently(routeHandler.SyncCallCount).Should(Equal(0))

				// return no errors
				close(errCh)
				syncCh <- struct{}{}

				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				Expect(bbsClient.DesiredLRPRoutingInfosCallCount()).To(Equal(2))
			})
		})

		Context("when fetching domains fails", func() {
			var (
				errCh chan error
			)

			BeforeEach(func() {
				errCh = make(chan error, 1)
				errCh <- errors.New("bam")

				bbsClient.DomainsStub = func(lager.Logger, string) ([]string, error) {
					return []string{}, <-errCh
				}
			})

			It("should not call sync until the error resolves", func() {
				Eventually(bbsClient.DomainsCallCount).Should(Equal(1))
				Consistently(routeHandler.SyncCallCount).Should(Equal(0))

				// return no errors
				close(errCh)
				syncCh <- struct{}{}

				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				Expect(bbsClient.DomainsCallCount()).To(Equal(2))
			})

			It("does not emit the sync duration metric", func() {
				Consistently(fakeMetronClient.SendDurationCallCount).Should(BeZero())
			})
		})

		Context("when the routing_info BBS endpoint is not there", func() {
			BeforeEach(func() {
				bbsClient.ActualLRPsStub = func(logger lager.Logger, traceId string, f models.ActualLRPFilter) ([]*models.ActualLRP, error) {
					clock.IncrementBySeconds(1)

					return []*models.ActualLRP{
						actualLRP1,
						actualLRP2,
						actualLRP3,
					}, nil
				}

				bbsClient.DesiredLRPRoutingInfosStub = func(logger lager.Logger, traceId string, f models.DesiredLRPFilter) ([]*models.DesiredLRP, error) {
					return []*models.DesiredLRP{}, bbs.EndpointNotFoundErr
				}

				bbsClient.DesiredLRPsStub = func(logger lager.Logger, traceId string, f models.DesiredLRPFilter) ([]*models.DesiredLRP, error) {
					return []*models.DesiredLRP{desiredLRP1, desiredLRP2}, nil
				}
			})

			It("calls RouteHandler Sync with correct arguments", func() {
				expectedDesired := []*models.DesiredLRP{
					desiredLRP1,
					desiredLRP2,
				}

				expectedActuals := []*models.ActualLRP{
					actualLRP1,
					actualLRP2,
					actualLRP3,
				}

				expectedDomains := models.DomainSet{}
				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				_, desired, actuals, domains, _ := routeHandler.SyncArgsForCall(0)

				Expect(domains).To(Equal(expectedDomains))
				Expect(desired).To(Equal(expectedDesired))
				Expect(actuals).To(Equal(expectedActuals))
			})

			It("should emit the sync duration, and allow event processing", func() {
				Eventually(fakeMetronClient.SendDurationCallCount).Should(Equal(1))
				metric, value, _ := fakeMetronClient.SendDurationArgsForCall(0)
				Expect(metric).To(Equal("RouteEmitterSyncDuration"))
				Expect(value).To(BeNumerically(">=", 100*time.Millisecond))

				By("completing, events are no longer cached")
				sendEvent()

				Eventually(routeHandler.HandleEventCallCount).Should(Equal(1))
			})

			It("gets all the desired lrps", func() {
				Eventually(bbsClient.DesiredLRPRoutingInfosCallCount).Should(Equal(1))
				Eventually(bbsClient.DesiredLRPsCallCount).Should(Equal(1))
				_, traceId, filter := bbsClient.DesiredLRPRoutingInfosArgsForCall(0)
				Expect(traceId).To(BeEmpty())
				Expect(filter.ProcessGuids).To(BeEmpty())
			})
		})

		Context("when desired lrps are retrieved", func() {
			BeforeEach(func() {
				bbsClient.ActualLRPsStub = func(logger lager.Logger, traceId string, f models.ActualLRPFilter) ([]*models.ActualLRP, error) {
					clock.IncrementBySeconds(1)

					return []*models.ActualLRP{
						actualLRP1,
						actualLRP2,
						actualLRP3,
					}, nil
				}

				bbsClient.DesiredLRPRoutingInfosStub = func(logger lager.Logger, traceID string, f models.DesiredLRPFilter) ([]*models.DesiredLRP, error) {
					defer GinkgoRecover()

					return []*models.DesiredLRP{desiredLRP1, desiredLRP2}, nil
				}
			})

			It("calls RouteHandler Sync with correct arguments", func() {
				expectedDesired := []*models.DesiredLRP{
					desiredLRP1,
					desiredLRP2,
				}

				expectedActuals := []*models.ActualLRP{
					actualLRP1,
					actualLRP2,
					actualLRP3,
				}

				expectedDomains := models.DomainSet{}
				Eventually(routeHandler.SyncCallCount).Should(Equal(1))
				_, desired, actuals, domains, _ := routeHandler.SyncArgsForCall(0)

				Expect(domains).To(Equal(expectedDomains))
				Expect(desired).To(Equal(expectedDesired))
				Expect(actuals).To(Equal(expectedActuals))
			})

			It("should emit the sync duration, and allow event processing", func() {
				Eventually(fakeMetronClient.SendDurationCallCount).Should(Equal(1))
				metric, value, _ := fakeMetronClient.SendDurationArgsForCall(0)
				Expect(metric).To(Equal("RouteEmitterSyncDuration"))
				Expect(value).To(BeNumerically(">=", 100*time.Millisecond))

				By("completing, events are no longer cached")
				sendEvent()

				Eventually(routeHandler.HandleEventCallCount).Should(Equal(1))
			})

			It("gets all the desired lrps", func() {
				Eventually(bbsClient.DesiredLRPRoutingInfosCallCount).Should(Equal(1))
				_, traceId, filter := bbsClient.DesiredLRPRoutingInfosArgsForCall(0)
				Expect(traceId).To(BeEmpty())
				Expect(filter.ProcessGuids).To(BeEmpty())
			})
		})

		Context("when the cell id is set", func() {
			BeforeEach(func() {
				cellID = "cell-id"
				actualLRP2.ActualLRPInstanceKey.CellId = cellID
			})

			Context("when the cell has actual lrps running", func() {
				BeforeEach(func() {
					bbsClient.ActualLRPsStub = func(lager.Logger, string, models.ActualLRPFilter) ([]*models.ActualLRP, error) {
						clock.IncrementBySeconds(1)

						return []*models.ActualLRP{
							actualLRP2,
						}, nil
					}

					bbsClient.DesiredLRPRoutingInfosStub = func(lager.Logger, string, models.DesiredLRPFilter) ([]*models.DesiredLRP, error) {
						return []*models.DesiredLRP{desiredLRP2}, nil
					}
				})

				It("calls Sync method with correct desired lrps", func() {
					Eventually(routeHandler.SyncCallCount).Should(Equal(1))
					_, desired, _, _, _ := routeHandler.SyncArgsForCall(0)
					Expect(desired).To(ContainElement(desiredLRP2))
					Eventually(bbsClient.DesiredLRPRoutingInfosCallCount).Should(Equal(1))
				})

				It("registers endpoints for lrps on this cell", func() {
					Eventually(routeHandler.SyncCallCount).Should(Equal(1))
					_, _, actual, _, _ := routeHandler.SyncArgsForCall(0)
					Expect(actual).To(ContainElement(actualLRP2))
				})

				It("fetches actual lrps that match the cell id", func() {
					Eventually(bbsClient.ActualLRPsCallCount).Should(Equal(1))
					_, traceId, filter := bbsClient.ActualLRPsArgsForCall(0)
					Expect(traceId).To(BeEmpty())
					Expect(filter.CellID).To(Equal(cellID))
				})

				It("fetches desired lrp scheduling info that match the cell id", func() {
					Eventually(bbsClient.DesiredLRPRoutingInfosCallCount).Should(Equal(1))
					_, traceId, filter := bbsClient.DesiredLRPRoutingInfosArgsForCall(0)
					Expect(traceId).To(BeEmpty())
					Expect(filter.ProcessGuids).To(ConsistOf(actualLRP2.ProcessGuid))
				})
			})

			Context("when desired lrp for the actual lrp is missing", func() {
				BeforeEach(func() {
					sendEvent = func() {
						beforeActualLRP3 := &models.ActualLRP{
							ActualLRPKey:         models.NewActualLRPKey("pg-3", 1, "domain"),
							ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint3.InstanceGUID, "cell-id"),
							State:                models.ActualLRPStateClaimed,
						}
						Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPInstanceChangedEvent(
							beforeActualLRP3,
							actualLRP3,
							"some-trace-id",
						)}))
					}

					bbsClient.DesiredLRPRoutingInfosStub = func(_ lager.Logger, _ string, f models.DesiredLRPFilter) ([]*models.DesiredLRP, error) {
						defer GinkgoRecover()
						if len(f.ProcessGuids) == 1 && f.ProcessGuids[0] == "pg-3" {
							return []*models.DesiredLRP{desiredLRP3}, nil
						}
						return []*models.DesiredLRP{desiredLRP1}, nil
					}

					routeHandler.ShouldRefreshDesiredReturns(true)
				})

				Context("when a running actual lrp event is received", func() {
					BeforeEach(func() {
						cellID = "cell-id"

						bbsClient.ActualLRPsStub = func(logger lager.Logger, _ string, f models.ActualLRPFilter) ([]*models.ActualLRP, error) {
							clock.IncrementBySeconds(1)
							return []*models.ActualLRP{actualLRP1}, nil
						}
					})

					JustBeforeEach(func() {
						Eventually(routeHandler.SyncCallCount).Should(Equal(1))
						sendEvent()
					})

					Context("when an invalid actual lrp created event is received", func() {
						BeforeEach(func() {
							sendEvent = func() {
								Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPInstanceCreatedEvent(nil, "some-trace-id")}))
							}
						})

						It("an error is logged", func() {
							Eventually(logger).Should(gbytes.Say("nil-actual-lrp"))
						})
					})

					Context("when an invalid actual lrp change event is received", func() {
						BeforeEach(func() {
							sendEvent = func() {
								Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPInstanceChangedEvent(nil, nil, "some-trace-id")}))
							}
						})

						It("an error is logged", func() {
							Eventually(logger).Should(gbytes.Say("nil-actual-lrp"))
						})
					})

					It("fetches the desired lrp and passes it to the route handler", func() {
						Eventually(bbsClient.DesiredLRPRoutingInfosCallCount).Should(Equal(2))

						_, traceId, filter := bbsClient.DesiredLRPRoutingInfosArgsForCall(0)
						Expect(traceId).To(BeEmpty())

						_, traceId, filter = bbsClient.DesiredLRPRoutingInfosArgsForCall(1)
						Expect(traceId).To(Equal("some-trace-id"))

						Expect(filter.ProcessGuids).To(HaveLen(1))
						Expect(filter.ProcessGuids).To(ConsistOf(actualLRP3.ProcessGuid))

						Eventually(routeHandler.ShouldRefreshDesiredCallCount).Should(Equal(1))
						Eventually(routeHandler.RefreshDesiredCallCount).Should(Equal(1))
						_, desiredInfo := routeHandler.RefreshDesiredArgsForCall(0)
						Expect(desiredInfo).To(ContainElement(desiredLRP3))

						Eventually(routeHandler.HandleEventCallCount).Should(Equal(1))
					})
				})

				Context("and the event is cached", func() {
					BeforeEach(func() {
						bbsClient.ActualLRPsStub = func(lager.Logger, string, models.ActualLRPFilter) ([]*models.ActualLRP, error) {
							clock.IncrementBySeconds(1)
							defer GinkgoRecover()
							sendEvent()
							Eventually(logger).Should(gbytes.Say("caching-event"))
							return []*models.ActualLRP{actualLRP1}, nil
						}
					})

					It("fetches the desired lrp and refreshes the handler", func() {
						Eventually(bbsClient.DesiredLRPRoutingInfosCallCount).Should(Equal(2))

						_, traceId, filter := bbsClient.DesiredLRPRoutingInfosArgsForCall(0)
						Expect(traceId).To(BeEmpty())

						_, traceId, filter = bbsClient.DesiredLRPRoutingInfosArgsForCall(1)
						Expect(traceId).To(Equal("some-trace-id"))

						Expect(filter.ProcessGuids).To(HaveLen(1))
						Expect(filter.ProcessGuids).To(ConsistOf(actualLRP3.ProcessGuid))

						Eventually(routeHandler.ShouldRefreshDesiredCallCount).Should(Equal(1))
						Eventually(routeHandler.SyncCallCount).Should(Equal(1))
						_, desiredInfo, _, _, _ := routeHandler.SyncArgsForCall(0)
						Expect(desiredInfo).To(ContainElement(desiredLRP3))
					})

					Context("and fetching desired scheduling info fails", func() {
						BeforeEach(func() {
							bbsClient.DesiredLRPRoutingInfosStub = func(l lager.Logger, _ string, f models.DesiredLRPFilter) ([]*models.DesiredLRP, error) {
								defer GinkgoRecover()
								if len(f.ProcessGuids) == 1 && f.ProcessGuids[0] == "pg-3" {
									return nil, errors.New("boom!")
								}
								return []*models.DesiredLRP{}, nil
							}
						})

						It("does not refresh the desired state", func() {
							Eventually(routeHandler.ShouldRefreshDesiredCallCount).Should(Equal(1))
							Eventually(bbsClient.DesiredLRPRoutingInfosCallCount).Should(Equal(2))
							Consistently(routeHandler.RefreshDesiredCallCount).Should(Equal(0))
						})
					})
				})

				Context("when fetching desired scheduling info fails", func() {
					BeforeEach(func() {
						bbsClient.ActualLRPsStub = func(lager.Logger, string, models.ActualLRPFilter) ([]*models.ActualLRP, error) {
							defer GinkgoRecover()
							sendEvent()
							Eventually(logger).Should(gbytes.Say("caching-event"))
							return []*models.ActualLRP{actualLRP1}, nil
						}
						bbsClient.DesiredLRPRoutingInfosStub = func(l lager.Logger, _ string, f models.DesiredLRPFilter) ([]*models.DesiredLRP, error) {
							defer GinkgoRecover()
							if len(f.ProcessGuids) == 1 && f.ProcessGuids[0] == "pg-3" {
								return nil, errors.New("boom!")
							}
							return []*models.DesiredLRP{}, nil
						}
					})

					It("does not refresh the desired state", func() {
						Eventually(routeHandler.ShouldRefreshDesiredCallCount).Should(Equal(1))
						Eventually(bbsClient.DesiredLRPRoutingInfosCallCount).Should(Equal(2))
						Consistently(routeHandler.RefreshDesiredCallCount).Should(Equal(0))
					})
				})
			})

			Context("when there are no running actual lrps on the cell", func() {
				BeforeEach(func() {
					bbsClient.ActualLRPsStub = func(logger lager.Logger, _ string, f models.ActualLRPFilter) ([]*models.ActualLRP, error) {
						return []*models.ActualLRP{}, nil
					}
				})

				It("does not fetch any desired lrp scheduling info", func() {
					Consistently(bbsClient.DesiredLRPRoutingInfosCallCount).Should(Equal(0))
				})
			})
		})

		Context("when actual lrp state is not running", func() {
			BeforeEach(func() {
				actualLRP4 := &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey("pg-4", 1, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(endpoint3.InstanceGUID, "cell-id"),
					State:                models.ActualLRPStateClaimed,
				}

				Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPInstanceCreatedEvent(
					actualLRP4,
					"some-trace-id",
				)}))
			})

			It("should not refresh desired lrps", func() {
				Consistently(routeHandler.ShouldRefreshDesiredCallCount).Should(Equal(0))
				Consistently(routeHandler.RefreshDesiredCallCount).Should(Equal(0))
			})
		})
	})
})
