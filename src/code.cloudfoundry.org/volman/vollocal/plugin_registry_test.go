package vollocal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/volman/voldocker"
	. "code.cloudfoundry.org/volman/vollocal"

	"code.cloudfoundry.org/dockerdriver/dockerdriverfakes"
	"code.cloudfoundry.org/volman"
)

var _ = Describe("PluginRegistry", func() {
	var (
		emptyRegistry, oneRegistry, manyRegistry volman.PluginRegistry
	)

	BeforeEach(func() {
		emptyRegistry = NewPluginRegistry()

		oneRegistry = NewPluginRegistryWith(map[string]volman.Plugin{
			"one": voldocker.NewVolmanPluginWithDockerDriver(new(dockerdriverfakes.FakeDriver), volman.PluginSpec{}),
		})

		manyRegistry = NewPluginRegistryWith(map[string]volman.Plugin{
			"one": voldocker.NewVolmanPluginWithDockerDriver(new(dockerdriverfakes.FakeDriver), volman.PluginSpec{}),
			"two": voldocker.NewVolmanPluginWithDockerDriver(new(dockerdriverfakes.FakeDriver), volman.PluginSpec{}),
		})
	})

	Describe("#Plugin", func() {
		It("sets the plugin to new value", func() {
			onePlugin, exists := oneRegistry.Plugin("one")
			Expect(exists).To(BeTrue())
			Expect(onePlugin).NotTo(BeNil())
		})

		It("returns nil and false if the plugin doesn't exist", func() {
			onePlugin, exists := oneRegistry.Plugin("doesnotexist")
			Expect(exists).To(BeFalse())
			Expect(onePlugin).To(BeNil())
		})
	})

	Describe("#Plugins", func() {
		It("should return return empty map for emptyRegistry", func() {
			plugins := emptyRegistry.Plugins()
			Expect(len(plugins)).To(Equal(0))
		})

		It("should return return one driver for oneRegistry", func() {
			plugins := oneRegistry.Plugins()
			Expect(len(plugins)).To(Equal(1))
		})
	})

	Describe("#Set", func() {
		It("replaces plugin if it already exists", func() {
			newPlugin := map[string]volman.Plugin{
				"one": voldocker.NewVolmanPluginWithDockerDriver(new(dockerdriverfakes.FakeDriver), volman.PluginSpec{}),
			}
			oneRegistry.Set(newPlugin)
			onePlugin, exists := oneRegistry.Plugin("one")
			Expect(exists).To(BeTrue())
			Expect(onePlugin).NotTo(BeNil())
		})

		It("adds plugin that does not exists", func() {
			newPlugin := map[string]volman.Plugin{
				"one":   voldocker.NewVolmanPluginWithDockerDriver(new(dockerdriverfakes.FakeDriver), volman.PluginSpec{}),
				"two":   voldocker.NewVolmanPluginWithDockerDriver(new(dockerdriverfakes.FakeDriver), volman.PluginSpec{}),
				"three": voldocker.NewVolmanPluginWithDockerDriver(new(dockerdriverfakes.FakeDriver), volman.PluginSpec{}),
			}
			manyRegistry.Set(newPlugin)
			threePlugin, exists := manyRegistry.Plugin("three")
			Expect(exists).To(BeTrue())
			Expect(threePlugin).NotTo(BeNil())
		})
	})

	Describe("#Keys", func() {
		It("should return return {'one'} for oneRegistry keys", func() {
			keys := emptyRegistry.Keys()
			Expect(len(keys)).To(Equal(0))
		})

		It("should return return {'one'} for oneRegistry keys", func() {
			keys := oneRegistry.Keys()
			Expect(len(keys)).To(Equal(1))
			Expect(keys[0]).To(Equal("one"))
		})
	})
})
