package matchers

import (
	"fmt"

	"code.cloudfoundry.org/bbs/models"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

func HaveTaskState(state models.Task_State) gomega.OmegaMatcher {
	return &HaveTaskStateMatcher{
		State: state,
	}
}

type HaveTaskStateMatcher struct {
	State models.Task_State
}

func (matcher *HaveTaskStateMatcher) Match(actual interface{}) (success bool, err error) {
	task, ok := actual.(*models.Task)
	if !ok {
		return false, fmt.Errorf("HaveTaskState matcher expects a *models.Task.  Got:\n%s", format.Object(actual, 1))
	}

	return task.GetState() == matcher.State, nil
}

func (matcher *HaveTaskStateMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto have state %s", format.Object(actual, 1), matcher.State)
}

func (matcher *HaveTaskStateMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to have state %s", format.Object(actual, 1), matcher.State)
}
