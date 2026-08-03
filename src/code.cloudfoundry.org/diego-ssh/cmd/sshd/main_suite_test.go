package main_test

import (
	"encoding/json"
	"os"
	"runtime"

	"code.cloudfoundry.org/diego-ssh/keys"
	"code.cloudfoundry.org/inigo/helpers/portauthority"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	sshdPath string

	sshdPort            uint16
	hostKeyPem          string
	privateKeyPem       string
	publicAuthorizedKey string

	portAllocator portauthority.PortAllocator
)

func TestSSHDaemon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sshd Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	if runtime.GOOS == "windows" {
		if os.Getenv("WINPTY_DLL_DIR") == "" {
			Fail("Missing WINPTY_DLL_DIR environment variable")
		}
	}
	sshd := buildSshd()

	hostKey, err := keys.RSAKeyPairFactory.NewKeyPair(1024)
	Expect(err).NotTo(HaveOccurred())

	privateKey, err := keys.RSAKeyPairFactory.NewKeyPair(1024)
	Expect(err).NotTo(HaveOccurred())

	payload, err := json.Marshal(map[string]string{
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
	startPort := 1070 * node
	portRange := 1000
	endPort := startPort + portRange

	portAllocator, err = portauthority.New(startPort, endPort)
	Expect(err).NotTo(HaveOccurred())

	sshdPort, err = portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())

	sshdPath = context["sshd"]
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
})
