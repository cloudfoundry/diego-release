package evacuation_context_test

import (
	"runtime"
	"sync"

	"code.cloudfoundry.org/rep/evacuation/evacuation_context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("EvacuationContext", func() {
	var (
		evacuatable        evacuation_context.Evacuatable
		evacuationReporter evacuation_context.EvacuationReporter
		evacuationNotifier evacuation_context.EvacuationNotifier
	)

	BeforeEach(func() {
		evacuatable, evacuationReporter, evacuationNotifier = evacuation_context.New()
	})

	Describe("Evacuatable", func() {
		Context("when Evacuate has not been called", func() {
			It("does not make the evacuation reporter return true for Evacuating", func() {
				Expect(evacuationReporter.Evacuating()).To(BeFalse())
			})

			It("does not close the channel provided by the evacuation notifier", func() {
				evacuateNotify := evacuationNotifier.EvacuateNotify()
				Consistently(evacuateNotify).ShouldNot(BeClosed())
			})
		})

		Context("when Evacuate has been called", func() {
			It("makes the evacuation reporter return true for Evacuating", func() {
				evacuatable.Evacuate()
				Expect(evacuationReporter.Evacuating()).To(BeTrue())
			})

			It("closes the channel provided by the evacuation notifier", func() {
				evacuateNotify := evacuationNotifier.EvacuateNotify()
				Consistently(evacuateNotify).ShouldNot(BeClosed())
				evacuatable.Evacuate()
				Eventually(evacuateNotify).Should(BeClosed())
			})
		})

		Context("when Evacuate is called repeatedly", func() {
			It("does not panic", func() {
				defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(runtime.NumCPU()))

				wg := sync.WaitGroup{}
				for i := 0; i < 5; i++ {
					wg.Add(1)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						Expect(evacuatable.Evacuate).NotTo(Panic())
					}()
				}
				wg.Wait()
			})
		})
	})
})
