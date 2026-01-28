package auctionmetricemitterdelegate_test

import (
	"time"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/auctioneer/auctionmetricemitterdelegate"
	"code.cloudfoundry.org/bbs/models"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/rep"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Auction Metric Emitter Delegate", func() {
	var delegate auctiontypes.AuctionMetricEmitterDelegate
	var fakeMetronClient *mfakes.FakeIngressClient

	BeforeEach(func() {
		fakeMetronClient = &mfakes.FakeIngressClient{}

		delegate = auctionmetricemitterdelegate.New(fakeMetronClient)
	})

	Describe("AuctionCompleted", func() {
		It("should adjust the metric counters", func() {
			resource := rep.NewResource(10, 10, 10)
			pc := rep.NewPlacementConstraint("linux", []string{}, []string{})
			delegate.AuctionCompleted(auctiontypes.AuctionResults{
				SuccessfulLRPs: []auctiontypes.LRPAuction{
					{
						LRP: rep.NewLRP("", models.NewActualLRPKey("successful-start", 0, "domain"), resource, pc),
					},
				},
				SuccessfulTasks: []auctiontypes.TaskAuction{
					{
						Task: rep.NewTask("successful-task", "domain", resource, pc),
					},
				},
				FailedLRPs: []auctiontypes.LRPAuction{
					{
						LRP:           rep.NewLRP("", models.NewActualLRPKey("insufficient-capacity", 0, "domain"), resource, pc),
						AuctionRecord: auctiontypes.AuctionRecord{PlacementError: "insufficient resources"},
					},
					{
						LRP:           rep.NewLRP("", models.NewActualLRPKey("incompatible-stacks", 0, "domain"), resource, pc),
						AuctionRecord: auctiontypes.AuctionRecord{PlacementError: "insufficient resources"},
					},
				},
				FailedTasks: []auctiontypes.TaskAuction{
					{
						Task:          rep.NewTask("failed-task", "domain", resource, pc),
						AuctionRecord: auctiontypes.AuctionRecord{PlacementError: "insufficient resources"},
					},
				},
			})

			Expect(fakeMetronClient.IncrementCounterWithDeltaCallCount()).To(Equal(4))

			name, value := fakeMetronClient.IncrementCounterWithDeltaArgsForCall(0)
			Expect(name).To(Equal("AuctioneerLRPAuctionsStarted"))
			Expect(value).To(BeEquivalentTo(1))

			name, value = fakeMetronClient.IncrementCounterWithDeltaArgsForCall(1)
			Expect(name).To(Equal("AuctioneerTaskAuctionsStarted"))
			Expect(value).To(BeEquivalentTo(1))

			name, value = fakeMetronClient.IncrementCounterWithDeltaArgsForCall(2)
			Expect(name).To(Equal("AuctioneerLRPAuctionsFailed"))
			Expect(value).To(BeEquivalentTo(2))

			name, value = fakeMetronClient.IncrementCounterWithDeltaArgsForCall(3)
			Expect(name).To(Equal("AuctioneerTaskAuctionsFailed"))
			Expect(value).To(BeEquivalentTo(1))
		})
	})

	Describe("FetchStatesCompleted", func() {
		It("should adjust the metric counters", func() {
			err := delegate.FetchStatesCompleted(1 * time.Second)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetronClient.SendDurationCallCount()).To(Equal(1))
			name, value, _ := fakeMetronClient.SendDurationArgsForCall(0)
			Expect(name).To(Equal("AuctioneerFetchStatesDuration"))
			Expect(value).To(Equal(1 * time.Second))
		})
	})

	Describe("FailedCellStateRequest", func() {
		It("should adjust the metric counters", func() {
			delegate.FailedCellStateRequest()

			Expect(fakeMetronClient.IncrementCounterCallCount()).To(Equal(1))
			name := fakeMetronClient.IncrementCounterArgsForCall(0)
			Expect(name).To(Equal("AuctioneerFailedCellStateRequests"))
		})
	})
})
