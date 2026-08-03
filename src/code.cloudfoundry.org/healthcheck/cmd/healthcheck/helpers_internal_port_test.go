//go:build !external
// +build !external

package main_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func buildHealthCheck() string {
	healthCheckPath, err := gexec.Build("code.cloudfoundry.org/healthcheck/cmd/healthcheck")
	Expect(err).NotTo(HaveOccurred())
	return healthCheckPath
}

func createHTTPHealthCheck(args []string, port string) *gexec.Session {
	args = append([]string{"-uri", "/api/_ping", "-port", port, "-timeout", "100ms"}, args...)
	session, err := gexec.Start(exec.Command(healthCheck, args...), GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return session
}

func createPortHealthCheck(args []string, port string) *gexec.Session {
	args = append([]string{"-port", port, "-timeout", "100ms"}, args...)
	session, err := gexec.Start(exec.Command(healthCheck, args...), GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return session
}
