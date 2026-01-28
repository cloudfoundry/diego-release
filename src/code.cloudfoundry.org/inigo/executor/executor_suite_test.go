package executor_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"code.cloudfoundry.org/localip"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/inigo/helpers"
	"code.cloudfoundry.org/inigo/helpers/certauthority"
	"code.cloudfoundry.org/inigo/helpers/portauthority"
	"code.cloudfoundry.org/inigo/world"
)

var (
	componentMaker world.ComponentMaker

	gardenProcess ifrit.Process
	gardenClient  garden.Client
	suiteTempDir  string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	suiteTempDir = world.TempDir("before-suite")
	payload, err := json.Marshal(world.BuiltArtifacts{
		Executables: CompileTestedExecutables(),
	})
	Expect(err).NotTo(HaveOccurred())

	return payload
}, func(encodedBuiltArtifacts []byte) {
	var builtArtifacts world.BuiltArtifacts

	err := json.Unmarshal(encodedBuiltArtifacts, &builtArtifacts)
	Expect(err).NotTo(HaveOccurred())

	_, dbBaseConnectionString := world.DBInfo()

	localIP, err := localip.LocalIP()
	Expect(err).NotTo(HaveOccurred())

	addresses := world.ComponentAddresses{
		Garden:              fmt.Sprintf("127.0.0.1:%d", 10000+GinkgoParallelProcess()),
		NATS:                fmt.Sprintf("127.0.0.1:%d", 11000+GinkgoParallelProcess()),
		Rep:                 fmt.Sprintf("127.0.0.1:%d", 14000+GinkgoParallelProcess()),
		FileServer:          fmt.Sprintf("%s:%d", localIP, 17000+GinkgoParallelProcess()),
		Router:              fmt.Sprintf("127.0.0.1:%d", 18000+GinkgoParallelProcess()),
		RouterStatus:        fmt.Sprintf("127.0.0.1:%d", 18100+GinkgoParallelProcess()),
		RouterRoutes:        fmt.Sprintf("127.0.0.1:%d", 18200+GinkgoParallelProcess()),
		RouterRouteServices: fmt.Sprintf("127.0.0.1:%d", 18300+GinkgoParallelProcess()),
		BBS:                 fmt.Sprintf("127.0.0.1:%d", 20500+GinkgoParallelProcess()*2),
		Health:              fmt.Sprintf("127.0.0.1:%d", 20500+GinkgoParallelProcess()*2+1),
		Auctioneer:          fmt.Sprintf("127.0.0.1:%d", 23000+GinkgoParallelProcess()),
		SSHProxy:            fmt.Sprintf("127.0.0.1:%d", 23500+GinkgoParallelProcess()),
		SSHProxyHealthCheck: fmt.Sprintf("127.0.0.1:%d", 24500+GinkgoParallelProcess()),
		FakeVolmanDriver:    fmt.Sprintf("127.0.0.1:%d", 25500+GinkgoParallelProcess()),
		Locket:              fmt.Sprintf("127.0.0.1:%d", 26500+GinkgoParallelProcess()),
		SQL:                 fmt.Sprintf("%sdiego_%d", dbBaseConnectionString, GinkgoParallelProcess()),
	}

	node := GinkgoParallelProcess()
	startPort := 1000 * node
	portRange := 950
	endPort := startPort + portRange

	allocator, err := portauthority.New(startPort, endPort)
	Expect(err).NotTo(HaveOccurred())

	certDepot := world.TempDirWithParent(suiteTempDir, "cert-depot")

	certAuthority, err := certauthority.NewCertAuthority(certDepot, "ca")
	Expect(err).NotTo(HaveOccurred())

	componentMaker = world.MakeComponentMaker(builtArtifacts, addresses, allocator, certAuthority)
	componentMaker.Setup()
})

var _ = AfterSuite(func() {
	componentMaker.Teardown()

	deleteSuiteTempDir := func() error { return os.RemoveAll(suiteTempDir) }
	Eventually(deleteSuiteTempDir).Should(Succeed())
})

var _ = BeforeEach(func() {
	gardenProcess = ginkgomon.Invoke(componentMaker.Garden())
	gardenClient = componentMaker.GardenClient()
})

var _ = AfterEach(func() {
	destroyContainerErrors := helpers.CleanupGarden(gardenClient)

	helpers.StopProcesses(gardenProcess)

	Expect(destroyContainerErrors).To(
		BeEmpty(),
		"%d containers failed to be destroyed!",
		len(destroyContainerErrors),
	)
})

func TestExecutor(t *testing.T) {
	helpers.RegisterDefaultTimeouts()

	RegisterFailHandler(Fail)

	RunSpecs(t, "Executor Integration Suite")
}

func CompileTestedExecutables() world.BuiltExecutables {
	var err error

	builtExecutables := world.BuiltExecutables{}

	cwd, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Chdir(os.Getenv("GARDEN_GOPATH"))).To(Succeed())
	builtExecutables["garden"], err = gexec.Build("./cmd/gdn", "-race", "-a", "-tags", "daemon")
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Chdir(cwd)).To(Succeed())

	return builtExecutables
}
