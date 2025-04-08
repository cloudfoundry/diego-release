package commands_test

import (
	"os"

	"code.cloudfoundry.org/cfdot/commands"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/spf13/cobra"
)

var _ = Describe("Locket Flags", func() {
	var validTLSFlags map[string]string
	var dummyCmd *cobra.Command
	var err error
	var output *gbytes.Buffer

	BeforeEach(func() {
		dummyCmd = &cobra.Command{
			Use: "dummy",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		commands.AddLocketFlags(dummyCmd)
		output = gbytes.NewBuffer()
		dummyCmd.SetOut(output)

		validTLSFlags = map[string]string{
			"--locketAPILocation": "127.0.0.1:9802",
			"--skipCertVerify":    "false",
			"--caCertFile":        "fixtures/bbsCACert.crt",
			"--clientCertFile":    "fixtures/bbsClient.crt",
			"--clientKeyFile":     "fixtures/bbsClient.key",
		}
	})

	JustBeforeEach(func() {
		err = dummyCmd.PreRunE(dummyCmd, dummyCmd.Flags().Args())
	})

	Describe("locketAPILocation", func() {
		Context("when the --locketAPILocation isn't given", func() {
			BeforeEach(func() {
				parseFlagsErr := dummyCmd.ParseFlags(removeFlag(validTLSFlags, "--locketAPILocation"))
				Expect(parseFlagsErr).NotTo(HaveOccurred())
			})

			It("returns an error message", func() {
				Expect(err).To(MatchError(
					"Locket API Location not set. Please specify one with the '--locketAPILocation' flag or the 'LOCKET_API_LOCATION' environment variable.",
				))
			})

			It("exits with code 3", func() {
				argsErr, ok := err.(commands.CFDotError)
				Expect(ok).To(BeTrue())
				Expect(argsErr.ExitCode()).To(Equal(3))
			})
		})

		Context("when a LOCKET_API_LOCATION environment variable is specified", func() {
			AfterEach(func() {
				os.Unsetenv("LOCKET_API_LOCATION")
			})

			Context("when the LOCKET_API_LOCATION is valid", func() {
				BeforeEach(func() {
					parseFlagsErr := dummyCmd.ParseFlags(removeFlag(validTLSFlags, "--locketAPILocation"))
					Expect(parseFlagsErr).NotTo(HaveOccurred())
					os.Setenv("LOCKET_API_LOCATION", "example.com:9083")
				})

				It("does not error", func() {
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when the --locketAPILocation flag is also specified", func() {
				BeforeEach(func() {
					parseFlagsErr := dummyCmd.ParseFlags(buildArgList(validTLSFlags))
					Expect(parseFlagsErr).NotTo(HaveOccurred())
					os.Setenv("LOCKET_API_LOCATION", "broken url")
				})

				It("uses the value from the flag instead of the environment variable", func() {
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})
	})
})
