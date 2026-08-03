package harmonizer_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/operationq"
	"code.cloudfoundry.org/operationq/fake_operationq"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context"
	"code.cloudfoundry.org/rep/generator/fake_generator"
	"code.cloudfoundry.org/rep/harmonizer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Bulker", func() {
	var (
		logger                 *lagertest.TestLogger
		pollInterval           time.Duration
		evacuationPollInterval time.Duration
		fakeClock              *fakeclock.FakeClock
		fakeGenerator          *fake_generator.FakeGenerator
		fakeQueue              *fake_operationq.FakeQueue
		evacuatable            evacuation_context.Evacuatable
		evacuationNotifier     evacuation_context.EvacuationNotifier
		fakeMetronClient       *mfakes.FakeIngressClient

		bulker  *harmonizer.Bulker
		process ifrit.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		pollInterval = 30 * time.Second
		evacuationPollInterval = 10 * time.Second
		fakeClock = fakeclock.NewFakeClock(time.Now())
		fakeGenerator = new(fake_generator.FakeGenerator)
		fakeQueue = new(fake_operationq.FakeQueue)
		fakeMetronClient = new(mfakes.FakeIngressClient)

		evacuatable, _, evacuationNotifier = evacuation_context.New()

		bulker = harmonizer.NewBulker(
			logger,
			pollInterval,
			evacuationPollInterval,
			evacuationNotifier,
			fakeClock,
			fakeGenerator,
			fakeQueue,
			fakeMetronClient,
		)
	})

	JustBeforeEach(func() {
		process = ifrit.Invoke(bulker)
		Eventually(fakeClock.WatcherCount).Should(Equal(1))
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
	})

	itPerformsBatchOperations := func(expectedQueueLength int) {
		Context("when generating the batch operations succeeds", func() {
			var (
				operation1 *fake_operationq.FakeOperation
				operation2 *fake_operationq.FakeOperation
			)

			BeforeEach(func() {
				operation1 = new(fake_operationq.FakeOperation)
				operation2 = new(fake_operationq.FakeOperation)

				fakeGenerator.BatchOperationsStub = func(lager.Logger) (map[string]operationq.Operation, error) {
					fakeClock.Increment(10 * time.Second)
					return map[string]operationq.Operation{"guid1": operation1, "guid2": operation2}, nil
				}
			})

			It("pushes them onto the queue", func() {
				Eventually(fakeQueue.PushCallCount).Should(Equal(expectedQueueLength))

				enqueuedOperations := make([]operationq.Operation, 0, 2)
				enqueuedOperations = append(enqueuedOperations, fakeQueue.PushArgsForCall(expectedQueueLength-2))
				enqueuedOperations = append(enqueuedOperations, fakeQueue.PushArgsForCall(expectedQueueLength-1))

				Expect(enqueuedOperations).To(ConsistOf(operation1, operation2))
			})

			It("emits the duration it took to generate the batch operations", func() {
				Eventually(fakeQueue.PushCallCount).Should(Equal(expectedQueueLength))

				name, value, _ := fakeMetronClient.SendDurationArgsForCall(0)
				Expect(name).To(Equal("RepBulkSyncDuration"))
				Expect(value).To(BeNumerically("==", 10*time.Second))
			})
		})

		Context("when generating the batch operations fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeGenerator.BatchOperationsReturns(nil, disaster)
			})

			It("logs the error", func() {
				Eventually(logger).Should(gbytes.Say("failed-to-generate-operations"))
				Eventually(logger).Should(gbytes.Say("nope"))
			})
		})
	}

	Context("when the poll interval elapses", func() {
		JustBeforeEach(func() {
			fakeClock.WaitForWatcherAndIncrement(pollInterval)
		})

		itPerformsBatchOperations(2)

		Context("and elapses again", func() {
			JustBeforeEach(func() {
				fakeClock.WaitForWatcherAndIncrement(pollInterval)
			})

			itPerformsBatchOperations(4)
		})
	})

	Context("when the poll interval has not elapsed", func() {
		JustBeforeEach(func() {
			fakeClock.WaitForWatcherAndIncrement(pollInterval - 1)
		})

		It("does not fetch batch operations", func() {
			Consistently(fakeGenerator.BatchOperationsCallCount).Should(BeZero())
		})
	})

	Context("when evacuation starts", func() {
		BeforeEach(func() {
			evacuatable.Evacuate()
		})

		itPerformsBatchOperations(2)

		It("batches operations only once", func() {
			Eventually(fakeGenerator.BatchOperationsCallCount).Should(Equal(1))
			Consistently(fakeGenerator.BatchOperationsCallCount).Should(Equal(1))
		})

		Context("when the evacuation interval elapses", func() {
			It("batches operations again", func() {
				Eventually(fakeGenerator.BatchOperationsCallCount).Should(Equal(1))
				fakeClock.Increment(evacuationPollInterval + time.Second)
				Eventually(fakeGenerator.BatchOperationsCallCount).Should(Equal(2))
				Consistently(fakeGenerator.BatchOperationsCallCount).Should(Equal(2))
			})
		})
	})
})
