package auctioneer_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAuctioneer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auctioneer Suite")
}
