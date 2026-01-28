package visualization

import (
	"sort"
	"sync"
	"time"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/workpool"
	"github.com/GaryBoone/GoStats/stats"
)

type Report struct {
	Cells                        map[string]rep.Client
	NumAuctions                  int
	AuctionResults               auctiontypes.AuctionResults
	AuctionDuration              time.Duration
	CellStates                   map[string]rep.CellState
	InstancesByRep               map[string][]rep.LRP
	auctionedInstancesByInstGuid map[string]bool
}

type Stat struct {
	Min    float64
	Max    float64
	Mean   float64
	StdDev float64
	Total  float64
}

func NewStat(data []float64) Stat {
	return Stat{
		Min:    stats.StatsMin(data),
		Max:    stats.StatsMax(data),
		Mean:   stats.StatsMean(data),
		StdDev: stats.StatsPopulationStandardDeviation(data),
		Total:  stats.StatsSum(data),
	}
}

func NewReport(numAuctions int, cells map[string]rep.Client, results auctiontypes.AuctionResults, duration time.Duration) *Report {
	states := fetchStates(cells)
	return &Report{
		Cells:           cells,
		NumAuctions:     numAuctions,
		AuctionResults:  results,
		AuctionDuration: duration,
		CellStates:      states,
		InstancesByRep:  instancesByRepFromStates(states),
	}
}

func (r *Report) IsAuctionedInstance(inst rep.LRP) bool {
	if r.auctionedInstancesByInstGuid == nil {
		r.auctionedInstancesByInstGuid = map[string]bool{}
		for _, result := range r.AuctionResults.SuccessfulLRPs {
			r.auctionedInstancesByInstGuid[result.Identifier()] = true
		}
	}

	return r.auctionedInstancesByInstGuid[inst.Identifier()]
}

func (r *Report) AuctionsPerformed() int {
	return len(r.AuctionResults.SuccessfulLRPs) + len(r.AuctionResults.FailedLRPs)
}

func (r *Report) NReps() int {
	return len(r.Cells)
}

func (r *Report) NMissingInstances() int {
	return r.NumAuctions - len(r.AuctionResults.SuccessfulLRPs)
}

func (r *Report) InitialDistributionScore() float64 {
	memoryCounts := []float64{}
	for _, instances := range r.InstancesByRep {
		memoryCount := 0.0
		for _, instance := range instances {
			if !r.IsAuctionedInstance(instance) {
				memoryCount += float64(instance.MemoryMB)
			}
		}
		memoryCounts = append(memoryCounts, memoryCount)
	}

	if stats.StatsSum(memoryCounts) == 0 {
		return 0
	}

	return stats.StatsPopulationStandardDeviation(memoryCounts) / stats.StatsMean(memoryCounts)
}

func (r *Report) DistributionScore() float64 {
	memoryCounts := []float64{}
	for _, instances := range r.InstancesByRep {
		memoryCount := 0.0
		for _, instance := range instances {
			memoryCount += float64(instance.MemoryMB)
		}
		memoryCounts = append(memoryCounts, memoryCount)
	}

	return stats.StatsPopulationStandardDeviation(memoryCounts) / stats.StatsMean(memoryCounts)
}

func (r *Report) AuctionsPerSecond() float64 {
	return float64(r.AuctionsPerformed()) / r.AuctionDuration.Seconds()
}

func (r *Report) WaitTimeStats() Stat {
	waitTimes := []float64{}
	for _, result := range r.AuctionResults.SuccessfulLRPs {
		waitTimes = append(waitTimes, result.WaitDuration.Seconds())
	}

	return NewStat(waitTimes)
}

func fetchStates(cells map[string]rep.Client) map[string]rep.CellState {
	logger := lager.NewLogger("fetch-states")
	lock := &sync.Mutex{}
	states := map[string]rep.CellState{}
	works := []func(){}

	for repGuid, cell := range cells {
		repGuid := repGuid
		cell := cell
		works = append(works, func() {
			state, _ := cell.State(logger)
			lock.Lock()
			states[repGuid] = state
			lock.Unlock()
		})
	}

	throttler, err := workpool.NewThrottler(500, works)
	if err != nil {
		panic(err) // should never happen
	}

	throttler.Work()

	return states
}

func instancesByRepFromStates(states map[string]rep.CellState) map[string][]rep.LRP {
	instancesByRepGuid := map[string][]rep.LRP{}
	for repGuid, state := range states {
		instances := state.LRPs
		sort.Sort(ByProcessGuid(instances))
		instancesByRepGuid[repGuid] = instances
	}

	return instancesByRepGuid
}

type ByProcessGuid []rep.LRP

func (a ByProcessGuid) Len() int           { return len(a) }
func (a ByProcessGuid) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByProcessGuid) Less(i, j int) bool { return a[i].ProcessGuid < a[j].ProcessGuid }
