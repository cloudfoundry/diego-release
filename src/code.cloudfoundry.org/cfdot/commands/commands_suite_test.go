package commands_test

import (
	"os"

	"code.cloudfoundry.org/cfdot/commands"
	"code.cloudfoundry.org/cfdot/commands/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

var _ = SynchronizedBeforeSuite(func() []byte {
	chmodErr := os.Chmod("fixtures/bbsClientBadPermissions.key", 0300)
	Expect(chmodErr).NotTo(HaveOccurred())

	return nil
}, func(_ []byte) {})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	chmodErr := os.Chmod("fixtures/bbsClientBadPermissions.key", 0644)
	Expect(chmodErr).NotTo(HaveOccurred())
})

var _ = BeforeEach(func() {
	commands.Config = helpers.TLSConfig{}
})

func TestCommands(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Commands Suite")
}

func removeFlag(flags map[string]string, toRemove string) []string {
	delete(flags, toRemove)

	return buildArgList(flags)
}

func replaceFlagValue(flags map[string]string, key string, newValue string) []string {
	flags[key] = newValue

	return buildArgList(flags)
}

func mergeFlags(flags map[string]string, moreFlags map[string]string) []string {
	for k, v := range moreFlags {
		flags[k] = v
	}
	return buildArgList(flags)
}

func buildArgList(flags map[string]string) []string {
	list := []string{}

	for key, value := range flags {
		list = append(list, key+"="+value)
	}

	return list
}
