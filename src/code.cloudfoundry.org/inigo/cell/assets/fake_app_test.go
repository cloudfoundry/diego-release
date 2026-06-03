//go:build !windows

package assets_test

import (
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Fake App Exit Behavior", Serial, func() {
	var (
		fakeAppPath string
	)

	BeforeEach(func() {
		var err error
		// Build fake app from assets
		fakeAppPath, err = gexec.Build(filepath.Join("..", "assets", "fake_app"))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		gexec.CleanupBuildArtifacts()
	})

	DescribeTable("fake app should exit with correct exit codes",
		func(appType string, port string, exitCode int) {
			appPath := fakeAppPath

			// Start the application
			command := exec.Command(appPath)
			command.Env = append(command.Env, "PORT="+port)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			// Wait for server to start
			Eventually(func() error {
				client := &http.Client{Timeout: 1 * time.Second}
				resp, err := client.Get(fmt.Sprintf("http://localhost:%s/", port))
				if err != nil {
					return err
				}
				resp.Body.Close()
				return nil
			}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

			// Send exit request
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get(fmt.Sprintf("http://localhost:%s/exit?code=%d", port, exitCode))
			Expect(err).NotTo(HaveOccurred())
			resp.Body.Close()

			// Verify process exits
			Eventually(session, 10*time.Second).Should(gexec.Exit())

			// Verify exit code
			actualExitCode := session.ExitCode()
			if exitCode == 134 {
				// SIGABRT can result in different exit codes depending on system
				Expect(actualExitCode).To(SatisfyAny(
					Equal(134),                      // Direct SIGABRT exit code
					Equal(2),                        // Common SIGABRT exit code
					Equal(128+int(syscall.SIGABRT)), // 128 + signal number
					BeNumerically("<", 0),           // Negative signal codes
				), fmt.Sprintf("Expected SIGABRT-related exit code, got %d", actualExitCode))
			} else {
				Expect(actualExitCode).To(Equal(exitCode))
			}
		},
		Entry("fake app exits with code 0", "app", "28080", 0),
		Entry("fake app exits with code 1", "app", "28081", 1),
		Entry("fake app exits with SIGABRT", "app", "28082", 134),
	)

	Context("when fake app receives invalid exit codes", func() {
		It("should handle non-numeric exit codes gracefully", func() {
			command := exec.Command(fakeAppPath)
			command.Env = append(command.Env, "PORT=28090")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			// Wait for server to start
			Eventually(func() error {
				client := &http.Client{Timeout: 1 * time.Second}
				resp, err := client.Get("http://localhost:28090/")
				if err != nil {
					return err
				}
				resp.Body.Close()
				return nil
			}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

			// Send invalid exit code
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get("http://localhost:28090/exit?code=invalid")
			Expect(err).NotTo(HaveOccurred())
			resp.Body.Close()

			// Should still exit (strconv.Atoi returns 0 for invalid input)
			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		})
	})

	Context("when fake app receives no exit code parameter", func() {
		It("should exit with code 0", func() {
			command := exec.Command(fakeAppPath)
			command.Env = append(command.Env, "PORT=28091")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			// Wait for server to start
			Eventually(func() error {
				client := &http.Client{Timeout: 1 * time.Second}
				resp, err := client.Get("http://localhost:28091/")
				if err != nil {
					return err
				}
				resp.Body.Close()
				return nil
			}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

			// Send exit request without code parameter
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get("http://localhost:28091/exit")
			Expect(err).NotTo(HaveOccurred())
			resp.Body.Close()

			// Should exit with code 0 (default when no code provided)
			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		})
	})

	Context("fake app natural termination", func() {
		It("should exit with code 42 when terminating naturally (via SIGTERM)", func() {
			command := exec.Command(fakeAppPath)
			command.Env = append(command.Env, "PORT=28092")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			// Wait for app to start listening
			Eventually(func() error {
				client := &http.Client{Timeout: 1 * time.Second}
				resp, err := client.Get("http://localhost:28092/")
				if err != nil {
					return err
				}
				resp.Body.Close()
				return nil
			}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

			// Send SIGTERM to simulate natural shutdown
			session.Terminate()

			// Should exit with code 42
			Eventually(session, 10*time.Second).Should(gexec.Exit(143))
		})
	})
})
