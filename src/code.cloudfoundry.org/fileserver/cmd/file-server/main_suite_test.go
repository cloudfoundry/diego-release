package main_test

import (
	"path"

	"code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	fileServerBinary  string
	testIngressServer *testhelpers.TestIngressServer

	testMetricsChan                                         chan *loggregator_v2.Envelope
	signalMetricsChan                                       chan struct{}
	metronCAFile, metronServerCertFile, metronServerKeyFile string
	fixturesPath                                            = "fixtures"
)

func TestFileServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "File Server Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	fileServerPath, err := gexec.Build("code.cloudfoundry.org/fileserver/cmd/file-server")
	Expect(err).NotTo(HaveOccurred())
	return []byte(fileServerPath)
}, func(fileServerPath []byte) {
	fileServerBinary = string(fileServerPath)

})

var _ = BeforeEach(func() {

	var err error
	metronCAFile = path.Join(fixturesPath, "metron", "CA.crt")
	metronServerCertFile = path.Join(fixturesPath, "metron", "metron.crt")
	metronServerKeyFile = path.Join(fixturesPath, "metron", "metron.key")
	testIngressServer, err = testhelpers.NewTestIngressServer(metronServerCertFile, metronServerKeyFile, metronCAFile)
	Expect(err).NotTo(HaveOccurred())
	receiversChan := testIngressServer.Receivers()
	testIngressServer.Start()

	testMetricsChan, signalMetricsChan = testhelpers.TestMetricChan(receiversChan)

})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = AfterEach(func() {
	testIngressServer.Stop()
	close(signalMetricsChan)

})
