package signals_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSignals(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Signals Suite")
}
