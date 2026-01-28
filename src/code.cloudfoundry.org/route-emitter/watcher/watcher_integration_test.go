package watcher_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/route-emitter/emitter"
	"code.cloudfoundry.org/route-emitter/routehandlers"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/unregistration"
	"code.cloudfoundry.org/route-emitter/watcher"
	"code.cloudfoundry.org/routing-api/fake_routing_api"
	"code.cloudfoundry.org/routing-api/uaaclient"
	"code.cloudfoundry.org/routing-info/cfroutes"
	"code.cloudfoundry.org/workpool"
	"github.com/nats-io/nats.go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Watcher Integration", func() {
	var (
		bbsClient        *fake_bbs.FakeClient
		eventSource      *eventfakes.FakeEventSource
		natsClient       *diegonats.FakeNATSClient
		routingApiClient *fake_routing_api.FakeClient
		syncCh           chan struct{}
		emitExternalCh   chan struct{}
		emitInternalCh   chan struct{}
		cellID           string
		testWatcher      *watcher.Watcher
		process          ifrit.Process
		logger           *lagertest.TestLogger
		fakeMetronClient *mfakes.FakeIngressClient
	)

	BeforeEach(func() {
		bbsClient = new(fake_bbs.FakeClient)
		eventSource = new(eventfakes.FakeEventSource)
		bbsClient.SubscribeToInstanceEventsByCellIDReturns(eventSource, nil)

		natsClient = diegonats.NewFakeClient()
		routingApiClient = new(fake_routing_api.FakeClient)

		syncCh = make(chan struct{})
		emitExternalCh = make(chan struct{})
		emitInternalCh = make(chan struct{})

		logger = lagertest.NewTestLogger("test")
		workPool, err := workpool.NewWorkPool(1)
		Expect(err).NotTo(HaveOccurred())
		fakeMetronClient = &mfakes.FakeIngressClient{}
		natsEmitter := emitter.NewNATSEmitter(natsClient, workPool, logger, fakeMetronClient, false)
		natsTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)

		clock := fakeclock.NewFakeClock(time.Now())
		uaaTokenFetcher, err := uaaclient.NewTokenFetcher(true, uaaclient.Config{}, clock, 0, 0, 0, logger)
		Expect(err).NotTo(HaveOccurred())
		routingAPIEmitter := emitter.NewRoutingAPIEmitter(logger, routingApiClient, uaaTokenFetcher, 100)
		unregistrationCache := unregistration.NewCache(logger)
		handler := routehandlers.NewHandler(natsTable, natsEmitter, routingAPIEmitter, false, false, fakeMetronClient, unregistrationCache)
		testWatcher = watcher.NewWatcher(
			cellID,
			bbsClient,
			clock,
			handler,
			syncCh,
			emitExternalCh,
			emitInternalCh,
			logger,
			fakeMetronClient,
		)
	})

	JustBeforeEach(func() {
		process = ifrit.Invoke(testWatcher)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
	})

	Describe("caching events", func() {
		var (
			errCh            chan error
			eventCh          chan EventHolder
			modTag           *models.ModificationTag
			desiredLRP1      *models.DesiredLRP
			actualLRP1       *models.ActualLRP
			removedActualLRP *models.ActualLRP
		)

		sendEvent := func() {
			Eventually(eventCh).Should(BeSent(EventHolder{models.NewActualLRPInstanceRemovedEvent(removedActualLRP, "some-trace-id")}))
			Eventually(logger).Should(gbytes.Say("caching-event"))
		}

		BeforeEach(func() {
			errCh = make(chan error, 10)
			eventCh = make(chan EventHolder, 1)
			// make the variables local to avoid race detection
			nextErr := errCh
			nextEventValue := eventCh

			modTag = &models.ModificationTag{Epoch: "abc", Index: 1}
			endpoint1 := routingtable.Endpoint{InstanceGUID: "ig-1", Host: "1.1.1.1", Index: 0, Port: 11, ContainerPort: 8080, Presence: models.ActualLRP_Ordinary, ModificationTag: modTag}

			hostname1 := "foo.example.com"
			routingInfo1 := cfroutes.CFRoutes{
				cfroutes.CFRoute{
					Hostnames:       []string{hostname1},
					Port:            8080,
					RouteServiceUrl: "https://rs.example.com",
				},
			}.RoutingInfo()
			desiredLRP1 = &models.DesiredLRP{
				ModificationTag: modTag,
				ProcessGuid:     "pg-1",
				Domain:          "tests",
				LogGuid:         "lg1",
				Routes:          &routingInfo1,
				Instances:       1,
			}

			actualLRP1 = &models.ActualLRP{
				ActualLrpKey:         models.NewActualLRPKey("pg-1", 0, "domain"),
				ActualLrpInstanceKey: models.NewActualLRPInstanceKey(endpoint1.InstanceGUID, "cell-id"),
				ActualLrpNetInfo:     models.NewActualLRPNetInfo(endpoint1.Host, "container-ip", models.ActualLRPNetInfo_PreferredAddressHost, models.NewPortMapping(endpoint1.Port, endpoint1.ContainerPort)),
				State:                models.ActualLRPStateRunning,
				ModificationTag:      *modTag,
			}

			removedActualLRP = &models.ActualLRP{
				ActualLrpKey:         models.NewActualLRPKey("pg-1", 0, "domain"),
				ActualLrpInstanceKey: models.NewActualLRPInstanceKey(endpoint1.InstanceGUID, "cell-id"),
				ActualLrpNetInfo:     models.NewActualLRPNetInfo(endpoint1.Host, "container-ip", models.ActualLRPNetInfo_PreferredAddressHost, models.NewPortMapping(endpoint1.Port, endpoint1.ContainerPort)),
				State:                models.ActualLRPStateRunning,
				ModificationTag:      *modTag,
			}

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

			bbsClient.ActualLRPsStub = func(logger lager.Logger, traceId string, filter models.ActualLRPFilter) ([]*models.ActualLRP, error) {
				defer GinkgoRecover()

				sendEvent()

				return []*models.ActualLRP{
					actualLRP1,
				}, nil
			}

			bbsClient.DesiredLRPRoutingInfosStub = func(logger lager.Logger, traceId string, f models.DesiredLRPFilter) ([]*models.DesiredLRP, error) {
				defer GinkgoRecover()
				return []*models.DesiredLRP{desiredLRP1}, nil
			}
		})

		JustBeforeEach(func() {
			syncCh <- struct{}{}
			Eventually(bbsClient.ActualLRPsCallCount).Should(Equal(1))
		})

		Context("when an old remove event is cached", func() {
			BeforeEach(func() {
				removedActualLRP.ModificationTag.Index = 0
			})

			It("registers the new route", func() {
				Eventually(func() []*nats.Msg {
					return natsClient.PublishedMessages("router.register")
				}).Should(HaveLen(1))
			})
		})

		Context("when a newer remove event is cached", func() {
			BeforeEach(func() {
				removedActualLRP.ModificationTag.Index = 2
			})

			It("does not register a new route", func() {
				Consistently(func() []*nats.Msg {
					return natsClient.PublishedMessages("router.register")
				}).Should(HaveLen(0))
			})
		})
	})
})
