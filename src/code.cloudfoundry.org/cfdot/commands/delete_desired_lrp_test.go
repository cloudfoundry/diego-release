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

var _ = Describe("DeleteDesiredLRP", func() {
	var (
		fakeBBSClient  *fake_bbs.FakeClient
		returnedError  error
		stdout, stderr *gbytes.Buffer
		processGuid    string
	)

	BeforeEach(func() {
		fakeBBSClient = &fake_bbs.FakeClient{}
		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()

		fakeBBSClient.RemoveDesiredLRPReturns(returnedError)
	})

	It("deletes the desired lrp", func() {
		err := commands.DeleteDesiredLRP(stdout, stderr, fakeBBSClient, processGuid)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeBBSClient.RemoveDesiredLRPCallCount()).To(Equal(1))
		_, traceID, lrp := fakeBBSClient.RemoveDesiredLRPArgsForCall(0)

		_, err = model.TraceIDFromHex(traceID)
		Expect(err).NotTo(HaveOccurred())
		Expect(lrp).To(Equal(processGuid))

	})

	Context("when the bbs errors", func() {
		BeforeEach(func() {
			fakeBBSClient.RemoveDesiredLRPReturns(models.ErrUnknownError)
		})

		It("fails with a relevant error", func() {
			err := commands.DeleteDesiredLRP(stdout, stderr, fakeBBSClient, "the-process-guid")
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(models.ErrUnknownError))
		})
	})
})
