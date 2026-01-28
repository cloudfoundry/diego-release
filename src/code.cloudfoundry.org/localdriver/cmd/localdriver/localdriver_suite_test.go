package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"

	"testing"
)

func TestLocalDriver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LocalDriver Main Suite")
}

var driverPath string

var _ = BeforeSuite(func() {
	var err error
	driverPath, err = Build("code.cloudfoundry.org/localdriver/cmd/localdriver")
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	CleanupBuildArtifacts()
})
