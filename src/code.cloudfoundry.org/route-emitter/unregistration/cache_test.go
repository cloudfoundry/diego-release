package unregistration_test

import (
	"sync"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/unregistration"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cache", func() {
	var (
		cache                              unregistration.Cache
		registryMessage1, registryMessage2 routingtable.RegistryMessage
		logger                             lager.Logger
	)

	BeforeEach(func() {
		endpoint1 := routingtable.Endpoint{
			InstanceGUID:  "instance-guid-1",
			Host:          "1.1.1.1",
			Port:          61001,
			ContainerPort: 11,
		}
		endpoint2 := routingtable.Endpoint{
			InstanceGUID:  "instance-guid-2",
			Host:          "2.2.2.2",
			Port:          61002,
			ContainerPort: 22,
		}

		route1 := routingtable.Route{
			Hostname: "host-1.example.com",
		}
		route2 := routingtable.Route{
			Hostname: "host-2.example.com",
		}

		logger = lagertest.NewTestLogger("test")
		cache = unregistration.NewCache(logger)

		registryMessage1 = routingtable.RegistryMessageFor(endpoint1, route1, false)
		registryMessage2 = routingtable.RegistryMessageFor(endpoint2, route2, false)
	})

	Describe("Add", func() {
		It("adds a message", func() {
			err := cache.Add([]routingtable.RegistryMessage{
				registryMessage1,
				registryMessage2,
			})
			Expect(err).NotTo(HaveOccurred())
			cachedMessages := cache.List()
			Expect(cachedMessages).To(HaveLen(2))
			Expect([]routingtable.RegistryMessage{
				cachedMessages[0].RegistryMessage,
				cachedMessages[1].RegistryMessage,
			}).To(ConsistOf(
				registryMessage1,
				registryMessage2,
			))
		})

		It("uses only the relevant ip/port fields in the cache key", func() {
			registryMessage2 = registryMessage1
			registryMessage2.App = "some-unique-app-id"
			registryMessage2.PrivateInstanceId = "some-unique-instance-id"
			registryMessage2.PrivateInstanceIndex = "some-unique-instance-index"
			registryMessage2.ServerCertDomainSAN = "some-unique-cert-domain-san"
			registryMessage2.IsolationSegment = "some-unique-isolation-segment"
			registryMessage2.EndpointUpdatedAtNs = 999
			registryMessage2.RouteServiceUrl = "some-unique-route-service-url"
			registryMessage2.Tags = map[string]string{"foo": "some-unique-tag"}

			err := cache.Add([]routingtable.RegistryMessage{
				registryMessage1,
				registryMessage2,
			})
			Expect(err).NotTo(HaveOccurred())
			cachedMessages := cache.List()
			Expect(cachedMessages).To(HaveLen(1))
		})
	})

	Describe("Remove", func() {
		It("removes a message", func() {
			err := cache.Add([]routingtable.RegistryMessage{
				registryMessage1,
				registryMessage2,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(cache.List()).To(HaveLen(2))
			err = cache.Remove([]routingtable.RegistryMessage{
				registryMessage1,
			})
			Expect(err).NotTo(HaveOccurred())
			cachedMessages := cache.List()
			Expect(cachedMessages).To(HaveLen(1))
			Expect([]routingtable.RegistryMessage{
				cachedMessages[0].RegistryMessage,
			}).To(ConsistOf(
				registryMessage2,
			))
		})

		It("uses only the relevant ip/port fields in the cache key", func() {
			registryMessage2 = registryMessage1
			registryMessage2.App = "some-unique-app-id"
			registryMessage2.PrivateInstanceId = "some-unique-instance-id"
			registryMessage2.PrivateInstanceIndex = "some-unique-instance-index"
			registryMessage2.ServerCertDomainSAN = "some-unique-cert-domain-san"
			registryMessage2.IsolationSegment = "some-unique-isolation-segment"
			registryMessage2.EndpointUpdatedAtNs = 999
			registryMessage2.RouteServiceUrl = "some-unique-route-service-url"
			registryMessage2.Tags = map[string]string{"foo": "some-unique-tag"}

			err := cache.Add([]routingtable.RegistryMessage{
				registryMessage1,
			})
			Expect(err).NotTo(HaveOccurred())
			err = cache.Remove([]routingtable.RegistryMessage{
				registryMessage2,
			})
			Expect(err).NotTo(HaveOccurred())
			cachedMessages := cache.List()
			Expect(cachedMessages).To(HaveLen(0))
		})
	})

	Describe("concurrent cache access", func() {
		It("does not cause a data race", func() {
			registryMessages := []routingtable.RegistryMessage{registryMessage1}

			var wg sync.WaitGroup
			wg.Add(3)
			go func() {
				defer wg.Done()
				for i := 0; i < 50; i++ {
					err := cache.Add(registryMessages)
					Expect(err).NotTo(HaveOccurred())
				}
			}()
			go func() {
				defer wg.Done()
				for i := 0; i < 50; i++ {
					err := cache.Remove(registryMessages)
					Expect(err).NotTo(HaveOccurred())
				}
			}()
			go func() {
				defer wg.Done()
				for i := 0; i < 50; i++ {
					cache.List()
				}
			}()
			wg.Wait()
		})
	})
})
