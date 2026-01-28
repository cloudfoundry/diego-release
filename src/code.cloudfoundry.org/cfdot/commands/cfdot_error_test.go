package commands_test

import (
	"errors"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfdot/commands"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("CfDotError", func() {
	var (
		err commands.CFDotError
		cmd *cobra.Command
	)

	BeforeEach(func() {
		cmd = &cobra.Command{}
	})

	Context("when a bbs error occurs", func() {
		BeforeEach(func() {
			err = commands.NewCFDotError(cmd, &models.Error{
				Type:    models.Error_Deadlock,
				Message: "The request failed due to a deadlock",
			})
		})

		Describe("Error()", func() {
			It("Returns the error code and error message", func() {
				Expect(err.Error()).To(ContainSubstring("BBS error"))
				Expect(err.Error()).To(ContainSubstring("Type 28: Deadlock"))
				Expect(err.Error()).To(ContainSubstring("Message: The request failed due to a deadlock"))
			})
		})

		It("returns an exit code of 4", func() {
			Expect(err.ExitCode()).To(Equal(4))
		})

		It("silence the usage message", func() {
			Expect(cmd.SilenceUsage).To(BeTrue())
		})
	})

	Context("when a component error occurs", func() {
		BeforeEach(func() {
			err = commands.NewCFDotComponentError(cmd, &models.Error{
				Type:    models.Error_UnknownError,
				Message: "connection refused",
			})
		})
		It("returns an exit code of 4", func() {
			Expect(err.ExitCode()).To(Equal(4))
		})

		It("silence the usage message", func() {
			Expect(cmd.SilenceUsage).To(BeTrue())
		})
	})

	Context("when a validation error occurs", func() {
		BeforeEach(func() {
			err = commands.NewCFDotValidationError(cmd, errors.New("some error"))
		})

		Describe("Error()", func() {
			It("Error() returns the error text", func() {
				Expect(err.Error()).To(Equal("some error"))
			})
		})

		It("returns an exit code of 3", func() {
			Expect(err.ExitCode()).To(Equal(3))
		})

		It("does not silence the usage message", func() {
			Expect(cmd.SilenceUsage).To(BeFalse())
		})
	})
})
