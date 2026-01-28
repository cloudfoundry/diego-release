package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/bbs/test_helpers/sqlrunner"
	"code.cloudfoundry.org/inigo/helpers/portauthority"
	"code.cloudfoundry.org/locket/cmd/locket/config"
	"code.cloudfoundry.org/locket/cmd/locket/testrunner"
	"code.cloudfoundry.org/tlsconfig"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"google.golang.org/grpc/grpclog"
)

var (
	cfdotPath, locketPath string
	locketRunner          *ginkgomon.Runner
	locketProcess         ifrit.Process
	dbRunner              sqlrunner.SQLRunner
	dbProcess             ifrit.Process
	locketAPILocation     string
	locketCACertFile      string
	locketClientCertFile  string
	locketClientKeyFile   string
	locketServerCertFile  string
	locketServerKeyFile   string
)

var bbsServer *ghttp.Server

var _ = SynchronizedBeforeSuite(func() []byte {
	binPath, err := gexec.Build("code.cloudfoundry.org/cfdot")
	Expect(err).NotTo(HaveOccurred())

	locketBinPath, err := gexec.Build("code.cloudfoundry.org/locket/cmd/locket")
	Expect(err).NotTo(HaveOccurred())

	bytes, err := json.Marshal([]string{binPath, locketBinPath})
	Expect(err).NotTo(HaveOccurred())

	return []byte(bytes)
}, func(data []byte) {
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, io.Discard, io.Discard))
	paths := []string{}
	err := json.Unmarshal(data, &paths)
	Expect(err).NotTo(HaveOccurred())
	cfdotPath = paths[0]
	locketPath = paths[1]
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	bbsServer = ghttp.NewUnstartedServer()
	defer bbsServer.HTTPTestServer.StartTLS()

	node := GinkgoParallelProcess()
	startPort := 1050 * node
	portRange := 1000
	endPort := startPort + portRange
	portAllocator, err := portauthority.New(startPort, endPort)
	Expect(err).NotTo(HaveOccurred())

	port, err := portAllocator.ClaimPorts(1)
	Expect(err).NotTo(HaveOccurred())

	locketAPILocation = fmt.Sprintf("localhost:%d", port)
	wd, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())
	locketCACertFile = fmt.Sprintf("%s/fixtures/locketCA.crt", wd)
	locketClientCertFile = fmt.Sprintf("%s/fixtures/locketClient.crt", wd)
	locketClientKeyFile = fmt.Sprintf("%s/fixtures/locketClient.key", wd)
	locketServerCertFile = fmt.Sprintf("%s/fixtures/locketServer.crt", wd)
	locketServerKeyFile = fmt.Sprintf("%s/fixtures/locketServer.key", wd)

	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(locketClientCertFile, locketClientKeyFile),
	).Client(tlsconfig.WithAuthorityFromFile(locketCACertFile))
	Expect(err).NotTo(HaveOccurred())
	bbsServer.HTTPTestServer.TLS = tlsConfig

	dbName := fmt.Sprintf("diego_%d", GinkgoParallelProcess())
	dbRunner = test_helpers.NewSQLRunner(dbName)
	dbProcess = ginkgomon.Invoke(dbRunner)

	locketRunner = testrunner.NewLocketRunner(locketPath, func(cfg *config.LocketConfig) {
		cfg.CaFile = locketCACertFile
		cfg.CertFile = locketServerCertFile
		cfg.KeyFile = locketServerKeyFile
		cfg.ListenAddress = locketAPILocation
		cfg.DatabaseDriver = dbRunner.DriverName()
		cfg.DatabaseConnectionString = dbRunner.ConnectionString()
	})
	locketProcess = ginkgomon.Invoke(locketRunner)
})

var _ = AfterEach(func() {
	bbsServer.CloseClientConnections()
	bbsServer.Close()
	ginkgomon.Interrupt(locketProcess, 5*time.Second)
	ginkgomon.Interrupt(dbProcess, 5*time.Second) // we've been seeing the sql teardown take longer than the default of 1s
})

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

// Pass arguments that would be passed to cfdot
// i.e. set-domain domain1
func itValidatesBBSFlags(args ...string) {
	Context("BBS Flag Validation", func() {
		It("exits with status 3 when no bbs flags are specified", func() {
			cmd := exec.Command(cfdotPath, args...)

			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess.Exited).Should(BeClosed())

			Expect(sess.ExitCode()).To(Equal(3))
		})
	})
}

// Pass arguments that would be passed to cfdot
func itValidatesLocketFlags(args ...string) {
	Context("Locket Flag Validation", func() {
		It("exits with status 3 when no locket flags are specified", func() {
			cmd := exec.Command(cfdotPath, args...)

			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess.Exited).Should(BeClosed())

			Expect(sess.ExitCode()).To(Equal(3))
		})
	})
}

// Pass arguments that would be passed to cfdot
func itValidatesTLSFlags(args ...string) {
	Context("TLS Flag Validation", func() {
		It("exits with status 3 when no TLS flags are specified", func() {
			cmd := exec.Command(cfdotPath, args...)

			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess.Exited).Should(BeClosed())

			Expect(sess.ExitCode()).To(Equal(3))
		})
	})
}

func itHasNoArgs(command string, locketFlags bool) {
	var (
		sess *gexec.Session
	)
	Context("when any arguments are passed", func() {
		BeforeEach(func() {
			urlFlag := "--bbsURL"
			url := bbsServer.URL()
			if locketFlags {
				urlFlag = "--locketAPILocation"
				url = locketAPILocation
			}
			cfdotCmd := exec.Command(cfdotPath, urlFlag, url, command, "extra-arg")

			var err error
			sess, err = gexec.Start(cfdotCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess.Exited).Should(BeClosed())
		})

		It("exits with status code of 3", func() {
			Expect(sess.ExitCode()).To(Equal(3))
		})

		It("prints the usage to stderr", func() {
			Expect(sess.Err).To(gbytes.Say(fmt.Sprintf("cfdot %s \\[flags\\]", command)))
		})
	})
}

func RunCFDot(args ...string) *gexec.Session {
	cmdArgs := []string{
		"--bbsURL", bbsServer.URL(),
		"--caCertFile", locketCACertFile,
		"--clientCertFile", locketClientCertFile,
		"--clientKeyFile", locketClientKeyFile,
	}
	cmdArgs = append(cmdArgs, args...)
	sess, err := gexec.Start(exec.Command(cfdotPath, cmdArgs...), GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return sess
}
