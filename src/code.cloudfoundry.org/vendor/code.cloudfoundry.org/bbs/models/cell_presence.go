package models

import (
	"encoding/json"
	"strings"
)

type CellSet map[string]*CellPresence

func NewCellSet() CellSet {
	return make(CellSet)
}

func NewCellSetFromList(cells []*CellPresence) CellSet {
	cellSet := NewCellSet()
	for _, v := range cells {
		cellSet.Add(v)
	}
	return cellSet
}

func (set CellSet) Add(cell *CellPresence) {
	set[cell.CellId] = cell
}

func (set CellSet) Each(predicate func(cell *CellPresence)) {
	for _, cell := range set {
		predicate(cell)
	}
}

func (set CellSet) HasCellID(cellID string) bool {
	_, ok := set[cellID]
	return ok
}

func (set CellSet) CellIDs() []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	return keys
}

func NewCellCapacity(memoryMB, diskMB, containers int32) CellCapacity {
	return CellCapacity{
		MemoryMb:   memoryMB,
		DiskMb:     diskMB,
		Containers: containers,
	}
}

func (cap CellCapacity) Validate() error {
	var validationError ValidationError

	if cap.MemoryMb <= 0 {
		validationError = validationError.Append(ErrInvalidField{"memory_mb"})
	}

	if cap.DiskMb < 0 {
		validationError = validationError.Append(ErrInvalidField{"disk_mb"})
	}

	if cap.Containers <= 0 {
		validationError = validationError.Append(ErrInvalidField{"containers"})
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func NewCellPresence(
	cellID, repAddress, repUrl, zone string,
	capacity CellCapacity,
	rootFSProviders, preloadedRootFSes, extraRootFSes, placementTags, optionalPlacementTags []string,
	annotations map[string]string,
) CellPresence {
	var providers []*Provider
	providers = append(providers, &Provider{PreloadedRootFSScheme, preloadedRootFSes})
	providers = append(providers, &Provider{PreloadedOCIRootFSScheme, preloadedRootFSes})
	providers = append(providers, &Provider{ExtraRootFSScheme, extraRootFSes})

	for _, prov := range rootFSProviders {
		providers = append(providers, &Provider{prov, []string{}})
	}

	if annotations == nil {
		annotations = map[string]string{}
	}

	return CellPresence{
		CellId:                cellID,
		RepAddress:            repAddress,
		RepUrl:                repUrl,
		Zone:                  zone,
		Capacity:              &capacity,
		RootfsProviders:       providers,
		PlacementTags:         placementTags,
		OptionalPlacementTags: optionalPlacementTags,
		Annotations:           annotations,
	}
}

func (c CellPresence) Validate() error {
	var validationError ValidationError

	if c.CellId == "" {
		validationError = validationError.Append(ErrInvalidField{"cell_id"})
	}

	if c.RepAddress == "" {
		validationError = validationError.Append(ErrInvalidField{"rep_address"})
	}

	if c.RepUrl != "" && !strings.HasPrefix(c.RepUrl, "http://") && !strings.HasPrefix(c.RepUrl, "https://") {
		validationError = validationError.Append(ErrInvalidField{"rep_url"})
	}

	if err := c.Capacity.Validate(); err != nil {
		validationError = validationError.Append(err)
	}

	for k, v := range c.Annotations {
		if k == "" || v == "" {
			validationError = validationError.Append(ErrInvalidField{"annotations"})
			break
		}
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

const (
	EventTypeCellDisappeared = "cell_disappeared"
)

type CellEvent interface {
	EventType() string
	CellIDs() []string
}

type CellDisappearedEvent struct {
	IDs []string
}

func NewCellDisappearedEvent(ids []string) CellDisappearedEvent {
	return CellDisappearedEvent{ids}
}

func (CellDisappearedEvent) EventType() string {
	return EventTypeCellDisappeared
}

func (e CellDisappearedEvent) CellIDs() []string {
	return e.IDs
}

func (c *CellPresence) Copy() *CellPresence {
	newCellPresense := *c
	return &newCellPresense
}

func (c CellPresence) MarshalJSON() ([]byte, error) {
	if c.Annotations == nil {
		c.Annotations = map[string]string{}
	}
	type cellPresenceAlias CellPresence
	return json.Marshal(cellPresenceAlias(c))
}
