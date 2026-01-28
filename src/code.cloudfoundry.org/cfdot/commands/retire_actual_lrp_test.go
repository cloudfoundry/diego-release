package commands_test

import (
	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfdot/commands"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/openzipkin/zipkin-go/model"
)

var _ = Describe("RetireActualLRP", func() {
	Context("ValidateRetireActualLRPArgs", func() {
		It("returns the process guid and index", func() {
			guid, index, err := commands.ValidateRetireActualLRPArgs([]string{"guid", "1"})
			Expect(err).NotTo(HaveOccurred())
			Expect(guid).To(Equal("guid"))
			Expect(index).To(Equal(1))
		})

		Context("when there are too many arguments", func() {
			It("returns an error", func() {
				_, _, err := commands.ValidateRetireActualLRPArgs([]string{"guid", "1", "3"})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when there are too few arguments", func() {
			It("returns an error", func() {
				_, _, err := commands.ValidateRetireActualLRPArgs([]string{})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the process guid is empty", func() {
			It("returns an error", func() {
				_, _, err := commands.ValidateRetireActualLRPArgs([]string{"", "1"})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the index is invalid", func() {
			It("returns an error", func() {
				_, _, err := commands.ValidateRetireActualLRPArgs([]string{"guid", "-1"})
				Expect(err).To(HaveOccurred())

				_, _, err = commands.ValidateRetireActualLRPArgs([]string{"guid", "not-a-number"})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("RetireActualLRP", func() {
		var (
			fakeBBSClient  *fake_bbs.FakeClient
			desiredLRP     *models.DesiredLRP
			stdout, stderr *gbytes.Buffer
		)

		BeforeEach(func() {
			stdout = gbytes.NewBuffer()
			stderr = gbytes.NewBuffer()
			fakeBBSClient = &fake_bbs.FakeClient{}

			desiredLRP = &models.DesiredLRP{
				Domain: "test-domain.com",
			}

			fakeBBSClient.RetireActualLRPReturns(nil)
			fakeBBSClient.DesiredLRPByProcessGuidReturns(desiredLRP, nil)
		})

		It("retires the actual lrp", func() {
			err := commands.RetireActualLRP(stdout, stderr, fakeBBSClient, "process-guid", 1)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBBSClient.DesiredLRPByProcessGuidCallCount()).To(Equal(1))
			_, traceID, processGuid := fakeBBSClient.DesiredLRPByProcessGuidArgsForCall(0)

			_, err = model.TraceIDFromHex(traceID)
			Expect(err).NotTo(HaveOccurred())
			Expect(processGuid).To(Equal("process-guid"))

			Expect(fakeBBSClient.RetireActualLRPCallCount()).To(Equal(1))
			_, traceID2, actualLRPKey := fakeBBSClient.RetireActualLRPArgsForCall(0)
			Expect(traceID2).To(Equal(traceID))

			Expect(actualLRPKey).To(Equal(&models.ActualLRPKey{
				ProcessGuid: "process-guid",
				Index:       1,
				Domain:      "test-domain.com",
			}))
		})

		Context("when retiring the actual lrp fails", func() {
			BeforeEach(func() {
				fakeBBSClient.RetireActualLRPReturns(models.ErrUnknownError)
			})

			It("fails with a relevant error ", func() {
				err := commands.RetireActualLRP(stdout, stderr, fakeBBSClient, "process-guid", 2)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrUnknownError))
			})
		})

		Context("when fetching the desired lrp fails", func() {
			BeforeEach(func() {
				fakeBBSClient.DesiredLRPByProcessGuidReturns(nil, models.ErrUnknownError)
			})

			It("fails with a relevant error ", func() {
				err := commands.RetireActualLRP(stdout, stderr, fakeBBSClient, "process-guid", 2)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrUnknownError))

				Expect(fakeBBSClient.DesiredLRPByProcessGuidCallCount()).To(Equal(1))
				Expect(fakeBBSClient.RetireActualLRPCallCount()).To(Equal(0))
			})
		})
	})
})
