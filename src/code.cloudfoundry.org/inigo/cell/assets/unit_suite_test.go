package assets_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func TestUnit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Fake App Unit Test Suite")
}

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
