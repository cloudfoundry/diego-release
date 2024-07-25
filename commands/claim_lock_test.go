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

var _ = Describe("ClaimLock", func() {
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
			response := &models.LockResponse{}
			fakeLocketClient.LockReturns(response, nil)
		})

		It("should claim the lock successfully", func() {
			err := commands.ClaimLock(
				stdout, stderr, fakeLocketClient,
				"key", "owner", "value", 60)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLocketClient.LockCallCount()).To(Equal(1))

			_, req, _ := fakeLocketClient.LockArgsForCall(0)
			Expect(req).To(Equal(&models.LockRequest{
				Resource: &models.Resource{
					Key:      "key",
					Owner:    "owner",
					Value:    "value",
					TypeCode: models.TypeCode_LOCK,
				},
				TtlInSeconds: 60,
			}))
		})
	})

	Context("when there is an error", func() {
		BeforeEach(func() {
			fakeLocketClient.LockReturns(nil, errors.New("random-error"))
		})

		It("should return an error", func() {
			err := commands.ClaimLock(
				stdout, stderr, fakeLocketClient,
				"key", "owner", "value", 60)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(errors.New("random-error")))
		})
	})

	Context("Validations", func() {
		var (
			key          string
			owner        string
			value        string
			ttlInSeconds int
		)

		BeforeEach(func() {
			key = "key"
			owner = "owner"
			value = "value"
			ttlInSeconds = 1
		})

		It("should not allow for any arguments", func() {
			err := commands.ValidateClaimLocksArguments(nil, []string{"1"}, key, owner, value, ttlInSeconds)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(errors.New("Too many arguments specified")))
		})

		It("should not allow for missing key", func() {
			key = ""
			err := commands.ValidateClaimLocksArguments(nil, []string{}, key, owner, value, ttlInSeconds)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("key cannot be empty"))
			Expect(err.(commands.CFDotError).ExitCode()).To(Equal(3))
		})

		It("should not allow for missing owner", func() {
			owner = ""
			err := commands.ValidateClaimLocksArguments(nil, []string{}, key, owner, value, ttlInSeconds)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("owner cannot be empty"))
			Expect(err.(commands.CFDotError).ExitCode()).To(Equal(3))
		})

		It("should not allow for negative ttl", func() {
			ttlInSeconds = -1
			err := commands.ValidateClaimLocksArguments(nil, []string{}, key, owner, value, ttlInSeconds)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("ttl should be an integer greater than zero"))
			Expect(err.(commands.CFDotError).ExitCode()).To(Equal(3))
		})
	})
})
