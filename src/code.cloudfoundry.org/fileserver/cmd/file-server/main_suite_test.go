package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var fileServerBinary string

func TestFileServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "File Server Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	fileServerPath, err := gexec.Build("code.cloudfoundry.org/fileserver/cmd/file-server")
	Expect(err).NotTo(HaveOccurred())
	return []byte(fileServerPath)
}, func(fileServerPath []byte) {
	fileServerBinary = string(fileServerPath)

})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
})
