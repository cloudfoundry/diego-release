package natsserverrunner

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"code.cloudfoundry.org/route-emitter/diegonats"
	"code.cloudfoundry.org/tlsconfig"

	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
)

func StartNatsServer(natsPort int) (ifrit.Process, diegonats.NATSClient) {
	ginkgomonRunner := NewNatsServerTestRunner(natsPort)
	natsServerProcess := ifrit.Invoke(ginkgomonRunner)
	Eventually(natsServerProcess.Ready(), "5s").Should(BeClosed())

	natsClient := diegonats.NewClient()
	_, err := natsClient.Connect([]string{fmt.Sprintf("nats://127.0.0.1:%d", natsPort)})
	Expect(err).ShouldNot(HaveOccurred())

	return natsServerProcess, natsClient
}

func NewNatsServerTestRunner(natsPort int) *ginkgomon.Runner {
	natsServerPath, exists := os.LookupEnv("NATS_SERVER_BINARY")
	if !exists {
		fmt.Println("You need nats-server install set NATS_SERVER_BINARY env variable")
		os.Exit(1)
	}
	return ginkgomon.New(ginkgomon.Config{
		Name:              "nats-server",
		AnsiColorCode:     "99m",
		StartCheck:        "Server is ready",
		StartCheckTimeout: 5 * time.Second,
		Command: exec.Command(
			natsServerPath,
			"-p", strconv.Itoa(natsPort),
		),
	})
}

func StartNatsServerWithTLS(natsPort int, caFile, certFile, keyFile string) (ifrit.Process, diegonats.NATSClient) {
	ginkgomonRunner := NewNatsServerWithTLSTestRunner(natsPort, caFile, certFile, keyFile)
	natsServerProcess := ifrit.Invoke(ginkgomonRunner)
	Eventually(natsServerProcess.Ready(), "5s").Should(BeClosed())

	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(certFile, keyFile),
	).Client(
		tlsconfig.WithAuthorityFromFile(caFile),
	)
	Expect(err).ShouldNot(HaveOccurred())

	natsClient := diegonats.NewClientWithTLSConfig(tlsConfig)
	_, err = natsClient.Connect([]string{fmt.Sprintf("nats://127.0.0.1:%d", natsPort)})
	Expect(err).ShouldNot(HaveOccurred())

	return natsServerProcess, natsClient
}

func NewNatsServerWithTLSTestRunner(natsPort int, caFile, certFile, keyFile string) *ginkgomon.Runner {
	natsServerPath, exists := os.LookupEnv("NATS_SERVER_BINARY")
	if !exists {
		fmt.Println("You need nats-server install set NATS_SERVER_BINARY env variable")
		os.Exit(1)
	}

	return ginkgomon.New(ginkgomon.Config{
		Name:              "nats-server",
		AnsiColorCode:     "99m",
		StartCheck:        "Server is ready",
		StartCheckTimeout: 5 * time.Second,
		Command: exec.Command(
			natsServerPath,
			"-p", strconv.Itoa(natsPort),
			"--tlsverify",
			"--tlscacert", caFile,
			"--tlscert", certFile,
			"--tlskey", keyFile,
		),
	})
}
