package calculator

import (
	"sort"

	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"
)

// LRP Instance Constraints:
// - an ActualLRP is identified by its ActualLRPKey + ActualLRPInstanceKey
// - as long as these don't change, any change in presence or state will result in a changed event
// - an ActualLRP transitioning from Unclaimed -> Claimed will result in a
//   changed event (this is not exactly an exception since an ActualLRPInstanceKey
//   doesn't have a defined identity by this constraint)
// - an ActualLRP cannot transition directly from having an
//   ActualLRPInstanceKey to having none (i.e. Running -> Unclaimed). This must
//   result in a removed and a created event.
//
// LRP Group emissions:
// - If the slot changes from nil to something, that's an ActualLRPCreated event
// - If the slot changes from something to nil, that's an ActualLRPRemoved event
// - If the Instance slot changes from one ActualLRPInstanceKey to another,
//   it's considered an ActualLRPCreated and ActualLRPRemoved event
// - If the instance slot does not change the ActualLRPInstanceKey, it's
//   considered an ActualLRPChanged event (including the transition from Running ->
//   Unclaimed, e.g. when the LRP crashes)
// - If the instance slot changes from Unclaimed to another state, it's still considered a changed event
//
// General:
//  - We should emit Crashed events followed by Create or Changed events where
//    the resulting LRP is in the running state before Remove events
//
// Events that follow are instance events. LRP group events go through a separate algorithm.
//
// ClaimActualLRP
//  Changes Allowed:
//      - Unclaimed -> Claimed (Changed event)
//      - Running -> Claimed (Changed event)
//      - Crashed -> Claimed (Changed event)
//      - Claimed -> Claimed (No event)
//
//  Transition is only allowed if LRPInstanceKey will be the same or if the LRP
//  is Unclaimed. No case this will result in any other event.
//
// UnclaimActualLRP
//  Changes Allowed:
//      Not Unclaimed -> Unclaimed (Should emit removed and created event)
//
// StartActualLRP
//  Changes Allowed:
//      nil -> Running (Created event)
//      Unclaimed -> Running (if instanceKey matches) (changed event)
//      Claimed -> Running (if instanceKey matches) (changed event)
//      Running -> Running (if instanceKey matches) (changed event) (Only allowed if netInfo has changed)
//
//      if the lrp being started is suspect:
//          do nothing
//      if suspect exists and it is not the lrp being started:
//          emit removed event for the suspect LRP
//
// CrashActualLRP
//  Changes Allowed:
//      Claimed -> Crashed (if instanceKey matches) (changed event)
//      Running -> Crashed (if instanceKey matches) (changed event)
//
//      Claimed -> Unclaimed  (if instanceKey matches) (if crashedCount is below) (created + removed event)
//      Running -> Unclaimed  (if instanceKey matches) (if crashCount is below) (created + removed event)
//
// FailActualLRP
//  Unclaimed -> Unclaimed (no events emitted)
//
// RemoveActualLRP
//  removed event

type ActualLRPEventCalculator struct {
	// Deprecated: use ActualLRPInstanceHub instead
	ActualLRPGroupHub    events.Hub
	ActualLRPInstanceHub events.Hub
}

// EmitCrashEvents emits only the events for a crash scenario. Specifically:
//   - A CrashEvent each for the Group Hub and the Instance Hub
//   - A ChangedEvent for the Group Hub
//   - Either a ChangedEvent or a Removed and Created Event for the Instance Hub, depending on the state
//     of the ActualLRP
//
// This function was added to work around a bug in the existing logic for EmitEvents, where CrashedEvents
// were not being emitted due to the CrashResetTimeout
func (e ActualLRPEventCalculator) EmitCrashEvents(traceId string, beforeSet, afterSet []*models.ActualLRP) {
	beforeGroup := models.ResolveActualLRPGroup(beforeSet)
	afterGroup := models.ResolveActualLRPGroup(removeNilLRPs(afterSet))

	groupEvents := []models.Event{}
	if !beforeGroup.Instance.Equal(afterGroup.Instance) {
		if afterGroup.Instance == nil {
			//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
			groupEvents = append(groupEvents, models.NewActualLRPRemovedEvent(beforeGroup.Instance.ToActualLRPGroup()))
		} else if beforeGroup.Instance == nil {
			//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
			groupEvents = append(groupEvents, models.NewActualLRPCreatedEvent(afterGroup.Instance.ToActualLRPGroup()))
		} else {
			groupEvents = append(groupEvents,
				models.NewActualLRPCrashedEvent(beforeGroup.Instance, afterGroup.Instance),
				//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
				models.NewActualLRPChangedEvent(beforeGroup, afterGroup),
			)
		}
	}

	if !beforeGroup.Evacuating.Equal(afterGroup.Evacuating) {
		if afterGroup.Evacuating == nil {
			//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
			groupEvents = append(groupEvents, models.NewActualLRPRemovedEvent(beforeGroup.Evacuating.ToActualLRPGroup()))
		} else if beforeGroup.Evacuating == nil {
			//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
			groupEvents = append(groupEvents, models.NewActualLRPCreatedEvent(afterGroup.Evacuating.ToActualLRPGroup()))
		} else {
			groupEvents = append(groupEvents,
				models.NewActualLRPCrashedEvent(beforeGroup.Evacuating, afterGroup.Evacuating),
				//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
				models.NewActualLRPChangedEvent(beforeGroup, afterGroup),
			)
		}
	}

	sort.Slice(groupEvents, func(i, j int) bool {
		return EventScore(groupEvents[i]) > EventScore(groupEvents[j])
	})

	for _, ev := range groupEvents {
		e.ActualLRPGroupHub.Emit(ev)
	}

	instanceEvents := []models.Event{}
	for i := range afterSet {
		before := beforeSet[i]
		after := afterSet[i]

		if before.Equal(after) {
			continue
		}

		if after == nil {
			instanceEvents = append(instanceEvents,
				models.NewActualLRPInstanceRemovedEvent(before, traceId),
			)
		} else if before == nil {
			instanceEvents = append(instanceEvents,
				models.NewActualLRPInstanceCreatedEvent(after, traceId),
			)
		} else if after.State == models.ActualLRPStateCrashed {
			instanceEvents = append(instanceEvents,
				models.NewActualLRPCrashedEvent(before, after),
				models.NewActualLRPInstanceChangedEvent(before, after, traceId),
			)
		} else {
			instanceEvents = append(instanceEvents,
				models.NewActualLRPCrashedEvent(before, after),
				models.NewActualLRPInstanceCreatedEvent(after, traceId),
				models.NewActualLRPInstanceRemovedEvent(before, traceId),
			)
		}
	}

	sort.Slice(instanceEvents, func(i, j int) bool {
		return EventScore(instanceEvents[i]) > EventScore(instanceEvents[j])
	})

	for _, ev := range instanceEvents {
		e.ActualLRPInstanceHub.Emit(ev)
	}
}

// EmitEvents emits the events such as when the changes identified in the
// events are applied to the beforeSet the resulting state is equal to
// afterSet.  The beforeSet and afterSet are assumed to have the same process
// guid and index.
func (e ActualLRPEventCalculator) EmitEvents(traceId string, beforeSet, afterSet []*models.ActualLRP) {
	events := []models.Event{}

	beforeGroup := models.ResolveActualLRPGroup(beforeSet)
	afterGroup := models.ResolveActualLRPGroup(removeNilLRPs(afterSet))

	for _, ev := range generateLRPGroupEvents(beforeGroup, afterGroup) {
		e.ActualLRPGroupHub.Emit(ev)
	}

	// stretch the two slices to be of equal size.  make sure we do this after
	// emitting the group events, otherwise ResolveActualLRPGroup will panic if
	// it encounters nil lrps.
	stretchSlice(&beforeSet, &afterSet)

	for i := range afterSet {
		events = append(events, generateLRPInstanceEvents(beforeSet[i], afterSet[i], traceId)...)
	}

	sort.Slice(events, func(i, j int) bool {
		return EventScore(events[i]) > EventScore(events[j])
	})

	for _, ev := range events {
		e.ActualLRPInstanceHub.Emit(ev)
	}
}

// RecordChange returns a new LRP set with the before LRP replaced with after
// LRP.  The index of after and before is the same.  New LRPs (i.e. when before
// is nil) are appended to the end of the lrp slice.
func (e ActualLRPEventCalculator) RecordChange(before, after *models.ActualLRP, lrps []*models.ActualLRP) []*models.ActualLRP {
	found := false
	newLRPs := []*models.ActualLRP{}
	for _, l := range lrps {
		if l == nil {
			// this entry is recording a LRP removal, just skip it
			newLRPs = append(newLRPs, nil)
			continue
		}

		if before != nil && l.ActualLRPInstanceKey.Equal(before.ActualLRPInstanceKey) {
			newLRPs = append(newLRPs, after)
			found = true
		} else {
			newLRPs = append(newLRPs, l)
		}
	}

	if !found {
		newLRPs = append(newLRPs, after)
	}

	return newLRPs
}

func generateCrashedInstanceEvents(before, after *models.ActualLRP, traceId string) []models.Event {
	return wrapEvent(
		models.NewActualLRPCrashedEvent(before, after),
		models.NewActualLRPInstanceChangedEvent(before, after, traceId),
	)
}

func generateUpdateInstanceEvents(before, after *models.ActualLRP, traceId string) []models.Event {
	return wrapEvent(
		models.NewActualLRPInstanceChangedEvent(before, after, traceId),
	)
}

func generateUnclaimedInstanceEvents(before, after *models.ActualLRP, traceId string) []models.Event {
	events := []models.Event{}

	// we can get here if auctioneer calls FailActualLRP
	if before.State == models.ActualLRPStateUnclaimed {
		return append(events, models.NewActualLRPInstanceChangedEvent(before, after, traceId))
	}

	return append(
		events,
		models.NewActualLRPInstanceCreatedEvent(after, traceId),
		models.NewActualLRPInstanceRemovedEvent(before, traceId),
	)
}

func generateLRPInstanceEvents(before, after *models.ActualLRP, traceId string) []models.Event {
	if before.Equal(after) {
		// nothing changed
		return nil
	}

	if after == nil {
		return wrapEvent(models.NewActualLRPInstanceRemovedEvent(before, traceId))
	}

	if before == nil {
		return wrapEvent(models.NewActualLRPInstanceCreatedEvent(after, traceId))
	}

	switch after.State {
	case models.ActualLRPStateUnclaimed:
		return generateUnclaimedInstanceEvents(before, after, traceId)
	case models.ActualLRPStateClaimed:
		return generateUpdateInstanceEvents(before, after, traceId)
	case models.ActualLRPStateRunning:
		return generateUpdateInstanceEvents(before, after, traceId)
	case models.ActualLRPStateCrashed:
		return generateCrashedInstanceEvents(before, after, traceId)
	default:
		return nil
	}
}

func generateCrashedGroupEvents(before, after *models.ActualLRP) []models.Event {
	return wrapEvent(
		models.NewActualLRPCrashedEvent(before, after),
		//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
		models.NewActualLRPChangedEvent(before.ToActualLRPGroup(), after.ToActualLRPGroup()),
	)
}

func generateUnclaimedGroupEvents(before, after *models.ActualLRP) []models.Event {
	events := []models.Event{}

	return append(
		events,
		//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
		models.NewActualLRPChangedEvent(before.ToActualLRPGroup(), after.ToActualLRPGroup()),
	)
}

func generateUpdateGroupEvents(before, after *models.ActualLRP) []models.Event {
	if !before.ActualLRPInstanceKey.Empty() &&
		!after.ActualLRPInstanceKey.Equal(before.ActualLRPInstanceKey) {
		// an Ordinary LRP replaced Suspect LRP
		return wrapEvent(
			//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
			models.NewActualLRPCreatedEvent(after.ToActualLRPGroup()),
			//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
			models.NewActualLRPRemovedEvent(before.ToActualLRPGroup()),
		)
	}

	//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
	return wrapEvent(models.NewActualLRPChangedEvent(before.ToActualLRPGroup(), after.ToActualLRPGroup()))
}

// The main difference between this function and generateLRPInstanceEvents
// (besides using different event types) is that the latter generates a
// remove+create events when the LRP is unclaimed.  This function return a
// ActualLRPChangedEvent instead to be compatible with old subscribers.
func generateLRPInstanceGroupEvents(before, after *models.ActualLRP) []models.Event {
	if before.Equal(after) {
		// nothing changed
		return nil
	}

	if after == nil {
		//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
		return wrapEvent(models.NewActualLRPRemovedEvent(before.ToActualLRPGroup()))
	}

	if before == nil {
		//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
		return wrapEvent(models.NewActualLRPCreatedEvent(after.ToActualLRPGroup()))
	}

	switch after.State {
	case models.ActualLRPStateUnclaimed:
		return generateUnclaimedGroupEvents(before, after)
	case models.ActualLRPStateClaimed:
		return generateUpdateGroupEvents(before, after)
	case models.ActualLRPStateRunning:
		return generateUpdateGroupEvents(before, after)
	case models.ActualLRPStateCrashed:
		return generateCrashedGroupEvents(before, after)
	default:
		return nil
	}
}

// return the resulting lrp of the given event, that is the lrp being created
// or the lrp in the new lrp in a ActualLRPChanged event.  Returns nil for
// crashed and removed events.  Returns true iff this is a crashed event.
func getEventLRP(e models.Event) (*models.ActualLRP, bool) {
	switch x := e.(type) {
	//lint:ignore SA1019 - need to support this event until the deprecation becomes deletion
	case *models.ActualLRPCreatedEvent:
		//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
		lrp, _, _ := x.ActualLrpGroup.Resolve()
		return lrp, false
	//lint:ignore SA1019 - need to support this event until the deprecation becomes deletion
	case *models.ActualLRPChangedEvent:
		//lint:ignore SA1019 - still need to emit these events until the ActaulLRPGroup api is deleted
		lrp, _, _ := x.After.Resolve()
		return lrp, false
	case *models.ActualLRPInstanceCreatedEvent:
		return x.ActualLrp, false
	case *models.ActualLRPInstanceChangedEvent:
		return x.After.ToActualLRP(x.ActualLRPKey, x.ActualLRPInstanceKey), false
	case *models.ActualLRPCrashedEvent:
		return nil, true
	}

	return nil, false
}

// Determine the score of an event. An event with higher score should be
// emitted before lower ones.  The score based ordering ensures continuous
// routability, so events with running instances should be emitted first
// followed by remove events.
func EventScore(e models.Event) int {
	lrp, crashed := getEventLRP(e)

	// sort crashed events first to be backward compatible with the old
	// event stream which emitted the crashed event before the
	// remove/changed events.
	if crashed {
		return 2
	}

	// this is an event with a running instance, this should be emitted before
	// any other event, such as removed ro changed event to non-running state.
	if lrp != nil && lrp.State == models.ActualLRPStateRunning {
		return 1
	}

	// The event is either a RemovedEvent or a ChangedEvent (to a non-RUNNING
	// state).  These are prioritized last, because those events cause loss of
	// routability.
	return 0
}

//lint:ignore SA1019 - deprecated model used in support of deprecated code until it gets deleted
func generateLRPGroupEvents(before, after *models.ActualLRPGroup) []models.Event {
	events := generateLRPInstanceGroupEvents(before.Instance, after.Instance)
	events = append(events, generateLRPInstanceGroupEvents(before.Evacuating, after.Evacuating)...)

	sort.Slice(events, func(i, j int) bool {
		return EventScore(events[i]) > EventScore(events[j])
	})

	return events
}

// A Helper function to remove null lrps that could be added to the set if an
// LRP is removed.
func removeNilLRPs(lrps []*models.ActualLRP) []*models.ActualLRP {
	newLRPs := []*models.ActualLRP{}
	for _, l := range lrps {
		if l == nil {
			continue
		}
		newLRPs = append(newLRPs, l)
	}
	return newLRPs
}

func stretchSlice(before, after *[]*models.ActualLRP) {
	if len(*before) < len(*after) {
		newLRPs := make([]*models.ActualLRP, len(*after))
		copy(newLRPs, *before)
		*before = newLRPs
	}
}

func wrapEvent(e ...models.Event) []models.Event {
	return e
}
