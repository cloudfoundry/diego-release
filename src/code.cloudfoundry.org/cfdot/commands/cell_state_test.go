package commands_test

import (
	"encoding/json"
	"errors"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfdot/commands"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/repfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("CellState", func() {
	Context("ValidateCellStateArguments", func() {
		It("validates there is one and only one argument", func() {
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

	Context("FetchCellRegistration", func() {
		var (
			fakeBBSClient *fake_bbs.FakeClient
			presence      *models.CellPresence
			cellId        string
		)

		BeforeEach(func() {
			cellId = "cell-id"
			presence = &models.CellPresence{
				CellId: cellId,
			}

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

		It("returns the cell presence", func() {
			traceID := "some-trace-id"
			receivedPresence, err := commands.FetchCellRegistration(fakeBBSClient, traceID, cellId)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeBBSClient.CellsCallCount()).To(Equal(1))

			_, actualTraceID := fakeBBSClient.CellsArgsForCall(0)
			Expect(actualTraceID).To(Equal(traceID))

			Expect(receivedPresence).To(Equal(presence))
		})

		Context("when fetching the cells fails", func() {
			BeforeEach(func() {
				fakeBBSClient.CellsReturns(nil, errors.New("boom"))
			})

			It("returns the error", func() {
				_, err := commands.FetchCellRegistration(fakeBBSClient, "some-trace-id", cellId)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the cell doesn't exist", func() {
			It("returns an error", func() {
				_, err := commands.FetchCellRegistration(fakeBBSClient, "some-trace-id", "non-existent")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("FetchCellState", func() {
		var (
			fakeRepClient        *repfakes.FakeClient
			fakeRepClientFactory *repfakes.FakeClientFactory
			stdout, stderr       *gbytes.Buffer
			registration         *models.CellPresence
			state                rep.CellState
		)

		BeforeEach(func() {
			state = rep.CellState{
				RepURL:             "https://7a66adb6-1ce5-4c12-9127-6ff13efd1a79.cell.service.cf.internal:1801",
				CellID:             "cell-id",
				RootFSProviders:    rep.RootFSProviders{},
				AvailableResources: rep.Resources{},
				TotalResources:     rep.Resources{},
				LRPs:               []rep.LRP{},
			}

			stdout = gbytes.NewBuffer()
			stderr = gbytes.NewBuffer()

			registration = &models.CellPresence{
				RepUrl:     "something",
				RepAddress: "something/else",
			}

			fakeRepClient = &repfakes.FakeClient{}
			fakeRepClient.StateReturns(state, nil)

			fakeRepClientFactory = &repfakes.FakeClientFactory{}
			fakeRepClientFactory.CreateClientReturns(fakeRepClient, nil)
		})

		It("outputs the cell state to stdout", func() {
			err := commands.FetchCellState(stdout, stderr, fakeRepClientFactory, registration, "some-trace-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeRepClient.StateCallCount()).To(Equal(1))
			_, _, actualTraceID := fakeRepClientFactory.CreateClientArgsForCall(0)
			Expect(actualTraceID).To(Equal("some-trace-id"))

			var receivedState rep.CellState
			err = json.Unmarshal(stdout.Contents(), &receivedState)
			Expect(err).NotTo(HaveOccurred())
			Expect(receivedState).To(Equal(state))
		})

		Context("when the rep fails to respond", func() {
			BeforeEach(func() {
				fakeRepClient.StateReturns(rep.CellState{}, errors.New("boom"))
			})

			It("returns an error", func() {
				err := commands.FetchCellState(stdout, stderr, fakeRepClientFactory, registration, "some-trace-id")
				Expect(err).To(HaveOccurred())
				Expect(fakeRepClient.StateCallCount()).To(Equal(1))
			})
		})
	})
})
