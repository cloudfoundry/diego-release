package workpool_test

import (
	"code.cloudfoundry.org/workpool"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Throttler", func() {
	var throttler *workpool.Throttler
	var expectedResults []int

	Context("when max workers is non-positive", func() {
		It("errors", func() {
			_, err := workpool.NewThrottler(0, []func(){})
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when max workers is positive", func() {
		var maxWorkers int
		var calledChan chan int
		var unblockChan chan struct{}
		var work func(int) func()

		BeforeEach(func() {
			maxWorkers = 2
			calledChan = make(chan int)
			unblockChan = make(chan struct{})
			work = func(i int) func() {
				return func() {
					calledChan := calledChan
					unblockChan := unblockChan
					calledChan <- i
					<-unblockChan
				}
			}
		})

		AfterEach(func() {
			close(calledChan)
			close(unblockChan)
		})

		Describe("Work", func() {
			Context("when requesting less work than the max number of workers", func() {
				BeforeEach(func() {
					works := make([]func(), maxWorkers-1)
					for i := range works {
						works[i] = work(i)
					}

					var err error
					throttler, err = workpool.NewThrottler(maxWorkers, works)
					Expect(err).NotTo(HaveOccurred())
					expectedResults = RangeArray(maxWorkers - 1)
				})

				It("should run the passed-in work", func() {
					go throttler.Work()

					results := []int{}
					for i := 0; i < maxWorkers-1; i++ {
						val := <-calledChan
						results = append(results, val)
					}
					Expect(results).To(ConsistOf(expectedResults))
				})
			})

			Context("when submitting work equal to the number of workers", func() {
				BeforeEach(func() {
					works := make([]func(), maxWorkers)
					for i := range works {
						works[i] = work(i)
					}

					var err error
					throttler, err = workpool.NewThrottler(maxWorkers, works)
					Expect(err).NotTo(HaveOccurred())
					expectedResults = RangeArray(maxWorkers)
				})

				It("should run the passed-in work concurrently", func() {
					go throttler.Work()

					results := []int{}
					for i := 0; i < maxWorkers; i++ {
						val := <-calledChan
						results = append(results, val)
					}
					Expect(results).To(ConsistOf(expectedResults))
				})
			})

			Context("when submitting more work than the max number of workers", func() {
				BeforeEach(func() {
					works := make([]func(), maxWorkers+1)
					for i := range works {
						works[i] = work(i)
					}

					var err error
					throttler, err = workpool.NewThrottler(maxWorkers, works)
					Expect(err).NotTo(HaveOccurred())
					expectedResults = RangeArray(maxWorkers)
				})

				It("should run the passed-in work concurrently up to the max number of workers at a time", func() {
					go throttler.Work()

					results := []int{}
					for i := 0; i < maxWorkers; i++ {
						val := <-calledChan
						results = append(results, val)
					}
					Expect(results).To(ConsistOf(expectedResults))
					Consistently(calledChan).ShouldNot(Receive())

					unblockChan <- struct{}{}

					Eventually(calledChan).Should(Receive(Equal(maxWorkers)))
				})
			})
		})
	})
})
