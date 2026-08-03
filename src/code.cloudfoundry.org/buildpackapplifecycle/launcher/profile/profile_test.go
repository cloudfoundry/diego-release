package profile_test

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"code.cloudfoundry.org/buildpackapplifecycle/launcher/profile"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Profile", func() {
	Context("ProfileEnv", func() {
		var (
			rootDir string
			appDir  string
			tmpDir  string
		)

		BeforeEach(func() {
			if runtime.GOOS != "windows" {
				Skip("only run on Windows")
			}
			var err error
			rootDir, err = os.MkdirTemp("", "root")
			Expect(err).NotTo(HaveOccurred())
			appDir = filepath.Join(rootDir, "app")
			Expect(os.MkdirAll(appDir, 0755)).To(Succeed())
			tmpDir, err = os.MkdirTemp("", "launcher-tmp")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			os.RemoveAll(rootDir)
			os.RemoveAll(tmpDir)
		})

		Context("there is a .profile.bat script", func() {
			It("has the env variable", func() {
				writeToFile("set FOO=bar\n", filepath.Join(appDir, ".profile.bat"))

				envs, err := profile.ProfileEnv(appDir, tmpDir, getenv, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Expect(envs).To(ContainElement("FOO=bar"))
			})

			It("has the env variable with spaces", func() {
				writeToFile("set FOO=b ar\n", filepath.Join(appDir, ".profile.bat"))
				envs, err := profile.ProfileEnv(appDir, tmpDir, getenv, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Expect(envs).To(ContainElement("FOO=b ar"))
			})

			It("has the env variable with json", func() {
				writeToFile(`set FOO={ "a": "b", "c": "d"}`+"\n", filepath.Join(appDir, ".profile.bat"))
				envs, err := profile.ProfileEnv(appDir, tmpDir, getenv, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Expect(envs).To(ContainElement(`FOO={ "a": "b", "c": "d"}`))
			})

			It("the process has an env variable with newlines", func() {
				key := randomKey()
				value := `bar
baz=`
				os.Setenv(key, value)
				defer os.Unsetenv(key)
				envs, err := profile.ProfileEnv(appDir, tmpDir, getenv, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Expect(envs).To(ContainElement(fmt.Sprintf("%s=bar\nbaz=", key)))
			})

			It("sets multiple env variable", func() {
				writeToFile("set FOO=bar\nset BAR=foo", filepath.Join(appDir, ".profile.bat"))
				envs, err := profile.ProfileEnv(appDir, tmpDir, getenv, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				Expect(envs).To(ContainElement("FOO=bar"))
				Expect(envs).To(ContainElement("BAR=foo"))
			})

			It("only returns strings of the form var=val", func() {
				writeToFile("echo hi from a batch file\nset BAR=foo", filepath.Join(appDir, ".profile.bat"))
				envs, err := profile.ProfileEnv(appDir, tmpDir, getenv, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				for _, v := range envs {
					subs := strings.SplitN(v, "=", 2)
					Expect(len(subs)).To(Equal(2), fmt.Sprintf("%+v\n", v))
					Expect(subs[0]).NotTo(Equal(""))
				}
			})

			It("captures stdout and stderr", func() {
				writeToFile("echo this is stdout\n echo this is stderr 1>&2", filepath.Join(appDir, ".profile.bat"))
				stdOut := new(bytes.Buffer)
				stdErr := new(bytes.Buffer)
				_, err := profile.ProfileEnv(appDir, tmpDir, getenv, stdOut, stdErr)
				Expect(err).NotTo(HaveOccurred())
				Expect(strings.TrimSpace(stdOut.String())).To(Equal("this is stdout"))
				Expect(strings.TrimSpace(stdErr.String())).To(Equal("this is stderr"))
			})

			It("errors if the .profile.bat errors", func() {
				writeToFile("exit 333\n", filepath.Join(appDir, ".profile.bat"))
				_, err := profile.ProfileEnv(appDir, tmpDir, getenv, GinkgoWriter, GinkgoWriter)
				Expect(err.Error()).To(Equal("running profile scripts failed: exit status 333"))
			})
		})

		Context("there is also a .profile.d directory with multiple scripts", func() {
			BeforeEach(func() {
				Expect(os.MkdirAll(filepath.Join(appDir, ".profile.d"), 0755)).To(Succeed())
				writeToFile("set FOO=%FOO%;bar2\n", filepath.Join(appDir, ".profile.d", "bp2.bat"))
				writeToFile("set FOO=bar1\n", filepath.Join(appDir, ".profile.d", "bp1.bat"))
				writeToFile("set FOO=%FOO%;bar3\n", filepath.Join(appDir, ".profile.bat"))
			})

			It("they all run in order", func() {
				envs, err := profile.ProfileEnv(appDir, tmpDir, getenv, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Expect(envs).To(ContainElement("FOO=bar1;bar2;bar3"))
			})

			It("errors if the a .profile.d script errors", func() {
				writeToFile("exit 333\n", filepath.Join(appDir, ".profile.d", "error.bat"))
				_, err := profile.ProfileEnv(appDir, tmpDir, getenv, GinkgoWriter, GinkgoWriter)
				Expect(err.Error()).To(Equal("running profile scripts failed: exit status 333"))
			})
		})

		Context("there is also a .profile.d directory with multiple scripts", func() {
			BeforeEach(func() {
				Expect(os.MkdirAll(filepath.Join(rootDir, "profile.d"), 0755)).To(Succeed())
				Expect(os.MkdirAll(filepath.Join(appDir, ".profile.d"), 0755)).To(Succeed())

				writeToFile("set FOO=bar1\n", filepath.Join(rootDir, "profile.d", "bp2.bat"))
				writeToFile("set FOO=%FOO%;bar2\n", filepath.Join(appDir, ".profile.d", "bp1.bat"))
				writeToFile("set FOO=%FOO%;bar3\n", filepath.Join(appDir, ".profile.bat"))
			})

			It("they all run in order", func() {
				envs, err := profile.ProfileEnv(appDir, tmpDir, getenv, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Expect(envs).To(ContainElement("FOO=bar1;bar2;bar3"))
			})

			It("errors if the a .profile.d script errors", func() {
				writeToFile("exit 333\n", filepath.Join(rootDir, "profile.d", "error.bat"))
				_, err := profile.ProfileEnv(appDir, tmpDir, getenv, GinkgoWriter, GinkgoWriter)
				Expect(err.Error()).To(Equal("running profile scripts failed: exit status 333"))
			})
		})

		Context("the environment variables overwrite eachother", func() {
			BeforeEach(func() {
				Expect(os.MkdirAll(filepath.Join(rootDir, "profile.d"), 0755)).To(Succeed())
				Expect(os.MkdirAll(filepath.Join(appDir, ".profile.d"), 0755)).To(Succeed())

				writeToFile("set FOO=bar1\n", filepath.Join(rootDir, "profile.d", "bp2.bat"))
				writeToFile("set FOO=bar2\n", filepath.Join(appDir, ".profile.d", "bp1.bat"))
				writeToFile("set FOO=bar3\n", filepath.Join(appDir, ".profile.bat"))
			})

			It("the .profile.bat script wins", func() {
				envs, err := profile.ProfileEnv(appDir, tmpDir, getenv, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Expect(envs).To(ContainElement("FOO=bar3"))
			})
		})

		Context("temp dir does not exist", func() {
			It("errors", func() {
				_, err := profile.ProfileEnv(appDir, filepath.Join(tmpDir, "not-exist"), getenv, GinkgoWriter, GinkgoWriter)
				Expect(err.Error()).To(ContainSubstring("invalid temp dir"))
				Expect(err.Error()).To(ContainSubstring("The system cannot find the file specified"))
			})
		})

		Context("temp dir is not a directory", func() {
			BeforeEach(func() {
				Expect(os.WriteFile(filepath.Join(tmpDir, "some-file"), []byte("xxx"), 0644)).To(Succeed())
			})

			It("errors", func() {
				_, err := profile.ProfileEnv(appDir, filepath.Join(tmpDir, "some-file"), getenv, GinkgoWriter, GinkgoWriter)
				Expect(err.Error()).To(Equal("temp dir must be a directory"))
			})
		})
	})
})

func writeToFile(content, file string) {
	err := os.WriteFile(file, []byte(content), 0755)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func randomKey() string {
	max := big.NewInt(math.MaxInt64)
	r, err := rand.Int(rand.Reader, max)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	return fmt.Sprintf("%d", r.Int64())
}
