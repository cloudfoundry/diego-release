package matchers

import (
	"fmt"
	"reflect"
	"sort"

	"code.cloudfoundry.org/route-emitter/routingtable"

	"github.com/onsi/gomega/format"
)

func MatchMessagesToEmit(messages routingtable.MessagesToEmit) *messagesToEmitMatcher {
	return &messagesToEmitMatcher{
		expected: messages,
	}
}

type messagesToEmitMatcher struct {
	expected routingtable.MessagesToEmit
}

func (m *messagesToEmitMatcher) Match(a interface{}) (success bool, err error) {
	actual, ok := a.(routingtable.MessagesToEmit)
	if !ok {
		return false, fmt.Errorf("%s is not a routingtable.MessagesToEmit", format.Object(actual, 1))
	}

	registrationsMatch := m.matchArrs(actual.RegistrationMessages, m.expected.RegistrationMessages)
	unregistrationsMatch := m.matchArrs(actual.UnregistrationMessages, m.expected.UnregistrationMessages)
	internalRegistrationsMatch := m.matchArrs(actual.InternalRegistrationMessages, m.expected.InternalRegistrationMessages)
	internalUnregistrationsMatch := m.matchArrs(actual.InternalUnregistrationMessages, m.expected.InternalUnregistrationMessages)
	return registrationsMatch && unregistrationsMatch && internalRegistrationsMatch && internalUnregistrationsMatch, nil
}

func (m *messagesToEmitMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to match", m.expected)
}

func (m *messagesToEmitMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to match", m.expected)
}

func (m *messagesToEmitMatcher) matchArrs(actual, expected []routingtable.RegistryMessage) bool {
	if len(actual) != len(expected) {
		return false
	}

	fixedActual := []routingtable.RegistryMessage{}
	fixedExpected := []routingtable.RegistryMessage{}

	for _, message := range actual {
		sort.Sort(sort.StringSlice(message.URIs))
		fixedActual = append(fixedActual, message)
	}

	for _, message := range expected {
		sort.Sort(sort.StringSlice(message.URIs))
		fixedExpected = append(fixedExpected, message)
	}

	sort.Sort(ByMessage(fixedActual))
	sort.Sort(ByMessage(fixedExpected))

	for i := 0; i < len(fixedActual); i++ {
		if !reflect.DeepEqual(fixedActual[i], fixedExpected[i]) {
			return false
		}
	}

	return true
}

type ByMessage []routingtable.RegistryMessage

func (a ByMessage) Len() int           { return len(a) }
func (a ByMessage) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByMessage) Less(i, j int) bool { return fmt.Sprintf("%v", a[i]) < fmt.Sprintf("%v", a[j]) }
