package commands_test

import (
	"os"

	"code.cloudfoundry.org/cfdot/commands"
	"github.com/onsi/gomega/gbytes"
	"github.com/spf13/cobra"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Timeout Flag", func() {
	var validFlags map[string]string
	var dummyCmd *cobra.Command
	var err error
	var output *gbytes.Buffer

	BeforeEach(func() {
		dummyCmd = &cobra.Command{
			Use: "dummy",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		commands.AddBBSAndTimeoutFlags(dummyCmd)
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

	Context("when flags are passed in as arguments", func() {
		BeforeEach(func() {
			validFlags["--timeout"] = "10"
			parseFlagsErr := dummyCmd.ParseFlags(buildArgList(validFlags))
			Expect(parseFlagsErr).NotTo(HaveOccurred())
		})

		It("should set the flag in the configuration", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(commands.Config.Timeout).To(Equal(10))
		})
	})

	Context("with env vars", func() {
		BeforeEach(func() {
			parseFlagsErr := dummyCmd.ParseFlags(buildArgList(validFlags))
			Expect(parseFlagsErr).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			os.Unsetenv("CFDOT_TIMEOUT")
		})

		Context("when set through env var", func() {
			BeforeEach(func() {
				os.Setenv("CFDOT_TIMEOUT", "20")
			})

			It("should set the flag in the configuration", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(commands.Config.Timeout).To(Equal(20))
			})
		})

		Context("when invalid env var is set", func() {
			BeforeEach(func() {
				os.Setenv("CFDOT_TIMEOUT", "random thing")
			})

			It("errors when set to an invalid env var", func() {
				Expect(err).To(HaveOccurred())
				Expect(commands.Config.Timeout).To(Equal(0))
			})
		})
	})
})
