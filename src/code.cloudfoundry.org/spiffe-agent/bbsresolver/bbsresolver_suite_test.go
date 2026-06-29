package bbsresolver_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBbsresolver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bbsresolver Suite")
}
