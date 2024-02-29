package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/format"
)

const (
	ActualLRPStateUnclaimed = "UNCLAIMED"
	ActualLRPStateClaimed   = "CLAIMED"
	ActualLRPStateRunning   = "RUNNING"
	ActualLRPStateCrashed   = "CRASHED"

	CrashResetTimeout            = 5 * time.Minute
	RetireActualLRPRetryAttempts = 5
)

var ActualLRPStates = []string{
	ActualLRPStateUnclaimed,
	ActualLRPStateClaimed,
	ActualLRPStateRunning,
	ActualLRPStateCrashed,
}

// DEPRECATED
type ActualLRPChange struct {
	Before *ActualLRPGroup
	After  *ActualLRPGroup
}

type ActualLRPFilter struct {
	Domain      string
	CellID      string
	ProcessGuid string
	Index       *int32
}

func NewActualLRPKey(processGuid string, index int32, domain string) ActualLRPKey {
	return ActualLRPKey{processGuid, index, domain}
}

func NewActualLRPInstanceKey(instanceGuid string, cellId string) ActualLRPInstanceKey {
	return ActualLRPInstanceKey{instanceGuid, cellId}
}

func NewActualLRPNetInfo(address string, instanceAddress string, preferredAddress ActualLRPNetInfo_PreferredAddress, ports ...*PortMapping) ActualLRPNetInfo {
	return ActualLRPNetInfo{address, ports, instanceAddress, preferredAddress}
}

func EmptyActualLRPNetInfo() ActualLRPNetInfo {
	return NewActualLRPNetInfo("", "", ActualLRPNetInfo_PreferredAddressUnknown)
}

func (info ActualLRPNetInfo) Empty() bool {
	return info.Address == "" && len(info.Ports) == 0 && info.PreferredAddress == ActualLRPNetInfo_PreferredAddressUnknown
}

func (*ActualLRPNetInfo) Version() format.Version {
	return format.V0
}

func (d *ActualLRPNetInfo_PreferredAddress) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}

	if v, found := ActualLRPNetInfo_PreferredAddress_value[name]; found {
		*d = ActualLRPNetInfo_PreferredAddress(v)
		return nil
	}
	return fmt.Errorf("invalid preferred address: %s", name)
}

func (d ActualLRPNetInfo_PreferredAddress) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func NewPortMapping(hostPort, containerPort uint32) *PortMapping {
	return &PortMapping{
		HostPort:      hostPort,
		ContainerPort: containerPort,
	}
}

func NewPortMappingWithTLSProxy(hostPort, containerPort, tlsHost, tlsContainer uint32) *PortMapping {
	return &PortMapping{
		HostPort:              hostPort,
		ContainerPort:         containerPort,
		ContainerTlsProxyPort: tlsContainer,
		HostTlsProxyPort:      tlsHost,
	}
}

func (key ActualLRPInstanceKey) Empty() bool {
	return key.InstanceGuid == "" && key.CellId == ""
}
func (a *ActualLRP) Copy() *ActualLRP {
	newActualLRP := *a
	return &newActualLRP
}

const StaleUnclaimedActualLRPDuration = 30 * time.Second

func (actual ActualLRP) ShouldStartUnclaimed(now time.Time) bool {
	if actual.State != ActualLRPStateUnclaimed {
		return false
	}

	if now.Sub(time.Unix(0, actual.Since)) > StaleUnclaimedActualLRPDuration {
		return true
	}

	return false
}

func (actual ActualLRP) CellIsMissing(cellSet CellSet) bool {
	if actual.State == ActualLRPStateUnclaimed ||
		actual.State == ActualLRPStateCrashed {
		return false
	}

	return !cellSet.HasCellID(actual.CellId)
}

func (actual ActualLRP) ShouldRestartImmediately(calc RestartCalculator) bool {
	if actual.State != ActualLRPStateCrashed {
		return false
	}

	return calc.ShouldRestart(0, 0, actual.CrashCount)
}

func (actual ActualLRP) ShouldRestartCrash(now time.Time, calc RestartCalculator) bool {
	if actual.State != ActualLRPStateCrashed {
		return false
	}

	return calc.ShouldRestart(now.UnixNano(), actual.Since, actual.CrashCount)
}

func (actual *ActualLRP) SetRoutable(routable bool) {
	actual.OptionalRoutable = &ActualLRP_Routable{
		Routable: routable,
	}
}

func (actual *ActualLRP) RoutableExists() bool {
	_, ok := actual.GetOptionalRoutable().(*ActualLRP_Routable)
	return ok
}

func (before ActualLRP) AllowsTransitionTo(lrpKey *ActualLRPKey, instanceKey *ActualLRPInstanceKey, newState string) bool {
	if !before.ActualLRPKey.Equal(lrpKey) {
		return false
	}

	var valid bool
	switch before.State {
	case ActualLRPStateUnclaimed:
		valid = newState == ActualLRPStateUnclaimed ||
			newState == ActualLRPStateClaimed ||
			newState == ActualLRPStateRunning
	case ActualLRPStateClaimed:
		valid = newState == ActualLRPStateUnclaimed && instanceKey.Empty() ||
			newState == ActualLRPStateClaimed && before.ActualLRPInstanceKey.Equal(instanceKey) ||
			newState == ActualLRPStateRunning ||
			newState == ActualLRPStateCrashed && before.ActualLRPInstanceKey.Equal(instanceKey)
	case ActualLRPStateRunning:
		valid = newState == ActualLRPStateUnclaimed && instanceKey.Empty() ||
			newState == ActualLRPStateClaimed && before.ActualLRPInstanceKey.Equal(instanceKey) ||
			newState == ActualLRPStateRunning && before.ActualLRPInstanceKey.Equal(instanceKey) ||
			newState == ActualLRPStateCrashed && before.ActualLRPInstanceKey.Equal(instanceKey)
	case ActualLRPStateCrashed:
		valid = newState == ActualLRPStateUnclaimed && instanceKey.Empty() ||
			newState == ActualLRPStateClaimed && before.ActualLRPInstanceKey.Equal(instanceKey) ||
			newState == ActualLRPStateRunning && before.ActualLRPInstanceKey.Equal(instanceKey)
	}

	return valid
}

func (d *ActualLRP_Presence) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}

	if v, found := ActualLRP_Presence_value[name]; found {
		*d = ActualLRP_Presence(v)
		return nil
	}
	return fmt.Errorf("invalid presence: %s", name)
}

func (d ActualLRP_Presence) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// DEPRECATED
func NewRunningActualLRPGroup(actualLRP *ActualLRP) *ActualLRPGroup {
	return &ActualLRPGroup{
		Instance: actualLRP,
	}
}

// DEPRECATED
func NewEvacuatingActualLRPGroup(actualLRP *ActualLRP) *ActualLRPGroup {
	return &ActualLRPGroup{
		Evacuating: actualLRP,
	}
}

// DEPRECATED
func (group ActualLRPGroup) Resolve() (*ActualLRP, bool, error) {
	switch {
	case group.Instance == nil && group.Evacuating == nil:
		return nil, false, ErrActualLRPGroupInvalid

	case group.Instance == nil:
		return group.Evacuating, true, nil

	case group.Evacuating == nil:
		return group.Instance, false, nil

	case group.Instance.State == ActualLRPStateRunning || group.Instance.State == ActualLRPStateCrashed:
		return group.Instance, false, nil

	default:
		return group.Evacuating, true, nil
	}
}

func NewUnclaimedActualLRP(lrpKey ActualLRPKey, since int64) *ActualLRP {
	return &ActualLRP{
		ActualLRPKey: lrpKey,
		State:        ActualLRPStateUnclaimed,
		Since:        since,
	}
}

func NewClaimedActualLRP(lrpKey ActualLRPKey, instanceKey ActualLRPInstanceKey, since int64) *ActualLRP {
	return &ActualLRP{
		ActualLRPKey:         lrpKey,
		ActualLRPInstanceKey: instanceKey,
		State:                ActualLRPStateClaimed,
		Since:                since,
	}
}

func NewRunningActualLRP(lrpKey ActualLRPKey, instanceKey ActualLRPInstanceKey, netInfo ActualLRPNetInfo, since int64) *ActualLRP {
	return &ActualLRP{
		ActualLRPKey:         lrpKey,
		ActualLRPInstanceKey: instanceKey,
		ActualLRPNetInfo:     netInfo,
		State:                ActualLRPStateRunning,
		Since:                since,
	}
}

func (*ActualLRP) Version() format.Version {
	return format.V0
}

func (actualLRPInfo *ActualLRPInfo) ToActualLRP(lrpKey ActualLRPKey, lrpInstanceKey ActualLRPInstanceKey) *ActualLRP {
	if actualLRPInfo == nil {
		return nil
	}
	lrp := ActualLRP{
		ActualLRPKey:         lrpKey,
		ActualLRPInstanceKey: lrpInstanceKey,
		ActualLRPNetInfo:     actualLRPInfo.ActualLRPNetInfo,
		AvailabilityZone:     actualLRPInfo.AvailabilityZone,
		CrashCount:           actualLRPInfo.CrashCount,
		CrashReason:          actualLRPInfo.CrashReason,
		State:                actualLRPInfo.State,
		PlacementError:       actualLRPInfo.PlacementError,
		Since:                actualLRPInfo.Since,
		ModificationTag:      actualLRPInfo.ModificationTag,
		Presence:             actualLRPInfo.Presence,
	}

	if actualLRPInfo.RoutableExists() {
		lrp.SetRoutable(actualLRPInfo.GetRoutable())
	}

	return &lrp
}

func (actual *ActualLRP) ToActualLRPInfo() *ActualLRPInfo {
	if actual == nil {
		return nil
	}
	info := ActualLRPInfo{
		ActualLRPNetInfo: actual.ActualLRPNetInfo,
		AvailabilityZone: actual.AvailabilityZone,
		CrashCount:       actual.CrashCount,
		CrashReason:      actual.CrashReason,
		State:            actual.State,
		PlacementError:   actual.PlacementError,
		Since:            actual.Since,
		ModificationTag:  actual.ModificationTag,
		Presence:         actual.Presence,
	}

	if actual.RoutableExists() {
		info.SetRoutable(actual.GetRoutable())
	}
	return &info
}

// DEPRECATED
func (actual *ActualLRP) ToActualLRPGroup() *ActualLRPGroup {
	if actual == nil {
		return nil
	}

	switch actual.Presence {
	case ActualLRP_Evacuating:
		return &ActualLRPGroup{Evacuating: actual}
	default:
		return &ActualLRPGroup{Instance: actual}
	}
}

func (actual ActualLRP) Validate() error {
	var validationError ValidationError

	err := actual.ActualLRPKey.Validate()
	if err != nil {
		validationError = validationError.Append(err)
	}

	if actual.Since == 0 {
		validationError = validationError.Append(ErrInvalidField{"since"})
	}

	switch actual.State {
	case ActualLRPStateUnclaimed:
		if !actual.ActualLRPInstanceKey.Empty() {
			validationError = validationError.Append(errors.New("instance key cannot be set when state is unclaimed"))
		}
		if !actual.ActualLRPNetInfo.Empty() {
			validationError = validationError.Append(errors.New("net info cannot be set when state is unclaimed"))
		}
		if actual.Presence != ActualLRP_Ordinary {
			validationError = validationError.Append(errors.New("presence cannot be set when state is unclaimed"))
		}

	case ActualLRPStateClaimed:
		if err := actual.ActualLRPInstanceKey.Validate(); err != nil {
			validationError = validationError.Append(err)
		}
		if !actual.ActualLRPNetInfo.Empty() {
			validationError = validationError.Append(errors.New("net info cannot be set when state is claimed"))
		}
		if strings.TrimSpace(actual.PlacementError) != "" {
			validationError = validationError.Append(errors.New("placement error cannot be set when state is claimed"))
		}

	case ActualLRPStateRunning:
		if err := actual.ActualLRPInstanceKey.Validate(); err != nil {
			validationError = validationError.Append(err)
		}
		if err := actual.ActualLRPNetInfo.Validate(); err != nil {
			validationError = validationError.Append(err)
		}
		if strings.TrimSpace(actual.PlacementError) != "" {
			validationError = validationError.Append(errors.New("placement error cannot be set when state is running"))
		}

	case ActualLRPStateCrashed:
		if !actual.ActualLRPInstanceKey.Empty() {
			validationError = validationError.Append(errors.New("instance key cannot be set when state is crashed"))
		}
		if !actual.ActualLRPNetInfo.Empty() {
			validationError = validationError.Append(errors.New("net info cannot be set when state is crashed"))
		}
		if strings.TrimSpace(actual.PlacementError) != "" {
			validationError = validationError.Append(errors.New("placement error cannot be set when state is crashed"))
		}

	default:
		validationError = validationError.Append(ErrInvalidField{"state"})
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (key *ActualLRPKey) Validate() error {
	var validationError ValidationError

	if key.ProcessGuid == "" {
		validationError = validationError.Append(ErrInvalidField{"process_guid"})
	}

	if key.Index < 0 {
		validationError = validationError.Append(ErrInvalidField{"index"})
	}

	if key.Domain == "" {
		validationError = validationError.Append(ErrInvalidField{"domain"})
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

func (key *ActualLRPNetInfo) Validate() error {
	var validationError ValidationError

	if key.Address == "" {
		return validationError.Append(ErrInvalidField{"address"})
	}

	return nil
}

func (key *ActualLRPInstanceKey) Validate() error {
	var validationError ValidationError

	if key.CellId == "" {
		validationError = validationError.Append(ErrInvalidField{"cell_id"})
	}

	if key.InstanceGuid == "" {
		validationError = validationError.Append(ErrInvalidField{"instance_guid"})
	}

	if !validationError.Empty() {
		return validationError
	}

	return nil
}

// hasHigherPriority returns true if lrp1 takes precendence over lrp2
func hasHigherPriority(lrp1, lrp2 *ActualLRP) bool {
	if lrp1 == nil {
		return false
	}

	if lrp2 == nil {
		return true
	}

	if lrp1.Presence == ActualLRP_Ordinary {
		switch lrp1.State {
		case ActualLRPStateRunning:
			return true
		case ActualLRPStateClaimed:
			return lrp2.State != ActualLRPStateRunning && lrp2.State != ActualLRPStateClaimed
		}
	} else if lrp1.Presence == ActualLRP_Suspect {
		switch lrp1.State {
		case ActualLRPStateRunning:
			return lrp2.State != ActualLRPStateRunning
		case ActualLRPStateClaimed:
			return lrp2.State != ActualLRPStateRunning
		}
	}
	// Cases where we are comparing two LRPs with the same presence have undefined behavior since it shouldn't happen
	// with the way they're stored in the database
	return false
}

// DEPRECATED
// ResolveActualLRPGroups convert the given set of lrp instances into
// ActualLRPGroup.  This conversion is lossy.  A suspect LRP is given
// precendence over an Ordinary instance if it is Running.  Otherwise, the
// Ordinary instance is returned in the Instance field of the ActualLRPGroup.
func ResolveActualLRPGroups(lrps []*ActualLRP) []*ActualLRPGroup {
	mapOfGroups := map[ActualLRPKey]*ActualLRPGroup{}
	result := []*ActualLRPGroup{}
	for _, actualLRP := range lrps {
		// Every actual LRP has potentially 2 rows in the database: one for the instance
		// one for the evacuating.  When building the list of actual LRP groups (where
		// a group is the instance and corresponding evacuating), make sure we don't add the same
		// actual lrp twice.
		if mapOfGroups[actualLRP.ActualLRPKey] == nil {
			mapOfGroups[actualLRP.ActualLRPKey] = &ActualLRPGroup{}
			result = append(result, mapOfGroups[actualLRP.ActualLRPKey])
		}
		if actualLRP.Presence == ActualLRP_Evacuating {
			mapOfGroups[actualLRP.ActualLRPKey].Evacuating = actualLRP
		} else if hasHigherPriority(actualLRP, mapOfGroups[actualLRP.ActualLRPKey].Instance) {
			mapOfGroups[actualLRP.ActualLRPKey].Instance = actualLRP
		}
	}

	return result
}

// DEPRECATED
// ResolveToActualLRPGroup calls ResolveActualLRPGroups and return the first
// LRP group.  It panics if there are more than one group.  If there no LRP
// groups were returned by ResolveActualLRPGroups, then an empty ActualLRPGroup
// is returned.
func ResolveActualLRPGroup(lrps []*ActualLRP) *ActualLRPGroup {
	actualLRPGroups := ResolveActualLRPGroups(lrps)
	switch len(actualLRPGroups) {
	case 0:
		return &ActualLRPGroup{}
	case 1:
		return actualLRPGroups[0]
	default:
		panic("shouldn't get here")
	}
}
