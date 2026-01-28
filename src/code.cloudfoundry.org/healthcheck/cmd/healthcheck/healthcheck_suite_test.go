package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var healthCheck string

func TestHealthCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HealthCheck CLI Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	healthCheckPath := buildHealthCheck()
	return []byte(healthCheckPath)
}, func(healthCheckPath []byte) {
	healthCheck = string(healthCheckPath)
})

var _ = SynchronizedAfterSuite(func() {
	//noop
}, func() {
	gexec.CleanupBuildArtifacts()
})
