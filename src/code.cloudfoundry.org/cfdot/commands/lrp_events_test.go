package commands_test

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/cfdot/commands"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("LRP Events", func() {
	var (
		fakeBBSClient           *fake_bbs.FakeClient
		fakeEventSource         *eventfakes.FakeEventSource
		fakeInstanceEventSource *eventfakes.FakeEventSource
		stdout, stderr          *gbytes.Buffer
		lrp                     *models.DesiredLRP
		actualLRP               *models.ActualLRP
	)

	BeforeEach(func() {
		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()
		fakeBBSClient = &fake_bbs.FakeClient{}
		fakeEventSource = &eventfakes.FakeEventSource{}
		fakeInstanceEventSource = &eventfakes.FakeEventSource{}
		fakeBBSClient.SubscribeToEventsByCellIDReturns(fakeEventSource, nil)
		fakeBBSClient.SubscribeToInstanceEventsByCellIDReturns(fakeInstanceEventSource, nil)

		actualLRP = model_helpers.NewValidActualLRP("some-actual", 0)

		desiredCounter := 0
		fakeEventSource.NextStub = func() (models.Event, error) {
			desiredCounter += 1
			switch desiredCounter {
			case 1:
				//lint:ignore SA1019 - cfdot needs to process deprecated ActualLRP data until it is removed from BBS
				return models.NewActualLRPCreatedEvent(actualLRP.ToActualLRPGroup()), nil
			case 2:
				//lint:ignore SA1019 - cfdot needs to process deprecated ActualLRP data until it is removed from BBS
				return models.NewActualLRPRemovedEvent(actualLRP.ToActualLRPGroup()), nil
			default:
				return nil, io.EOF
			}
		}

		actualCounter := 0
		fakeInstanceEventSource.NextStub = func() (models.Event, error) {
			actualCounter += 1
			switch actualCounter {
			case 1:
				return models.NewActualLRPInstanceCreatedEvent(actualLRP, "some-trace-id"), nil
			case 2:
				return models.NewActualLRPInstanceRemovedEvent(actualLRP, "some-trace-id"), nil
			default:
				return nil, io.EOF
			}
		}
	})

	var eventString = func(event models.Event) string {
		lrpEvent := commands.LRPEvent{
			Type: event.EventType(),
			Data: event,
		}
		data, err := json.Marshal(lrpEvent)
		Expect(err).NotTo(HaveOccurred())
		return string(data)
	}

	It("prints a JSON object for each event that occurred", func() {
		expectedEvents := []string{
			//lint:ignore SA1019 - cfdot needs to process deprecated ActualLRP data until it is removed from BBS
			eventString(models.NewActualLRPCreatedEvent(actualLRP.ToActualLRPGroup())),
			//lint:ignore SA1019 - cfdot needs to process deprecated ActualLRP data until it is removed from BBS
			eventString(models.NewActualLRPRemovedEvent(actualLRP.ToActualLRPGroup())),
			eventString(models.NewActualLRPInstanceCreatedEvent(actualLRP, "some-trace-id")),
			eventString(models.NewActualLRPInstanceRemovedEvent(actualLRP, "some-trace-id")),
		}

		err := commands.LRPEvents(stdout, stderr, fakeBBSClient, "", false)
		Expect(err).NotTo(HaveOccurred())

		stdoutData := strings.TrimSpace(string(stdout.Contents()))
		lines := strings.Split(stdoutData, "\n")
		Expect(lines).To(HaveLen(4))

		Expect(lines).To(ConsistOf(expectedEvents))
	})

	Context("when --exclude-actual-lrp-groups flag is set", func() {
		It("only prints instance events", func() {
			expectedEvents := []string{
				eventString(models.NewActualLRPInstanceCreatedEvent(actualLRP, "some-trace-id")),
				eventString(models.NewActualLRPInstanceRemovedEvent(actualLRP, "some-trace-id")),
			}

			err := commands.LRPEvents(stdout, stderr, fakeBBSClient, "", true)
			Expect(err).NotTo(HaveOccurred())

			stdoutData := strings.TrimSpace(string(stdout.Contents()))
			lines := strings.Split(stdoutData, "\n")
			Expect(lines).To(HaveLen(2))

			Expect(lines).To(ConsistOf(expectedEvents))
		})
	})

	Context("deduping DesiredLRP events", func() {
		var event models.Event

		JustBeforeEach(func() {
			fakeEventSource.NextReturnsOnCall(0, event, nil)
			fakeEventSource.NextReturnsOnCall(1, nil, io.EOF)
			fakeInstanceEventSource.NextReturnsOnCall(0, event, nil)
			fakeInstanceEventSource.NextReturnsOnCall(1, nil, io.EOF)
		})

		Context("when there are duplicate DesiredLRPCreatedEvents", func() {
			BeforeEach(func() {
				event = models.NewDesiredLRPCreatedEvent(lrp, "some-trace-id")
			})

			It("dedups them in the output", func() {
				err := commands.LRPEvents(stdout, stderr, fakeBBSClient, "", false)
				Expect(err).NotTo(HaveOccurred())

				desiredLRPEvent := commands.LRPEvent{
					Type: event.EventType(),
					Data: event,
				}
				data, err := json.Marshal(desiredLRPEvent)
				Expect(err).NotTo(HaveOccurred())
				desiredData := string(data)

				stdoutData := strings.TrimSpace(string(stdout.Contents()))
				Expect(stdoutData).To(Equal(desiredData))
			})
		})

		Context("when there are duplicate DesiredLRPChangedEvents", func() {
			BeforeEach(func() {
				event = models.NewDesiredLRPChangedEvent(lrp, lrp, "some-trace-id")
			})

			It("dedups them in the output", func() {
				err := commands.LRPEvents(stdout, stderr, fakeBBSClient, "", false)
				Expect(err).NotTo(HaveOccurred())

				desiredLRPEvent := commands.LRPEvent{
					Type: event.EventType(),
					Data: event,
				}
				data, err := json.Marshal(desiredLRPEvent)
				Expect(err).NotTo(HaveOccurred())
				desiredData := string(data)

				stdoutData := strings.TrimSpace(string(stdout.Contents()))
				Expect(stdoutData).To(Equal(desiredData))
			})
		})

		Context("when there are duplicate DesiredLRPRemovedEvents", func() {
			BeforeEach(func() {
				event = models.NewDesiredLRPRemovedEvent(lrp, "some-trace-id")
			})

			It("dedups them in the output", func() {
				err := commands.LRPEvents(stdout, stderr, fakeBBSClient, "", false)
				Expect(err).NotTo(HaveOccurred())

				desiredLRPEvent := commands.LRPEvent{
					Type: event.EventType(),
					Data: event,
				}
				data, err := json.Marshal(desiredLRPEvent)
				Expect(err).NotTo(HaveOccurred())
				desiredData := string(data)

				stdoutData := strings.TrimSpace(string(stdout.Contents()))
				Expect(stdoutData).To(Equal(desiredData))
			})
		})
	})

	It("closes the event streams", func() {
		err := commands.LRPEvents(stdout, stderr, fakeBBSClient, "", false)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeEventSource.CloseCallCount()).To(Equal(1))
		Expect(fakeInstanceEventSource.CloseCallCount()).To(Equal(1))
	})

	Context("when failing to subscribe to events", func() {
		BeforeEach(func() {
			fakeBBSClient.SubscribeToEventsByCellIDReturns(nil, errors.New("failed to connect"))
		})

		It("returns an error", func() {
			err := commands.LRPEvents(stdout, stderr, fakeBBSClient, "", false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to connect"))
		})
	})

	Context("when failing to receive an event", func() {
		BeforeEach(func() {
			fakeEventSource.NextStub = nil
			fakeEventSource.NextReturns(nil, errors.New("boom"))
		})

		It("returns an error", func() {
			err := commands.LRPEvents(stdout, stderr, fakeBBSClient, "", false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("boom"))
		})
	})
})
