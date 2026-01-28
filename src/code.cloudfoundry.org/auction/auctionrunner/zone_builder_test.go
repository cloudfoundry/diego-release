package auctionrunner_test

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"code.cloudfoundry.org/auction/auctionrunner"
	"code.cloudfoundry.org/auction/auctiontypes/fakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/repfakes"
	"code.cloudfoundry.org/workpool"

	"code.cloudfoundry.org/lager/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

type logDataMatcher struct {
	expected interface{}
}

func IncludeLogData(expected interface{}) *logDataMatcher {
	return &logDataMatcher{
		expected: expected,
	}
}

func (matcher *logDataMatcher) Match(actual interface{}) (success bool, err error) {
	actualLog, ok := actual.(lager.LogFormat)
	if !ok {
		return false, fmt.Errorf("logMessageMatcher expects to validate an object of type lager.LogFormat")
	}

	var expectedLogData map[string]interface{}
	expectedLogData, ok = matcher.expected.(lager.Data)
	if !ok {
		return false, fmt.Errorf("logMessageMatcher validates the Data of a lager.LogFormat object against a desired map[string]interface{} or map[string]types.GomegaMatcher.")
	}

	for key, expectedValue := range expectedLogData {
		actualValue := actualLog.Data[key]
		if actualValue == nil {
			return false, nil
		}

		matcher, ok := expectedValue.(types.GomegaMatcher)
		if ok {
			matchSuccess, err := matcher.Match(actualValue)
			if err != nil {
				return false, err
			}

			if !matchSuccess {
				return false, nil
			}
		} else {
			if actualValue != expectedValue {
				return false, nil
			}
		}
	}

	return true, nil
}

func (matcher *logDataMatcher) FailureMessage(actual interface{}) string {
	actualLog := actual.(lager.LogFormat)

	return fmt.Sprintf("The log messages did not match. Expected \"%s\" but got \"%s\".", matcher.expected, actualLog.Message)
}

func (matcher *logDataMatcher) NegatedFailureMessage(actual interface{}) string {
	return "The log messages were supposed to be different, but they were the same."
}

var _ = Describe("ZoneBuilder", func() {
	var repA, repB, repC *repfakes.FakeSimClient
	var clients map[string]rep.Client
	var workPool *workpool.WorkPool
	var logger *lagertest.TestLogger
	var metricEmitter *fakes.FakeAuctionMetricEmitterDelegate
	var binPackFirstFitWeight float64

	getCellIndices := func(zone auctionrunner.Zone) []int {
		cellIndices := []int{}

		for _, cell := range zone {
			cellIndices = append(cellIndices, cell.Index)
		}

		return cellIndices
	}

	assertCellIndicesWithOrder := func(zone auctionrunner.Zone, expectedIndices []int) {
		cellIndices := getCellIndices(zone)
		Expect(cellIndices).To(Equal(expectedIndices))
	}

	assertCellIndices := func(zone auctionrunner.Zone, expectedIndices []int) {
		cellIndices := getCellIndices(zone)
		Expect(cellIndices).To(ConsistOf(expectedIndices))
	}

	assertCellIDs := func(zone auctionrunner.Zone, expectedIDs []string) {
		cellIDs := []string{}

		for _, cell := range zone {
			cellIDs = append(cellIDs, cell.State().CellID)
		}

		Expect(cellIDs).To(Equal(expectedIDs))
	}

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		var err error
		workPool, err = workpool.NewWorkPool(5)
		Expect(err).NotTo(HaveOccurred())

		repA = new(repfakes.FakeSimClient)
		repB = new(repfakes.FakeSimClient)
		repC = new(repfakes.FakeSimClient)

		clients = map[string]rep.Client{
			"A": repA,
			"B": repB,
			"C": repC,
		}

		repA.StateReturns(BuildCellState("A", 0, "the-zone", 100, 200, 100, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0), nil)
		repB.StateReturns(BuildCellState("B", 2, "the-zone", 10, 10, 100, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0), nil)
		repC.StateReturns(BuildCellState("C", 5, "other-zone", 100, 10, 100, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0), nil)

		metricEmitter = new(fakes.FakeAuctionMetricEmitterDelegate)

		binPackFirstFitWeight = 0.0
	})

	AfterEach(func() {
		workPool.Stop()
	})

	It("fetches state by calling each client", func() {
		zones := auctionrunner.FetchStateAndBuildZones(logger, workPool, clients, metricEmitter, binPackFirstFitWeight)
		Expect(zones).To(HaveLen(2))

		cells := map[string]*auctionrunner.Cell{}
		for _, cell := range zones["the-zone"] {
			cells[cell.Guid] = cell
		}
		Expect(cells).To(HaveLen(2))
		Expect(cells).To(HaveKey("A"))
		Expect(cells).To(HaveKey("B"))

		Expect(repA.StateCallCount()).To(Equal(1))
		Expect(repB.StateCallCount()).To(Equal(1))

		otherZone := zones["other-zone"]
		Expect(otherZone).To(HaveLen(1))
		Expect(otherZone[0].Guid).To(Equal("C"))

		Expect(repC.StateCallCount()).To(Equal(1))
	})

	It("logs that it successfully fetched the state of the cells", func() {
		auctionrunner.FetchStateAndBuildZones(logger, workPool, clients, metricEmitter, binPackFirstFitWeight)

		Expect(logger.LogMessages()).To(ContainElement("test.fetched-cell-state"))
		Expect(logger.Logs()).To(ContainElement(IncludeLogData(lager.Data{"cell-guid": "A", "duration_ns": BeNumerically(">", 0)})))
		Expect(logger.Logs()).To(ContainElement(IncludeLogData(lager.Data{"cell-guid": "B", "duration_ns": BeNumerically(">", 0)})))
	})

	Context("when cells are evacuating", func() {
		BeforeEach(func() {
			repB.StateReturns(BuildCellState("B", 0, "the-zone", 10, 10, 100, true, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0), nil)
		})

		It("does not include them in the map", func() {
			zones := auctionrunner.FetchStateAndBuildZones(logger, workPool, clients, metricEmitter, binPackFirstFitWeight)
			Expect(zones).To(HaveLen(2))

			cells := zones["the-zone"]
			Expect(cells).To(HaveLen(1))
			Expect(cells[0].Guid).To(Equal("A"))

			cells = zones["other-zone"]
			Expect(cells).To(HaveLen(1))
			Expect(cells[0].Guid).To(Equal("C"))
		})

		It("logs that it ignored the evacuating cell", func() {
			auctionrunner.FetchStateAndBuildZones(logger, workPool, clients, metricEmitter, binPackFirstFitWeight)

			Expect(logger.LogMessages()).To(ContainElement("test.ignored-evacuating-cell"))
			Expect(logger.Logs()).To(ContainElement(IncludeLogData(lager.Data{"cell-guid": "B", "duration_ns": BeNumerically(">", 0)})))
		})
	})

	Context("when a cell ID does not match the cell state ID", func() {
		BeforeEach(func() {
			repB.StateReturns(BuildCellState("badCellID", 0, "the-zone", 10, 10, 100, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0), nil)
		})

		It("does not include that cell in the map", func() {
			zones := auctionrunner.FetchStateAndBuildZones(logger, workPool, clients, metricEmitter, binPackFirstFitWeight)
			Expect(zones).To(HaveLen(2))

			cells := zones["the-zone"]
			Expect(cells).To(HaveLen(1))
			Expect(cells[0].Guid).To(Equal("A"))

			cells = zones["other-zone"]
			Expect(cells).To(HaveLen(1))
			Expect(cells[0].Guid).To(Equal("C"))
		})

		Context("when the cell id is empty", func() {
			BeforeEach(func() {
				repB.StateReturns(BuildCellState("", 0, "the-zone", 10, 10, 100, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0), nil)
			})

			It("includes that cell in the map", func() {
				zones := auctionrunner.FetchStateAndBuildZones(logger, workPool, clients, metricEmitter, binPackFirstFitWeight)
				Expect(zones).To(HaveLen(2))

				cells := zones["the-zone"]
				guids := []string{}
				for _, cell := range cells {
					guids = append(guids, cell.Guid)
				}
				Expect(guids).To(ConsistOf("A", "B"))

				cells = zones["other-zone"]
				Expect(cells).To(HaveLen(1))
				Expect(cells[0].Guid).To(Equal("C"))
			})
		})

		It("logs that there was a cell ID mismatch", func() {
			auctionrunner.FetchStateAndBuildZones(logger, workPool, clients, metricEmitter, binPackFirstFitWeight)

			Expect(logger.LogMessages()).To(ContainElement("test.cell-id-mismatch"))
			Expect(logger.Logs()).To(ContainElement(IncludeLogData(lager.Data{"cell-guid": "B"})))
			Expect(logger.Logs()).To(ContainElement(IncludeLogData(lager.Data{"cell-state-guid": "badCellID", "duration_ns": BeNumerically(">", 0)})))
		})
	})

	Context("when a client fails", func() {
		BeforeEach(func() {
			repB.StateReturns(BuildCellState("B", 0, "the-zone", 10, 10, 100, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0), errors.New("boom"))
		})

		It("does not include the client in the map", func() {
			zones := auctionrunner.FetchStateAndBuildZones(logger, workPool, clients, metricEmitter, binPackFirstFitWeight)
			Expect(zones).To(HaveLen(2))

			cells := zones["the-zone"]
			Expect(cells).To(HaveLen(1))
			Expect(cells[0].Guid).To(Equal("A"))

			cells = zones["other-zone"]
			Expect(cells).To(HaveLen(1))
			Expect(cells[0].Guid).To(Equal("C"))
		})

		It("it emits metrics for the failure", func() {
			zones := auctionrunner.FetchStateAndBuildZones(logger, workPool, clients, metricEmitter, binPackFirstFitWeight)
			Expect(zones).To(HaveLen(2))
			Expect(metricEmitter.FailedCellStateRequestCallCount()).To(Equal(1))
		})

		It("logs that it failed to fetch cell state", func() {
			auctionrunner.FetchStateAndBuildZones(logger, workPool, clients, metricEmitter, binPackFirstFitWeight)

			Expect(logger.LogMessages()).To(ContainElement("test.failed-to-get-state"))
			Expect(logger.Logs()).To(ContainElement(IncludeLogData(lager.Data{"cell-guid": "B", "error": "boom", "duration_ns": BeNumerically(">", 0)})))
		})
	})

	Context("when clients are slow to respond", func() {
		BeforeEach(func() {
			repA.StateReturns(BuildCellState("A", 0, "the-zone", 10, 10, 100, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0), errors.New("timeout"))
			repA.StateClientTimeoutReturns(5 * time.Second)
			repA.SetStateClientStub = func(client *http.Client) {
				repA.StateClientTimeoutReturns(client.Timeout)
			}
			repB.StateReturns(BuildCellState("B", 0, "the-zone", 10, 10, 100, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0), errors.New("timeout"))
			repB.StateClientTimeoutReturns(2 * time.Second)
			repB.SetStateClientStub = func(client *http.Client) {
				repB.StateClientTimeoutReturns(client.Timeout)
			}
			repC.StateReturns(BuildCellState("C", 0, "the-zone", 10, 10, 100, false, 0, linuxOnlyRootFSProviders, nil, []string{}, []string{}, []string{}, 0), errors.New("timeout"))
			repC.StateClientTimeoutReturns(4 * time.Second)
			repC.SetStateClientStub = func(client *http.Client) {
				repC.StateClientTimeoutReturns(client.Timeout)
			}
		})
	})

	It("separately orders cells in each zone by cell index when bin pack first-fit weight is provided", func() {
		binPackFirstFitWeight := 1.0
		zones := auctionrunner.FetchStateAndBuildZones(logger, workPool, clients, metricEmitter, binPackFirstFitWeight)

		assertCellIndicesWithOrder(zones["the-zone"], []int{0, 1})
		assertCellIDs(zones["the-zone"], []string{"A", "B"})

		assertCellIndicesWithOrder(zones["other-zone"], []int{0})
		assertCellIDs(zones["other-zone"], []string{"C"})
	})

	It("keeps the original cell indices intact when bin pack first-fit weight is not provided", func() {
		zones := auctionrunner.FetchStateAndBuildZones(logger, workPool, clients, metricEmitter, binPackFirstFitWeight)

		assertCellIndices(zones["the-zone"], []int{0, 2})
		assertCellIndices(zones["other-zone"], []int{5})
	})
})
