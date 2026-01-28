package vollocal_test

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/volman"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
)

var client volman.Manager

var defaultPluginsDirectory string
var secondPluginsDirectory string

var localDriverPath string
var localDriverServerPort int
var debugServerAddress string
var localDriverProcess ifrit.Process
var localDriverRunner *ginkgomon.Runner

var tmpDriversPath string

func TestDriver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Volman Local Client Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error

	localDriverPath, err = gexec.Build("code.cloudfoundry.org/localdriver/cmd/localdriver", "-race")
	Expect(err).NotTo(HaveOccurred())
	return []byte(localDriverPath)
}, func(pathsByte []byte) {
	path := string(pathsByte)
	localDriverPath = strings.Split(path, ",")[0]
})

var _ = BeforeEach(func() {
	var err error
	tmpDriversPath, err = os.MkdirTemp("", "driversPath")
	Expect(err).NotTo(HaveOccurred())

	defaultPluginsDirectory, err = os.MkdirTemp(os.TempDir(), "clienttest")
	Expect(err).ShouldNot(HaveOccurred())

	secondPluginsDirectory, err = os.MkdirTemp(os.TempDir(), "clienttest2")
	Expect(err).ShouldNot(HaveOccurred())

	localDriverServerPort = 9750 + GinkgoParallelProcess()

	debugServerAddress = fmt.Sprintf("0.0.0.0:%d", 9850+GinkgoParallelProcess())
	localDriverRunner = ginkgomon.New(ginkgomon.Config{
		Name: "localdriver",
		Command: exec.Command(
			localDriverPath,
			"-listenAddr", fmt.Sprintf("0.0.0.0:%d", localDriverServerPort),
			"-debugAddr", debugServerAddress,
			"-driversPath", defaultPluginsDirectory,
		),
		StartCheck: "localdriver-server.started",
	})
})

var _ = AfterEach(func() {
	ginkgomon.Kill(localDriverProcess)
})

var _ = SynchronizedAfterSuite(func() {

}, func() {
	gexec.CleanupBuildArtifacts()
})
