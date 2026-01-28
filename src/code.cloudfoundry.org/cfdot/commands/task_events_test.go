package commands_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfdot/commands"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Task Events", func() {
	var (
		fakeBBSClient   *fake_bbs.FakeClient
		fakeEventSource *eventfakes.FakeEventSource
		stdout, stderr  *gbytes.Buffer
		task            *models.Task
	)

	BeforeEach(func() {
		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()
		fakeBBSClient = &fake_bbs.FakeClient{}
		fakeEventSource = &eventfakes.FakeEventSource{}
		fakeBBSClient.SubscribeToTaskEventsReturns(fakeEventSource, nil)

		task = &models.Task{
			TaskGuid: "some-task",
		}
		count := 0
		fakeEventSource.NextStub = func() (models.Event, error) {
			count += 1
			if count > 2 {
				return nil, io.EOF
			}
			return models.NewTaskCreatedEvent(task), nil
		}
	})

	It("prints a JSON object", func() {
		event := models.NewTaskCreatedEvent(task)

		taskEvent := commands.TaskEvent{
			Type: event.EventType(),
			Data: event,
		}
		data, err := json.Marshal(taskEvent)
		Expect(err).NotTo(HaveOccurred())

		expectedLines := []string{string(data), string(data)}

		err = commands.TaskEvents(stdout, stderr, fakeBBSClient, "")
		Expect(err).NotTo(HaveOccurred())

		stdoutData := stdout.Contents()
		lines := bytes.SplitN(stdoutData, []byte{'\n'}, 3)
		Expect(lines).To(HaveLen(3))

		for i := 0; i < 2; i++ {
			Expect(string(lines[i])).To(Equal(expectedLines[i]))
		}
	})

	It("closes the event stream", func() {
		err := commands.TaskEvents(stdout, stderr, fakeBBSClient, "")
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeEventSource.CloseCallCount()).To(Equal(1))
	})

	Context("when failing to subscribe to events", func() {
		BeforeEach(func() {
			fakeBBSClient.SubscribeToTaskEventsReturns(nil, errors.New("failed to connect"))
		})

		It("returns an error", func() {
			err := commands.TaskEvents(stdout, stderr, fakeBBSClient, "")
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
			err := commands.TaskEvents(stdout, stderr, fakeBBSClient, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("boom"))
		})
	})
})
