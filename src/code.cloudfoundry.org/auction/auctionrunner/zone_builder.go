package auctionrunner

import (
	"sort"
	"sync"
	"time"

	"code.cloudfoundry.org/auction/auctiontypes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/workpool"
)

const MinBinPackFirstFitWeight = 0.0

func FetchStateAndBuildZones(logger lager.Logger, workPool *workpool.WorkPool, clients map[string]rep.Client, metricEmitter auctiontypes.AuctionMetricEmitterDelegate, binPackFirstFitWeight float64) map[string]Zone {
	var zones map[string]Zone
	for i := 0; ; i++ {
		zones = fetchStateAndBuildZones(logger, workPool, clients, metricEmitter, binPackFirstFitWeight)
		if len(zones) > 0 {
			break
		}
		if i == 3 {
			logger.Info("failed-to-communicate-to-cells-abort")
			break
		}
		logger.Info("failed-to-communicate-to-cells-retry")
	}
	return zones
}

func fetchStateAndBuildZones(logger lager.Logger, workPool *workpool.WorkPool, clients map[string]rep.Client, metricEmitter auctiontypes.AuctionMetricEmitterDelegate, binPackFirstFitWeight float64) map[string]Zone {
	wg := &sync.WaitGroup{}
	zones := map[string]Zone{}
	lock := &sync.Mutex{}

	wg.Add(len(clients))
	for guid, client := range clients {
		guid, client := guid, client
		workPool.Submit(func() {
			defer wg.Done()

			startTime := time.Now()
			state, err := client.State(logger)
			if err != nil {
				metricErr := metricEmitter.FailedCellStateRequest()
				if metricErr != nil {
					logger.Debug("failed-to-emit-get-cell-state-failure-metric", lager.Data{"error": err})
				}
				logger.Error("failed-to-get-state", err, lager.Data{"cell-guid": guid, "duration_ns": time.Since(startTime)})
				return
			}

			if state.Evacuating {
				logger.Info("ignored-evacuating-cell", lager.Data{"cell-guid": guid, "duration_ns": time.Since(startTime)})
				return
			}

			if state.CellID != "" && state.CellID != guid {
				logger.Error("cell-id-mismatch", nil, lager.Data{"cell-guid": guid, "cell-state-guid": state.CellID, "duration_ns": time.Since(startTime)})
				return
			}

			cell := NewCell(logger, guid, client, state)

			lock.Lock()
			zones[state.Zone] = append(zones[state.Zone], cell)
			lock.Unlock()
			logger.Debug("fetched-cell-state", lager.Data{"cell-guid": guid, "duration_ns": time.Since(startTime)})
		})
	}

	wg.Wait()

	if isBinPackFirstFitWeightProvided(binPackFirstFitWeight) {
		return normaliseCellIndices(zones)
	}

	return zones
}

func isBinPackFirstFitWeightProvided(binPackFirstFitWeight float64) bool {
	return binPackFirstFitWeight > MinBinPackFirstFitWeight
}

func normaliseCellIndices(zones map[string]Zone) map[string]Zone {
	for _, zone := range zones {
		sort.Sort(zone)

		for normalisedIndex, cell := range zone {
			cell.Index = normalisedIndex
		}
	}

	return zones
}
