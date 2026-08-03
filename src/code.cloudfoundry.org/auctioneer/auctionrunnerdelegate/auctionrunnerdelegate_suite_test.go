package auctionrunnerdelegate_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAuctionrunnerdelegate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auction Runner Delegate Suite")
}
