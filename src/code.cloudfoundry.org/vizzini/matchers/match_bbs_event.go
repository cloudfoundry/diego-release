package matchers

import (
	"fmt"

	"code.cloudfoundry.org/bbs/models"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

func MatchDesiredLRPCreatedEvent(processGuid string) gomega.OmegaMatcher {
	return &DesiredLRPCreatedEventMatcher{
		ProcessGuid: processGuid,
	}
}

type DesiredLRPCreatedEventMatcher struct {
	ProcessGuid string
}

func (matcher *DesiredLRPCreatedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(*models.DesiredLRPCreatedEvent)
	if !ok {
		return false, fmt.Errorf("DesiredLRPCreatedEventMatcher matcher expects a models.DesiredLRPCreatedEvent.  Got:\n%s", format.Object(actual, 1))
	}
	return event.DesiredLrp.ProcessGuid == matcher.ProcessGuid, nil
}

func (matcher *DesiredLRPCreatedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a DesiredLRPCreatedEvent with\n  ProcessGuid=%s", format.Object(actual, 1), matcher.ProcessGuid)
}

func (matcher *DesiredLRPCreatedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a DesiredLRPCreatedEvent with\n  ProcessGuid=%s", format.Object(actual, 1), matcher.ProcessGuid)
}

//

func MatchDesiredLRPChangedEvent(processGuid string) gomega.OmegaMatcher {
	return &DesiredLRPChangedEventMatcher{
		ProcessGuid: processGuid,
	}
}

type DesiredLRPChangedEventMatcher struct {
	ProcessGuid string
}

func (matcher *DesiredLRPChangedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(*models.DesiredLRPChangedEvent)
	if !ok {
		return false, fmt.Errorf("DesiredLRPChangedEventMatcher matcher expects a models.DesiredLRPChangedEvent.  Got:\n%s", format.Object(actual, 1))
	}

	return event.After.ProcessGuid == matcher.ProcessGuid, nil
}

func (matcher *DesiredLRPChangedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a DesiredLRPChangedEvent with\n  ProcessGuid=%s\n", format.Object(actual, 1), matcher.ProcessGuid)
}

func (matcher *DesiredLRPChangedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a DesiredLRPChangedEvent with\n  ProcessGuid=%s\n", format.Object(actual, 1), matcher.ProcessGuid)
}

//

func MatchDesiredLRPRemovedEvent(processGuid string) gomega.OmegaMatcher {
	return &DesiredLRPRemovedEventMatcher{
		ProcessGuid: processGuid,
	}
}

type DesiredLRPRemovedEventMatcher struct {
	ProcessGuid string
}

func (matcher *DesiredLRPRemovedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(*models.DesiredLRPRemovedEvent)
	if !ok {
		return false, fmt.Errorf("DesiredLRPRemovedEventMatcher matcher expects a models.DesiredLRPRemovedEvent.  Got:\n%s", format.Object(actual, 1))
	}
	return event.DesiredLrp.ProcessGuid == matcher.ProcessGuid, nil
}

func (matcher *DesiredLRPRemovedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a DesiredLRPRemovedEvent with\n  ProcessGuid=%s", format.Object(actual, 1), matcher.ProcessGuid)
}

func (matcher *DesiredLRPRemovedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a DesiredLRPRemovedEvent with\n  ProcessGuid=%s", format.Object(actual, 1), matcher.ProcessGuid)
}

//

func MatchActualLRPInstanceCreatedEvent(processGuid string, index int) gomega.OmegaMatcher {
	return &ActualLRPInstanceCreatedEventMatcher{
		ProcessGuid: processGuid,
		Index:       index,
	}
}

type ActualLRPInstanceCreatedEventMatcher struct {
	ProcessGuid string
	Index       int
}

func (matcher *ActualLRPInstanceCreatedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(*models.ActualLRPInstanceCreatedEvent)
	if !ok {
		return false, fmt.Errorf("ActualLRPInstanceCreatedEventMatcher matcher expects a models.ActualLRPInstanceCreatedEvent.  Got:\n%s", format.Object(actual, 1))
	}
	actualLRP := event.ActualLrp
	return actualLRP.ProcessGuid == matcher.ProcessGuid && actualLRP.Index == int32(matcher.Index), nil
}

func (matcher *ActualLRPInstanceCreatedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a ActualLRPInstanceCreatedEvent with\n  ProcessGuid=%s\n  Index=%d", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index)
}

func (matcher *ActualLRPInstanceCreatedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a ActualLRPInstanceCreatedEvent with\n  ProcessGuid=%s\n  Index=%d", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index)
}

//

func MatchActualLRPInstanceChangedEvent(processGuid string, index int, state string) gomega.OmegaMatcher {
	return &ActualLRPInstanceChangedEventMatcher{
		ProcessGuid: processGuid,
		Index:       index,
		State:       state,
	}
}

type ActualLRPInstanceChangedEventMatcher struct {
	ProcessGuid string
	Index       int
	State       string
}

func (matcher *ActualLRPInstanceChangedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(*models.ActualLRPInstanceChangedEvent)
	if !ok {
		return false, fmt.Errorf("ActualLRPInstanceChangedEventMatcher matcher expects a models.ActualLRPInstanceChangedEvent.  Got:\n%s", format.Object(actual, 1))
	}

	actualLRP := event.ActualLRPKey
	if err != nil {
		return false, err
	}
	return actualLRP.ProcessGuid == matcher.ProcessGuid && actualLRP.Index == int32(matcher.Index) && event.After.State == matcher.State, nil
}

func (matcher *ActualLRPInstanceChangedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a ActualLRPInstanceChangedEvent with\n  ProcessGuid=%s\n  Index=%d\n  State=%s", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index, matcher.State)
}

func (matcher *ActualLRPInstanceChangedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a ActualLRPInstanceChangedEvent with\n  ProcessGuid=%s\n  Index=%d\n  State=%s", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index, matcher.State)
}

//

func MatchActualLRPInstanceRemovedEvent(processGuid string, index int) gomega.OmegaMatcher {
	return &ActualLRPInstanceRemovedEventMatcher{
		ProcessGuid: processGuid,
		Index:       index,
	}
}

type ActualLRPInstanceRemovedEventMatcher struct {
	ProcessGuid string
	Index       int
}

func (matcher *ActualLRPInstanceRemovedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(*models.ActualLRPInstanceRemovedEvent)
	if !ok {
		return false, fmt.Errorf("ActualLRPInstanceRemovedEventMatcher matcher expects a models.ActualLRPInstanceRemovedEvent.  Got:\n%s", format.Object(actual, 1))
	}
	actualLRP := event.ActualLrp
	return actualLRP.ProcessGuid == matcher.ProcessGuid && actualLRP.Index == int32(matcher.Index), nil
}

func (matcher *ActualLRPInstanceRemovedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a ActualLRPInstanceRemovedEvent with\n  ProcessGuid=%s\n  Index=%d", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index)
}

func (matcher *ActualLRPInstanceRemovedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a ActualLRPInstanceRemovedEvent with\n  ProcessGuid=%s\n  Index=%d", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index)
}
