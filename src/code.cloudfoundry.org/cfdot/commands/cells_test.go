package commands_test

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfdot/commands"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Cells", func() {
	var (
		fakeBBSClient  *fake_bbs.FakeClient
		cellPresences  []*models.CellPresence
		returnedError  error
		stdout, stderr *gbytes.Buffer
	)

	BeforeEach(func() {
		cellPresences = nil
		returnedError = nil
		stdout = gbytes.NewBuffer()
		stderr = gbytes.NewBuffer()
		fakeBBSClient = &fake_bbs.FakeClient{}
	})

	JustBeforeEach(func() {
		fakeBBSClient.CellsReturns(cellPresences, returnedError)
	})

	Context("when the bbs responds with cell presences", func() {
		BeforeEach(func() {
			cellPresences = []*models.CellPresence{
				{
					CellId:     "cell-1",
					RepAddress: "rep-1",
					Zone:       "zone1",
					Capacity: &models.CellCapacity{
						MemoryMb:   1024,
						DiskMb:     1024,
						Containers: 10,
					},
					RootfsProviders: []*models.Provider{
						{
							Name: "rootfs1",
						},
					},
					RepUrl: "http://rep1.com",
				},
			}
		})

		It("prints a json stream of all the cell presences", func() {
			err := commands.Cells(stdout, stderr, fakeBBSClient)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBBSClient.CellsCallCount()).To(Equal(1))

			expectedOutput := ""
			for _, cellPresence := range cellPresences {
				d, err := json.Marshal(cellPresence)
				Expect(err).NotTo(HaveOccurred())
				expectedOutput += string(d) + "\n"
			}

			Expect(string(stdout.Contents())).To(Equal(expectedOutput))
		})
	})

	Context("when the bbs errors", func() {
		JustBeforeEach(func() {
			fakeBBSClient.CellsReturns(nil, models.ErrUnknownError)
		})

		It("fails with a relevant error", func() {
			err := commands.Cells(stdout, stderr, fakeBBSClient)
			Expect(err).To(HaveOccurred())

			Expect(err).To(Equal(models.ErrUnknownError))
		})
	})
})
