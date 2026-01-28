package auctionrunner

import (
	"sort"

	"code.cloudfoundry.org/auction/auctiontypes"
)

type lrpByZone struct {
	zone      Zone
	instances int
}

type zoneSorterByInstances struct {
	zones []lrpByZone
}

func (s zoneSorterByInstances) Len() int      { return len(s.zones) }
func (s zoneSorterByInstances) Swap(i, j int) { s.zones[i], s.zones[j] = s.zones[j], s.zones[i] }
func (s zoneSorterByInstances) Less(i, j int) bool {
	return s.zones[i].instances < s.zones[j].instances
}

func accumulateZonesByInstances(zones map[string]Zone, processGuid string) []lrpByZone {
	lrpZones := []lrpByZone{}

	for _, zone := range zones {
		instances := 0
		for _, cell := range zone {
			for i := range cell.state.LRPs {
				if cell.state.LRPs[i].ProcessGuid == processGuid {
					instances++
				}
			}
		}
		lrpZones = append(lrpZones, lrpByZone{zone, instances})
	}

	return lrpZones
}

func sortZonesByInstances(zones []lrpByZone) []lrpByZone {
	sorter := zoneSorterByInstances{zones: zones}
	sort.Sort(sorter)
	return sorter.zones
}

func filterZones(zones []lrpByZone, lrpAuction *auctiontypes.LRPAuction) ([]lrpByZone, error) {
	filteredZones := []lrpByZone{}
	var zoneError error

	for _, lrpZone := range zones {
		cells, err := lrpZone.zone.filterCells(lrpAuction.PlacementConstraint)
		if err != nil {
			_, isZoneErrorPlacementTagMismatchError := zoneError.(auctiontypes.PlacementTagMismatchError)
			_, isErrPlacementTagMismatchError := err.(auctiontypes.PlacementTagMismatchError)

			if isZoneErrorPlacementTagMismatchError ||
				(zoneError == auctiontypes.ErrorVolumeDriverMismatch && isErrPlacementTagMismatchError) ||
				zoneError == auctiontypes.ErrorCellMismatch || zoneError == nil {
				zoneError = err
			}
			continue
		}

		filteredZone := lrpByZone{
			zone:      Zone(cells),
			instances: lrpZone.instances,
		}
		filteredZones = append(filteredZones, filteredZone)
	}

	if len(filteredZones) == 0 {
		return nil, zoneError
	}

	return filteredZones, nil
}
