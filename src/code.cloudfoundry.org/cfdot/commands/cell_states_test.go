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
	"github.com/spf13/cobra"
)

var _ = Describe("CellState", func() {
	Context("ValidateCellStatesArguments", func() {
		It("validates there are no arguments", func() {
			err := commands.ValidateCellStatesArguments([]string{})
			Expect(err).NotTo(HaveOccurred())
		})

		Context("more than 0 argument", func() {
			It("returns an extra arguments error", func() {
				err := commands.ValidateCellStatesArguments([]string{"extra-arg"})
				Expect(err).To(MatchError("Too many arguments specified"))
			})
		})
	})

	Context("FetchCellStates", func() {
		var (
			cmd                            *cobra.Command
			fakeRepClient1, fakeRepClient2 *repfakes.FakeClient
			fakeRepClientFactory           *repfakes.FakeClientFactory
			stdout, stderr                 *gbytes.Buffer
			state1, state2                 rep.CellState
			fakeBBSClient                  *fake_bbs.FakeClient
		)

		BeforeEach(func() {
			cmd = &cobra.Command{}

			state1 = rep.CellState{
				RepURL:             "https://cell-1.cell.service.cf.internal:1801",
				CellID:             "cell-id1",
				RootFSProviders:    rep.RootFSProviders{},
				AvailableResources: rep.Resources{},
				TotalResources:     rep.Resources{},
				LRPs:               []rep.LRP{},
			}
			state2 = rep.CellState{
				RepURL:             "https://cell-2.cell.service.cf.internal:1801",
				CellID:             "cell-id2",
				RootFSProviders:    rep.RootFSProviders{},
				AvailableResources: rep.Resources{},
				TotalResources:     rep.Resources{},
				LRPs:               []rep.LRP{},
			}

			stdout = gbytes.NewBuffer()
			stderr = gbytes.NewBuffer()

			fakeBBSClient = &fake_bbs.FakeClient{}
			fakeBBSClient.CellsReturns([]*models.CellPresence{
				{
					CellId:     "cell-id1",
					RepUrl:     "rep-url-1",
					RepAddress: "rep-address-1",
				},
				{
					CellId:     "cell-id2",
					RepUrl:     "rep-url-2",
					RepAddress: "rep-address-2",
				},
			}, nil)

			fakeRepClient1 = &repfakes.FakeClient{}
			fakeRepClient1.StateReturns(state1, nil)
			fakeRepClient2 = &repfakes.FakeClient{}
			fakeRepClient2.StateReturns(state2, nil)

			fakeRepClientFactory = &repfakes.FakeClientFactory{}
			fakeRepClientFactory.CreateClientReturnsOnCall(0, fakeRepClient1, nil)
			fakeRepClientFactory.CreateClientReturnsOnCall(1, fakeRepClient2, nil)
		})

		It("retrieves the cell registrations", func() {
			commands.FetchCellStates(cmd, stdout, stderr, fakeRepClientFactory, fakeBBSClient)
			Expect(fakeBBSClient.CellsCallCount()).To(Equal(1))
		})

		It("outputs the cell state to stdout", func() {
			commands.FetchCellStates(cmd, stdout, stderr, fakeRepClientFactory, fakeBBSClient)
			Expect(fakeRepClient1.StateCallCount()).To(Equal(1))
			Expect(fakeRepClient2.StateCallCount()).To(Equal(1))

			decoder := json.NewDecoder(stdout)
			var receivedState rep.CellState

			err := decoder.Decode(&receivedState)
			Expect(err).NotTo(HaveOccurred())
			Expect(receivedState).To(Equal(state1))

			err = decoder.Decode(&receivedState)
			Expect(err).NotTo(HaveOccurred())
			Expect(receivedState).To(Equal(state2))
		})

		Context("when the bbs fail to return cell registrations", func() {
			BeforeEach(func() {
				fakeBBSClient.CellsReturns(nil, errors.New("boom"))
			})

			It("prints an error", func() {
				err := commands.FetchCellStates(cmd, stdout, stderr, fakeRepClientFactory, fakeBBSClient)
				Expect(err).To(MatchError("BBS error: Failed to get cell registrations from BBS: boom"))
			})
		})

		Context("when one of the rep fails to respond", func() {
			BeforeEach(func() {
				fakeRepClient1.StateReturns(rep.CellState{}, errors.New("boom"))
			})

			It("prints an error", func() {
				err := commands.FetchCellStates(cmd, stdout, stderr, fakeRepClientFactory, fakeBBSClient)
				Expect(fakeRepClient2.StateCallCount()).To(Equal(1))
				Expect(err).To(MatchError(ContainSubstring("Rep error: Failed to get cell state for cell cell-id1: boom")))
			})

			It("prints the cell stats of the other cells", func() {
				commands.FetchCellStates(cmd, stdout, stderr, fakeRepClientFactory, fakeBBSClient)
				var receivedState rep.CellState
				err := json.NewDecoder(stdout).Decode(&receivedState)
				Expect(err).NotTo(HaveOccurred())
				Expect(receivedState).To(Equal(state2))
			})
		})
	})
})
