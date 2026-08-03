package credhub_flags_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCredhubFlags(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CredhubFlags Suite")
}
