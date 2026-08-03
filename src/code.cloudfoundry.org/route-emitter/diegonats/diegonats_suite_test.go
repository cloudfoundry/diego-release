package diegonats_test

import (
	"encoding/json"
	"os"
	"testing"

	"code.cloudfoundry.org/inigo/helpers/portauthority"
	"code.cloudfoundry.org/route-emitter/diegonats/natsserverrunner"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
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

var _ = SynchronizedBeforeSuite(func() []byte {
	natsServer := os.Getenv("NATS_SERVER_BINARY")
	if natsServer == "" {
		var err error
		natsServer, err = gexec.Build("github.com/nats-io/nats-server/v2", "-race")
		Expect(err).NotTo(HaveOccurred())
	}
	payload, err := json.Marshal(map[string]string{
		"nats-server": natsServer,
	})
	Expect(err).NotTo(HaveOccurred())
	return payload
}, func(payload []byte) {
	binaries := map[string]string{}
	Expect(json.Unmarshal(payload, &binaries)).To(Succeed())
	Expect(os.Setenv("NATS_SERVER_BINARY", binaries["nats-server"])).To(Succeed())

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

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
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
