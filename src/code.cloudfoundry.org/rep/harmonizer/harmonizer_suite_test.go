package harmonizer_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHarmonizer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Harmonizer Suite")
}
