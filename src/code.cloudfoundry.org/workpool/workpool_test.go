package workpool_test

import (
	"code.cloudfoundry.org/workpool"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkPool", func() {
	var pool *workpool.WorkPool

	AfterEach(func() {
		if pool != nil {
			pool.Stop()
		}
	})

	Context("when max workers is non-positive", func() {
		It("errors", func() {
			_, err := workpool.NewWorkPool(0)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when max workers is positive", func() {
		var maxWorkers int
		var calledChan chan int
		var unblockChan chan struct{}
		var work func(int) func()
		var expectedResults []int

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

			var err error
			pool, err = workpool.NewWorkPool(maxWorkers)
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("Submit", func() {
			Context("when submitting less work than the max number of workers", func() {

				BeforeEach(func() {
					expectedResults = RangeArray(maxWorkers - 1)
				})

				It("should run the passed-in work", func() {
					for i := 0; i < maxWorkers-1; i++ {
						pool.Submit(work(i))
					}

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
					expectedResults = RangeArray(maxWorkers)
				})
				It("should run the passed-in work concurrently", func() {
					for i := 0; i < maxWorkers; i++ {
						pool.Submit(work(i))
					}

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
					expectedResults = RangeArray(maxWorkers)
				})
				It("should run the passed-in work concurrently up to the max number of workers at a time", func() {
					for i := 0; i < maxWorkers+1; i++ {
						pool.Submit(work(i))
					}

					results := []int{}
					for i := 0; i < maxWorkers; i++ {
						val := <-calledChan
						results = append(results, val)
					}
					Expect(results).To(ConsistOf(expectedResults))
					Consistently(calledChan).ShouldNot(Receive(Equal(maxWorkers)))

					unblockChan <- struct{}{}

					Eventually(calledChan).Should(Receive(Equal(maxWorkers)))
				})
			})
		})

		Describe("Stop", func() {
			BeforeEach(func() {
				expectedResults = RangeArray(maxWorkers)
			})
			It("does not start any newly-submitted work", func() {
				pool.Stop()
				pool.Submit(work(0))

				Consistently(calledChan).ShouldNot(Receive())
			})

			It("does not start any pending work", func() {
				for i := 0; i < maxWorkers+1; i++ {
					pool.Submit(work(i))
				}

				results := []int{}
				for i := 0; i < maxWorkers; i++ {
					val := <-calledChan
					results = append(results, val)
				}
				Expect(results).To(ConsistOf(expectedResults))
				Consistently(calledChan).ShouldNot(Receive(Equal(maxWorkers)))

				pool.Stop()
				unblockChan <- struct{}{}

				Consistently(calledChan).ShouldNot(Receive())
			})
		})
	})
})
