package containerstore_test

import (
	"time"

	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

var logger *lagertest.TestLogger

func TestContainerstore(t *testing.T) {
	SetDefaultConsistentlyDuration(5 * time.Second)
	SetDefaultEventuallyTimeout(5 * time.Second)
	RegisterFailHandler(Fail)

	RunSpecs(t, "Containerstore Suite")
}

var _ = BeforeEach(func() {
	logger = lagertest.NewTestLogger("test")
})
