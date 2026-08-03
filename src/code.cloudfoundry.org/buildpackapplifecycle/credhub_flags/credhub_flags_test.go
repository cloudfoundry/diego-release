package credhub_flags_test

import (
	"flag"
	"time"

	"code.cloudfoundry.org/buildpackapplifecycle/credhub_flags"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("credhub_flags", func() {

	Context("NewCredhubFlags - for when they are the only flags", func() {
		var flags credhub_flags.CredhubFlags
		JustBeforeEach(func() {
			flags = credhub_flags.NewCredhubFlags("testing")
		})

		Context("when no flags are passed", func() {
			It("uses the defaults", func() {
				args := []string{}
				flags.Parse(args)

				Expect(flags.ConnectAttempts()).To(Equal(3))
				Expect(flags.RetryDelay()).To(Equal(1 * time.Second))
			})
		})

		Context("when flags are passed", func() {
			It("uses the provided values", func() {
				args := []string{"-credhubConnectAttempts=5", "-credhubRetryDelay=5s"}
				flags.Parse(args)

				Expect(flags.ConnectAttempts()).To(Equal(5))
				Expect(flags.RetryDelay()).To(Equal(5 * time.Second))
			})
		})
	})

	Context("AddCredhubFlags - for when you want to merge them to other flags", func() {
		var flags *flag.FlagSet
		JustBeforeEach(func() {
			flags = flag.NewFlagSet("testing", flag.ExitOnError)
			credhub_flags.AddCredhubFlags(flags)
		})

		Context("when no flags are passed", func() {
			It("uses the defaults", func() {
				args := []string{}
				flags.Parse(args)

				Expect(credhub_flags.ConnectAttempts(flags)).To(Equal(3))
				Expect(credhub_flags.RetryDelay(flags)).To(Equal(1 * time.Second))
			})
		})

		Context("when flags are passed", func() {
			It("uses the provided values", func() {
				args := []string{"-credhubConnectAttempts=5", "-credhubRetryDelay=5s"}
				flags.Parse(args)

				Expect(credhub_flags.ConnectAttempts(flags)).To(Equal(5))
				Expect(credhub_flags.RetryDelay(flags)).To(Equal(5 * time.Second))
			})
		})
	})
})
