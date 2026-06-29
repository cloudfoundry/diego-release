package workloadapi_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWorkloadAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "WorkloadAPI Suite")
}
