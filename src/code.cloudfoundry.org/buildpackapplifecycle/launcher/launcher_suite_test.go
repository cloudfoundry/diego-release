package main_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var launcher string
var hello string
var customEntrypoint string

const defaultTimeout = time.Second * 5
const defaultInterval = time.Millisecond * 100

func TestBuildpackLifecycleLauncher(t *testing.T) {
	SetDefaultEventuallyTimeout(defaultTimeout)
	SetDefaultEventuallyPollingInterval(defaultInterval)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Buildpack-Lifecycle-Launcher Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	helloPath, err := gexec.Build("code.cloudfoundry.org/buildpackapplifecycle/launcher/fixtures/hello")
	Expect(err).NotTo(HaveOccurred())

	customEntrypointPath, err := gexec.Build("code.cloudfoundry.org/buildpackapplifecycle/launcher/fixtures/custom_entrypoint")
	Expect(err).NotTo(HaveOccurred())

	launcherPath := buildLauncher()

	getenvPath, err := gexec.Build("code.cloudfoundry.org/buildpackapplifecycle/getenv")
	Expect(err).NotTo(HaveOccurred())

	return []byte(helloPath + "^" + customEntrypointPath + "^" + launcherPath + "^" + getenvPath)
}, func(exePaths []byte) {
	paths := strings.Split(string(exePaths), "^")
	hello = paths[0]
	customEntrypoint = paths[1]
	launcher = paths[2]

	if runtime.GOOS == "windows" {
		getenv := paths[3]

		launcherDir := filepath.Dir(launcher)

		getenvContents, err := os.ReadFile(getenv)
		Expect(err).NotTo(HaveOccurred())

		err = os.WriteFile(filepath.Join(launcherDir, "getenv.exe"), getenvContents, 0644)
		Expect(err).NotTo(HaveOccurred())
	}
})

var _ = SynchronizedAfterSuite(func() {
	//noop
}, func() {
	gexec.CleanupBuildArtifacts()
})
