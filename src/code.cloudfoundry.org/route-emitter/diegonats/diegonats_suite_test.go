package diegonats_test

import (
	"testing"

	"code.cloudfoundry.org/inigo/helpers/portauthority"
	"code.cloudfoundry.org/route-emitter/diegonats/natsserverrunner"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
)

func TestDiegoNATS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Diego NATS Suite")
}

var (
	natsPort          uint16
	natsServerProcess ifrit.Process
	portAllocator     portauthority.PortAllocator
)

var _ = BeforeSuite(func() {
	node := GinkgoParallelProcess()
	startPort := 1050 * node
	portRange := 1000
	endPort := startPort + portRange
	var err error
	portAllocator, err = portauthority.New(startPort, endPort)
	Expect(err).NotTo(HaveOccurred())

	natsPort, err = portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
})

func startNATS() {
	natsServerProcess = ginkgomon.Invoke(natsserverrunner.NewNatsServerTestRunner(int(natsPort)))
}

func startNATSWithTLS(caFile, certFile, keyFile string) {
	natsServerProcess = ginkgomon.Invoke(natsserverrunner.NewNatsServerWithTLSTestRunner(int(natsPort), caFile, certFile, keyFile))
}

func stopNATS() {
	ginkgomon.Kill(natsServerProcess)
}
