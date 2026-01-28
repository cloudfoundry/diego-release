package main_test

import (
	"encoding/json"
	"fmt"
	"runtime"
	"testing"
	"time"

	"code.cloudfoundry.org/diego-ssh/cmd/sshd/testrunner"
	"code.cloudfoundry.org/diego-ssh/keys"
	"code.cloudfoundry.org/inigo/helpers/portauthority"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
)

var (
	sshProxyPath string
	sshdPath     string
	sshdProcess  ifrit.Process

	sshdPort             uint16
	sshdTLSPort          uint16
	sshdContainerPort    uint16
	sshdContainerTLSPort uint16
	sshProxyPort         uint16
	healthCheckProxyPort uint16

	sshdAddress string

	hostKeyPem          string
	privateKeyPem       string
	publicAuthorizedKey string

	portAllocator portauthority.PortAllocator
)

func TestSSHProxy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SSH Proxy Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	sshProxy, err := gexec.Build("code.cloudfoundry.org/diego-ssh/cmd/ssh-proxy", "-race")
	Expect(err).NotTo(HaveOccurred())

	sshd, err := gexec.Build("code.cloudfoundry.org/diego-ssh/cmd/sshd", "-race")
	Expect(err).NotTo(HaveOccurred())

	hostKey, err := keys.RSAKeyPairFactory.NewKeyPair(1024)
	Expect(err).NotTo(HaveOccurred())

	privateKey, err := keys.RSAKeyPairFactory.NewKeyPair(1024)
	Expect(err).NotTo(HaveOccurred())

	payload, err := json.Marshal(map[string]string{
		"ssh-proxy":      sshProxy,
		"sshd":           sshd,
		"host-key":       hostKey.PEMEncodedPrivateKey(),
		"private-key":    privateKey.PEMEncodedPrivateKey(),
		"authorized-key": privateKey.AuthorizedKey(),
	})

	Expect(err).NotTo(HaveOccurred())

	return payload
}, func(payload []byte) {
	context := map[string]string{}

	err := json.Unmarshal(payload, &context)
	Expect(err).NotTo(HaveOccurred())

	hostKeyPem = context["host-key"]
	privateKeyPem = context["private-key"]
	publicAuthorizedKey = context["authorized-key"]

	node := GinkgoParallelProcess()
	startPort := 1070*node + 10
	portRange := 1000
	endPort := startPort + portRange

	portAllocator, err = portauthority.New(startPort, endPort)
	Expect(err).NotTo(HaveOccurred())

	sshdPort, err = portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())

	sshdContainerPort, err = portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())
	sshdPath = context["sshd"]

	sshdTLSPort, err = portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())

	sshdContainerTLSPort, err = portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())
	sshdPath = context["sshd"]

	sshProxyPort, err = portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())
	sshProxyPath = context["ssh-proxy"]

	healthCheckProxyPort, err = portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())
})

var _ = BeforeEach(func() {

	if runtime.GOOS == "windows" {
		Skip("SSH not supported on Windows, and SSH proxy never runs on Windows anyway")
	}

	sshdAddress = fmt.Sprintf("127.0.0.1:%d", sshdPort)
	sshdArgs := testrunner.Args{
		Address:       sshdAddress,
		HostKey:       hostKeyPem,
		AuthorizedKey: publicAuthorizedKey,
	}

	runner := testrunner.New(sshdPath, sshdArgs)
	sshdProcess = ifrit.Invoke(runner)
})

var _ = AfterEach(func() {
	ginkgomon.Kill(sshdProcess, 5*time.Second)
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
})
