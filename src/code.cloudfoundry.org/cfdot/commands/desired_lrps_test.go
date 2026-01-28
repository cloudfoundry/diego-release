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

var _ = Describe("DesiredLRPs", func() {
	var (
		fakeBBSClient  *fake_bbs.FakeClient
		desiredLrps    []*models.DesiredLRP
		returnedError  error
		stdout, stderr *gbytes.Buffer
	)

	BeforeEach(func() {
		fakeBBSClient = &fake_bbs.FakeClient{}
		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()

		desiredLrps = []*models.DesiredLRP{
			{
				Instances: 1,
			},
		}
		fakeBBSClient.DesiredLRPsReturns(desiredLrps, returnedError)
	})

	It("prints a json stream of all the desired lrps", func() {
		err := commands.DesiredLRPs(stdout, stderr, fakeBBSClient, "domain")
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeBBSClient.DesiredLRPsCallCount()).To(Equal(1))
		_, traceID, filter := fakeBBSClient.DesiredLRPsArgsForCall(0)

		_, err = model.TraceIDFromHex(traceID)
		Expect(err).NotTo(HaveOccurred())
		Expect(filter).To(Equal(models.DesiredLRPFilter{Domain: "domain"}))

		expectedOutput := ""
		for _, info := range desiredLrps {
			d, err := json.Marshal(info)
			Expect(err).NotTo(HaveOccurred())
			expectedOutput += string(d) + "\n"
		}

		Expect(string(stdout.Contents())).To(Equal(expectedOutput))
	})

	Context("when the bbs errors", func() {
		BeforeEach(func() {
			fakeBBSClient.DesiredLRPsReturns(nil, models.ErrUnknownError)
		})

		It("fails with a relevant error", func() {
			err := commands.DesiredLRPs(stdout, stderr, fakeBBSClient, "domain")
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(models.ErrUnknownError))
		})
	})
})
