package healthcheck_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHealthcheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Healthcheck Suite")
}
