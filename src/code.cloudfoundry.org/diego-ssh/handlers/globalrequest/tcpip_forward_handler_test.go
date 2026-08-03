package globalrequest_test

import (
	"bufio"
	"fmt"
	"io"
	"net"

	"golang.org/x/crypto/ssh"

	"code.cloudfoundry.org/diego-ssh/daemon"
	"code.cloudfoundry.org/diego-ssh/handlers"
	"code.cloudfoundry.org/diego-ssh/handlers/globalrequest"
	"code.cloudfoundry.org/diego-ssh/test_helpers"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/localip"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TCPIPForward Handler", func() {
	var (
		remoteAddress string
		sshClient     *ssh.Client
		logger        *lagertest.TestLogger
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("tcpip-forward-test")

		remotePort, err := localip.LocalPort()
		Expect(err).NotTo(HaveOccurred())
		remoteAddress = fmt.Sprintf("127.0.0.1:%d", remotePort)

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

	testTCPIPForwardAndReturnConn := func(remoteAddr string) net.Conn {
		remoteConn, err := net.Dial("tcp", remoteAddr)
		Expect(err).NotTo(HaveOccurred())

		expectedMsg := "hello\n"
		_, err = fmt.Fprint(remoteConn, expectedMsg)
		Expect(err).NotTo(HaveOccurred())
		r := bufio.NewReader(remoteConn)
		l, err := r.ReadString('\n')
		Expect(err).ToNot(HaveOccurred())
		Expect(l).To(Equal(expectedMsg))
		return remoteConn
	}

	testTCPIPForward := func(remoteAddr string) {
		conn := testTCPIPForwardAndReturnConn(remoteAddr)
		Expect(conn.Close()).To(Succeed())
	}

	It("listens for multiple connections on the interface/port specified", func() {
		listener, err := sshClient.Listen("tcp", remoteAddress)
		Expect(err).NotTo(HaveOccurred())

		defer listener.Close()
		go ServeListener(listener, logger.Session("local"))

		conn1 := testTCPIPForwardAndReturnConn(remoteAddress)
		conn2 := testTCPIPForwardAndReturnConn(remoteAddress)

		Expect(conn1.Close()).To(Succeed())
		Expect(conn2.Close()).To(Succeed())
	})

	It("allows the requester to ask for connections to be forwarded from an unused port", func() {
		listener, err := sshClient.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		defer listener.Close()
		go ServeListener(listener, logger.Session("local"))

		testTCPIPForward(listener.Addr().String())
	})

	It("allows the requester to ask for connections to be forwarded from all interfaces", func() {
		listener, err := sshClient.Listen("tcp", "0.0.0.0:0")
		Expect(err).NotTo(HaveOccurred())
		defer listener.Close()
		go ServeListener(listener, logger.Session("local"))

		testTCPIPForward(listener.Addr().String())
	})

	It("can listen again after cancelling the request", func() {
		listener, err := sshClient.Listen("tcp", remoteAddress)
		Expect(err).NotTo(HaveOccurred())
		Expect(listener.Close()).To(Succeed())

		listener, err = sshClient.Listen("tcp", remoteAddress)
		Expect(err).NotTo(HaveOccurred())

		defer listener.Close()
		go ServeListener(listener, logger.Session("local"))

		testTCPIPForward(remoteAddress)
	})

	Context("when listener cannot be created", func() {
		var (
			ln net.Listener
		)

		BeforeEach(func() {
			var err error
			ln, err = net.Listen("tcp", ":0")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(ln.Close()).To(Succeed())
		})

		It("reject the request", func() {
			_, err := sshClient.Listen("tcp", ln.Addr().String())
			Expect(err).To(HaveOccurred())
		})
	})
})

func ServeListener(ln net.Listener, logger lager.Logger) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			logger.Error("listener-failed-to-accept", err)
			return
		}

		go func() {
			defer conn.Close()
			defer GinkgoRecover()

			for {
				r := bufio.NewReader(conn)
				l, err := r.ReadString('\n')
				if err == io.EOF {
					return
				}
				n, err := conn.Write([]byte(l))
				if err != nil {
					logger.Error("server-sent-message-error", err)
					return
				}
				logger.Info("server-sent-message-success", lager.Data{"bytes-sent": n})
			}
		}()
	}
}
