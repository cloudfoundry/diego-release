package certauthority_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCertauthority(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Certauthority Suite")
}
