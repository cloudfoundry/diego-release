package evacuation_context_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestEvacuationContext(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EvacuationContext Suite")
}
