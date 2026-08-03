package main_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Getenv", func() {
	var (
		outputDir string
		fileEnv   string
		cmd       *exec.Cmd
	)

	BeforeEach(func() {
		var err error
		outputDir, err = os.MkdirTemp("", "getenv")
		Expect(err).ToNot(HaveOccurred())
		fileEnv = filepath.Join(outputDir, "file.env")

		cmd = exec.Command(getenv, "-output", fileEnv)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(outputDir)).To(Succeed())
	})

	Context("when there is a valid environmental variable", func() {
		BeforeEach(func() {
			cmd.Env = append(os.Environ(), "FOO=bar")
		})

		It("writes the current environment variables to a file", func() {
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			Expect(fileEnv).To(BeAnExistingFile())

			envs, err := os.ReadFile(fileEnv)
			Expect(err).NotTo(HaveOccurred())

			cleanedVars := []string{}
			err = json.Unmarshal(envs, &cleanedVars)
			Expect(err).NotTo(HaveOccurred())

			Expect(cleanedVars).To(ContainElement("FOO=bar"))
		})
	})

	Context("when there is an environmental variable of length 0", func() {
		BeforeEach(func() {
			cmd.Env = append(os.Environ(), `=C:=C:\Users\vagrant`)
		})

		It("does not write out that variable", func() {
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))

			Expect(fileEnv).To(BeAnExistingFile())

			envs, err := os.ReadFile(fileEnv)
			Expect(err).NotTo(HaveOccurred())

			cleanedVars := []string{}
			err = json.Unmarshal(envs, &cleanedVars)
			Expect(err).NotTo(HaveOccurred())

			Expect(cleanedVars).NotTo(ContainElement(`=C:=C:\Users\vagrant`))
		})
	})

	Context("when no output flag is passed", func() {
		BeforeEach(func() {
			cmd.Args = []string{}
		})

		It("fails with a helpful error message", func() {
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(string(session.Err.Contents())).To(ContainSubstring("output file must not be empty"))
		})
	})

	Context("when output file is invalid", func() {
		BeforeEach(func() {
			cmd.Args = []string{getenv, "-output", outputDir}
		})

		It("fails with a helpful error message", func() {
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(session).Should(gexec.Exit(1))
			Expect(string(session.Err.Contents())).To(ContainSubstring(fmt.Sprintf("cannot write to output file: '%s'", outputDir)))
		})
	})
})
