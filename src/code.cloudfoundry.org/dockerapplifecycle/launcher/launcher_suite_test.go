package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var launcher string

func TestDockerLifecycleLauncher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Docker-App-Lifecycle-Launcher Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	launcherPath, err := gexec.Build("code.cloudfoundry.org/dockerapplifecycle/launcher", "-race")
	Expect(err).NotTo(HaveOccurred())
	return []byte(launcherPath)
}, func(launcherPath []byte) {
	launcher = string(launcherPath)
})

var _ = SynchronizedAfterSuite(func() {
	//noop
}, func() {
	gexec.CleanupBuildArtifacts()
})
