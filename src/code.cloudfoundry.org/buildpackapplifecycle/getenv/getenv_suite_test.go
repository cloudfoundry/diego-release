package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var getenv string

func TestGetEnv(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Getenv")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	getenvPath, err := gexec.Build("code.cloudfoundry.org/buildpackapplifecycle/getenv")
	Expect(err).NotTo(HaveOccurred())
	return []byte(getenvPath)
}, func(getenvExe []byte) {
	getenv = string(getenvExe)
})

var _ = SynchronizedAfterSuite(func() {
	//noop
}, func() {
	gexec.CleanupBuildArtifacts()
})
