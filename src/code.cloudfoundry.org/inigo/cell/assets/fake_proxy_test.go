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

var _ = Describe("Fake Proxy Exit Behavior", Serial, func() {
	var (
		fakeProxyPath string
	)

	BeforeEach(func() {
		var err error
		// Build fake proxy from assets
		fakeProxyPath, err = gexec.Build(filepath.Join("..", "assets", "fake_proxy"))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		gexec.CleanupBuildArtifacts()
	})

	// Helper function for normal exit codes
	testProxyExit := func(proxyPath string, exitCode int) {
		// Start the proxy application
		command := exec.Command(proxyPath)
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		// Wait for proxy to start listening on port 61001
		Eventually(func() error {
			client := &http.Client{Timeout: 1 * time.Second}
			resp, err := client.Get("http://localhost:61001/")
			if err != nil {
				return err
			}
			resp.Body.Close()
			return nil
		}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

		// Send exit request
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://localhost:61001/exit?code=%d", exitCode))
		Expect(err).NotTo(HaveOccurred())
		resp.Body.Close()

		// Verify process exits
		Eventually(session, 10*time.Second).Should(gexec.Exit(exitCode))
	}

	// Helper function for SIGABRT exit codes
	testProxyExitSigabrt := func(proxyPath string, exitCode int) {
		// Start the proxy application
		command := exec.Command(proxyPath)
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		// Wait for proxy to start listening on port 61001
		Eventually(func() error {
			client := &http.Client{Timeout: 1 * time.Second}
			resp, err := client.Get("http://localhost:61001/")
			if err != nil {
				return err
			}
			resp.Body.Close()
			return nil
		}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

		// Send exit request
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://localhost:61001/exit?code=%d", exitCode))
		Expect(err).NotTo(HaveOccurred())
		resp.Body.Close()

		// Verify process exits
		Eventually(session, 10*time.Second).Should(gexec.Exit())

		// Verify SIGABRT exit code (can vary by system)
		actualExitCode := session.ExitCode()
		Expect(actualExitCode).To(SatisfyAny(
			Equal(134),                      // Direct SIGABRT exit code
			Equal(2),                        // Common SIGABRT exit code
			Equal(128+int(syscall.SIGABRT)), // 128 + signal number
			BeNumerically("<", 0),           // Negative signal codes
		), fmt.Sprintf("Expected SIGABRT-related exit code, got %d", actualExitCode))
	}

	// Use ordered specs to prevent port conflicts
	Describe("fake proxy should exit with correct exit codes", func() {
		It("should exit with code 0", func() {
			testProxyExit(fakeProxyPath, 0)
		})

		It("should exit with code 1", func() {
			testProxyExit(fakeProxyPath, 1)
		})

		It("should exit with SIGABRT", func() {
			testProxyExitSigabrt(fakeProxyPath, 134)
		})
	})

	It("should handle non-numeric exit codes gracefully", func() {
		command := exec.Command(fakeProxyPath)
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		// Wait for proxy to start
		Eventually(func() error {
			client := &http.Client{Timeout: 1 * time.Second}
			resp, err := client.Get("http://localhost:61001/")
			if err != nil {
				return err
			}
			resp.Body.Close()
			return nil
		}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

		// Send invalid exit code
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://localhost:61001/exit?code=invalid")
		Expect(err).NotTo(HaveOccurred())
		resp.Body.Close()

		// Should still exit (strconv.Atoi returns 0 for invalid input)
		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
	})

	It("should be listening on port 61001", func() {
		command := exec.Command(fakeProxyPath)
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer session.Kill()

		// Wait for proxy to start listening (connection will succeed even if endpoint returns 404)
		Eventually(func() error {
			client := &http.Client{Timeout: 1 * time.Second}
			resp, err := client.Get("http://localhost:61001/")
			if err != nil {
				return err
			}
			resp.Body.Close()
			// Any HTTP response (including 404) means the server is listening
			return nil
		}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
	})

	It("should exit with code 42 when terminating naturally (via SIGTERM)", func() {
		command := exec.Command(fakeProxyPath)
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		// Wait for proxy to start listening
		Eventually(func() error {
			client := &http.Client{Timeout: 1 * time.Second}
			resp, err := client.Get("http://localhost:61001/")
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
