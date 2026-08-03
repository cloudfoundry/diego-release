package operationq_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestOperationq(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Operationq Suite")
}
