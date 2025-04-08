package commands_test

import (
	"os"

	"code.cloudfoundry.org/cfdot/commands"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/spf13/cobra"
)

var _ = Describe("BBS Flags", func() {
	var validFlags map[string]string
	var dummyCmd *cobra.Command
	var err error
	var output *gbytes.Buffer

	BeforeEach(func() {
		dummyCmd = &cobra.Command{
			Use: "dummy",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		commands.AddBBSFlags(dummyCmd)
		output = gbytes.NewBuffer()
		dummyCmd.SetOut(output)

		validFlags = map[string]string{
			"--bbsURL":         "https://example.com",
			"--skipCertVerify": "false",
			"--caCertFile":     "fixtures/bbsCACert.crt",
			"--clientCertFile": "fixtures/bbsClient.crt",
			"--clientKeyFile":  "fixtures/bbsClient.key",
		}
	})

	JustBeforeEach(func() {
		err = dummyCmd.PreRunE(dummyCmd, dummyCmd.Flags().Args())
	})

	Describe("bbsURL", func() {
		Context("when the --bbsURL isn't given", func() {
			BeforeEach(func() {
				parseFlagsErr := dummyCmd.ParseFlags(removeFlag(validFlags, "--bbsURL"))
				Expect(parseFlagsErr).NotTo(HaveOccurred())
			})

			It("returns an error message", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).Should(Equal(
					"BBS URL not set. Please specify one with the '--bbsURL' flag or the 'BBS_URL' environment variable."))
			})

			It("exits with code 3", func() {
				argsErr, ok := err.(commands.CFDotError)
				Expect(ok).To(BeTrue())
				Expect(argsErr.ExitCode()).To(Equal(3))
			})
		})

		Context("when the --bbsURL isn't a valid URL", func() {
			BeforeEach(func() {
				parseFlagsErr := dummyCmd.ParseFlags(replaceFlagValue(validFlags, "--bbsURL", ":"))
				Expect(parseFlagsErr).NotTo(HaveOccurred())
			})

			It("returns an error message", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).Should(Equal(
					"The value ':' is not a valid BBS URL. Please specify one with the '--bbsURL' flag or the 'BBS_URL' environment variable."))
			})

			It("exits with code 3", func() {
				argsErr, ok := err.(commands.CFDotError)
				Expect(ok).To(BeTrue())
				Expect(argsErr.ExitCode()).To(Equal(3))
			})
		})

		Context("when the --bbsURL is not http or https", func() {
			BeforeEach(func() {
				parseFlagsErr := dummyCmd.ParseFlags(replaceFlagValue(validFlags, "--bbsURL", "nohttp.com"))
				Expect(parseFlagsErr).NotTo(HaveOccurred())
			})

			It("returns an error message", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).Should(Equal(
					"The URL 'nohttp.com' does not have an 'https' scheme. Please specify one with the '--bbsURL' flag or the 'BBS_URL' environment variable."))
			})

			It("exits with code 3", func() {
				argsErr, ok := err.(commands.CFDotError)
				Expect(ok).To(BeTrue())
				Expect(argsErr.ExitCode()).To(Equal(3))
			})
		})

		Context("when a BBS_URL environment variable is specified", func() {
			AfterEach(func() {
				os.Unsetenv("BBS_URL")
			})

			Context("when the BBS_URL is valid", func() {
				BeforeEach(func() {
					parseFlagsErr := dummyCmd.ParseFlags(removeFlag(validFlags, "--bbsURL"))
					Expect(parseFlagsErr).NotTo(HaveOccurred())
					os.Setenv("BBS_URL", "https://example.com")
				})

				It("does not error", func() {
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when the BBS_URL is not valid", func() {
				BeforeEach(func() {
					parseFlagsErr := dummyCmd.ParseFlags(removeFlag(validFlags, "--bbsURL"))
					Expect(parseFlagsErr).NotTo(HaveOccurred())
					os.Setenv("BBS_URL", ":")
				})

				It("returns an err", func() {
					Expect(err).To(MatchError("The value ':' is not a valid BBS URL. Please specify one with the '--bbsURL' flag or the 'BBS_URL' environment variable."))
				})
			})

			Context("when the --bbsURL flag is also specified", func() {
				BeforeEach(func() {
					parseFlagsErr := dummyCmd.ParseFlags(buildArgList(validFlags))
					Expect(parseFlagsErr).NotTo(HaveOccurred())
					os.Setenv("BBS_URL", "broken url")
				})

				It("uses the value from the flag instead of the environment variable", func() {
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})
	})
})
