//go:build !windows
// +build !windows

package shell_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/buildpackapplifecycle/shell"
	"code.cloudfoundry.org/goshims/osshim/os_fake"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type FakeExec struct {
	ExecCalled     bool
	ExecCalledWith struct {
		Dir     string
		Command string
	}
}

func (e *FakeExec) Exec(dir, launcher, args, command string, environ []string) error {
	e.ExecCalled = true
	e.ExecCalledWith.Dir = dir
	e.ExecCalledWith.Command = command
	return nil
}

var _ = Describe("Shell", func() {
	Describe("[integration]", func() {
		var appDir string
		var session *gexec.Session
		var homeDir string

		BeforeEach(func() {
			shellPath, err := gexec.Build("code.cloudfoundry.org/buildpackapplifecycle/shell/shell", "-race")
			Expect(err).NotTo(HaveOccurred())

			homeDir, err = os.MkdirTemp("", "vcap")
			Expect(err).NotTo(HaveOccurred())
			appDir = filepath.Join(homeDir, "app")
			Expect(os.MkdirAll(appDir, 0755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(homeDir, "profile.d"), 0755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(appDir, ".profile.d"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(homeDir, "profile.d", "stuff"), []byte("echo homedir profile.d sourced\n"), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(appDir, ".profile.d", "stuff"), []byte("echo appdir profile.d sourced\n"), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(appDir, ".profile"), []byte("echo appdir profile sourced\n"), 0644)).To(Succeed())

			shellCmd := &exec.Cmd{
				Path: shellPath,
				Dir:  homeDir,
				Args: []string{
					"shell",
					appDir,
					"env; echo running app",
				},
				Env: append(
					os.Environ(),
					"CALLERENV=some-value",
					"TEST_CREDENTIAL_FILTER_WHITELIST=CALLERENV,DEPS_DIR,VCAP_APPLICATION,VCAP_SERVICES,A,B,C,INSTANCE_GUID,INSTANCE_INDEX,PORT,DATABASE_URL",
					"PORT=8080",
					"INSTANCE_GUID=some-instance-guid",
					"INSTANCE_INDEX=123",
					`VCAP_APPLICATION={"foo":1}`,
				),
			}
			session, err = gexec.Start(shellCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		It("runs the command with desired environment", func() {
			Eventually(session).Should(gexec.Exit(0))

			By("executes with the environment of the caller", func() {
				Expect(string(session.Out.Contents())).To(ContainSubstring("CALLERENV=some-value"))
			})

			By("executes the start command with $HOME as the given dir", func() {
				Expect(string(session.Out.Contents())).To(ContainSubstring("HOME=" + appDir))
			})

			By("sources appDir/../profile.d/*", func() {
				Expect(string(session.Out.Contents())).To(ContainSubstring("homedir profile.d sourced"))
			})

			By("sources appDir/.profile.d/*", func() {
				Expect(string(session.Out.Contents())).To(ContainSubstring("appdir profile.d sourced"))
			})

			By("NOT sourcing appDir/.profile", func() {
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("appdir profile sourced"))
			})
		})
	})

	Describe("[unit]", func() {
		var (
			fakeOs   *os_fake.FakeOs
			fakeExec *FakeExec
			fakeEnv  map[string]string
		)
		BeforeEach(func() {
			fakeOs = &os_fake.FakeOs{}
			fakeExec = &FakeExec{}
			fakeEnv = make(map[string]string)
			fakeOs.GetenvStub = func(key string) string {
				return fakeEnv[key]
			}
		})

		Context("no arguments supplied", func() {
			const homeDir string = "/a/b/f"
			BeforeEach(func() {
				fakeEnv["HOME"] = homeDir
			})
			Context("app is at $HOME/app", func() {
				BeforeEach(func() { fakeOs.StatReturns(nil, nil) })

				It("runs bash in the $HOME/app directory", func() {
					Expect(shell.Run(fakeOs, fakeExec, []string{"./shell"})).To(Succeed())
					Expect(fakeExec.ExecCalled).To(BeTrue())
					Expect(fakeExec.ExecCalledWith.Dir).To(Equal(filepath.Join(homeDir, "app")))
					Expect(fakeExec.ExecCalledWith.Command).To(Equal("bash"))
				})
			})

			Context("$HOME/app is not a directory", func() {
				BeforeEach(func() { fakeOs.StatReturns(nil, fmt.Errorf("not exist")) })

				It("returns an error", func() {
					err := shell.Run(fakeOs, fakeExec, []string{"./shell"})
					Expect(err).To(MatchError("Could not infer app directory, please provide one"))
					Expect(fakeExec.ExecCalled).To(BeFalse())
				})
			})
		})

		Context("directory is supplied, but command is not", func() {
			const appDir string = "/b/d/f/app"

			Context("app directory exists", func() {
				BeforeEach(func() { fakeOs.StatReturns(nil, nil) })

				It("runs bash in the supplied app directory", func() {
					Expect(shell.Run(fakeOs, fakeExec, []string{"./shell", appDir})).To(Succeed())
					Expect(fakeExec.ExecCalled).To(BeTrue())
					Expect(fakeExec.ExecCalledWith.Dir).To(Equal(appDir))
					Expect(fakeExec.ExecCalledWith.Command).To(Equal("bash"))
				})
			})

			Context("app directory does not exist", func() {
				BeforeEach(func() { fakeOs.StatReturns(nil, fmt.Errorf("not exist")) })

				It("returns an error", func() {
					err := shell.Run(fakeOs, fakeExec, []string{"./shell", appDir})
					Expect(err).To(MatchError("Provided app direcory does not exist"))
					Expect(fakeExec.ExecCalled).To(BeFalse())
				})
			})
		})
		Context("directory and command are supplied", func() {
			BeforeEach(func() { fakeOs.StatReturns(nil, nil) })

			Context("app directory exists", func() {
				It("runs the supllied command in the supplied app directory", func() {
					appDir := "/b/d/f/app"
					Expect(shell.Run(fakeOs, fakeExec, []string{"./shell", appDir, "command"})).To(Succeed())
					Expect(fakeExec.ExecCalled).To(BeTrue())
					Expect(fakeExec.ExecCalledWith.Dir).To(Equal(appDir))
					Expect(fakeExec.ExecCalledWith.Command).To(Equal("command"))
				})
			})
		})
	})
})
