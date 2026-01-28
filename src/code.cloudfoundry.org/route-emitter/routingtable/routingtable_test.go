package routingtable_test

import (
	"fmt"

	"code.cloudfoundry.org/bbs/models"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/route-emitter/routingtable"
	. "code.cloudfoundry.org/route-emitter/routingtable/matchers"
	tcpmodels "code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/routing-info/cfroutes"
	"code.cloudfoundry.org/routing-info/internalroutes"
	"code.cloudfoundry.org/routing-info/tcp_routes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func createDesiredLRP(
	processGuid string, instances int32, port uint32, logGuid, rsURL string,
	currentTag models.ModificationTag, runInfo models.DesiredLRPRunInfo,
	hostnames ...string,
) *models.DesiredLRP {
	routingInfo := cfroutes.CFRoutes{
		{
			Hostnames:       hostnames,
			Port:            port,
			RouteServiceUrl: rsURL,
		},
	}.RoutingInfo()

	routes := models.Routes{}

	for key, message := range routingInfo {
		routes[key] = message
	}

	return createDesiredLRPWithRoutes(
		processGuid, instances, routes, logGuid, currentTag, runInfo,
	)
}

func createRoutingInfo(port uint32, hostnames, internalHostnames []string, rsURL string, externalPorts []uint32, routerGroupGuid string) models.Routes {
	routingInfo := cfroutes.CFRoutes{
		{
			Hostnames:       hostnames,
			Port:            port,
			RouteServiceUrl: rsURL,
		},
	}.RoutingInfo()

	routes := models.Routes{}

	for key, message := range routingInfo {
		routes[key] = message
	}

	for _, e := range externalPorts {
		tcpRoutes := tcp_routes.TCPRoutes{
			{
				RouterGroupGuid: routerGroupGuid,
				ExternalPort:    e,
				ContainerPort:   port,
			},
		}.RoutingInfo()
		for key, message := range *tcpRoutes {
			routes[key] = message
		}
	}

	internalRoutes := internalroutes.InternalRoutes{}

	for _, host := range internalHostnames {
		internalRoutes = append(internalRoutes,
			internalroutes.InternalRoute{Hostname: host},
		)
	}
	internalRoutingInfo := internalRoutes.RoutingInfo()
	for key, message := range internalRoutingInfo {
		routes[key] = message
	}

	return routes
}

func createDesiredLRPWithRoutes(
	processGuid string, instances int32, routes models.Routes, logGuid string,
	currentTag models.ModificationTag, runInfo models.DesiredLRPRunInfo,
) *models.DesiredLRP {
	return &models.DesiredLRP{
		ProcessGuid: processGuid,
		Domain:      "domain",
		LogGuid:     logGuid,
		Annotation:  "",
		Instances:   instances,

		MemoryMb: 0,
		DiskMb:   0,
		RootFs:   "",
		MaxPids:  0,

		Routes:          &routes,
		ModificationTag: &currentTag,
		VolumeMounts:    nil,
		PlacementTags:   nil,
	}
}

func createActualLRP(
	key routingtable.RoutingKey,
	instance routingtable.Endpoint,
	domain string,
) *models.ActualLRP {

	var portMapping *models.PortMapping

	if instance.TlsProxyPort == 0 && instance.ContainerTlsProxyPort == 0 {
		portMapping = models.NewPortMapping(instance.Port, instance.ContainerPort)
	} else {
		portMapping = models.NewPortMappingWithTLSProxy(instance.Port, instance.ContainerPort, instance.TlsProxyPort, instance.ContainerTlsProxyPort)
	}

	return &models.ActualLRP{
		ActualLrpKey:         models.NewActualLRPKey(key.ProcessGUID, instance.Index, domain),
		ActualLrpInstanceKey: models.NewActualLRPInstanceKey(instance.InstanceGUID, "cell-id"),
		ActualLrpNetInfo: models.NewActualLRPNetInfo(
			instance.Host,
			instance.ContainerIP,
			instance.PreferredAddress,
			portMapping,
		),
		Presence:        instance.Presence,
		State:           models.ActualLRPStateRunning,
		Since:           instance.Since,
		ModificationTag: *instance.ModificationTag,
	}
}

func createActualLRPWithPortMappings(
	key routingtable.RoutingKey,
	instance routingtable.Endpoint,
	domain string,
	ports ...*models.PortMapping,
) *models.ActualLRP {

	return &models.ActualLRP{
		ActualLrpKey:         models.NewActualLRPKey(key.ProcessGUID, instance.Index, domain),
		ActualLrpInstanceKey: models.NewActualLRPInstanceKey(instance.InstanceGUID, "cell-id"),
		ActualLrpNetInfo: models.NewActualLRPNetInfo(
			instance.Host,
			instance.ContainerIP,
			models.ActualLRPNetInfo_PreferredAddressHost,
			ports...,
		),
		Presence:        instance.Presence,
		Since:           instance.Since,
		State:           models.ActualLRPStateRunning,
		ModificationTag: *instance.ModificationTag,
	}
}

var _ = Describe("RoutingTable", func() {
	var (
		table                           routingtable.RoutingTable
		messagesToEmit                  routingtable.MessagesToEmit
		tcpRouteMappings                routingtable.TCPRouteMappings
		logger                          *lagertest.TestLogger
		endpoint1, endpoint2, endpoint3 routingtable.Endpoint
		key                             routingtable.RoutingKey
		fakeMetronClient                *mfakes.FakeIngressClient
	)

	domain := "domain"
	hostname1 := "foo.example.com"

	noFreshDomains := models.NewDomainSet([]string{})
	freshDomains := models.NewDomainSet([]string{domain})

	currentTag := &models.ModificationTag{Epoch: "abc", Index: 1}
	newerTag := &models.ModificationTag{Epoch: "def", Index: 0}

	logGuid := "some-log-guid"

	runInfo := models.DesiredLRPRunInfo{}

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-route-emitter")
		fakeMetronClient = &mfakes.FakeIngressClient{}
		table = routingtable.NewRoutingTable(false, true, fakeMetronClient)

		endpoint1 = routingtable.Endpoint{
			InstanceGUID:          "ig-1",
			Host:                  "1.1.1.1",
			ContainerIP:           "1.2.3.4",
			Index:                 0,
			Port:                  11,
			ContainerPort:         8080,
			TlsProxyPort:          61443,
			ContainerTlsProxyPort: 5443,
			Presence:              models.ActualLRP_Ordinary,
			PreferredAddress:      models.ActualLRPNetInfo_PreferredAddressInstance,
			Since:                 1,
			ModificationTag:       currentTag,
		}
		endpoint2 = routingtable.Endpoint{
			InstanceGUID:          "ig-2",
			Host:                  "2.2.2.2",
			ContainerIP:           "2.3.4.5",
			Index:                 1,
			Port:                  22,
			ContainerPort:         8080,
			TlsProxyPort:          61443,
			ContainerTlsProxyPort: 5443,
			Presence:              models.ActualLRP_Ordinary,
			PreferredAddress:      models.ActualLRPNetInfo_PreferredAddressHost,
			Since:                 2,
			ModificationTag:       currentTag,
		}
		endpoint3 = routingtable.Endpoint{
			InstanceGUID:     "ig-3",
			Host:             "3.3.3.3",
			ContainerIP:      "3.4.5.6",
			Index:            2,
			Port:             33,
			ContainerPort:    8080,
			Presence:         models.ActualLRP_Ordinary,
			PreferredAddress: models.ActualLRPNetInfo_PreferredAddressUnknown,
			Since:            3,
			ModificationTag:  currentTag,
		}

		key = routingtable.RoutingKey{ProcessGUID: "some-process-guid", ContainerPort: 8080}
	})

	Describe("SetRoutes", func() {
		var (
			instances        int32
			beforeDesiredLRP *models.DesiredLRP
			internalHostname string
		)

		BeforeEach(func() {
			instances = 1
		})

		JustBeforeEach(func() {
			internalHostname = "internal"

			// Add routes and internal routes
			routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{internalHostname}, "", []uint32{}, logGuid)

			// Set routes on desired lrp
			beforeDesiredLRP = createDesiredLRPWithRoutes(key.ProcessGUID, instances, routes, logGuid, *currentTag, runInfo)
			table.SetRoutes(logger, nil, beforeDesiredLRP)

			actualLRP := createActualLRP(key, endpoint1, domain)
			table.AddEndpoint(logger, actualLRP)
		})

		Context("when a http2 route is added", func() {
			var createDesiredLRPWithProtocol = func(
				processGuid string, instances int32, port uint32, logGuid, rsURL string,
				currentTag models.ModificationTag, runInfo models.DesiredLRPRunInfo, protocol string, hostnames ...string,
			) *models.DesiredLRP {
				routingInfo := cfroutes.CFRoutes{
					{
						Hostnames:       hostnames,
						Port:            port,
						RouteServiceUrl: rsURL,
						Protocol:        protocol,
					},
				}.RoutingInfo()

				routes := models.Routes{}

				for key, message := range routingInfo {
					routes[key] = message
				}

				return createDesiredLRPWithRoutes(
					processGuid, instances, routes, logGuid, currentTag, runInfo,
				)
			}

			BeforeEach(func() {
				endpoint1.PreferredAddress = 0
			})

			JustBeforeEach(func() {
				afterDesiredLRP := createDesiredLRPWithProtocol(key.ProcessGUID, instances, key.ContainerPort, logGuid, "", *newerTag, runInfo, "http2", "grpc.example.com")
				_, messagesToEmit = table.SetRoutes(logger, beforeDesiredLRP, afterDesiredLRP)
			})

			It("emits a registration", func() {
				expectedRegistrationMessage := []routingtable.RegistryMessage{
					routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: "grpc.example.com", LogGUID: logGuid, Protocol: "http2"}, false),
				}
				Expect(messagesToEmit.RegistrationMessages).To(Equal(expectedRegistrationMessage))
			})
		})

		Context("when the route is removed", func() {
			JustBeforeEach(func() {
				afterDesiredLRP := createDesiredLRP(key.ProcessGUID, instances, key.ContainerPort, logGuid, "", *newerTag, runInfo, "")
				afterDesiredLRP.Routes = nil
				tcpRouteMappings, messagesToEmit = table.SetRoutes(logger, beforeDesiredLRP, afterDesiredLRP)
			})

			It("emits an unregistration", func() {
				expectedUnregistrationMessages := []routingtable.RegistryMessage{
					routingtable.InternalAddressRegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
				}
				Expect(messagesToEmit.UnregistrationMessages).To(Equal(expectedUnregistrationMessages))
			})

			Context("when the a new route is added", func() {
				JustBeforeEach(func() {
					newerTag := &models.ModificationTag{Epoch: "ghi", Index: 0}
					afterDesiredLRP := createDesiredLRP(key.ProcessGUID, instances, key.ContainerPort, logGuid, "", *newerTag, runInfo, "bar.example.com")
					tcpRouteMappings, messagesToEmit = table.SetRoutes(logger, beforeDesiredLRP, afterDesiredLRP)
				})

				It("emits a registration", func() {
					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.InternalAddressRegistryMessageFor(endpoint1, routingtable.Route{Hostname: "bar.example.com", LogGUID: logGuid}, false),
						},
					}
					Expect(messagesToEmit).To(Equal(expected))
				})

				Context("when the endpoint has a tls proxy port", func() {
					BeforeEach(func() {
						endpoint1.TlsProxyPort = 61001
						endpoint1.ContainerTlsProxyPort = 61002
					})

					It("emits a registration", func() {
						expected := routingtable.MessagesToEmit{
							RegistrationMessages: []routingtable.RegistryMessage{
								routingtable.InternalAddressRegistryMessageFor(endpoint1, routingtable.Route{Hostname: "bar.example.com", LogGUID: logGuid}, false),
							},
						}
						Expect(messagesToEmit).To(Equal(expected))
					})
				})
			})

			Context("when an update is received with old modification tag", func() {
				JustBeforeEach(func() {
					afterDesiredLRP := createDesiredLRP(key.ProcessGUID, instances, key.ContainerPort, logGuid, "", *newerTag, runInfo, "bar.example.com")
					tcpRouteMappings, messagesToEmit = table.SetRoutes(logger, beforeDesiredLRP, afterDesiredLRP)
				})

				It("does not emit anything", func() {
					Expect(messagesToEmit).To(BeZero())
				})
			})

			Context("when the route metric tags are updated", func() {
				var afterDesiredLRP *models.DesiredLRP

				JustBeforeEach(func() {
					newerTag := &models.ModificationTag{Epoch: "ghi", Index: 0}
					afterDesiredLRP = createDesiredLRP(key.ProcessGUID, instances, key.ContainerPort, logGuid, "", *newerTag, runInfo, "bar.example.com")
					tcpRouteMappings, messagesToEmit = table.SetRoutes(logger, beforeDesiredLRP, afterDesiredLRP)
				})

				It("emits a registration message with new tags", func() {
					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.InternalAddressRegistryMessageFor(endpoint1, routingtable.Route{Hostname: "bar.example.com", LogGUID: logGuid}, false),
						},
					}
					Expect(messagesToEmit).To(Equal(expected))

					afterDesiredLRP.MetricTags = map[string]*models.MetricTagValue{"foo": &models.MetricTagValue{Static: "bar"}, "doo": &models.MetricTagValue{Dynamic: models.MetricTagValue_MetricTagDynamicValueIndex}}
					afterDesiredLRP.ModificationTag = &models.ModificationTag{Epoch: "lmn", Index: 0}
					table.SetRoutes(logger, beforeDesiredLRP, afterDesiredLRP)

					tcpRouteMappings, messagesToEmit = table.GetExternalRoutingEvents()
					Expect(tcpRouteMappings).To(BeZero())
					expected = routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.InternalAddressRegistryMessageFor(
								endpoint1,
								routingtable.Route{
									Hostname:   "bar.example.com",
									LogGUID:    logGuid,
									MetricTags: afterDesiredLRP.MetricTags,
								},
								false,
							),
						},
					}
					Expect(messagesToEmit).To(Equal(expected))
				})
			})
		})

		Context("when the internal route is removed", func() {
			JustBeforeEach(func() {
				afterDesiredLRP := createDesiredLRP(key.ProcessGUID, instances, key.ContainerPort, logGuid, "", *newerTag, runInfo, hostname1)
				tcpRouteMappings, messagesToEmit = table.SetRoutes(logger, beforeDesiredLRP, afterDesiredLRP)
			})

			It("emits an internal unregistration", func() {
				expected := routingtable.MessagesToEmit{
					InternalUnregistrationMessages: []routingtable.RegistryMessage{
						{
							URIs:                 []string{internalHostname, fmt.Sprintf("%d.%s", 0, internalHostname)},
							Host:                 endpoint1.ContainerIP,
							Tags:                 map[string]string{"component": "route-emitter"},
							App:                  logGuid,
							PrivateInstanceIndex: "0",
						},
					},
				}
				Expect(messagesToEmit).To(Equal(expected))
			})

			Context("when an update is received with old modification tag", func() {
				JustBeforeEach(func() {
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{}, "", []uint32{}, logGuid)

					afterDesiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, instances, routes, logGuid, *newerTag, runInfo)
					tcpRouteMappings, messagesToEmit = table.SetRoutes(logger, beforeDesiredLRP, afterDesiredLRP)
				})

				It("does not emit anything", func() {
					Expect(messagesToEmit).To(BeZero())
				})
			})
		})

		Context("when a new internal route is added", func() {
			var internalHostname2 string
			JustBeforeEach(func() {
				internalHostname2 = "internal-2"
				newerTag := &models.ModificationTag{Epoch: "ghi", Index: 0}
				routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{internalHostname, internalHostname2}, "", []uint32{}, logGuid)

				afterDesiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, instances, routes, logGuid, *newerTag, runInfo)

				tcpRouteMappings, messagesToEmit = table.SetRoutes(logger, beforeDesiredLRP, afterDesiredLRP)
			})

			It("emits an internal registration", func() {
				expected := routingtable.MessagesToEmit{
					InternalRegistrationMessages: []routingtable.RegistryMessage{
						{
							URIs:                 []string{internalHostname2, fmt.Sprintf("%d.%s", 0, internalHostname2)},
							Host:                 endpoint1.ContainerIP,
							Tags:                 map[string]string{"component": "route-emitter"},
							App:                  logGuid,
							EndpointUpdatedAtNs:  0, // do not emit this field for route updates
							PrivateInstanceIndex: "0",
						},
					},
				}
				Expect(messagesToEmit).To(Equal(expected))
			})
		})

		Context("when the instances are scaled down", func() {
			BeforeEach(func() {
				instances = 3
			})

			JustBeforeEach(func() {
				// add 2 more instances
				actualLRP := createActualLRP(key, endpoint2, domain)
				table.AddEndpoint(logger, actualLRP)
				actualLRP = createActualLRP(key, endpoint3, domain)
				table.AddEndpoint(logger, actualLRP)

				//changedLRP
				routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{internalHostname}, "", []uint32{}, logGuid)
				afterDesiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 1, routes, logGuid, *newerTag, runInfo)

				tcpRouteMappings, messagesToEmit = table.SetRoutes(logger, beforeDesiredLRP, afterDesiredLRP)
			})

			It("should unregisters extra endpoints", func() {
				Expect(tcpRouteMappings).To(BeZero())
				expected := routingtable.MessagesToEmit{
					UnregistrationMessages: []routingtable.RegistryMessage{
						routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
						routingtable.RegistryMessageFor(endpoint3, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
					},
					InternalUnregistrationMessages: []routingtable.RegistryMessage{
						{
							URIs:                 []string{internalHostname, fmt.Sprintf("%d.%s", 1, internalHostname)},
							Host:                 endpoint2.ContainerIP,
							Tags:                 map[string]string{"component": "route-emitter"},
							App:                  logGuid,
							PrivateInstanceIndex: "1",
						},
						{
							URIs:                 []string{internalHostname, fmt.Sprintf("%d.%s", 2, internalHostname)},
							Host:                 endpoint3.ContainerIP,
							Tags:                 map[string]string{"component": "route-emitter"},
							App:                  logGuid,
							PrivateInstanceIndex: "2",
						},
					},
				}
				Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
			})

			It("no longer emits the extra endpoints", func() {
				tcpRouteMappings, messagesToEmit = table.GetExternalRoutingEvents()
				_, internalMessagesToEmit := table.GetInternalRoutingEvents()
				messagesToEmit = messagesToEmit.Merge(internalMessagesToEmit)

				Expect(tcpRouteMappings).To(BeZero())
				expected := routingtable.MessagesToEmit{
					RegistrationMessages: []routingtable.RegistryMessage{
						routingtable.InternalAddressRegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
					},
					InternalRegistrationMessages: []routingtable.RegistryMessage{
						{
							URIs:                 []string{internalHostname, fmt.Sprintf("%d.%s", 0, internalHostname)},
							Host:                 endpoint1.ContainerIP,
							Tags:                 map[string]string{"component": "route-emitter"},
							App:                  logGuid,
							EndpointUpdatedAtNs:  0,
							PrivateInstanceIndex: "0",
						},
					},
				}
				Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
			})
		})
	})

	Describe("Swap", func() {
		It("preserves the desired LRP domain", func() {
			By("creating a routing table with a route and endpoint")
			routingInfo := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{}, "", []uint32{5222}, "router-group-guid")
			beforeDesiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routingInfo, logGuid, *currentTag, runInfo)
			table.SetRoutes(logger, nil, beforeDesiredLRP)
			actualLRP := createActualLRP(key, endpoint1, domain)
			table.AddEndpoint(logger, actualLRP)

			By("removing the route and making the domains unfresh")
			tempTable := routingtable.NewRoutingTable(false, true, fakeMetronClient)
			actualLRP = createActualLRP(key, endpoint1, domain)
			tempTable.AddEndpoint(logger, actualLRP)
			table.Swap(logger, tempTable, noFreshDomains)

			By("making the domain fresh again")
			tempTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
			actualLRP = createActualLRP(key, endpoint1, domain)
			tempTable.AddEndpoint(logger, actualLRP)
			tcpRouteMappings, messagesToEmit = table.Swap(logger, tempTable, freshDomains)

			expectedHTTP := routingtable.MessagesToEmit{
				UnregistrationMessages: []routingtable.RegistryMessage{
					routingtable.InternalAddressRegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
				},
			}
			Expect(messagesToEmit).To(MatchMessagesToEmit(expectedHTTP))

			Expect(tcpRouteMappings.Unregistrations).Should(ConsistOf(
				tcpmodels.NewTcpRouteMapping("router-group-guid", 5222, endpoint1.ContainerIP, uint16(endpoint1.ContainerPort), int(endpoint1.ContainerTlsProxyPort), "ig-1", nil, 0, tcpmodels.ModificationTag{}),
			))
		})

		Context("when there is internal routable endpoint", func() {
			BeforeEach(func() {
				routingInfo := createRoutingInfo(key.ContainerPort, []string{}, []string{"internal"}, "", []uint32{5222}, "")
				beforeDesiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routingInfo, logGuid, *currentTag, runInfo)
				table.SetRoutes(logger, nil, beforeDesiredLRP)

				actualLRP := createActualLRP(key, endpoint1, domain)
				table.AddEndpoint(logger, actualLRP)
			})

			Context("and the domain is not fresh", func() {
				It("saves the previous tables routes and emits them when an endpoint is added", func() {
					actualLRP := createActualLRP(key, endpoint1, domain)
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					tempTable.AddEndpoint(logger, actualLRP)
					_, messagesToEmit := table.Swap(logger, tempTable, noFreshDomains)
					Expect(messagesToEmit.InternalUnregistrationMessages).To(BeEmpty())
					Expect(messagesToEmit.InternalRegistrationMessages).To(BeEmpty())

					tcpRouteMappings, messagesToEmit = table.GetInternalRoutingEvents()

					Expect(messagesToEmit.InternalUnregistrationMessages).To(BeEmpty())
					Expect(messagesToEmit.InternalRegistrationMessages).To(ConsistOf(routingtable.RegistryMessage{
						URIs:                 []string{"internal", "0.internal"},
						Host:                 endpoint1.ContainerIP,
						Tags:                 map[string]string{"component": "route-emitter"},
						App:                  logGuid,
						PrivateInstanceIndex: "0",
					}))
				})
			})
		})

		Context("when the table has a routable endpoint", func() {
			BeforeEach(func() {
				routingInfo := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{}, "", []uint32{5222}, "router-group-guid")
				beforeDesiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routingInfo, logGuid, *currentTag, runInfo)
				table.SetRoutes(logger, nil, beforeDesiredLRP)

				actualLRP := createActualLRP(key, endpoint1, domain)
				table.AddEndpoint(logger, actualLRP) // route+lrp
			})

			Context("when the domain is not fresh", func() {
				Context("and the new table has nothing in it", func() {
					BeforeEach(func() {
						tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
						tcpRouteMappings, messagesToEmit = table.Swap(logger, tempTable, noFreshDomains)
					})

					It("unregisters non-existent endpoints", func() {
						expectedHTTP := routingtable.MessagesToEmit{
							UnregistrationMessages: []routingtable.RegistryMessage{
								routingtable.InternalAddressRegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
							},
						}
						Expect(messagesToEmit).To(MatchMessagesToEmit(expectedHTTP))

						Expect(tcpRouteMappings.Unregistrations).Should(ConsistOf(
							tcpmodels.NewTcpRouteMapping("router-group-guid", 5222, endpoint1.ContainerIP, uint16(endpoint1.ContainerPort), int(endpoint1.ContainerTlsProxyPort), "ig-1", nil, 0, tcpmodels.ModificationTag{}),
						))
					})

					It("saves the previous tables routes and emits them when an endpoint is added", func() {
						actualLRP := createActualLRP(key, endpoint1, domain)
						tempTable := routingtable.NewRoutingTable(false, true, fakeMetronClient)
						tempTable.AddEndpoint(logger, actualLRP)
						tcpRouteMappings, messagesToEmit = table.Swap(logger, tempTable, noFreshDomains)

						expectedHTTP := routingtable.MessagesToEmit{
							RegistrationMessages: []routingtable.RegistryMessage{
								routingtable.InternalAddressRegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, true),
							},
						}
						Expect(messagesToEmit).To(MatchMessagesToEmit(expectedHTTP))

						Expect(tcpRouteMappings.Registrations).Should(ConsistOf(
							tcpmodels.NewTcpRouteMapping("router-group-guid", 5222, endpoint1.ContainerIP, uint16(endpoint1.ContainerPort), int(endpoint1.ContainerTlsProxyPort), "ig-1", nil, 0, tcpmodels.ModificationTag{}),
						))
					})
				})
			})
		})
	})

	Describe("TableSize", func() {
		var (
			desiredLRP *models.DesiredLRP
			actualLRP  *models.ActualLRP
		)

		BeforeEach(func() {
			routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{"internal-hostname"}, "", []uint32{5222}, "router-group-guid")
			desiredLRP = createDesiredLRPWithRoutes(key.ProcessGUID, 1, routes, logGuid, *currentTag, runInfo)
			table.SetRoutes(logger, nil, desiredLRP)
			actualLRP = createActualLRP(key, endpoint1, domain)
			table.AddEndpoint(logger, actualLRP)

			Expect(table.TableSize()).To(Equal(3))
		})

		Context("when routes are deleted", func() {
			BeforeEach(func() {
				newDesiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 1, nil, logGuid, *newerTag, runInfo)
				table.SetRoutes(logger, desiredLRP, newDesiredLRP)
			})

			It("doesn't remove the entry from the table", func() {
				Expect(table.TableSize()).To(Equal(3))
			})

			Context("and endpoints are deleted", func() {
				BeforeEach(func() {
					table.RemoveEndpoint(logger, actualLRP)
				})

				It("removes the entry", func() {
					Expect(table.TableSize()).To(Equal(0))
				})
			})
		})

		Context("when all endpoints are deleted", func() {
			BeforeEach(func() {
				table.RemoveEndpoint(logger, actualLRP)
			})

			It("doesn't remove the entry from the table", func() {
				Expect(table.TableSize()).To(Equal(3))
			})

			Context("and endpoints are deleted", func() {
				BeforeEach(func() {
					newDesiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 1, nil, logGuid, *newerTag, runInfo)
					table.SetRoutes(logger, desiredLRP, newDesiredLRP)
				})

				It("removes the entry", func() {
					Expect(table.TableSize()).To(Equal(0))
				})
			})
		})

		Context("when the table is swaped and the lrp is deleted", func() {
			BeforeEach(func() {
				tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
				table.Swap(logger, tempTable, freshDomains)
			})

			It("removes the entry", func() {
				Expect(table.TableSize()).To(Equal(0))
			})
		})
	})

	Describe("RouteCounts", func() {
		BeforeEach(func() {
			internalHostname := "internal"
			routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{internalHostname}, "", []uint32{5222}, "router-group-guid")
			afterDesiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 1, routes, logGuid, *currentTag, runInfo)
			table.SetRoutes(logger, nil, afterDesiredLRP)
			actualLRP1 := createActualLRP(key, endpoint1, domain)
			table.AddEndpoint(logger, actualLRP1)
		})

		It("returns the right tcp route count", func() {
			Expect(table.TCPAssociationsCount()).To(Equal(1))
		})

		It("returns the right http route count", func() {
			Expect(table.HTTPAssociationsCount()).To(Equal(1))
		})

		It("returns the right internal route count", func() {
			Expect(table.InternalAssociationsCount()).To(Equal(2))
		})
	})

	Describe("AddEndpoint", func() {
		var (
			internalHostname                                                                    string
			actualLRPWithInstanceAddress, actualLRPWithHostAddress, actualLRPWithUnknownAddress *models.ActualLRP
		)

		BeforeEach(func() {
			internalHostname = "internal-hostname"
			actualLRPWithInstanceAddress = createActualLRP(key, endpoint1, domain)
			actualLRPWithHostAddress = createActualLRP(key, endpoint2, domain)
			actualLRPWithUnknownAddress = createActualLRP(key, endpoint3, domain)
		})

		Context("when the routing table is configured to use direct instance route", func() {
			BeforeEach(func() {
				table = routingtable.NewRoutingTable(true, true, fakeMetronClient)
				routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{internalHostname}, "", []uint32{9999}, "")
				afterDesiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
				table.SetRoutes(logger, nil, afterDesiredLRP)
			})

			Context("when endpoint preferred address is instance", func() {
				BeforeEach(func() {
					tcpRouteMappings, messagesToEmit = table.AddEndpoint(logger, actualLRPWithInstanceAddress)
				})

				It("is using container IP in registration message", func() {
					Expect(tcpRouteMappings.Registrations).To(HaveLen(1))
					Expect(tcpRouteMappings.Registrations[0].HostIP).To(Equal(endpoint1.ContainerIP))
					Expect(tcpRouteMappings.Registrations[0].HostPort).To(Equal(uint16(endpoint1.ContainerPort)))
					Expect(tcpRouteMappings.Registrations[0].HostTLSPort).To(Equal(int(endpoint1.ContainerTlsProxyPort)))
					Expect(messagesToEmit.RegistrationMessages).To(HaveLen(1))
					Expect(messagesToEmit.RegistrationMessages[0].Host).To(Equal(endpoint1.ContainerIP))
					Expect(messagesToEmit.RegistrationMessages[0].Port).To(Equal(endpoint1.ContainerPort))
					Expect(messagesToEmit.RegistrationMessages[0].TlsPort).To(Equal(endpoint1.ContainerTlsProxyPort))
				})

				It("is using container IP in internal registration message", func() {
					Expect(messagesToEmit.InternalRegistrationMessages).To(HaveLen(1))
					Expect(messagesToEmit.InternalRegistrationMessages[0].Host).To(Equal(endpoint1.ContainerIP))
				})
			})

			Context("when endpoint preferred address is host", func() {
				BeforeEach(func() {
					tcpRouteMappings, messagesToEmit = table.AddEndpoint(logger, actualLRPWithHostAddress)
				})

				It("is using host IP in registration message", func() {
					Expect(tcpRouteMappings.Registrations).To(HaveLen(1))
					Expect(tcpRouteMappings.Registrations[0].HostIP).To(Equal(endpoint2.Host))
					Expect(tcpRouteMappings.Registrations[0].HostPort).To(Equal(uint16(endpoint2.Port)))
					Expect(tcpRouteMappings.Registrations[0].HostTLSPort).To(Equal(int(endpoint2.TlsProxyPort)))
					Expect(messagesToEmit.RegistrationMessages).To(HaveLen(1))
					Expect(messagesToEmit.RegistrationMessages[0].Host).To(Equal(endpoint2.Host))
					Expect(messagesToEmit.RegistrationMessages[0].Port).To(Equal(endpoint2.Port))
					Expect(messagesToEmit.RegistrationMessages[0].TlsPort).To(Equal(endpoint2.TlsProxyPort))
				})

				It("is using container IP in internal registration message", func() {
					Expect(messagesToEmit.InternalRegistrationMessages).To(HaveLen(1))
					Expect(messagesToEmit.InternalRegistrationMessages[0].Host).To(Equal(endpoint2.ContainerIP))
				})
			})

			Context("when endpoint preferred address is unknown", func() {
				BeforeEach(func() {
					tcpRouteMappings, messagesToEmit = table.AddEndpoint(logger, actualLRPWithUnknownAddress)
				})

				It("is using container IP in registration message", func() {
					Expect(tcpRouteMappings.Registrations).To(HaveLen(1))
					Expect(tcpRouteMappings.Registrations[0].HostIP).To(Equal(endpoint3.ContainerIP))
					Expect(tcpRouteMappings.Registrations[0].HostPort).To(Equal(uint16(endpoint3.ContainerPort)))
					Expect(tcpRouteMappings.Registrations[0].HostTLSPort).To(Equal(int(endpoint3.ContainerTlsProxyPort)))
					Expect(messagesToEmit.RegistrationMessages).To(HaveLen(1))
					Expect(messagesToEmit.RegistrationMessages[0].Host).To(Equal(endpoint3.ContainerIP))
					Expect(messagesToEmit.RegistrationMessages[0].Port).To(Equal(endpoint3.ContainerPort))
					Expect(messagesToEmit.RegistrationMessages[0].TlsPort).To(Equal(endpoint3.ContainerTlsProxyPort))
				})

				It("is using container IP in internal registration message", func() {
					Expect(messagesToEmit.InternalRegistrationMessages).To(HaveLen(1))
					Expect(messagesToEmit.InternalRegistrationMessages[0].Host).To(Equal(endpoint3.ContainerIP))
				})
			})
		})

		Context("when the routing table is configured not to use direct instance route", func() {
			BeforeEach(func() {
				table = routingtable.NewRoutingTable(false, true, fakeMetronClient)
				routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{internalHostname}, "", []uint32{9999}, "")
				afterDesiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
				table.SetRoutes(logger, nil, afterDesiredLRP)
			})

			Context("when endpoint preferred address is instance", func() {
				BeforeEach(func() {
					tcpRouteMappings, messagesToEmit = table.AddEndpoint(logger, actualLRPWithInstanceAddress)
				})

				It("is using container IP in registration message", func() {
					Expect(tcpRouteMappings.Registrations).To(HaveLen(1))
					Expect(tcpRouteMappings.Registrations[0].HostIP).To(Equal(endpoint1.ContainerIP))
					Expect(tcpRouteMappings.Registrations[0].HostPort).To(Equal(uint16(endpoint1.ContainerPort)))
					Expect(tcpRouteMappings.Registrations[0].HostTLSPort).To(Equal(int(endpoint1.ContainerTlsProxyPort)))
					Expect(messagesToEmit.RegistrationMessages).To(HaveLen(1))
					Expect(messagesToEmit.RegistrationMessages[0].Host).To(Equal(endpoint1.ContainerIP))
					Expect(messagesToEmit.RegistrationMessages[0].Port).To(Equal(endpoint1.ContainerPort))
					Expect(messagesToEmit.RegistrationMessages[0].TlsPort).To(Equal(endpoint1.ContainerTlsProxyPort))
				})

				It("is using container IP in internal registration message", func() {
					Expect(messagesToEmit.InternalRegistrationMessages).To(HaveLen(1))
					Expect(messagesToEmit.InternalRegistrationMessages[0].Host).To(Equal(endpoint1.ContainerIP))
				})
			})

			Context("when endpoint preferred address is host", func() {
				BeforeEach(func() {
					tcpRouteMappings, messagesToEmit = table.AddEndpoint(logger, actualLRPWithHostAddress)
				})

				It("is using host IP in registration message", func() {
					Expect(tcpRouteMappings.Registrations).To(HaveLen(1))
					Expect(tcpRouteMappings.Registrations[0].HostIP).To(Equal(endpoint2.Host))
					Expect(tcpRouteMappings.Registrations[0].HostPort).To(Equal(uint16(endpoint2.Port)))
					Expect(tcpRouteMappings.Registrations[0].HostTLSPort).To(Equal(int(endpoint2.TlsProxyPort)))
					Expect(messagesToEmit.RegistrationMessages).To(HaveLen(1))
					Expect(messagesToEmit.RegistrationMessages[0].Host).To(Equal(endpoint2.Host))
					Expect(messagesToEmit.RegistrationMessages[0].Port).To(Equal(endpoint2.Port))
					Expect(messagesToEmit.RegistrationMessages[0].TlsPort).To(Equal(endpoint2.TlsProxyPort))
				})

				It("is using container IP in internal registration message", func() {
					Expect(messagesToEmit.InternalRegistrationMessages).To(HaveLen(1))
					Expect(messagesToEmit.InternalRegistrationMessages[0].Host).To(Equal(endpoint2.ContainerIP))
				})
			})

			Context("when endpoint preferred address is unknown", func() {
				BeforeEach(func() {
					tcpRouteMappings, messagesToEmit = table.AddEndpoint(logger, actualLRPWithUnknownAddress)
				})

				It("is using host IP in registration message", func() {
					Expect(tcpRouteMappings.Registrations).To(HaveLen(1))
					Expect(tcpRouteMappings.Registrations[0].HostIP).To(Equal(endpoint3.Host))
					Expect(tcpRouteMappings.Registrations[0].HostPort).To(Equal(uint16(endpoint3.Port)))
					Expect(tcpRouteMappings.Registrations[0].HostTLSPort).To(Equal(int(endpoint3.TlsProxyPort)))
					Expect(messagesToEmit.RegistrationMessages).To(HaveLen(1))
					Expect(messagesToEmit.RegistrationMessages[0].Host).To(Equal(endpoint3.Host))
					Expect(messagesToEmit.RegistrationMessages[0].Port).To(Equal(endpoint3.Port))
					Expect(messagesToEmit.RegistrationMessages[0].TlsPort).To(Equal(endpoint3.TlsProxyPort))
				})

				It("is using container IP in internal registration message", func() {
					Expect(messagesToEmit.InternalRegistrationMessages).To(HaveLen(1))
					Expect(messagesToEmit.InternalRegistrationMessages[0].Host).To(Equal(endpoint3.ContainerIP))
				})
			})
		})

		Context("when a desired LRP has instances field less than number of actual LRP instances", func() {
			BeforeEach(func() {
				// Add internal routes
				routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{internalHostname}, "", []uint32{}, "")

				// Set routes on desired lrp
				afterDesiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 1, routes, logGuid, *currentTag, runInfo)
				table.SetRoutes(logger, nil, afterDesiredLRP)
			})

			It("only registers in the number of instances defined in the desired LRP", func() {
				actualLRP1 := createActualLRP(key, endpoint1, domain)
				actualLRP2 := createActualLRP(key, endpoint2, domain)
				actualLRP3 := createActualLRP(key, endpoint3, domain)
				tcpRouteMappings, messagesToEmit = table.AddEndpoint(logger, actualLRP1)
				Expect(tcpRouteMappings).To(BeZero())
				expected := routingtable.MessagesToEmit{
					RegistrationMessages: []routingtable.RegistryMessage{
						routingtable.InternalAddressRegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, true),
					},
					InternalRegistrationMessages: []routingtable.RegistryMessage{
						routingtable.RegistryMessage{
							Host: endpoint1.ContainerIP,
							URIs: []string{
								internalHostname,
								fmt.Sprintf("%d.%s", 0, internalHostname),
							},
							EndpointUpdatedAtNs: endpoint1.Since,
							Tags: map[string]string{
								"component": "route-emitter",
							},
							PrivateInstanceIndex: "0",
							App:                  logGuid,
						},
					},
				}
				Expect(messagesToEmit).To(MatchMessagesToEmit(expected))

				tcpRouteMappings, messagesToEmit = table.AddEndpoint(logger, actualLRP2)
				Expect(tcpRouteMappings).To(BeZero())
				Expect(messagesToEmit).To(BeZero())

				tcpRouteMappings, messagesToEmit = table.AddEndpoint(logger, actualLRP3)
				Expect(tcpRouteMappings).To(BeZero())
				Expect(messagesToEmit).To(BeZero())
			})
		})

		Context("when a desired LRP has metric tags", func() {
			BeforeEach(func() {
				routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{}, "", []uint32{}, "")
				desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 1, routes, logGuid, *currentTag, runInfo)
				desiredLRP.MetricTags = map[string]*models.MetricTagValue{"foo": &models.MetricTagValue{Static: "bar"}, "doo": &models.MetricTagValue{Dynamic: models.MetricTagValue_MetricTagDynamicValueIndex}}
				table.SetRoutes(logger, nil, desiredLRP)
			})

			It("emits registration messages with the appropriate tags", func() {
				actualLRP1 := createActualLRP(key, endpoint1, domain)
				tcpRouteMappings, messagesToEmit = table.AddEndpoint(logger, actualLRP1)
				expected := routingtable.MessagesToEmit{
					RegistrationMessages: []routingtable.RegistryMessage{
						routingtable.InternalAddressRegistryMessageFor(
							endpoint1,
							routingtable.Route{
								Hostname: hostname1,
								LogGUID:  logGuid,
								MetricTags: map[string]*models.MetricTagValue{
									"foo": &models.MetricTagValue{Static: "bar"},
									"doo": &models.MetricTagValue{Dynamic: models.MetricTagValue_MetricTagDynamicValueIndex},
								},
							},
							true,
						),
					},
				}
				Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
			})
		})
	})

	Describe("GetInternalRoutingEvents", func() {
		It("returns an empty array", func() {
			tcpRouteMappings, messagesToEmit = table.GetInternalRoutingEvents()
			Expect(messagesToEmit).To(Equal(routingtable.MessagesToEmit{}))
		})

		Context("when the table has routes but no endpoints", func() {
			var beforeLRP *models.DesiredLRP
			BeforeEach(func() {
				routes := createRoutingInfo(key.ContainerPort, []string{}, []string{"internal"}, "https://rs.example.com", []uint32{}, "")
				beforeLRP = createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
				table.SetRoutes(logger, nil, beforeLRP)
			})

			It("should be empty", func() {
				_, messagesToEmit = table.GetInternalRoutingEvents()
				Expect(messagesToEmit).To(BeZero())
			})
		})

		Context("when the table has endpoints but no routes", func() {
			var lrp1, lrp2 *models.ActualLRP

			BeforeEach(func() {
				lrp1 = createActualLRP(key, endpoint1, domain)
				lrp2 = createActualLRP(key, endpoint2, domain)
				table.AddEndpoint(logger, lrp1)
				table.AddEndpoint(logger, lrp2)
			})

			It("should be empty", func() {
				_, messagesToEmit = table.GetInternalRoutingEvents()
				Expect(messagesToEmit).To(BeZero())
			})
		})

		Context("when the table has routes and endpoints", func() {
			var beforeLRP *models.DesiredLRP
			var lrp1, lrp2 *models.ActualLRP
			var hostname2, internalHostname1 string

			BeforeEach(func() {
				hostname2 = "bar.example.com"
				internalHostname1 = "internal"
				routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")
				beforeLRP = createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
				table.SetRoutes(logger, nil, beforeLRP)
				lrp1 = createActualLRP(key, endpoint1, domain)
				lrp2 = createActualLRP(key, endpoint2, domain)
				table.AddEndpoint(logger, lrp1)
				table.AddEndpoint(logger, lrp2)
			})

			It("emits the internal registrations", func() {
				tcpRouteMappings, messagesToEmit = table.GetInternalRoutingEvents()

				Expect(tcpRouteMappings).To(BeZero())
				expected := routingtable.MessagesToEmit{
					InternalRegistrationMessages: []routingtable.RegistryMessage{
						{
							Host:                 endpoint2.ContainerIP,
							URIs:                 []string{internalHostname1, fmt.Sprintf("%d.%s", 1, internalHostname1)},
							PrivateInstanceIndex: "1",
							App:                  logGuid,
							Tags: map[string]string{
								"component": "route-emitter",
							},
						},
						{
							Host:                 endpoint1.ContainerIP,
							URIs:                 []string{internalHostname1, fmt.Sprintf("%d.%s", 0, internalHostname1)},
							PrivateInstanceIndex: "0",
							App:                  logGuid,
							Tags: map[string]string{
								"component": "route-emitter",
							},
						},
					},
				}
				Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
			})
		})
	})

	Describe("GetExternalRoutingEvents", func() {
		It("returns an empty array", func() {
			tcpRouteMappings, messagesToEmit = table.GetExternalRoutingEvents()
			Expect(messagesToEmit).To(Equal(routingtable.MessagesToEmit{}))
		})

		Context("when the table has routes but no endpoints", func() {
			var beforeLRP *models.DesiredLRP
			BeforeEach(func() {
				routes := createRoutingInfo(key.ContainerPort, []string{}, []string{}, "https://rs.example.com", []uint32{}, "")
				beforeLRP = createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
				table.SetRoutes(logger, nil, beforeLRP)
			})

			It("should be empty", func() {
				_, messagesToEmit = table.GetExternalRoutingEvents()
				Expect(messagesToEmit).To(BeZero())
			})
		})

		Context("when the table has endpoints but no routes", func() {
			var lrp1, lrp2 *models.ActualLRP

			BeforeEach(func() {
				lrp1 = createActualLRP(key, endpoint1, domain)
				lrp2 = createActualLRP(key, endpoint2, domain)
				table.AddEndpoint(logger, lrp1)
				table.AddEndpoint(logger, lrp2)
			})

			It("should be empty", func() {
				_, messagesToEmit = table.GetExternalRoutingEvents()
				Expect(messagesToEmit).To(BeZero())
			})
		})

		Context("when the table has routes and endpoints", func() {
			var beforeLRP *models.DesiredLRP
			var lrp1, lrp2 *models.ActualLRP
			var hostname2, internalHostname1 string

			BeforeEach(func() {
				hostname2 = "bar.example.com"
				internalHostname1 = "internal"
				routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")
				beforeLRP = createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
				table.SetRoutes(logger, nil, beforeLRP)
				lrp1 = createActualLRP(key, endpoint1, domain)
				lrp2 = createActualLRP(key, endpoint2, domain)
				table.AddEndpoint(logger, lrp1)
				table.AddEndpoint(logger, lrp2)
			})

			It("emits the external registrations", func() {
				tcpRouteMappings, messagesToEmit = table.GetExternalRoutingEvents()

				Expect(tcpRouteMappings).To(BeZero())
				expected := routingtable.MessagesToEmit{
					RegistrationMessages: []routingtable.RegistryMessage{
						routingtable.InternalAddressRegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
						routingtable.InternalAddressRegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
						routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
						routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
					},
				}
				Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
			})
		})

		Context("when there are external TCP routes", func() {
			var (
				internalHostname string
			)
			BeforeEach(func() {
				internalHostname = "internal"

				// Add routes and internal routes
				routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{internalHostname}, "", []uint32{9999}, logGuid)

				// Set routes on desired lrp
				beforeDesiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 2, routes, logGuid, *currentTag, runInfo)
				table.SetRoutes(logger, nil, beforeDesiredLRP)

				actualLRP := createActualLRP(key, endpoint1, domain)
				table.AddEndpoint(logger, actualLRP)

				actualLRP = createActualLRP(key, endpoint2, domain)
				table.AddEndpoint(logger, actualLRP)

				// TODO: Setup the routing table to return a internal registration and internal unregistration from GetInternalRoutingEvents
			})

			It("returns only the external messages to emit", func() {
				tcpRouteMappings, messagesToEmit = table.GetExternalRoutingEvents()
				Expect(tcpRouteMappings.Registrations).Should(ConsistOf(
					tcpmodels.NewTcpRouteMapping(logGuid, 9999, endpoint1.ContainerIP, uint16(endpoint1.ContainerPort), int(endpoint1.ContainerTlsProxyPort), "ig-1", nil, 0, tcpmodels.ModificationTag{}),
					tcpmodels.NewTcpRouteMapping(logGuid, 9999, endpoint2.Host, uint16(endpoint2.Port), int(endpoint2.TlsProxyPort), "ig-2", nil, 0, tcpmodels.ModificationTag{}),
				))

				expected := routingtable.MessagesToEmit{
					RegistrationMessages: []routingtable.RegistryMessage{
						routingtable.InternalAddressRegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
						routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
					},
				}
				Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
			})
		})
	})
})
