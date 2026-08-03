package auctioncellrep_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAuctionDelegate(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auction CellRep Suite")
}
