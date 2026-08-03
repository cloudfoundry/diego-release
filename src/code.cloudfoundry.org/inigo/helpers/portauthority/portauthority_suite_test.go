package portauthority_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPortauthority(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Portauthority Suite")
}
