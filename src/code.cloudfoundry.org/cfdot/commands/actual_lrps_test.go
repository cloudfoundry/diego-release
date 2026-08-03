package commands_test

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfdot/commands"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/openzipkin/zipkin-go/model"
)

var _ = Describe("ActualLRPs", func() {
	var (
		fakeBBSClient  *fake_bbs.FakeClient
		actualLRPs     []*models.ActualLRP
		returnedError  error
		stdout, stderr *gbytes.Buffer
	)

	BeforeEach(func() {
		actualLRPs = nil
		returnedError = nil
		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()
		fakeBBSClient = &fake_bbs.FakeClient{}
	})

	JustBeforeEach(func() {
		fakeBBSClient.ActualLRPsReturns(actualLRPs, returnedError)
	})

	Context("when the bbs responds with actual lrps", func() {
		BeforeEach(func() {
			actualLRPs = []*models.ActualLRP{
				&models.ActualLRP{
					State: "running",
				},
			}
		})

		It("prints a json stream of all the actual lrps", func() {
			index := int32(4)
			err := commands.ActualLRPs(stdout, stderr, fakeBBSClient, "domain-1", "cell-1", "pg-2", &index)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBBSClient.ActualLRPsCallCount()).To(Equal(1))

			_, traceID, filter := fakeBBSClient.ActualLRPsArgsForCall(0)
			Expect(filter).To(Equal(models.ActualLRPFilter{
				CellID:      "cell-1",
				Domain:      "domain-1",
				ProcessGuid: "pg-2",
				Index:       &index,
			}))
			_, err = model.TraceIDFromHex(traceID)
			Expect(err).NotTo(HaveOccurred())

			expectedOutput := ""
			for _, lrp := range actualLRPs {
				d, err := json.Marshal(lrp)
				Expect(err).NotTo(HaveOccurred())
				expectedOutput += string(d) + "\n"
			}

			Expect(string(stdout.Contents())).To(Equal(expectedOutput))
		})
	})

	Context("when the bbs errors", func() {
		BeforeEach(func() {
			returnedError = models.ErrUnknownError
		})

		It("fails with a relevant error", func() {
			err := commands.ActualLRPs(stdout, stderr, fakeBBSClient, "", "", "", nil)
			Expect(err).To(HaveOccurred())

			Expect(err).To(Equal(models.ErrUnknownError))
		})
	})
})
