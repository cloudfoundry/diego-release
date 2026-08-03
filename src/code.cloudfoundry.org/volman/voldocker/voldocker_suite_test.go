package voldocker_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestVoldocker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Voldocker Suite")
}
