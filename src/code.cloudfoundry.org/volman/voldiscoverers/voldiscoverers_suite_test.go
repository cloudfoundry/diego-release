package voldiscoverers_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

var defaultPluginsDirectory string
var secondPluginsDirectory string

var _ = BeforeEach(func() {
	var err error

	defaultPluginsDirectory, err = os.MkdirTemp(os.TempDir(), "clienttest")
	Expect(err).ShouldNot(HaveOccurred())

	secondPluginsDirectory, err = os.MkdirTemp(os.TempDir(), "clienttest2")
	Expect(err).ShouldNot(HaveOccurred())
})

func TestVoldiscoverers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Voldiscoverers Suite")
}
