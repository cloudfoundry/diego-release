package vizzini_test

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	vizziniconfig "code.cloudfoundry.org/vizzini/config"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/onsi/say"
)

var (
	bbsClient     bbs.InternalClient
	domain        string
	otherDomain   string
	guid          string
	startTime     time.Time
	timeout       time.Duration
	dockerTimeout time.Duration
	logger        lager.Logger
	sshHost       string
	sshPort       string

	config vizziniconfig.VizziniConfig
)

func init() {
	var err error
	config, err = vizziniconfig.NewVizziniConfig()
	if err != nil {
		log.Fatal(err)
	}

	if config.BBSAddress == "" {
		log.Fatal("i need a bbs address to talk to Diego...")
	}

	if config.SSHAddress == "" {
		log.Fatal("i need an SSH address to talk to Diego...")
	}
}

func TestVizziniSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Vizzini Suite")
}

func NewGuid() string {
	u, err := uuid.NewV4()
	Expect(err).NotTo(HaveOccurred())
	return domain + "-" + u.String()[:8]
}

const DefaultEventuallyTimeout = 20 * time.Second

var taskFailureTimeout time.Duration

var traceID = "vizzini-trace-id"

var _ = BeforeSuite(func() {
	var err error
	timeout = DefaultEventuallyTimeout
	dockerTimeout = 120 * time.Second

	timeoutArg := os.Getenv("DEFAULT_EVENTUALLY_TIMEOUT")
	if timeoutArg != "" {
		timeout, err = time.ParseDuration(timeoutArg)
		Expect(err).NotTo(HaveOccurred(), "invalid value '"+timeoutArg+"' for DEFAULT_EVENTUALLY_TIMEOUT")
		fmt.Fprintf(GinkgoWriter, "Setting Default Eventually Timeout to %s\n", timeout)
	}

	SetDefaultEventuallyTimeout(timeout)
	SetDefaultEventuallyPollingInterval(500 * time.Millisecond)
	SetDefaultConsistentlyPollingInterval(200 * time.Millisecond)
	domain = fmt.Sprintf("vizzini-%d", GinkgoParallelProcess())
	otherDomain = fmt.Sprintf("vizzini-other-%d", GinkgoParallelProcess())

	rootfsURI, err := url.Parse(config.DefaultRootFS)
	Expect(err).NotTo(HaveOccurred())
	Expect(rootfsURI.Scheme).To(Equal("preloaded"))
	Expect(rootfsURI.Opaque).NotTo(BeEmpty())

	bbsClient = initializeBBSClient()

	sshHost, sshPort, err = net.SplitHostPort(config.SSHAddress)
	Expect(err).NotTo(HaveOccurred())

	// conservative taskFailureTimeout since tasks retries happen during convergence
	taskFailureTimeout = ConvergerInterval * time.Duration(config.MaxTaskRetries+1)

	logger = lagertest.NewTestLogger("vizzini")
})

var _ = BeforeEach(func() {
	startTime = time.Now()
	guid = NewGuid()
})

var _ = AfterEach(func() {
	defer func() {
		endTime := time.Now()
		fmt.Fprint(GinkgoWriter, say.F("{{cyan}}\n%s\nThis test referenced GUID %s\nStart time: %s (%d)\nEnd time: %s (%d)\n{{/}}", CurrentSpecReport().FullText(), guid, startTime, startTime.Unix(), endTime, endTime.Unix()))
	}()

	for _, domain := range []string{domain, otherDomain} {
		ClearOutTasksInDomain(domain)
		ClearOutDesiredLRPsInDomain(domain)
	}
})

var _ = AfterSuite(func() {
	for _, domain := range []string{domain, otherDomain} {
		bbsClient.UpsertDomain(logger, traceID, domain, 5*time.Minute) //leave the domain around forever so that Diego cleans up if need be
	}

	for _, domain := range []string{domain, otherDomain} {
		ClearOutDesiredLRPsInDomain(domain)
		ClearOutTasksInDomain(domain)
	}
})

func initializeBBSClient() bbs.InternalClient {
	bbsClient, err := bbs.NewSecureSkipVerifyClient(config.BBSAddress, config.BBSClientCertPath, config.BBSClientKeyPath, 0, 0)
	Expect(err).NotTo(HaveOccurred())
	return bbsClient
}
