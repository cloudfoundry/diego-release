package server_test

import (
	"code.cloudfoundry.org/inigo/helpers/portauthority"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

var portAllocator portauthority.PortAllocator
var _ = BeforeSuite(func() {
	node := GinkgoParallelProcess()
	startPort := 1070 * node
	portRange := 1000
	endPort := startPort + portRange

	var err error
	portAllocator, err = portauthority.New(startPort, endPort)
	Expect(err).NotTo(HaveOccurred())
})

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Suite")
}
