//go:build external
// +build external

package main_test

import (
	"fmt"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func buildHealthCheck() string {
	healthCheckPath, err := gexec.Build("code.cloudfoundry.org/healthcheck/cmd/healthcheck", "-tags", "external")
	Expect(err).NotTo(HaveOccurred())
	return healthCheckPath
}

func createHTTPHealthCheck(args []string, port string) *gexec.Session {
	args = append([]string{"-uri", "/api/_ping", "-port", port, "-timeout", "100ms"}, args...)
	command := exec.Command(healthCheck, args...)
	command.Env = append(
		os.Environ(),
		fmt.Sprintf(`CF_INSTANCE_PORTS=[{"external":%s,"internal":%s}]`, port, "8080"),
	)
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return session
}

func createPortHealthCheck(args []string, port string) *gexec.Session {
	args = append([]string{"-port", port, "-timeout", "100ms"}, args...)
	command := exec.Command(healthCheck, args...)
	command.Env = append(
		os.Environ(),
		fmt.Sprintf(`CF_INSTANCE_PORTS=[{"external":%s,"internal":%s}]`, port, "8080"),
	)
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return session
}
