package handlers_test

import (
	"os"
	"runtime"

	"code.cloudfoundry.org/diego-ssh/keys"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"

	"testing"
)

var TestHostKey ssh.Signer

func TestHandlers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Handlers Suite")
}

var _ = BeforeSuite(func() {
	hostKey, err := keys.RSAKeyPairFactory.NewKeyPair(1024)
	Expect(err).NotTo(HaveOccurred())

	TestHostKey = hostKey.PrivateKey()

	if runtime.GOOS == "windows" {
		if os.Getenv("WINPTY_DLL_DIR") == "" {
			Fail("Missing WINPTY_DLL_DIR environment variable")
		}
	}
})
