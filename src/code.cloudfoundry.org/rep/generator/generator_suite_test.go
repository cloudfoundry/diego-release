package generator_test

import (
	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGenerator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Generator Suite")
}

var (
	logger  *lagertest.TestLogger
	fakeBBS *fake_bbs.FakeInternalClient
)

var _ = BeforeEach(func() {
	logger = lagertest.NewTestLogger("test")
	fakeBBS = new(fake_bbs.FakeInternalClient)
})
