package commands_test

import (
	"errors"

	"code.cloudfoundry.org/cfdot/commands"
	"code.cloudfoundry.org/locket/models"
	"code.cloudfoundry.org/locket/models/modelsfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ReleaseLock", func() {
	var (
		fakeLocketClient *modelsfakes.FakeLocketClient
		stdout, stderr   *gbytes.Buffer
	)

	BeforeEach(func() {
		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()
		fakeLocketClient = &modelsfakes.FakeLocketClient{}
	})

	Context("when there is no error", func() {
		BeforeEach(func() {
			response := &models.ReleaseResponse{}
			fakeLocketClient.ReleaseReturns(response, nil)
		})

		It("should release the lock successfully", func() {
			err := commands.ReleaseLock(
				stdout, stderr, fakeLocketClient, "key", "owner")
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLocketClient.ReleaseCallCount()).To(Equal(1))

			_, req, _ := fakeLocketClient.ReleaseArgsForCall(0)
			Expect(req).To(Equal(&models.ReleaseRequest{
				Resource: &models.Resource{
					Key:   "key",
					Owner: "owner",
				},
			}))
		})
	})

	Context("when there is an error", func() {
		BeforeEach(func() {
			fakeLocketClient.ReleaseReturns(nil, errors.New("random-error"))
		})

		It("should return an error", func() {
			err := commands.ReleaseLock(
				stdout, stderr, fakeLocketClient, "key", "owner")
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(errors.New("random-error")))
		})
	})

	Context("Validations", func() {
		var (
			key   string
			owner string
		)

		BeforeEach(func() {
			key = "key"
			owner = "owner"
		})

		It("should not allow for any arguments", func() {
			err := commands.ValidateReleaseLocksArguments(nil, []string{"1"}, key, owner)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(errors.New("Too many arguments specified")))
		})

		It("should not allow for missing key", func() {
			key = ""
			err := commands.ValidateReleaseLocksArguments(nil, []string{}, key, owner)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("key cannot be empty"))
			Expect(err.(commands.CFDotError).ExitCode()).To(Equal(3))
		})

		It("should not allow for missing owner", func() {
			owner = ""
			err := commands.ValidateReleaseLocksArguments(nil, []string{}, key, owner)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("owner cannot be empty"))
			Expect(err.(commands.CFDotError).ExitCode()).To(Equal(3))
		})
	})
})
