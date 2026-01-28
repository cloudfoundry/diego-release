package simulation_test

import (
	"time"

	"code.cloudfoundry.org/auction/auctiontypes"
)

type auctionMetricEmitterDelegate struct{}

func NewAuctionMetricEmitterDelegate() auctionMetricEmitterDelegate {
	return auctionMetricEmitterDelegate{}
}

func (auctionMetricEmitterDelegate) FetchStatesCompleted(_ time.Duration) error {
	return nil
}

func (auctionMetricEmitterDelegate) FailedCellStateRequest() error { return nil }

func (auctionMetricEmitterDelegate) AuctionCompleted(_ auctiontypes.AuctionResults) error { return nil }
