package containermetrics_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestContainerMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ContainerMetrics Suite")
}
