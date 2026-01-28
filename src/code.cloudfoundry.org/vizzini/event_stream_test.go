package vizzini_test

import (
	"sync"

	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"

	. "code.cloudfoundry.org/vizzini/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("EventStream", func() {
	var desiredLRP *models.DesiredLRP
	var eventSource events.EventSource
	var done chan struct{}
	var lock *sync.Mutex
	var events []models.Event

	getEvents := func() []models.Event {
		lock.Lock()
		defer lock.Unlock()
		return events
	}

	BeforeEach(func() {
		var err error
		desiredLRP = DesiredLRPWithGuid(guid)
		eventSource, err = bbsClient.SubscribeToInstanceEvents(logger)
		Expect(err).NotTo(HaveOccurred())

		done = make(chan struct{})
		lock = &sync.Mutex{}
		events = []models.Event{}

		go func() {
			for {
				event, err := eventSource.Next()
				if err != nil {
					close(done)
					return
				}
				lock.Lock()
				events = append(events, event)
				lock.Unlock()
			}
		}()
	})

	AfterEach(func() {
		eventSource.Close()
		Eventually(done).Should(BeClosed())
	})

	It("should receive events as the LRP goes through its lifecycle", func() {
		bbsClient.DesireLRP(logger, traceID, desiredLRP)
		Eventually(getEvents).Should(ContainElement(MatchDesiredLRPCreatedEvent(guid)))
		Eventually(getEvents).Should(ContainElement(MatchActualLRPInstanceCreatedEvent(guid, 0)))
		Eventually(getEvents).Should(ContainElement(MatchActualLRPInstanceChangedEvent(guid, 0, models.ActualLRPStateClaimed)))
		Eventually(getEvents).Should(ContainElement(MatchActualLRPInstanceChangedEvent(guid, 0, models.ActualLRPStateRunning)))

		bbsClient.RemoveDesiredLRP(logger, traceID, guid)
		Eventually(getEvents).Should(ContainElement(MatchDesiredLRPRemovedEvent(guid)))
		Eventually(getEvents).Should(ContainElement(MatchActualLRPInstanceRemovedEvent(guid, 0)))
	})
})
