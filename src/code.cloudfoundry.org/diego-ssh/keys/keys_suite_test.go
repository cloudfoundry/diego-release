package keys_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestKeys(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Keys Suite")
}
