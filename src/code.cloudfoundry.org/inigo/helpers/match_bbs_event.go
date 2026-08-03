package helpers

import (
	"fmt"

	"code.cloudfoundry.org/bbs/models"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

//

func MatchActualLRPCrashedEvent(processGuid, instanceGuid, cellId string, index int) gomega.OmegaMatcher {
	return &ActualLRPCrashedEventMatcher{
		ProcessGuid:  processGuid,
		Index:        index,
		InstanceGuid: instanceGuid,
		CellId:       cellId,
	}
}

type ActualLRPCrashedEventMatcher struct {
	ProcessGuid  string
	InstanceGuid string
	CellId       string
	Index        int
}

func (matcher *ActualLRPCrashedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(*models.ActualLRPCrashedEvent)
	if !ok {
		return false, nil
	}
	return event.ProcessGuid == matcher.ProcessGuid &&
		event.Index == int32(matcher.Index) &&
		event.CellId == matcher.CellId &&
		event.InstanceGuid == matcher.InstanceGuid, nil
}

func (matcher *ActualLRPCrashedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a ActualLRPCrashedEvent with\n  ProcessGuid=%s\n  Index=%d", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index)
}

func (matcher *ActualLRPCrashedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a ActualLRPCrashedEvent with\n  ProcessGuid=%s\n  Index=%d", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index)
}
