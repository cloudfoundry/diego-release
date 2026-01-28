package globalrequest_test

import (
	"net"
	"strconv"

	"code.cloudfoundry.org/diego-ssh/daemon"
	"code.cloudfoundry.org/diego-ssh/handlers"
	"code.cloudfoundry.org/diego-ssh/handlers/globalrequest"
	"code.cloudfoundry.org/diego-ssh/handlers/globalrequest/internal"
	"code.cloudfoundry.org/diego-ssh/test_helpers"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"golang.org/x/crypto/ssh"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CancelTcpipForwardHandler", func() {
	var (
		sshClient *ssh.Client
		logger    *lagertest.TestLogger
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("tcpip-forward-test")

		globalRequestHandlers := map[string]handlers.GlobalRequestHandler{
			globalrequest.TCPIPForward:       new(globalrequest.TCPIPForwardHandler),
			globalrequest.CancelTCPIPForward: new(globalrequest.CancelTCPIPForwardHandler),
		}

		serverSSHConfig := &ssh.ServerConfig{
			NoClientAuth: true,
		}
		serverSSHConfig.AddHostKey(TestHostKey)

		sshd := daemon.New(logger, serverSSHConfig, globalRequestHandlers, nil)

		serverNetConn, clientNetConn := test_helpers.Pipe()
		go sshd.HandleConnection(serverNetConn)
		sshClient = test_helpers.NewClient(clientNetConn, nil)
	})

	Context("when the request is invalid", func() {
		It("rejects the request", func() {
			payload := ssh.Marshal(struct {
				port uint16
			}{
				port: 10,
			})
			ok, _, err := sshClient.SendRequest("cancel-tcpip-forward", true, payload)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).NotTo(BeTrue())
		})
	})

	Context("when the listener isn't found", func() {
		It("rejects the request", func() {
			payload := ssh.Marshal(internal.TCPIPForwardRequest{
				Address: "127.0.0.1",
				Port:    9090,
			})
			ok, _, err := sshClient.SendRequest("cancel-tcpip-forward", true, payload)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).NotTo(BeTrue())
		})
	})

	Context("when the listener exists", func() {
		var (
			ok      bool
			err     error
			address string
		)

		BeforeEach(func() {
			addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
			Expect(err).NotTo(HaveOccurred())
			ln, err := sshClient.ListenTCP(addr)
			Expect(err).NotTo(HaveOccurred())

			_, portStr, err := net.SplitHostPort(ln.Addr().String())
			Expect(err).NotTo(HaveOccurred())

			address = "127.0.0.1:" + portStr
			port, err := strconv.Atoi(portStr)
			Expect(err).NotTo(HaveOccurred())

			_, err = net.Dial("tcp", address)
			Expect(err).NotTo(HaveOccurred())

			payload := ssh.Marshal(internal.TCPIPForwardRequest{
				Address: "127.0.0.1",
				Port:    uint32(port),
			})
			ok, _, err = sshClient.SendRequest("cancel-tcpip-forward", true, payload)
			Expect(err).ToNot(HaveOccurred())
		})

		It("successfully process the request", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		It("stops listening to the port", func() {
			// the reason for the eventually instead of Expect is that a Close
			// doesn't guarantee that the linux socket is actually closed. See
			// https://github.com/golang/go/issues/10527 and build failures in
			// https://diego.ci.cf-app.com/teams/main/pipelines/main/jobs/units-common/builds/1207
			Eventually(func() error {
				_, err := net.Dial("tcp", address)
				return err
			}).Should(MatchError(ContainSubstring("refused")))
		})
	})
})
