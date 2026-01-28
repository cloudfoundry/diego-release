package auctionrunner_test

import (
	"sort"
	"time"

	"code.cloudfoundry.org/auction/auctionrunner"
	"code.cloudfoundry.org/auction/auctiontypes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sortable Auctions", func() {
	Describe("LRP Auctions", func() {
		var lrps []auctiontypes.LRPAuction

		JustBeforeEach(func() {
			sort.Sort(auctionrunner.SortableLRPAuctions(lrps))
		})

		Context("when LRP indexes match", func() {
			BeforeEach(func() {
				lrps = []auctiontypes.LRPAuction{
					BuildLRPAuction("pg-6", "domain", 0, "linux", 10, 10, 10, time.Time{}, nil, []string{}),
					BuildLRPAuction("pg-7", "domain", 0, "linux", 20, 10, 10, time.Time{}, nil, []string{}),
					BuildLRPAuction("pg-8", "domain", 0, "linux", 30, 10, 10, time.Time{}, nil, []string{}),
					BuildLRPAuction("pg-9", "domain", 0, "linux", 40, 10, 10, time.Time{}, nil, []string{}),
				}
			})

			It("sorts boulders before pebbles", func() {
				Expect(lrps[0].ProcessGuid).To((Equal("pg-9")))
				Expect(lrps[1].ProcessGuid).To((Equal("pg-8")))
				Expect(lrps[2].ProcessGuid).To((Equal("pg-7")))
				Expect(lrps[3].ProcessGuid).To((Equal("pg-6")))
			})
		})

		Context("when LRP indexes differ", func() {
			BeforeEach(func() {
				lrps = make([]auctiontypes.LRPAuction, 5)
				for i := cap(lrps) - 1; i >= 0; i-- {
					lrps[i] = BuildLRPAuction("pg", "domain", i, "linux", int32(40+i), int32(40+i), int32(10+i), time.Time{}, nil, []string{})
				}
			})

			It("sorts by index", func() {
				for i := 0; i < len(lrps); i++ {
					Expect(lrps[i].Index).To(BeEquivalentTo(i))
				}
			})
		})
	})

	Describe("Task Auctions", func() {
		var tasks []auctiontypes.TaskAuction

		BeforeEach(func() {
			tasks = []auctiontypes.TaskAuction{
				BuildTaskAuction(BuildTask("tg-6", "domain", "linux", 10, 10, 10, []string{}, []string{}), time.Time{}),
				BuildTaskAuction(BuildTask("tg-7", "domain", "linux", 20, 10, 10, []string{}, []string{}), time.Time{}),
				BuildTaskAuction(BuildTask("tg-8", "domain", "linux", 30, 10, 10, []string{}, []string{}), time.Time{}),
				BuildTaskAuction(BuildTask("tg-9", "domain", "linux", 40, 10, 10, []string{}, []string{}), time.Time{}),
			}

			sort.Sort(auctionrunner.SortableTaskAuctions(tasks))
		})

		It("sorts boulders before pebbles", func() {
			Expect(tasks[0].Task.TaskGuid).To((Equal("tg-9")))
			Expect(tasks[1].Task.TaskGuid).To((Equal("tg-8")))
			Expect(tasks[2].Task.TaskGuid).To((Equal("tg-7")))
			Expect(tasks[3].Task.TaskGuid).To((Equal("tg-6")))
		})
	})
})
