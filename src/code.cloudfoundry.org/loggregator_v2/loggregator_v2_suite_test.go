package loggregator_v2_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLoggregatorV2(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LoggregatorV2 Suite")
}
