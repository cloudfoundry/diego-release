package main_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"code.cloudfoundry.org/buildpackapplifecycle/test_helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	builderPath string
	tarPath     string
)

func TestBuildpackLifecycleBuilder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Buildpack-Lifecycle-Builder Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {

	var builderArgs []string
	var discoveredTarPath string
	if runtime.GOOS == "windows" {
		builderArgs = []string{"-tags=windows2012R2"}
		discoveredTarPath = test_helpers.DownloadOrFindWindowsTar()
	}

	builder := buildBuilder(builderArgs)

	return []byte(builder + "^" + discoveredTarPath)
}, func(exePaths []byte) {
	paths := strings.Split(string(exePaths), "^")
	builderPath = paths[0]
	tarPath = paths[1]

	SetDefaultEventuallyTimeout(10 * time.Second)
})

var _ = SynchronizedAfterSuite(func() {
	//noop
}, func() {
	gexec.CleanupBuildArtifacts()
	if test_helpers.GetWindowsTarURL() != "" {
		Expect(os.RemoveAll(filepath.Dir(tarPath))).To(Succeed())
	}
})
