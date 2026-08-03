package main_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"

	"testing"
)

var (
	drainPath    string
	fixturesPath string
	apiServer    *ghttp.Server
)

var _ = SynchronizedBeforeSuite(func() []byte {
	binPath, err := gexec.Build("code.cloudfoundry.org/rep/cmd/gocurl")
	Expect(err).NotTo(HaveOccurred())

	bytes, err := json.Marshal([]string{binPath})
	Expect(err).NotTo(HaveOccurred())

	return []byte(bytes)
}, func(data []byte) {
	paths := []string{}
	err := json.Unmarshal(data, &paths)
	Expect(err).NotTo(HaveOccurred())
	drainPath = paths[0]
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	apiServer = ghttp.NewUnstartedServer()
	fixturesPath = "fixtures"
})

var _ = AfterEach(func() {
})

func TestMain(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Drain Suite")
}
