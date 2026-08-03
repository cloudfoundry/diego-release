package integration_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("help", func() {

	var cfdotCmd *exec.Cmd

	itPrintsHelp := func() {
		It("prints help", func() {

			sess, err := gexec.Start(cfdotCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say("A command-line tool to interact with a Cloud Foundry Diego deployment"))
			Expect(sess.Out).To(gbytes.Say("Usage:"))
			Expect(sess.Out).To(gbytes.Say("cfdot"))
			Expect(sess.Out).To(gbytes.Say("Available Commands:"))
			Expect(sess.Out).To(gbytes.Say("delete-desired-lrp"))
			Expect(sess.Out).To(gbytes.Say("domains"))
			Expect(sess.Out).To(gbytes.Say("help"))
			Expect(sess.Out).To(gbytes.Say("set-domain"))
		})
	}

	Context("called with no command", func() {
		BeforeEach(func() {
			cfdotCmd = exec.Command(cfdotPath)
		})
		itPrintsHelp()
	})

	Context("called with -h", func() {
		BeforeEach(func() {
			cfdotCmd = exec.Command(cfdotPath, "-h")
		})
		itPrintsHelp()
	})

	Context("called with --help", func() {
		BeforeEach(func() {
			cfdotCmd = exec.Command(cfdotPath, "--help")
		})
		itPrintsHelp()
	})

	Context("help task", func() {
		It("should print the usage for the help command", func() {
			cfdotCmd = exec.Command(cfdotPath, "help")
			sess, err := gexec.Start(cfdotCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say("Get help on using cfdot commands"))
			Expect(sess.Out).To(gbytes.Say("Usage:"))
			Expect(sess.Out).To(gbytes.Say("cfdot help CMD"))
		})
	})

	Context("called `cfdot help set-domain`", func() {
		BeforeEach(func() {
			cfdotCmd = exec.Command(cfdotPath, "help", "set-domain")
		})

		It("displays the set-domain usage message", func() {
			sess, err := gexec.Start(cfdotCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say("Mark a domain as fresh"))
			Expect(sess.Out).To(gbytes.Say("Usage:"))
			Expect(sess.Out).To(gbytes.Say("cfdot set-domain DOMAIN"))
			Expect(sess.Out).To(gbytes.Say("Flags:"))
			Expect(sess.Out).To(gbytes.Say("--bbsURL"))
			Expect(sess.Out).To(gbytes.Say("--caCertFile"))
			Expect(sess.Out).To(gbytes.Say("--clientCertFile"))
			Expect(sess.Out).To(gbytes.Say("--skipCertVerify"))
			Expect(sess.Out).To(gbytes.Say("--timeout"))
			Expect(sess.Out).To(gbytes.Say("-t"))
			Expect(sess.Out).To(gbytes.Say("--ttl"))
		})
	})

	Context("called `cfdot help locks`", func() {
		BeforeEach(func() {
			cfdotCmd = exec.Command(cfdotPath, "help", "locks")
		})

		It("displays the locks usage message", func() {
			sess, err := gexec.Start(cfdotCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say("List locks from Locket"))
			Expect(sess.Out).To(gbytes.Say("Usage:"))
			Expect(sess.Out).To(gbytes.Say("cfdot locks"))
			Expect(sess.Out).To(gbytes.Say("Flags:"))
			Expect(sess.Out).To(gbytes.Say("--caCertFile"))
			Expect(sess.Out).To(gbytes.Say("--clientCertFile"))
			Expect(sess.Out).To(gbytes.Say("--locketAPILocation"))
			Expect(sess.Out).To(gbytes.Say("--skipCertVerify"))
		})
	})

	Context("called `cfdot help delete-desired-lrp`", func() {
		BeforeEach(func() {
			cfdotCmd = exec.Command(cfdotPath, "help", "delete-desired-lrp")
		})

		It("displays the delete-desired-lrp usage message", func() {
			sess, err := gexec.Start(cfdotCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(sess).Should(gexec.Exit(0))
			Expect(sess.Out).To(gbytes.Say("Delete a desired LRP"))
		})
	})
})
