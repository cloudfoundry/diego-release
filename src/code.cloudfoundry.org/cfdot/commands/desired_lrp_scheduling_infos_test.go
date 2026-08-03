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

var _ = Describe("DesiredLRPSchedulingInfos", func() {
	var (
		fakeBBSClient   *fake_bbs.FakeClient
		schedulingInfos []*models.DesiredLRPSchedulingInfo
		stdout, stderr  *gbytes.Buffer
	)

	BeforeEach(func() {
		fakeBBSClient = &fake_bbs.FakeClient{}
		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()

		schedulingInfos = []*models.DesiredLRPSchedulingInfo{
			{
				Instances: 1,
			},
		}
		fakeBBSClient.DesiredLRPSchedulingInfosReturns(schedulingInfos, nil)
	})

	It("prints a json stream of all the desired lrp scheduling infos", func() {
		err := commands.DesiredLRPSchedulingInfos(stdout, stderr, fakeBBSClient, "domain")
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeBBSClient.DesiredLRPSchedulingInfosCallCount()).To(Equal(1))
		_, traceID, filter := fakeBBSClient.DesiredLRPSchedulingInfosArgsForCall(0)

		_, err = model.TraceIDFromHex(traceID)
		Expect(err).NotTo(HaveOccurred())
		Expect(filter).To(Equal(models.DesiredLRPFilter{Domain: "domain"}))

		expectedOutput := ""
		for _, info := range schedulingInfos {
			d, err := json.Marshal(info)
			Expect(err).NotTo(HaveOccurred())
			expectedOutput += string(d) + "\n"
		}
		Expect(string(stdout.Contents())).To(Equal(expectedOutput))
	})

	Context("when the bbs errors", func() {
		BeforeEach(func() {
			fakeBBSClient.DesiredLRPSchedulingInfosReturns(nil, models.ErrUnknownError)
		})

		It("fails with a relevant error", func() {
			err := commands.DesiredLRPSchedulingInfos(stdout, stderr, fakeBBSClient, "")
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(models.ErrUnknownError))
		})
	})
})
