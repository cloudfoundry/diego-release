package gcrhelper_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGcrhelper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gcrhelper Suite")
}
