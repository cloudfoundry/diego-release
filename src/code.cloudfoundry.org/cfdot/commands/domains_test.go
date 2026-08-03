package commands_test

import (
	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfdot/commands"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Domains", func() {
	var (
		fakeBBSClient  *fake_bbs.FakeClient
		stdout, stderr *gbytes.Buffer
	)

	BeforeEach(func() {
		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()
		fakeBBSClient = &fake_bbs.FakeClient{}
	})

	Context("when the bbs responds with domains", func() {
		BeforeEach(func() {
			fakeBBSClient.DomainsReturns([]string{"domain-1", "domain-2"}, nil)
		})

		It("prints a json stream of all the domains", func() {
			err := commands.Domains(stdout, stderr, fakeBBSClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(gbytes.Say(`"domain-1"\n"domain-2"\n`))
		})
	})

	Context("when the bbs responds with no domains", func() {
		BeforeEach(func() {
			fakeBBSClient.DomainsReturns([]string{}, nil)
		})

		It("returns an empty response", func() {
			err := commands.Domains(stdout, stderr, fakeBBSClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout.Contents()).To(BeEmpty())
		})
	})

	Context("when the bbs errors", func() {
		BeforeEach(func() {
			fakeBBSClient.DomainsReturns(nil, models.ErrUnknownError)
		})

		It("fails with a relevant error", func() {
			err := commands.Domains(stdout, stderr, fakeBBSClient)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(models.ErrUnknownError))
		})
	})
})
