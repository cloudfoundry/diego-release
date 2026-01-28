package commands_test

import (
	"encoding/json"
	"errors"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfdot/commands"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("DesiredLRP Command", func() {
	Context("DesiredLRP", func() {
		var (
			fakeBBSClient  *fake_bbs.FakeClient
			stdout, stderr *gbytes.Buffer
			desiredLRP     *models.DesiredLRP
		)

		BeforeEach(func() {
			fakeBBSClient = &fake_bbs.FakeClient{}
			stdout = gbytes.NewBuffer()
			stderr = gbytes.NewBuffer()

			desiredLRP = &models.DesiredLRP{
				ProcessGuid: "test-guid",
				Instances:   1,
			}

			fakeBBSClient.DesiredLRPByProcessGuidReturns(desiredLRP, nil)
		})

		It("writes the json representation of the desired LRP to stdout", func() {
			err := commands.DesiredLRP(stdout, stderr, fakeBBSClient, "test-guid")
			Expect(err).NotTo(HaveOccurred())

			jsonData, err := json.Marshal(desiredLRP)
			Expect(err).NotTo(HaveOccurred())

			Expect(stdout).To(gbytes.Say(string(jsonData)))
		})

		Context("when fetching the desired lrp fails", func() {
			BeforeEach(func() {
				fakeBBSClient.DesiredLRPByProcessGuidReturns(nil, errors.New("i failed"))
			})

			It("returns the error", func() {
				err := commands.DesiredLRP(stdout, stderr, fakeBBSClient, "test-guid")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("ValidateDesiredLRPArguments", func() {
		It("returns the process guid", func() {
			processGuid, err := commands.ValidateDesiredLRPArguments([]string{"test-guid"})
			Expect(err).NotTo(HaveOccurred())
			Expect(processGuid).To(Equal("test-guid"))
		})

		Context("when no arguments are supplied", func() {
			It("returns an error", func() {
				_, err := commands.ValidateDesiredLRPArguments([]string{})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when too many arguments are supplied", func() {
			It("returns an error", func() {
				_, err := commands.ValidateDesiredLRPArguments([]string{"one", "two"})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when invalid arguments are supplied", func() {
			It("returns an error", func() {
				_, err := commands.ValidateDesiredLRPArguments([]string{""})
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
