package workpool_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestWorkpool(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Workpool Suite")
}

func RangeArray(len int) []int {
	result := make([]int, len)
	for i := range result {
		result[i] = i
	}
	return result
}
