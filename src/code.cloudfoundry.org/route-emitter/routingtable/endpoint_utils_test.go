package routingtable_test

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/routing-info/tcp_routes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LRP Utils", func() {
	Describe("Hash", func() {
		var (
			routeA routingtable.Route
			routeB routingtable.Route
			routeC routingtable.Route
		)
		BeforeEach(func() {
			routeA = routingtable.Route{
				Hostname:         "routeA.com",
				RouteServiceUrl:  "routeA-service-url",
				IsolationSegment: "routeA-iso-seg",
				LogGUID:          "routeA-log-guid",
				Protocol:         "protocolA",
			}
			routeB = routingtable.Route{
				Hostname:         "routeA.com",
				RouteServiceUrl:  "routeA-service-url",
				IsolationSegment: "routeA-iso-seg",
				LogGUID:          "routeA-log-guid",
				Protocol:         "protocolA",
			}
			routeC = routingtable.Route{
				Hostname:         "routeA.com",
				RouteServiceUrl:  "routeA-service-url",
				IsolationSegment: "routeC-iso-seg",
				LogGUID:          "routeA-log-guid",
				Protocol:         "protocolC",
			}
		})

		It("uniquely identifies a route", func() {
			hashA := routeA.Hash()
			hashB := routeB.Hash()
			hashC := routeC.Hash()

			Expect(hashA).To(Equal(hashB))
			Expect(hashA).NotTo(Equal(hashC))
		})
	})

	Describe("NewEndpointsFromActual", func() {
		Context("when actual is not evacuating", func() {
			It("builds a map of container port to endpoint", func() {
				tag := models.ModificationTag{Epoch: "abc", Index: 0}
				actualInfo := &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						"1.1.1.1",
						"2.2.2.2",
						models.ActualLRPNetInfo_PreferredAddressHost,
						models.NewPortMapping(11, 44),
						models.NewPortMapping(66, 99),
					),
					Presence:         models.ActualLRP_Ordinary,
					State:            models.ActualLRPStateRunning,
					ModificationTag:  tag,
					AvailabilityZone: "some-zone",
				}

				endpoints := routingtable.NewEndpointsFromActual(actualInfo)

				Expect(endpoints).To(ConsistOf([]routingtable.Endpoint{
					routingtable.NewEndpoint("instance-guid", models.ActualLRP_Ordinary, "1.1.1.1", "2.2.2.2", 11, 44, models.ActualLRPNetInfo_PreferredAddressHost, &tag, "some-zone"),
					routingtable.NewEndpoint("instance-guid", models.ActualLRP_Ordinary, "1.1.1.1", "2.2.2.2", 66, 99, models.ActualLRPNetInfo_PreferredAddressHost, &tag, "some-zone"),
				}))
			})

			Context("with TLS proxy ports", func() {
				It("builds a map of tls proxy ports to endpoints", func() {
					tag := models.ModificationTag{Epoch: "abc", Index: 0}
					actualInfo := &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"1.1.1.1",
							"2.2.2.2",
							models.ActualLRPNetInfo_PreferredAddressInstance,
							models.NewPortMappingWithTLSProxy(11, 44, 61004, 61005),
							models.NewPortMappingWithTLSProxy(66, 99, 61006, 61007),
						),
						Presence:         models.ActualLRP_Ordinary,
						State:            models.ActualLRPStateRunning,
						ModificationTag:  tag,
						AvailabilityZone: "some-zone",
					}

					endpoints := routingtable.NewEndpointsFromActual(actualInfo)

					Expect(endpoints).To(ConsistOf([]routingtable.Endpoint{
						newEndpointWithTlsProxyPort("instance-guid", models.ActualLRP_Ordinary, "1.1.1.1", "2.2.2.2", 11, 44, 61004, 61005, models.ActualLRPNetInfo_PreferredAddressInstance, &tag, "some-zone"),
						newEndpointWithTlsProxyPort("instance-guid", models.ActualLRP_Ordinary, "1.1.1.1", "2.2.2.2", 66, 99, 61006, 61007, models.ActualLRPNetInfo_PreferredAddressInstance, &tag, "some-zone"),
					}))
				})
			})
		})

		Context("when actual has an IPv6 address", func() {
			It("populates ContainerIPv6 on the endpoint", func() {
				tag := models.ModificationTag{Epoch: "abc", Index: 0}
				actualInfo := &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
					ActualLRPNetInfo: models.NewActualLRPNetInfoWithIPv6(
						"1.1.1.1",
						"2.2.2.2",
						"fd00::1",
						models.ActualLRPNetInfo_PreferredAddressHost,
						models.NewPortMapping(11, 44),
					),
					Presence:         models.ActualLRP_Ordinary,
					State:            models.ActualLRPStateRunning,
					ModificationTag:  tag,
					AvailabilityZone: "some-zone",
				}

				endpoints := routingtable.NewEndpointsFromActual(actualInfo)

				Expect(endpoints).To(HaveLen(1))
				Expect(endpoints[0].ContainerIPv6).To(Equal("fd00::1"))
			})
		})

		Context("when actual is evacuating", func() {
			It("builds a map of container port to endpoint", func() {
				tag := models.ModificationTag{Epoch: "abc", Index: 0}

				actualInfo := &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						"1.1.1.1",
						"2.2.2.2",
						models.ActualLRPNetInfo_PreferredAddressHost,
						models.NewPortMapping(11, 44),
						models.NewPortMapping(66, 99),
					),
					Presence:         models.ActualLRP_Evacuating,
					State:            models.ActualLRPStateRunning,
					ModificationTag:  tag,
					AvailabilityZone: "some-zone",
				}

				endpoints := routingtable.NewEndpointsFromActual(actualInfo)

				Expect(endpoints).To(ConsistOf([]routingtable.Endpoint{
					routingtable.NewEndpoint("instance-guid", models.ActualLRP_Evacuating, "1.1.1.1", "2.2.2.2", 11, 44, models.ActualLRPNetInfo_PreferredAddressHost, &tag, "some-zone"),
					routingtable.NewEndpoint("instance-guid", models.ActualLRP_Evacuating, "1.1.1.1", "2.2.2.2", 66, 99, models.ActualLRPNetInfo_PreferredAddressHost, &tag, "some-zone"),
				}))
			})
		})
	})

	Describe("NewRoutingKeysFromActual", func() {
		It("creates a list of keys for an actual LRP", func() {
			keys := routingtable.NewRoutingKeysFromActual(&models.ActualLRP{
				ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
				ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
				ActualLRPNetInfo: models.NewActualLRPNetInfo(
					"1.1.1.1",
					"2.2.2.2",
					models.ActualLRPNetInfo_PreferredAddressHost,
					models.NewPortMapping(11, 44),
					models.NewPortMapping(66, 99),
				),
				State: models.ActualLRPStateRunning,
			})

			Expect(keys).To(HaveLen(2))
			Expect(keys).To(ContainElement(routingtable.NewRoutingKey("process-guid", 44)))
			Expect(keys).To(ContainElement(routingtable.NewRoutingKey("process-guid", 99)))
		})

		Context("when the actual lrp has tls proxy ports", func() {
			It("creates a list of keys for an actual LRP", func() {
				keys := routingtable.NewRoutingKeysFromActual(&models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						"1.1.1.1",
						"2.2.2.2",
						models.ActualLRPNetInfo_PreferredAddressHost,
						models.NewPortMappingWithTLSProxy(11, 44, 61004, 61005),
						models.NewPortMappingWithTLSProxy(66, 99, 61006, 61007),
					),
					State: models.ActualLRPStateRunning,
				})

				Expect(keys).To(HaveLen(4))
				Expect(keys).To(ContainElement(routingtable.NewRoutingKey("process-guid", 44)))
				Expect(keys).To(ContainElement(routingtable.NewRoutingKey("process-guid", 99)))
				Expect(keys).To(ContainElement(routingtable.NewRoutingKey("process-guid", 61005)))
				Expect(keys).To(ContainElement(routingtable.NewRoutingKey("process-guid", 61007)))
			})
		})

		Context("when the actual lrp has no port mappings", func() {
			It("returns no keys", func() {
				keys := routingtable.NewRoutingKeysFromActual(&models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
					ActualLRPNetInfo: models.NewActualLRPNetInfo(
						"1.1.1.1",
						"2.2.2.2",
						models.ActualLRPNetInfo_PreferredAddressHost,
					),
					State: models.ActualLRPStateRunning,
				})

				Expect(keys).To(HaveLen(0))
			})
		})
	})

	Describe("NewRoutingKeysFromDesired", func() {
		It("creates a list of keys for an actual LRP", func() {
			routes := tcp_routes.TCPRoutes{
				{ExternalPort: 61000, ContainerPort: 8080},
				{ExternalPort: 61001, ContainerPort: 9090},
			}

			desired := &models.DesiredLRP{
				Domain:      "tests",
				ProcessGuid: "process-guid",
				Ports:       []uint32{8080, 9090},
				Routes:      routes.RoutingInfo(),
				LogGuid:     "abc-guid",
			}

			keys := routingtable.NewRoutingKeysFromDesired(desired)

			Expect(keys).To(HaveLen(2))
			Expect(keys).To(ContainElement(routingtable.NewRoutingKey("process-guid", 8080)))
			Expect(keys).To(ContainElement(routingtable.NewRoutingKey("process-guid", 9090)))
		})

		Context("when the desired LRP does not define any container ports", func() {
			It("returns no keys", func() {
				routes := tcp_routes.TCPRoutes{}

				desired := &models.DesiredLRP{
					Domain:      "tests",
					ProcessGuid: "process-guid",
					Routes:      routes.RoutingInfo(),
					LogGuid:     "abc-guid",
				}

				keys := routingtable.NewRoutingKeysFromDesired(desired)
				Expect(keys).To(HaveLen(0))
			})
		})
	})

	Describe("MessageFor", func() {
		var (
			endpoint             routingtable.Endpoint
			externalEndpointInfo routingtable.ExternalEndpointInfo
		)

		BeforeEach(func() {
			endpoint = routingtable.Endpoint{
				InstanceGUID:          "instance-guid",
				Host:                  "host",
				Port:                  1234,
				ContainerIP:           "container-ip",
				ContainerPort:         5678,
				TlsProxyPort:          61001,
				ContainerTlsProxyPort: 61002,
			}
			externalEndpointInfo = routingtable.ExternalEndpointInfo{
				RouterGroupGUID:      "router-group-guid",
				Port:                 8080,
				TLSEnabled:           true,
				SniHostname:          "sni-hostname",
				TerminateFrontendTLS: true,
			}
		})

		It("returns a TcpRouteMapping with the expected fields", func() {
			_, mapping, _ := externalEndpointInfo.MessageFor(endpoint, false, false)
			Expect(mapping.RouterGroupGuid).To(Equal("router-group-guid"))
			Expect(mapping.ExternalPort).To(Equal(uint16(8080)))
			Expect(mapping.HostIP).To(Equal("host"))
			Expect(mapping.HostPort).To(Equal(uint16(1234)))
			Expect(mapping.HostTLSPort).To(Equal(61001))
			Expect(mapping.InstanceId).To(Equal("instance-guid"))
			Expect(*mapping.SniHostname).To(Equal("sni-hostname"))
			Expect(mapping.TerminateFrontendTLS).To(BeTrue())
		})
	})
})

func newEndpointWithTlsProxyPort(
	instanceGUID string, presence models.ActualLRP_Presence,
	host, containerIP string,
	port, containerPort, tlsProxyPort, containerTlsProxyPort uint32,
	preferredAddress models.ActualLRPNetInfo_PreferredAddress,
	modificationTag *models.ModificationTag,
	availabiltiyZone string,
) routingtable.Endpoint {
	return routingtable.Endpoint{
		InstanceGUID:          instanceGUID,
		Presence:              presence,
		Host:                  host,
		ContainerIP:           containerIP,
		Port:                  port,
		ContainerPort:         containerPort,
		TlsProxyPort:          tlsProxyPort,
		ContainerTlsProxyPort: containerTlsProxyPort,
		PreferredAddress:      preferredAddress,
		ModificationTag:       modificationTag,
		AvailabilityZone:      availabiltiyZone,
	}
}
