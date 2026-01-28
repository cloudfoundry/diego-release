//go:build windows2012R2

package main_test

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/diego-ssh/cmd/sshd/testrunner"

	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
	"golang.org/x/crypto/ssh"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("SSH daemon", func() {
	Describe("SSH features", func() {
		var (
			process      ifrit.Process
			address      string
			clientConfig *ssh.ClientConfig
			client       *ssh.Client
		)

		BeforeEach(func() {
			args := testrunner.Args{
				HostKey:       string(privateKeyPem),
				AuthorizedKey: string(publicAuthorizedKey),

				AllowUnauthenticatedClients: true,
				InheritDaemonEnv:            false,
			}
			address = fmt.Sprintf("127.0.0.1:%d", sshdPort)
			_, process = startSshd(sshdPath, args, "127.0.0.1", sshdPort)
			clientConfig = &ssh.ClientConfig{
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			}
			Expect(process).NotTo(BeNil())

			var dialErr error
			client, dialErr = ssh.Dial("tcp", address, clientConfig)
			Expect(dialErr).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			ginkgomon.Kill(process, 3*time.Second)
			client.Close()
		})

		Context("when a client requests the execution of a command", func() {
			It("runs the command", func() {
				_, err := client.NewSession()
				Expect(err).To(MatchError(ContainSubstring("not supported")))
			})
		})

		Context("when a client requests a local port forward", func() {
			var server *ghttp.Server
			BeforeEach(func() {
				server = ghttp.NewServer()
			})

			It("forwards the local port to the target from the server side", func() {
				_, err := client.Dial("tcp", server.Addr())
				Expect(err).To(MatchError(ContainSubstring("unknown channel type")))
			})

			It("server should not receive any connections", func() {
				Expect(server.ReceivedRequests()).To(BeEmpty())
			})
		})
	})
})
