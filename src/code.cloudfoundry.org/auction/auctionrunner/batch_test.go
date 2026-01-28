package auctionrunner_test

import (
	"time"

	"code.cloudfoundry.org/auction/auctionrunner"
	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/clock/fakeclock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Batch", func() {
	var lrpStart auctioneer.LRPStartRequest
	var task auctioneer.TaskStartRequest
	var batch *auctionrunner.Batch
	var clock *fakeclock.FakeClock

	BeforeEach(func() {
		clock = fakeclock.NewFakeClock(time.Now())
		batch = auctionrunner.NewBatch(clock)
	})

	It("should start off empty", func() {
		Expect(batch.HasWork).NotTo(Receive())
		starts, tasks := batch.DedupeAndDrain()
		Expect(starts).To(BeEmpty())
		Expect(tasks).To(BeEmpty())
	})

	Describe("adding work", func() {
		Context("when adding start auctions", func() {
			BeforeEach(func() {
				lrpStart = BuildLRPStartRequest("pg-1", "domain", []int{1}, "linux", 10, 10, 10, []string{}, []string{})
				batch.AddLRPStarts([]auctioneer.LRPStartRequest{lrpStart}, "some-trace-id")
			})

			It("makes the start auction available when drained", func() {
				lrpAuctions, _ := batch.DedupeAndDrain()
				Expect(lrpAuctions).To(ConsistOf(BuildLRPAuctions(lrpStart, clock.Now())))
			})

			It("should have work", func() {
				Expect(batch.HasWork).To(Receive())
			})
		})

		Context("when adding tasks", func() {
			BeforeEach(func() {
				task = BuildTaskStartRequest("tg-1", "domain", "linux", 10, 10, 10)
				batch.AddTasks([]auctioneer.TaskStartRequest{task}, "some-trace-id")
			})

			It("makes the stop auction available when drained", func() {
				_, taskAuctions := batch.DedupeAndDrain()
				Expect(taskAuctions).To(ConsistOf(BuildTaskAuction(&task.Task, clock.Now())))
			})

			It("should have work", func() {
				Expect(batch.HasWork).To(Receive())
			})
		})
	})

	Describe("DedupeAndDrain", func() {
		BeforeEach(func() {
			batch.AddLRPStarts([]auctioneer.LRPStartRequest{
				BuildLRPStartRequest("pg-1", "domain", []int{1}, "linux", 10, 10, 10, []string{"driver-1"}, []string{"tag-1"}),
				BuildLRPStartRequest("pg-1", "domain", []int{1}, "linux", 10, 10, 10, []string{"driver-1"}, []string{"tag-1"}),
				BuildLRPStartRequest("pg-2", "domain", []int{2}, "linux", 10, 10, 10, []string{"driver-2"}, []string{"tag-2"}),
			}, "some-trace-id")

			batch.AddTasks([]auctioneer.TaskStartRequest{
				BuildTaskStartRequest("tg-1", "domain", "linux", 10, 10, 10),
				BuildTaskStartRequest("tg-1", "domain", "linux", 10, 10, 10),
				BuildTaskStartRequest("tg-2", "domain", "linux", 10, 10, 10)}, "some-trace-id")
		})

		It("should dedupe any duplicate start auctions and stop auctions", func() {
			lrpAuctions, taskAuctions := batch.DedupeAndDrain()
			Expect(lrpAuctions).To(Equal([]auctiontypes.LRPAuction{
				BuildLRPAuction("pg-1", "domain", 1, "linux", 10, 10, 10, clock.Now(), []string{"driver-1"}, []string{"tag-1"}),
				BuildLRPAuction("pg-2", "domain", 2, "linux", 10, 10, 10, clock.Now(), []string{"driver-2"}, []string{"tag-2"}),
			}))

			Expect(taskAuctions).To(Equal([]auctiontypes.TaskAuction{
				BuildTaskAuction(
					BuildTask("tg-1", "domain", "linux", 10, 10, 10, []string{}, []string{}),
					clock.Now(),
				),
				BuildTaskAuction(
					BuildTask("tg-2", "domain", "linux", 10, 10, 10, []string{}, []string{}),
					clock.Now(),
				),
			}))
		})

		It("should clear out its cache, so a subsequent call shouldn't fetch anything", func() {
			batch.DedupeAndDrain()
			lrpAuctions, taskAuctions := batch.DedupeAndDrain()
			Expect(lrpAuctions).To(BeEmpty())
			Expect(taskAuctions).To(BeEmpty())
		})

		It("should no longer have work after draining", func() {
			batch.DedupeAndDrain()
			Expect(batch.HasWork).NotTo(Receive())
		})

		It("should not hang forever if the work channel was already drained", func() {
			Expect(batch.HasWork).To(Receive())
			batch.DedupeAndDrain()
			Expect(batch.HasWork).NotTo(Receive())
		})
	})
})
