package routehandlers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestRoutehandlers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Routehandlers Suite")
}
