package containerpath_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestContainerpath(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Containerpath Suite")
}
