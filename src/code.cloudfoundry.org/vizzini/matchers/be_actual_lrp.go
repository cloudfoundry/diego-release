package matchers

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/bbs/models"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

const NoCrashCount = -1
const AtLeastOneCrashCount = -2

func BeActualLRP(processGuid string, index int) gomega.OmegaMatcher {
	return &BeActualLRPMatcher{
		ProcessGuid: processGuid,
		Index:       index,
		CrashCount:  NoCrashCount,
	}
}

func BeUnclaimedActualLRPWithPlacementError(processGuid string, index int) gomega.OmegaMatcher {
	return &BeActualLRPMatcher{
		ProcessGuid:       processGuid,
		Index:             index,
		CrashCount:        NoCrashCount,
		State:             models.ActualLRPStateUnclaimed,
		HasPlacementError: true,
	}
}

func BeActualLRPWithState(processGuid string, index int, state string) gomega.OmegaMatcher {
	return &BeActualLRPMatcher{
		ProcessGuid: processGuid,
		Index:       index,
		State:       state,
		CrashCount:  NoCrashCount,
	}
}

func BeActualLRPThatHasCrashed(processGuid string, index int) gomega.OmegaMatcher {
	return &BeActualLRPMatcher{
		ProcessGuid: processGuid,
		Index:       index,
		CrashCount:  AtLeastOneCrashCount,
	}
}

func BeActualLRPWithCrashCount(processGuid string, index int, crashCount int) gomega.OmegaMatcher {
	return &BeActualLRPMatcher{
		ProcessGuid: processGuid,
		Index:       index,
		CrashCount:  crashCount,
	}
}

func BeActualLRPWithStateAndCrashCount(processGuid string, index int, state string, crashCount int) gomega.OmegaMatcher {
	return &BeActualLRPMatcher{
		ProcessGuid: processGuid,
		Index:       index,
		State:       state,
		CrashCount:  crashCount,
	}
}

type BeActualLRPMatcher struct {
	ProcessGuid       string
	Index             int
	State             string
	CrashCount        int
	HasPlacementError bool
}

func (matcher *BeActualLRPMatcher) Match(actual interface{}) (success bool, err error) {
	lrp, ok := actual.(models.ActualLRP)
	if !ok {
		return false, fmt.Errorf("BeActualLRP matcher expects a models.ActualLRP.  Got:\n%s", format.Object(actual, 1))
	}

	matchesState := true
	if matcher.State != "" {
		matchesState = matcher.State == lrp.State
	}
	matchesCrashCount := true
	if matcher.CrashCount != NoCrashCount {
		if matcher.CrashCount == AtLeastOneCrashCount {
			matchesCrashCount = lrp.CrashCount > 0
		} else {
			matchesCrashCount = matcher.CrashCount == int(lrp.CrashCount)
		}
	}
	matchesPlacementErrorRequirement := true
	if matcher.HasPlacementError {
		matchesPlacementErrorRequirement = lrp.PlacementError != ""
	}

	return matchesPlacementErrorRequirement && matchesState && matchesCrashCount && lrp.ActualLrpKey.ProcessGuid == matcher.ProcessGuid && int(lrp.ActualLrpKey.Index) == matcher.Index, nil
}

func (matcher *BeActualLRPMatcher) expectedContents() string {
	expectedContents := []string{
		fmt.Sprintf("ProcessGuid: %s", matcher.ProcessGuid),
		fmt.Sprintf("Index: %d", matcher.Index),
	}
	if matcher.State != "" {
		expectedContents = append(expectedContents, fmt.Sprintf("State: %s", matcher.State))
	}
	if matcher.CrashCount != NoCrashCount {
		if matcher.CrashCount == AtLeastOneCrashCount {
			expectedContents = append(expectedContents, "CrashCount: > 0")
		} else {
			expectedContents = append(expectedContents, fmt.Sprintf("CrashCount: %d", matcher.CrashCount))
		}
	}
	if matcher.HasPlacementError {
		expectedContents = append(expectedContents, "PlacementError Exists")
	}

	return strings.Join(expectedContents, "\n")
}

func (matcher *BeActualLRPMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto have:\n%s", format.Object(actual, 1), matcher.expectedContents())
}

func (matcher *BeActualLRPMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to have:\n%s", format.Object(actual, 1), matcher.expectedContents())
}
