package test_helpers

import (
	"fmt"
	"math"
	strings "strings"

	"github.com/go-test/deep"
	"github.com/onsi/gomega"
)

func init() {
	deep.MaxDepth = math.MaxInt32
}

type DeepEqualMatcher struct {
	val interface{}
}

func DeepEqual(expected interface{}) DeepEqualMatcher {
	return DeepEqualMatcher{val: expected}
}

func (m DeepEqualMatcher) Match(actual interface{}) (success bool, err error) {
	diffs := deep.Equal(actual, m.val)
	return len(diffs) == 0, nil
}

func (m DeepEqualMatcher) FailureMessage(actual interface{}) (message string) {

	diffs := deep.Equal(actual, m.val)
	var messages []string
	for _, diff := range diffs {
		var object, comparison string

		split := strings.Split(diff, ": ")
		if len(split) == 1 {
			object = ""
			comparison = split[0]
		} else {
			object = split[0]
			comparison = split[1]
		}

		var expected, actual string
		actual = strings.Split(comparison, " != ")[0]
		expected = strings.Split(comparison, " != ")[1]

		message := gomega.Equal(expected).FailureMessage(actual)
		if object != "" {
			message = fmt.Sprintf("For %s,\n%s", object, message)
		}

		messages = append(messages, message)
	}

	return strings.Join(messages, "\n\n")
}

func (m DeepEqualMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return "no differences found"
}
