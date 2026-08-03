package auctionmetricemitterdelegate_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAuctionmetricemitterdelegate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auction Metric Emitter Delegate Suite")
}
