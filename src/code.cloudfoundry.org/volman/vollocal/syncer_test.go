package vollocal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/v3/lagertest"

	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	. "code.cloudfoundry.org/volman/vollocal"
	"code.cloudfoundry.org/volman/volmanfakes"
	"github.com/tedsuo/ifrit"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"

	"code.cloudfoundry.org/volman"
)

var _ = Describe("Syncer", func() {
	var (
		syncer          *Syncer
		registry        volman.PluginRegistry
		logger          *lagertest.TestLogger
		scanInterval    time.Duration
		fakeClock       *fakeclock.FakeClock
		fakeDiscoverer1 *volmanfakes.FakeDiscoverer
		fakeDiscoverer2 *volmanfakes.FakeDiscoverer
		fakeDiscoverer3 *volmanfakes.FakeDiscoverer
		process         ifrit.Process
	)

	BeforeEach(func() {
		scanInterval = 10 * time.Second
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))

		logger = lagertest.NewTestLogger("plugin-syncer-test")

		registry = NewPluginRegistry()
	})

	JustBeforeEach(func() {
		syncer = NewSyncer(logger, registry, []volman.Discoverer{fakeDiscoverer1, fakeDiscoverer2, fakeDiscoverer3}, scanInterval, fakeClock)
	})

	Describe("#Runner", func() {
		It("has a non-nil runner", func() {
			Expect(syncer.Runner()).NotTo(BeNil())
		})

		It("has a non-nil and empty driver registry", func() {
			Expect(registry).NotTo(BeNil())
			Expect(len(registry.Plugins())).To(Equal(0))
		})
	})

	Describe("#Run", func() {

		JustBeforeEach(func() {
			process = ginkgomon.Invoke(syncer.Runner())
		})

		AfterEach(func() {
			ginkgomon.Kill(process)
		})

		Context("given three discoverers", func() {
			BeforeEach(func() {
				fakeDiscoverer1 = &volmanfakes.FakeDiscoverer{}
				fakeDiscoverer2 = &volmanfakes.FakeDiscoverer{}
				fakeDiscoverer3 = &volmanfakes.FakeDiscoverer{}
			})

			Context("given each discoverer returns a plugin", func() {
				BeforeEach(func() {
					fakePluginDiscovered1 := map[string]volman.Plugin{"plugin1": &volmanfakes.FakePlugin{}}
					fakeDiscoverer1.DiscoverReturns(fakePluginDiscovered1, nil)
				})

				It("should add both to the registry", func() {
					Expect(registry).NotTo(BeNil())
					Expect(len(registry.Plugins())).To(Equal(1))
				})

			})

			Context("given plugins are added over time", func() {
				It("should discover each new plugin", func() {
					Eventually(registry.Plugins).Should(HaveLen(0))

					fakePluginDiscovered1 := map[string]volman.Plugin{"plugin1": &volmanfakes.FakePlugin{}}
					fakeDiscoverer1.DiscoverReturns(fakePluginDiscovered1, nil)
					fakeClock.Increment(scanInterval + 1)
					Eventually(registry.Plugins).Should(HaveLen(1))

					fakePluginDiscovered2 := map[string]volman.Plugin{"plugin2": &volmanfakes.FakePlugin{}}
					fakeDiscoverer2.DiscoverReturns(fakePluginDiscovered2, nil)
					fakeClock.Increment(scanInterval + 1)
					Eventually(registry.Plugins).Should(HaveLen(2))

					fakePluginDiscovered3 := map[string]volman.Plugin{"plugin3": &volmanfakes.FakePlugin{}}
					fakeDiscoverer3.DiscoverReturns(fakePluginDiscovered3, nil)
					fakeClock.Increment(scanInterval + 1)
					Eventually(registry.Plugins).Should(HaveLen(3))

					Expect(fakeDiscoverer1.DiscoverCallCount()).To(Equal(4))
					Expect(fakeDiscoverer2.DiscoverCallCount()).To(Equal(4))
					Expect(fakeDiscoverer3.DiscoverCallCount()).To(Equal(4))
				})
			})

			Context("given plugins are removed over time", func() {
				BeforeEach(func() {
					fakePluginDiscovered1 := map[string]volman.Plugin{"plugin1": &volmanfakes.FakePlugin{}}
					fakeDiscoverer1.DiscoverReturns(fakePluginDiscovered1, nil)

					fakePluginDiscovered2 := map[string]volman.Plugin{"plugin2": &volmanfakes.FakePlugin{}}
					fakeDiscoverer2.DiscoverReturns(fakePluginDiscovered2, nil)

					fakePluginDiscovered3 := map[string]volman.Plugin{"plugin3": &volmanfakes.FakePlugin{}}
					fakeDiscoverer3.DiscoverReturns(fakePluginDiscovered3, nil)
				})

				It("should remove the plugins", func() {
					Eventually(registry.Plugins).Should(HaveLen(3))

					fakePluginDiscovered1 := map[string]volman.Plugin{}
					fakeDiscoverer1.DiscoverReturns(fakePluginDiscovered1, nil)
					fakeClock.Increment(scanInterval + 1)
					Eventually(registry.Plugins).Should(HaveLen(2))

					fakePluginDiscovered2 := map[string]volman.Plugin{}
					fakeDiscoverer2.DiscoverReturns(fakePluginDiscovered2, nil)
					fakeClock.Increment(scanInterval + 1)
					Eventually(registry.Plugins).Should(HaveLen(1))

					fakePluginDiscovered3 := map[string]volman.Plugin{}
					fakeDiscoverer3.DiscoverReturns(fakePluginDiscovered3, nil)
					fakeClock.Increment(scanInterval + 1)
					Eventually(registry.Plugins).Should(HaveLen(0))

					Expect(fakeDiscoverer1.DiscoverCallCount()).To(Equal(4))
					Expect(fakeDiscoverer2.DiscoverCallCount()).To(Equal(4))
					Expect(fakeDiscoverer3.DiscoverCallCount()).To(Equal(4))
				})
			})
		})
	})
})
