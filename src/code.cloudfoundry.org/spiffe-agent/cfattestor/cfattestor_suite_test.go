package cfattestor

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCfattestor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cfattestor Suite")
}
