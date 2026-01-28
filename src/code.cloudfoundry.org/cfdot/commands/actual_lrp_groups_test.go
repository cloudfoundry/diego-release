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

var _ = Describe("ActualLRPGroups", func() {
	var (
		fakeBBSClient *fake_bbs.FakeClient

		//lint:ignore SA1019 - deprecated model used for testing deprecated functionality
		actualLRPGroups []*models.ActualLRPGroup
		returnedError   error
		stdout, stderr  *gbytes.Buffer
	)

	BeforeEach(func() {
		actualLRPGroups = nil
		returnedError = nil
		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()
		fakeBBSClient = &fake_bbs.FakeClient{}
	})

	JustBeforeEach(func() {
		fakeBBSClient.ActualLRPGroupsReturns(actualLRPGroups, returnedError)
	})

	Context("when the bbs responds with actual lrp groups", func() {
		BeforeEach(func() {
			//lint:ignore SA1019 - deprecated model used for testing deprecated functionality
			actualLRPGroups = []*models.ActualLRPGroup{
				{
					Instance: &models.ActualLRP{
						State: "running",
					},
				},
			}
		})

		It("prints a json stream of all the actual lrp groups", func() {
			err := commands.ActualLRPGroups(stdout, stderr, fakeBBSClient, "domain-1", "cell-1")
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBBSClient.ActualLRPGroupsCallCount()).To(Equal(1))

			_, traceID, filter := fakeBBSClient.ActualLRPGroupsArgsForCall(0)
			Expect(filter).To(Equal(models.ActualLRPFilter{CellID: "cell-1", Domain: "domain-1"}))
			_, err = model.TraceIDFromHex(traceID)
			Expect(err).NotTo(HaveOccurred())

			expectedOutput := ""
			for _, group := range actualLRPGroups {
				d, err := json.Marshal(group)
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
			err := commands.ActualLRPGroups(stdout, stderr, fakeBBSClient, "", "")
			Expect(err).To(HaveOccurred())

			Expect(err).To(Equal(models.ErrUnknownError))
		})
	})
})
