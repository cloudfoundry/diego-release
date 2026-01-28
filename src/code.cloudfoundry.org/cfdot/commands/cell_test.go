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

var _ = Describe("Cell", func() {
	Context("ValidateCellArguments", func() {
		It("validates the arguments", func() {
			err := commands.ValidateCellArguments([]string{"cell-id"})
			Expect(err).NotTo(HaveOccurred())
		})

		Context("more than 1 argument", func() {
			It("returns an extra arguments error", func() {
				err := commands.ValidateCellArguments([]string{"cell-id", "extra-arg"})
				Expect(err).To(MatchError("Too many arguments specified"))
			})
		})

		Context("no arguments", func() {
			It("returns a missing arguments error", func() {
				err := commands.ValidateCellArguments([]string{})
				Expect(err).To(MatchError("Missing arguments"))
			})
		})
	})

	Context("Cell", func() {
		var (
			fakeBBSClient  *fake_bbs.FakeClient
			stdout, stderr *gbytes.Buffer
			presence       *models.CellPresence
			cellId         string
		)

		BeforeEach(func() {
			cellId = "cell-id"
			presence = &models.CellPresence{
				CellId: cellId,
			}

			stdout = gbytes.NewBuffer()
			stderr = gbytes.NewBuffer()
			fakeBBSClient = &fake_bbs.FakeClient{}
			fakeBBSClient.CellsReturns([]*models.CellPresence{
				{
					CellId: "cell-id2",
				},
				presence,
				{
					CellId: "cell-id3",
				},
			}, nil)
		})

		It("fetches the cell presence", func() {
			err := commands.Cell(stdout, stderr, fakeBBSClient, cellId)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeBBSClient.CellsCallCount()).To(Equal(1))
		})

		It("outputs the cell presence to stdout", func() {
			err := commands.Cell(stdout, stderr, fakeBBSClient, cellId)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeBBSClient.CellsCallCount()).To(Equal(1))

			var receivedPresence models.CellPresence
			err = json.Unmarshal(stdout.Contents(), &receivedPresence)
			Expect(err).NotTo(HaveOccurred())
			Expect(receivedPresence).To(Equal(*presence))
		})

		Context("when encoding to stdout fails", func() {
			BeforeEach(func() {
				Expect(stdout.Close()).To(Succeed())
			})

			It("returns the error", func() {
				err := commands.Cell(stdout, stderr, fakeBBSClient, cellId)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when fetching the cells fails", func() {
			BeforeEach(func() {
				fakeBBSClient.CellsReturns(nil, errors.New("boom"))
			})

			It("returns the error", func() {
				err := commands.Cell(stdout, stderr, fakeBBSClient, cellId)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the cell doesn't exist", func() {
			It("returns an error", func() {
				err := commands.Cell(stdout, stderr, fakeBBSClient, "non-existent")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
