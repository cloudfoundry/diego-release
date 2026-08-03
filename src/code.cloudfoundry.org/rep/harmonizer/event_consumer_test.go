package harmonizer_test

import (
	"errors"
	"os"

	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/operationq"
	"code.cloudfoundry.org/operationq/fake_operationq"
	"code.cloudfoundry.org/rep/generator/fake_generator"
	"code.cloudfoundry.org/rep/harmonizer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("EventConsumer", func() {
	var (
		logger        *lagertest.TestLogger
		fakeGenerator *fake_generator.FakeGenerator
		fakeQueue     *fake_operationq.FakeQueue

		consumer *harmonizer.EventConsumer
		process  ifrit.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeGenerator = new(fake_generator.FakeGenerator)
		fakeQueue = new(fake_operationq.FakeQueue)

		consumer = harmonizer.NewEventConsumer(logger, fakeGenerator, fakeQueue)
	})

	JustBeforeEach(func() {
		process = ifrit.Invoke(consumer)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait()).Should(Receive())
	})

	Context("when subscribing to the operation stream succeeds", func() {
		var (
			receivedOperations chan<- operationq.Operation
		)

		BeforeEach(func() {
			operations := make(chan operationq.Operation)
			receivedOperations = operations

			fakeGenerator.OperationStreamReturns(operations, nil)
		})

		Context("when an operation is received", func() {
			var fakeOperation *fake_operationq.FakeOperation

			BeforeEach(func() {
				fakeOperation = new(fake_operationq.FakeOperation)
			})

			It("pushes it onto the queue", func() {
				receivedOperations <- fakeOperation

				Eventually(fakeQueue.PushCallCount).Should(Equal(1))
				Expect(fakeQueue.PushArgsForCall(0)).To(Equal(fakeOperation))
			})
		})

		Context("when the operation stream terminates", func() {
			It("exits happily", func() {
				close(receivedOperations)

				Eventually(process.Wait()).Should(Receive(BeNil()))
			})
		})
	})

	Context("when subscribing to events fails", func() {
		disaster := errors.New("nope")

		BeforeEach(func() {
			fakeGenerator.OperationStreamReturns(nil, disaster)
		})

		It("exits with failure", func() {
			Eventually(process.Wait()).Should(Receive(Equal(disaster)))
		})
	})
})
