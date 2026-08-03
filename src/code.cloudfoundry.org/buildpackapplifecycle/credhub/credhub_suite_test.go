package credhub_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCredhub(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Credhub Suite")
}
