package routingtable_test

import (
	"fmt"

	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/route-emitter/routingtable"
	. "code.cloudfoundry.org/route-emitter/routingtable/matchers"
	"code.cloudfoundry.org/routing-info/cfroutes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
)

var _ = Describe("RoutingTable", func() {
	var (
		table            routingtable.RoutingTable
		messagesToEmit   routingtable.MessagesToEmit
		logger           *lagertest.TestLogger
		fakeMetronClient *mfakes.FakeIngressClient
	)

	key := routingtable.RoutingKey{ProcessGUID: "some-process-guid", ContainerPort: 8080}

	hostname1 := "foo.example.com"
	hostname2 := "bar.example.com"
	hostname3 := "baz.example.com"

	internalHostname1 := "internal-1"
	internalHostname2 := "internal-2"

	domain := "domain"

	olderTag := &models.ModificationTag{Epoch: "abc", Index: 0}
	currentTag := &models.ModificationTag{Epoch: "abc", Index: 1}
	newerTag := &models.ModificationTag{Epoch: "def", Index: 0}

	endpoint1 := routingtable.Endpoint{
		InstanceGUID:    "ig-1",
		Host:            "1.1.1.1",
		ContainerIP:     "1.2.3.4",
		Index:           0,
		Port:            11,
		ContainerPort:   8080,
		Presence:        models.ActualLRP_Ordinary,
		Since:           1,
		ModificationTag: currentTag,
	}
	endpoint2 := routingtable.Endpoint{
		InstanceGUID:    "ig-2",
		Host:            "2.2.2.2",
		ContainerIP:     "2.3.4.5",
		Index:           1,
		Port:            22,
		ContainerPort:   8080,
		Presence:        models.ActualLRP_Ordinary,
		Since:           2,
		ModificationTag: currentTag,
	}
	endpoint3 := routingtable.Endpoint{
		InstanceGUID:    "ig-3",
		Host:            "3.3.3.3",
		ContainerIP:     "3.4.5.6",
		Index:           2,
		Port:            33,
		ContainerPort:   8080,
		Presence:        models.ActualLRP_Ordinary,
		Since:           3,
		ModificationTag: currentTag,
	}
	collisionEndpoint := routingtable.Endpoint{
		InstanceGUID:    "ig-4",
		Host:            "1.1.1.1",
		ContainerIP:     "1.2.3.4",
		Index:           3,
		Port:            11,
		ContainerPort:   8080,
		Presence:        models.ActualLRP_Ordinary,
		ModificationTag: currentTag,
	}
	newInstanceEndpointAfterEvacuation := routingtable.Endpoint{
		InstanceGUID:    "ig-5",
		Host:            "5.5.5.5",
		ContainerIP:     "4.5.6.7",
		Index:           0,
		Port:            55,
		ContainerPort:   8080,
		Presence:        models.ActualLRP_Ordinary,
		ModificationTag: currentTag,
	}
	evacuating1 := routingtable.Endpoint{
		InstanceGUID:    "ig-1",
		Host:            "1.1.1.1",
		ContainerIP:     "1.2.3.4",
		Index:           0,
		Port:            11,
		ContainerPort:   8080,
		Presence:        models.ActualLRP_Evacuating,
		ModificationTag: currentTag,
	}

	logGuid := "some-log-guid"

	domains := models.NewDomainSet([]string{domain})
	noFreshDomains := models.NewDomainSet([]string{})

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-route-emitter")

		fakeMetronClient = &mfakes.FakeIngressClient{}
		table = routingtable.NewRoutingTable(false, false, fakeMetronClient)
	})

	runInfo := models.DesiredLRPRunInfo{}

	createDesiredLRPWithFixtures := func(serviceURL string) *models.DesiredLRP {
		desiredLRP := createDesiredLRP(
			key.ProcessGUID, 3, key.ContainerPort, logGuid, serviceURL, *currentTag,
			runInfo, hostname1, hostname2,
		)
		return desiredLRP
	}

	createDesiredLRPWithIS := func(isolationSegment string) *models.DesiredLRP {
		routingInfo := cfroutes.CFRoutes{
			{
				Hostnames:        []string{hostname1, hostname2},
				Port:             key.ContainerPort,
				IsolationSegment: isolationSegment,
			},
		}.RoutingInfo()
		routes := models.Routes{}
		for key, message := range routingInfo {
			routes[key] = message
		}

		return createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
	}

	Describe("Evacuating endpoints", func() {
		BeforeEach(func() {
			desiredLRP := createDesiredLRP(key.ProcessGUID, int32(3), key.ContainerPort, logGuid, "", *currentTag, models.DesiredLRPRunInfo{}, hostname1)
			_, messagesToEmit = table.SetRoutes(logger, nil, desiredLRP)
			Expect(messagesToEmit).To(BeZero())

			actualLRP := createActualLRP(key, endpoint1, domain)
			_, messagesToEmit = table.AddEndpoint(logger, actualLRP)
			expected := routingtable.MessagesToEmit{
				RegistrationMessages: []routingtable.RegistryMessage{
					routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, true),
				},
			}
			Expect(messagesToEmit).To(MatchMessagesToEmit(expected))

			actualLRP = createActualLRP(key, evacuating1, domain)
			_, messagesToEmit = table.AddEndpoint(logger, actualLRP)
			Expect(messagesToEmit).To(BeZero())

			actualLRP = createActualLRP(key, endpoint1, domain)
			_, messagesToEmit = table.RemoveEndpoint(logger, actualLRP)
			Expect(messagesToEmit).To(BeZero())
		})

		It("does not log an address collision", func() {
			Consistently(logger).ShouldNot(Say("collision-detected-with-endpoint"))
		})

		Context("when we have an evacuating endpoint and an instance for that added", func() {
			It("emits a registration for the instance and a unregister for the evacuating", func() {
				evacuatingActualLRP := createActualLRP(key, newInstanceEndpointAfterEvacuation, domain)
				_, messagesToEmit = table.AddEndpoint(logger, evacuatingActualLRP)
				expected := routingtable.MessagesToEmit{
					RegistrationMessages: []routingtable.RegistryMessage{
						routingtable.RegistryMessageFor(newInstanceEndpointAfterEvacuation, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, true),
					},
				}
				Expect(messagesToEmit).To(MatchMessagesToEmit(expected))

				actualLRP := createActualLRP(key, evacuating1, domain)
				_, messagesToEmit = table.RemoveEndpoint(logger, actualLRP)
				expected = routingtable.MessagesToEmit{
					UnregistrationMessages: []routingtable.RegistryMessage{
						routingtable.RegistryMessageFor(evacuating1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
					},
				}
				Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
			})
		})
	})

	Context("when internal address message builder is used", func() {
		BeforeEach(func() {
			table = routingtable.NewRoutingTable(true, false, fakeMetronClient)
			desiredLRP := createDesiredLRP(key.ProcessGUID, int32(3), key.ContainerPort, logGuid, "", *currentTag, models.DesiredLRPRunInfo{}, hostname1)
			desiredLRP.MetricTags = map[string]*models.MetricTagValue{"foo": &models.MetricTagValue{Static: "bar"}, "doo": &models.MetricTagValue{Dynamic: models.MetricTagValue_MetricTagDynamicValueIndex}}
			table.SetRoutes(logger, nil, desiredLRP)
		})

		Context("and an endpoint is added", func() {
			var (
				actualLRP *models.ActualLRP
			)

			BeforeEach(func() {
				actualLRP = createActualLRP(key, endpoint1, domain)
				_, messagesToEmit = table.AddEndpoint(logger, actualLRP)
			})

			It("should log the added LRP net info", func() {
				Expect(logger).To(Say(
					`"address":"%s".*"ports":\[{"container_port":%d,"host_port":%d,"container_tls_proxy_port":0,"host_tls_proxy_port":0}\]`,
					endpoint1.Host,
					endpoint1.ContainerPort,
					endpoint1.Port,
				))
			})

			It("emits the container ip and port instead of the host ip and port", func() {
				expected := routingtable.MessagesToEmit{
					RegistrationMessages: []routingtable.RegistryMessage{
						{
							URIs:             []string{hostname1},
							Host:             "1.2.3.4",
							Port:             8080,
							App:              logGuid,
							IsolationSegment: "",
							Tags:             map[string]string{"component": "route-emitter", "foo": "bar", "doo": "0"},

							EndpointUpdatedAtNs:  endpoint1.Since,
							ServerCertDomainSAN:  "ig-1",
							PrivateInstanceId:    "ig-1",
							PrivateInstanceIndex: "0",
							RouteServiceUrl:      "",
						},
					},
				}
				Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
			})

			Context("then the endpoint is removed", func() {
				BeforeEach(func() {
					_, messagesToEmit = table.RemoveEndpoint(logger, actualLRP)
				})

				It("should log the removed LRP net info", func() {
					Expect(logger).To(Say(
						`"address":"%s".*"ports":\[{"container_port":%d,"host_port":%d,"container_tls_proxy_port":0,"host_tls_proxy_port":0}\]`,
						endpoint1.Host,
						endpoint1.ContainerPort,
						endpoint1.Port,
					))
				})

				It("emits the container ip and port", func() {
					expected := routingtable.MessagesToEmit{
						UnregistrationMessages: []routingtable.RegistryMessage{
							{
								URIs:                []string{hostname1},
								Host:                "1.2.3.4",
								Port:                8080,
								App:                 logGuid,
								IsolationSegment:    "",
								EndpointUpdatedAtNs: 0,
								Tags:                map[string]string{"component": "route-emitter", "foo": "bar", "doo": "0"},

								ServerCertDomainSAN:  "ig-1",
								PrivateInstanceId:    "ig-1",
								PrivateInstanceIndex: "0",
								RouteServiceUrl:      "",
							},
						},
					}
					Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
				})
			})
		})
	})

	Describe("Swap", func() {
		Context("when we have existing stuff in the table and an unfresh domain", func() {
			BeforeEach(func() {
				tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)

				routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")
				desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
				lrp := createActualLRP(key, endpoint1, domain)
				tempTable.SetRoutes(logger, nil, desiredLRP)
				tempTable.AddEndpoint(logger, lrp)

				table.Swap(logger, tempTable, domains)

				tempTable = routingtable.NewRoutingTable(false, false, fakeMetronClient)
				routes = createRoutingInfo(key.ContainerPort, []string{hostname1, hostname3}, []string{internalHostname2}, "", []uint32{}, "")
				desiredLRP = createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
				tempTable.SetRoutes(logger, nil, desiredLRP)
				tempTable.AddEndpoint(logger, lrp)

				_, messagesToEmit = table.Swap(logger, tempTable, noFreshDomains)
			})

			It("emits only additive changes", func() {
				expected := routingtable.MessagesToEmit{
					RegistrationMessages: []routingtable.RegistryMessage{
						routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname3, LogGUID: logGuid}, false),
					},
					InternalRegistrationMessages: []routingtable.RegistryMessage{
						{
							Host: endpoint1.ContainerIP,
							URIs: []string{internalHostname2, fmt.Sprintf("%d.%s", 0, internalHostname2)},
							Tags: map[string]string{
								"component": "route-emitter",
							},
							PrivateInstanceIndex: "0",
							App:                  logGuid,
						},
					},
				}
				Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
			})

			Context("subsequent swaps with still not fresh domain", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					desiredLRP := createDesiredLRP(key.ProcessGUID, int32(3), key.ContainerPort, logGuid, "", *currentTag, models.DesiredLRPRunInfo{}, hostname1, hostname3)
					lrp := createActualLRP(key, endpoint1, domain)
					tempTable.SetRoutes(logger, nil, desiredLRP)
					tempTable.AddEndpoint(logger, lrp)

					_, messagesToEmit = table.Swap(logger, tempTable, noFreshDomains)
				})

				It("emits nothing", func() {
					Expect(messagesToEmit.RegistrationMessages).To(BeEmpty())
					Expect(messagesToEmit.UnregistrationMessages).To(BeEmpty())
				})
			})

			Context("subsequent swaps with fresh", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					desiredLRP := createDesiredLRP(key.ProcessGUID, int32(3), key.ContainerPort, logGuid, "", *currentTag, models.DesiredLRPRunInfo{}, hostname1, hostname3)
					lrp := createActualLRP(key, endpoint1, domain)
					tempTable.SetRoutes(logger, nil, desiredLRP)
					tempTable.AddEndpoint(logger, lrp)
					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("emits unregisters the old route", func() {
					expected := []routingtable.RegistryMessage{
						routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
					}

					Expect(messagesToEmit.UnregistrationMessages).To(Equal(expected))
					Expect(messagesToEmit.RegistrationMessages).To(BeEmpty())
				})
			})
		})

		Context("when a new routing key arrives", func() {
			Context("when the routing key has both routes and endpoints", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)

					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
					lrp1 := createActualLRP(key, endpoint1, domain)
					lrp2 := createActualLRP(key, endpoint2, domain)
					tempTable.SetRoutes(logger, nil, desiredLRP)
					tempTable.AddEndpoint(logger, lrp1)
					tempTable.AddEndpoint(logger, lrp2)

					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("emits registrations for each pairing", func() {
					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
						},
						InternalRegistrationMessages: []routingtable.RegistryMessage{
							{
								Host:                 endpoint1.ContainerIP,
								URIs:                 []string{internalHostname1, fmt.Sprintf("%d.%s", 0, internalHostname1)},
								PrivateInstanceIndex: "0",
								App:                  logGuid,
								Tags: map[string]string{
									"component": "route-emitter",
								},
							},
							{
								Host:                 endpoint2.ContainerIP,
								URIs:                 []string{internalHostname1, fmt.Sprintf("%d.%s", 1, internalHostname1)},
								PrivateInstanceIndex: "1",
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

			Context("when the process only has routes", func() {
				var desiredLRP *models.DesiredLRP
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{internalHostname1}, "", []uint32{}, "")
					desiredLRP = createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
					tempTable.SetRoutes(logger, nil, desiredLRP)

					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("should not emit a registration", func() {
					Expect(messagesToEmit).To(BeZero())
				})

				Context("when the endpoints subsequently arrive", func() {
					BeforeEach(func() {
						tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
						lrp := createActualLRP(key, endpoint1, domain)
						tempTable.SetRoutes(logger, nil, desiredLRP)
						tempTable.AddEndpoint(logger, lrp)

						_, messagesToEmit = table.Swap(logger, tempTable, domains)
					})

					It("emits registrations for each pairing", func() {
						expected := routingtable.MessagesToEmit{
							RegistrationMessages: []routingtable.RegistryMessage{
								routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, true),
							},
							InternalRegistrationMessages: []routingtable.RegistryMessage{
								{
									Host:                 endpoint1.ContainerIP,
									URIs:                 []string{internalHostname1, fmt.Sprintf("%d.%s", 0, internalHostname1)},
									PrivateInstanceIndex: "0",
									App:                  logGuid,
									EndpointUpdatedAtNs:  endpoint1.Since,
									Tags: map[string]string{
										"component": "route-emitter",
									},
								},
							},
						}
						Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
					})
				})

				Context("when the routing key subsequently disappears", func() {
					BeforeEach(func() {
						tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
						_, messagesToEmit = table.Swap(logger, tempTable, domains)
					})

					It("emits nothing", func() {
						Expect(messagesToEmit).To(BeZero())
					})
				})
			})

			Context("when the process only has endpoints", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					lrp := createActualLRP(key, endpoint1, domain)
					tempTable.AddEndpoint(logger, lrp)

					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("should not emit a registration", func() {
					Expect(messagesToEmit).To(BeZero())
				})

				Context("when the routes subsequently arrive", func() {
					BeforeEach(func() {
						tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
						routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{internalHostname1}, "", []uint32{}, "")
						desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
						lrp := createActualLRP(key, endpoint1, domain)
						tempTable.SetRoutes(logger, nil, desiredLRP)
						tempTable.AddEndpoint(logger, lrp)

						_, messagesToEmit = table.Swap(logger, tempTable, domains)
					})

					It("emits registrations for each pairing", func() {
						expected := routingtable.MessagesToEmit{
							RegistrationMessages: []routingtable.RegistryMessage{
								routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
							},
							InternalRegistrationMessages: []routingtable.RegistryMessage{
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

				Context("when the endpoint subsequently disappears", func() {
					BeforeEach(func() {
						tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
						_, messagesToEmit = table.Swap(logger, tempTable, domains)
					})

					It("emits nothing", func() {
						Expect(messagesToEmit).To(BeZero())
					})
				})
			})
		})

		Context("when there is an existing routing key with an isolation segment", func() {
			var (
				desiredLRP *models.DesiredLRP
			)

			BeforeEach(func() {
				tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
				desiredLRP = createDesiredLRPWithIS("isolation-segment-1")
				tempTable.SetRoutes(logger, nil, desiredLRP)
				lrp := createActualLRP(key, endpoint1, domain)
				tempTable.AddEndpoint(logger, lrp)
				table.Swap(logger, tempTable, domains)
			})

			Context("when the isolation segment changes in an event", func() {
				BeforeEach(func() {
					afterDesiredLRP := createDesiredLRPWithIS("isolation-segment-2")
					afterDesiredLRP.ModificationTag.Index++
					_, messagesToEmit = table.SetRoutes(logger, desiredLRP, afterDesiredLRP)
				})

				It("emits a registration and unregistration", func() {
					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, IsolationSegment: "isolation-segment-2"}, false),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, IsolationSegment: "isolation-segment-2"}, false),
						},
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, IsolationSegment: "isolation-segment-1"}, false),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, IsolationSegment: "isolation-segment-1"}, false),
						},
					}
					Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
				})
			})

			Context("when the isolation segment changes in sync", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					desiredLRP := createDesiredLRPWithIS("isolation-segment-2")
					tempTable.SetRoutes(logger, nil, desiredLRP)
					lrp := createActualLRP(key, endpoint1, domain)
					tempTable.AddEndpoint(logger, lrp)
					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("emits all registrations and no unregistration", func() {
					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, IsolationSegment: "isolation-segment-2"}, false),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, IsolationSegment: "isolation-segment-2"}, false),
						},
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, IsolationSegment: "isolation-segment-1"}, false),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, IsolationSegment: "isolation-segment-1"}, false),
						},
					}
					Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
				})
			})
		})

		Context("when there is an existing routing key with a route service url", func() {
			var (
				desiredLRP *models.DesiredLRP
			)

			BeforeEach(func() {
				tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
				desiredLRP = createDesiredLRPWithFixtures("https://rs.example.com")
				tempTable.SetRoutes(logger, nil, desiredLRP)
				lrp := createActualLRP(key, endpoint1, domain)
				tempTable.AddEndpoint(logger, lrp)
				table.Swap(logger, tempTable, domains)
			})

			Context("when the route service url changes in an event", func() {
				BeforeEach(func() {
					afterDesiredLRP := createDesiredLRPWithFixtures("https://rs.new.example.com")
					afterDesiredLRP.ModificationTag.Index++
					_, messagesToEmit = table.SetRoutes(logger, desiredLRP, afterDesiredLRP)
				})

				It("emits all registrations and no unregistration", func() {
					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, RouteServiceUrl: "https://rs.new.example.com"}, false),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, RouteServiceUrl: "https://rs.new.example.com"}, false),
						},
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, false),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, false),
						},
					}
					Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
				})
			})

			Context("when the route service url changes during sync", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					desiredLRP := createDesiredLRPWithFixtures("https://rs.new.example.com")
					tempTable.SetRoutes(logger, nil, desiredLRP)
					lrp1 := createActualLRP(key, endpoint1, domain)
					tempTable.AddEndpoint(logger, lrp1)
					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("emits all registrations and no unregistration", func() {
					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, RouteServiceUrl: "https://rs.new.example.com"}, false),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, RouteServiceUrl: "https://rs.new.example.com"}, false),
						},
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, false),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, false),
						},
					}
					Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
				})
			})
		})

		Context("when the routing key has an evacuating and instance endpoint", func() {
			BeforeEach(func() {
				tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
				routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")
				desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
				tempTable.SetRoutes(logger, nil, desiredLRP)
				evacuating := createActualLRP(key, evacuating1, domain)
				tempTable.AddEndpoint(logger, evacuating)
				lrp2 := createActualLRP(key, endpoint2, domain)
				tempTable.AddEndpoint(logger, lrp2)

				_, messagesToEmit = table.Swap(logger, tempTable, domains)
			})

			It("should not emit an unregistration ", func() {
				expected := routingtable.MessagesToEmit{
					RegistrationMessages: []routingtable.RegistryMessage{
						routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
						routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
						routingtable.RegistryMessageFor(evacuating1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
						routingtable.RegistryMessageFor(evacuating1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
					},
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
							Host:                 evacuating1.ContainerIP,
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

		Context("when there is an existing routing key", func() {
			BeforeEach(func() {
				tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
				routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")
				desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
				tempTable.SetRoutes(logger, nil, desiredLRP)
				lrp1 := createActualLRP(key, endpoint1, domain)
				tempTable.AddEndpoint(logger, lrp1)
				lrp2 := createActualLRP(key, endpoint2, domain)
				tempTable.AddEndpoint(logger, lrp2)

				table.Swap(logger, tempTable, domains)
			})

			Context("when nothing changes", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
					tempTable.SetRoutes(logger, nil, desiredLRP)
					lrp1 := createActualLRP(key, endpoint1, domain)
					tempTable.AddEndpoint(logger, lrp1)
					lrp2 := createActualLRP(key, endpoint2, domain)
					tempTable.AddEndpoint(logger, lrp2)

					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("emits nothing", func() {
					Expect(messagesToEmit).To(BeZero())
				})
			})

			Context("when the routing key gets new routes", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2, hostname3}, []string{internalHostname1, internalHostname2}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
					tempTable.SetRoutes(logger, nil, desiredLRP)
					lrp1 := createActualLRP(key, endpoint1, domain)
					tempTable.AddEndpoint(logger, lrp1)
					lrp2 := createActualLRP(key, endpoint2, domain)
					tempTable.AddEndpoint(logger, lrp2)

					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("emits only the new route", func() {
					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname3, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname3, LogGUID: logGuid}, false),
						},
						InternalRegistrationMessages: []routingtable.RegistryMessage{
							{
								Host:                 endpoint2.ContainerIP,
								URIs:                 []string{internalHostname2, fmt.Sprintf("%d.%s", 1, internalHostname2)},
								PrivateInstanceIndex: "1",
								App:                  logGuid,
								Tags: map[string]string{
									"component": "route-emitter",
								},
							},
							{
								Host:                 endpoint1.ContainerIP,
								URIs:                 []string{internalHostname2, fmt.Sprintf("%d.%s", 0, internalHostname2)},
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

			Context("when the routing key without any route service url gets routes with a new route service url", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "https://rs.example.com", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
					tempTable.SetRoutes(logger, nil, desiredLRP)
					lrp1 := createActualLRP(key, endpoint1, domain)
					tempTable.AddEndpoint(logger, lrp1)
					lrp2 := createActualLRP(key, endpoint2, domain)
					tempTable.AddEndpoint(logger, lrp2)

					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("emits registrations and unregistration", func() {
					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, false),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, false),
						},
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, RouteServiceUrl: ""}, false),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, RouteServiceUrl: ""}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, RouteServiceUrl: ""}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, RouteServiceUrl: ""}, false),
						},
					}
					Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
				})
			})

			Context("when the routing key gets new endpoints", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
					tempTable.SetRoutes(logger, nil, desiredLRP)
					lrp1 := createActualLRP(key, endpoint1, domain)
					tempTable.AddEndpoint(logger, lrp1)
					lrp2 := createActualLRP(key, endpoint2, domain)
					tempTable.AddEndpoint(logger, lrp2)
					lrp3 := createActualLRP(key, endpoint3, domain)
					tempTable.AddEndpoint(logger, lrp3)

					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("emits only the new registrations and no unregistration", func() {
					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint3, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, true),
							routingtable.RegistryMessageFor(endpoint3, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, true),
						},
						InternalRegistrationMessages: []routingtable.RegistryMessage{
							{
								Host:                 endpoint3.ContainerIP,
								URIs:                 []string{internalHostname1, fmt.Sprintf("%d.%s", 2, internalHostname1)},
								PrivateInstanceIndex: "2",
								EndpointUpdatedAtNs:  endpoint3.Since,
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

			Context("when the routing key gets a new evacuating endpoint", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
					tempTable.SetRoutes(logger, nil, desiredLRP)
					lrp1 := createActualLRP(key, endpoint1, domain)
					tempTable.AddEndpoint(logger, lrp1)
					lrp2 := createActualLRP(key, endpoint2, domain)
					tempTable.AddEndpoint(logger, lrp2)
					evacuating := createActualLRP(key, evacuating1, domain)
					tempTable.AddEndpoint(logger, evacuating)

					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("emits no unregistration", func() {
					Expect(messagesToEmit).To(BeZero())
				})

				Context("when running instance is removed", func() {
					BeforeEach(func() {
						tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
						routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")
						desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
						tempTable.SetRoutes(logger, nil, desiredLRP)
						lrp2 := createActualLRP(key, endpoint2, domain)
						tempTable.AddEndpoint(logger, lrp2)
						evacuating := createActualLRP(key, evacuating1, domain)
						tempTable.AddEndpoint(logger, evacuating)

						_, messagesToEmit = table.Swap(logger, tempTable, domains)
					})

					It("emits no unregistration", func() {
						Expect(messagesToEmit).To(BeZero())
					})
				})
			})

			Context("when the routing key gets new routes and endpoints", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2, hostname3}, []string{internalHostname1, internalHostname2}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
					tempTable.SetRoutes(logger, nil, desiredLRP)
					lrp1 := createActualLRP(key, endpoint1, domain)
					tempTable.AddEndpoint(logger, lrp1)
					lrp2 := createActualLRP(key, endpoint2, domain)
					tempTable.AddEndpoint(logger, lrp2)
					lrp3 := createActualLRP(key, endpoint3, domain)
					tempTable.AddEndpoint(logger, lrp3)

					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("emits the relevant registrations and no unregisration", func() {
					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname3, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname3, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint3, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, true),
							routingtable.RegistryMessageFor(endpoint3, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, true),
							routingtable.RegistryMessageFor(endpoint3, routingtable.Route{Hostname: hostname3, LogGUID: logGuid}, false),
						},
						InternalRegistrationMessages: []routingtable.RegistryMessage{
							{
								Host:                 endpoint3.ContainerIP,
								URIs:                 []string{internalHostname1, fmt.Sprintf("%d.%s", 2, internalHostname1)},
								PrivateInstanceIndex: "2",
								App:                  logGuid,
								EndpointUpdatedAtNs:  endpoint3.Since,
								Tags: map[string]string{
									"component": "route-emitter",
								},
							},
							{
								Host:                 endpoint3.ContainerIP,
								URIs:                 []string{internalHostname2, fmt.Sprintf("%d.%s", 2, internalHostname2)},
								PrivateInstanceIndex: "2",
								App:                  logGuid,
								Tags: map[string]string{
									"component": "route-emitter",
								},
							},
							{
								Host:                 endpoint2.ContainerIP,
								URIs:                 []string{internalHostname2, fmt.Sprintf("%d.%s", 1, internalHostname2)},
								PrivateInstanceIndex: "1",
								App:                  logGuid,
								Tags: map[string]string{
									"component": "route-emitter",
								},
							},
							{
								Host:                 endpoint1.ContainerIP,
								URIs:                 []string{internalHostname2, fmt.Sprintf("%d.%s", 0, internalHostname2)},
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

			Context("when the routing key loses routes", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
					tempTable.SetRoutes(logger, nil, desiredLRP)
					lrp1 := createActualLRP(key, endpoint1, domain)
					tempTable.AddEndpoint(logger, lrp1)
					lrp2 := createActualLRP(key, endpoint2, domain)
					tempTable.AddEndpoint(logger, lrp2)

					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("emits the relevant unregistrations", func() {
					expected := routingtable.MessagesToEmit{
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
						},
						InternalUnregistrationMessages: []routingtable.RegistryMessage{
							{
								Host:                 endpoint1.ContainerIP,
								URIs:                 []string{internalHostname1, fmt.Sprintf("%d.%s", 0, internalHostname1)},
								PrivateInstanceIndex: "0",
								App:                  logGuid,
								Tags: map[string]string{
									"component": "route-emitter",
								},
							},
							{
								Host:                 endpoint2.ContainerIP,
								URIs:                 []string{internalHostname1, fmt.Sprintf("%d.%s", 1, internalHostname1)},
								PrivateInstanceIndex: "1",
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

			Context("when the routing key loses endpoints", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
					tempTable.SetRoutes(logger, nil, desiredLRP)
					lrp1 := createActualLRP(key, endpoint1, domain)
					tempTable.AddEndpoint(logger, lrp1)

					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("emits the relevant unregistrations", func() {
					expected := routingtable.MessagesToEmit{
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
						},
						InternalUnregistrationMessages: []routingtable.RegistryMessage{
							{
								Host:                 endpoint2.ContainerIP,
								URIs:                 []string{internalHostname1, fmt.Sprintf("%d.%s", 1, internalHostname1)},
								PrivateInstanceIndex: "1",
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

			Context("when the routing key loses http/internal routes and endpoints", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
					tempTable.SetRoutes(logger, nil, desiredLRP)
					lrp1 := createActualLRP(key, endpoint1, domain)
					tempTable.AddEndpoint(logger, lrp1)

					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("emits no registrations and the relevant unregisrations", func() {
					expected := routingtable.MessagesToEmit{
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
						},
						InternalUnregistrationMessages: []routingtable.RegistryMessage{
							{
								Host:                 endpoint1.ContainerIP,
								URIs:                 []string{internalHostname1, fmt.Sprintf("%d.%s", 0, internalHostname1)},
								PrivateInstanceIndex: "0",
								App:                  logGuid,
								Tags: map[string]string{
									"component": "route-emitter",
								},
							},
							{
								Host:                 endpoint2.ContainerIP,
								URIs:                 []string{internalHostname1, fmt.Sprintf("%d.%s", 1, internalHostname1)},
								PrivateInstanceIndex: "1",
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

			Context("when the routing key gains routes but loses endpoints", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2, hostname3}, []string{internalHostname1, internalHostname2}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
					tempTable.SetRoutes(logger, nil, desiredLRP)
					lrp1 := createActualLRP(key, endpoint1, domain)
					tempTable.AddEndpoint(logger, lrp1)

					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("emits the relevant registrations and the relevant unregisrations", func() {
					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname3, LogGUID: logGuid}, false),
						},
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
						},
						InternalRegistrationMessages: []routingtable.RegistryMessage{
							{
								Host:                 endpoint1.ContainerIP,
								URIs:                 []string{internalHostname2, fmt.Sprintf("%d.%s", 0, internalHostname2)},
								PrivateInstanceIndex: "0",
								App:                  logGuid,
								Tags: map[string]string{
									"component": "route-emitter",
								},
							},
						},
						InternalUnregistrationMessages: []routingtable.RegistryMessage{
							{
								Host:                 endpoint2.ContainerIP,
								URIs:                 []string{internalHostname1, fmt.Sprintf("%d.%s", 1, internalHostname1)},
								PrivateInstanceIndex: "1",
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

			Context("when the routing key loses routes but gains endpoints", func() {
				BeforeEach(func() {
					tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
					tempTable.SetRoutes(logger, nil, desiredLRP)
					lrp1 := createActualLRP(key, endpoint1, domain)
					tempTable.AddEndpoint(logger, lrp1)
					lrp2 := createActualLRP(key, endpoint2, domain)
					tempTable.AddEndpoint(logger, lrp2)
					lrp3 := createActualLRP(key, endpoint3, domain)
					tempTable.AddEndpoint(logger, lrp3)

					_, messagesToEmit = table.Swap(logger, tempTable, domains)
				})

				It("emits the relevant registrations and the relevant unregisrations", func() {
					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint3, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, true),
						},
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
						},
						InternalUnregistrationMessages: []routingtable.RegistryMessage{
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

			Context("when the routing key disappears entirely", func() {
				var tempTable routingtable.RoutingTable
				var domainSet models.DomainSet

				BeforeEach(func() {
					tempTable = routingtable.NewRoutingTable(false, false, fakeMetronClient)
				})

				JustBeforeEach(func() {
					_, messagesToEmit = table.Swap(logger, tempTable, domainSet)
				})

				Context("when the domain is fresh", func() {
					BeforeEach(func() {
						domainSet = domains
					})

					It("should unregister the missing guids", func() {
						expected := routingtable.MessagesToEmit{
							UnregistrationMessages: []routingtable.RegistryMessage{
								routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
								routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
								routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
								routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
							},
							InternalUnregistrationMessages: []routingtable.RegistryMessage{
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

				Context("when the domain is not fresh", func() {
					BeforeEach(func() {
						domainSet = noFreshDomains
					})

					It("should unregister the missing guids", func() {
						expected := routingtable.MessagesToEmit{
							UnregistrationMessages: []routingtable.RegistryMessage{
								routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
								routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
								routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
								routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
							},
							InternalUnregistrationMessages: []routingtable.RegistryMessage{
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

				Context("when the table is repeatedly swapped", func() {
					JustBeforeEach(func() {
						lrp1 := createActualLRP(key, endpoint1, domain)
						tempTable.AddEndpoint(logger, lrp1)
						lrp2 := createActualLRP(key, endpoint2, domain)
						tempTable.AddEndpoint(logger, lrp2)
						// doing another swap to make sure the old table is still good
						table.Swap(logger, tempTable, domainSet)
						_, messagesToEmit = table.Swap(logger, tempTable, domainSet)
					})

					It("logs the collision", func() {
						lrp := createActualLRP(key, collisionEndpoint, domain)
						table.AddEndpoint(logger, lrp)
						Eventually(logger).Should(Say(
							fmt.Sprintf(
								`\{"Address":\{"Host":"%s","Port":%d\},"instance_guid_a":"%s","instance_guid_b":"%s"`,
								endpoint1.Host,
								endpoint1.Port,
								endpoint1.InstanceGUID,
								collisionEndpoint.InstanceGUID,
							),
						))
					})

					It("should not emit anything since unregistrations were previously sent", func() {
						Expect(messagesToEmit).To(BeZero())
					})
				})
			})

			Describe("edge cases", func() {
				Context("when the original registration had no routes, and then the routing key loses endpoints", func() {
					BeforeEach(func() {
						//override previous set up
						tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
						lrp1 := createActualLRP(key, endpoint1, domain)
						tempTable.AddEndpoint(logger, lrp1)
						lrp2 := createActualLRP(key, endpoint2, domain)
						tempTable.AddEndpoint(logger, lrp2)
						_, messagesToEmit = table.Swap(logger, tempTable, domains)
						Expect(messagesToEmit.InternalUnregistrationMessages).To(HaveLen(2))

						tempTable = routingtable.NewRoutingTable(false, false, fakeMetronClient)
						lrp1 = createActualLRP(key, endpoint1, domain)
						tempTable.AddEndpoint(logger, lrp1)
						_, messagesToEmit = table.Swap(logger, tempTable, domains)
					})

					It("emits nothing", func() {
						Expect(messagesToEmit).To(BeZero())
					})
				})

				Context("when the original registration had no endpoints, and then the routing key loses a route", func() {
					BeforeEach(func() {
						//override previous set up
						tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
						desiredLRP := createDesiredLRP(key.ProcessGUID, int32(3), key.ContainerPort, logGuid, "", *currentTag, models.DesiredLRPRunInfo{}, hostname1, hostname2)
						tempTable.SetRoutes(logger, nil, desiredLRP)
						table.Swap(logger, tempTable, domains)

						tempTable = routingtable.NewRoutingTable(false, false, fakeMetronClient)
						desiredLRP = createDesiredLRP(key.ProcessGUID, int32(3), key.ContainerPort, logGuid, "", *currentTag, models.DesiredLRPRunInfo{}, hostname1)
						tempTable.SetRoutes(logger, nil, desiredLRP)
						_, messagesToEmit = table.Swap(logger, tempTable, domains)
					})

					It("emits nothing", func() {
						Expect(messagesToEmit).To(BeZero())
					})
				})
			})
		})
	})

	Describe("Processing deltas", func() {
		Context("when the table is empty", func() {
			Context("When setting routes", func() {
				It("emits nothing", func() {
					desiredLRP := createDesiredLRP(key.ProcessGUID, int32(3), key.ContainerPort, logGuid, "", *currentTag, runInfo, hostname1, hostname2)
					_, messagesToEmit = table.SetRoutes(logger, nil, desiredLRP)
					Expect(messagesToEmit).To(BeZero())
				})
			})

			Context("when removing routes", func() {
				It("emits nothing", func() {
					desiredLRP := createDesiredLRP(key.ProcessGUID, int32(3), key.ContainerPort, logGuid, "", *currentTag, runInfo, hostname1, hostname2)
					_, messagesToEmit = table.SetRoutes(logger, nil, desiredLRP)
					Expect(messagesToEmit).To(BeZero())
				})
			})

			Context("when adding/updating endpoints", func() {
				It("emits nothing", func() {
					lrp1 := createActualLRP(key, endpoint1, domain)
					_, messagesToEmit := table.AddEndpoint(logger, lrp1)
					Expect(messagesToEmit).To(BeZero())
				})
			})

			Context("when removing endpoints", func() {
				It("emits nothing", func() {
					lrp1 := createActualLRP(key, endpoint1, domain)
					_, messagesToEmit := table.RemoveEndpoint(logger, lrp1)
					Expect(messagesToEmit).To(BeZero())
				})
			})
		})

		Context("when there are both endpoints and routes in the table", func() {
			var beforeLRP *models.DesiredLRP
			BeforeEach(func() {
				tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
				routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")

				beforeLRP = createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
				tempTable.SetRoutes(logger, nil, beforeLRP)
				lrp1 := createActualLRP(key, endpoint1, domain)
				tempTable.AddEndpoint(logger, lrp1)
				lrp2 := createActualLRP(key, endpoint2, domain)
				tempTable.AddEndpoint(logger, lrp2)

				table.Swap(logger, tempTable, domains)
			})

			Describe("SetRoutes", func() {
				It("emits nothing when the route's hostnames do not change", func() {
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
					_, messagesToEmit = table.SetRoutes(logger, beforeLRP, desiredLRP)
					Expect(messagesToEmit).To(BeZero())
				})

				It("emits unregistration and registration when the route service url changes", func() {
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "https://rs.example.com", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *newerTag, runInfo)
					_, messagesToEmit = table.SetRoutes(logger, beforeLRP, desiredLRP)

					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, false),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, false),
						},
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, RouteServiceUrl: ""}, false),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, RouteServiceUrl: ""}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, RouteServiceUrl: ""}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, RouteServiceUrl: ""}, false),
						},
					}
					Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
				})

				It("emits nothing when a hostname is added to a route with an older tag", func() {
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "https://rs.example.com", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *olderTag, runInfo)
					_, messagesToEmit = table.SetRoutes(logger, beforeLRP, desiredLRP)
					Expect(messagesToEmit).To(BeZero())
				})

				It("emits registrations when a hostname is added to a route with a newer tag", func() {
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2, hostname3}, []string{internalHostname1}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *newerTag, runInfo)
					_, messagesToEmit = table.SetRoutes(logger, beforeLRP, desiredLRP)

					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname3, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname3, LogGUID: logGuid}, false),
						},
					}
					Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
				})

				It("emits nothing when a hostname is removed from a route with an older tag", func() {
					desiredLRP := createDesiredLRP(key.ProcessGUID, int32(3), key.ContainerPort, logGuid, "", *olderTag, models.DesiredLRPRunInfo{}, hostname1)
					_, messagesToEmit = table.SetRoutes(logger, beforeLRP, desiredLRP)
					Expect(messagesToEmit).To(BeZero())
				})

				It("emits unregistrations when a hostname is removed from a route with a newer tag", func() {
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *newerTag, runInfo)
					_, messagesToEmit = table.SetRoutes(logger, beforeLRP, desiredLRP)

					expected := routingtable.MessagesToEmit{
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
						},
						InternalUnregistrationMessages: []routingtable.RegistryMessage{
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

				It("emits nothing when hostnames are added and removed from a route with an older tag", func() {
					desiredLRP := createDesiredLRP(key.ProcessGUID, int32(3), key.ContainerPort, logGuid, "", *olderTag, runInfo, hostname1, hostname3)
					_, messagesToEmit = table.SetRoutes(logger, beforeLRP, desiredLRP)
					Expect(messagesToEmit).To(BeZero())
				})

				It("emits registrations and unregistrations when hostnames are added and removed from a route with a newer tag", func() {
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname3}, []string{internalHostname2}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *newerTag, runInfo)
					_, messagesToEmit = table.SetRoutes(logger, beforeLRP, desiredLRP)

					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname3, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname3, LogGUID: logGuid}, false),
						},
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
						},
						InternalRegistrationMessages: []routingtable.RegistryMessage{
							{
								Host:                 endpoint2.ContainerIP,
								URIs:                 []string{internalHostname2, fmt.Sprintf("%d.%s", 1, internalHostname2)},
								PrivateInstanceIndex: "1",
								App:                  logGuid,
								Tags: map[string]string{
									"component": "route-emitter",
								},
							},
							{
								Host:                 endpoint1.ContainerIP,
								URIs:                 []string{internalHostname2, fmt.Sprintf("%d.%s", 0, internalHostname2)},
								PrivateInstanceIndex: "0",
								App:                  logGuid,
								Tags: map[string]string{
									"component": "route-emitter",
								},
							},
						},
						InternalUnregistrationMessages: []routingtable.RegistryMessage{
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

			Context("RemoveRoutes", func() {
				It("emits unregistrations with a newer tag", func() {
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *newerTag, runInfo)
					_, messagesToEmit = table.RemoveRoutes(logger, desiredLRP)

					expected := routingtable.MessagesToEmit{
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
						},
						InternalUnregistrationMessages: []routingtable.RegistryMessage{
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

				It("updates routing table with a newer tag", func() {
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *newerTag, runInfo)
					_, messagesToEmit = table.RemoveRoutes(logger, desiredLRP)
					Expect(table.HTTPAssociationsCount()).To(Equal(0))
					Expect(table.InternalAssociationsCount()).To(Equal(0))
				})

				It("emits unregistrations with the same tag", func() {
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
					_, messagesToEmit = table.RemoveRoutes(logger, desiredLRP)

					expected := routingtable.MessagesToEmit{
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
						},
						InternalUnregistrationMessages: []routingtable.RegistryMessage{
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

				It("updates routing table with a same tag", func() {
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{internalHostname1}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
					_, messagesToEmit = table.RemoveRoutes(logger, desiredLRP)
					Expect(table.HTTPAssociationsCount()).To(Equal(0))
					Expect(table.InternalAssociationsCount()).To(Equal(0))
				})

				It("emits nothing when the tag is older", func() {
					routes := createRoutingInfo(key.ContainerPort, []string{hostname1, hostname2}, []string{}, "", []uint32{}, "")
					desiredLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *olderTag, runInfo)
					_, messagesToEmit = table.RemoveRoutes(logger, desiredLRP)
					Expect(messagesToEmit).To(BeZero())
				})

				It("does NOT update routing table with an older tag", func() {
					beforeRouteCount := table.HTTPAssociationsCount()
					beforeInternalRouteCount := table.InternalAssociationsCount()
					desiredLRP := createDesiredLRP(key.ProcessGUID, int32(3), key.ContainerPort, logGuid, "", *olderTag, runInfo, hostname1, hostname2)
					_, messagesToEmit = table.RemoveRoutes(logger, desiredLRP)
					Expect(table.HTTPAssociationsCount()).To(Equal(beforeRouteCount))
					Expect(table.InternalAssociationsCount()).To(Equal(beforeInternalRouteCount))
				})
			})

			Context("AddEndpoint", func() {
				It("emits nothing when the tag is the same", func() {
					lrp1 := createActualLRP(key, endpoint1, domain)
					_, messagesToEmit := table.AddEndpoint(logger, lrp1)
					Expect(messagesToEmit).To(BeZero())
				})

				It("emits nothing when updating an endpoint with an older tag", func() {
					updatedEndpoint := endpoint1
					updatedEndpoint.ModificationTag = olderTag
					lrp1 := createActualLRP(key, updatedEndpoint, domain)
					_, messagesToEmit := table.AddEndpoint(logger, lrp1)

					Expect(messagesToEmit).To(BeZero())
				})

				It("emits nothing when updating an endpoint with a newer tag", func() {
					updatedEndpoint := endpoint1
					updatedEndpoint.ModificationTag = newerTag
					lrp1 := createActualLRP(key, updatedEndpoint, domain)
					_, messagesToEmit := table.AddEndpoint(logger, lrp1)
					Expect(messagesToEmit).To(BeZero())
				})

				It("emits registrations when adding an endpoint", func() {
					lrp1 := createActualLRP(key, endpoint3, domain)
					_, messagesToEmit = table.AddEndpoint(logger, lrp1)

					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint3, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, true),
							routingtable.RegistryMessageFor(endpoint3, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, true),
						},
						InternalRegistrationMessages: []routingtable.RegistryMessage{
							{
								Host:                 endpoint3.ContainerIP,
								URIs:                 []string{internalHostname1, fmt.Sprintf("%d.%s", 2, internalHostname1)},
								PrivateInstanceIndex: "2",
								App:                  logGuid,
								EndpointUpdatedAtNs:  endpoint3.Since,
								Tags: map[string]string{
									"component": "route-emitter",
								},
							},
						},
					}
					Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
				})

				It("does not log a collision", func() {
					lrp := createActualLRP(key, endpoint3, domain)
					table.AddEndpoint(logger, lrp)
					Consistently(logger).ShouldNot(Say("collision-detected-with-endpoint"))
				})

				Context("when adding an endpoint with IP and port that collide with existing endpoint", func() {
					var counterChan chan string
					BeforeEach(func() {
						counterChan = make(chan string, 10)
						fakeMetronClient.IncrementCounterStub = func(name string) error {
							counterChan <- name
							return nil
						}

						lrp := createActualLRP(key, collisionEndpoint, domain)
						table.AddEndpoint(logger, lrp)
					})

					It("logs the collision", func() {
						Eventually(logger).Should(Say(
							fmt.Sprintf(
								`\{"Address":\{"Host":"%s","Port":%d\},"instance_guid_a":"%s","instance_guid_b":"%s"`,
								endpoint1.Host,
								endpoint1.Port,
								endpoint1.InstanceGUID,
								collisionEndpoint.InstanceGUID,
							),
						))
					})

					It("emits metrics about the address collisions", func() {
						Eventually(counterChan).Should(Receive(Equal("AddressCollisions")))
						Consistently(counterChan).ShouldNot(Receive())
					})
				})

				Context("when an evacuating endpoint is added for an instance that already exists", func() {
					It("emits nothing", func() {
						lrp1 := createActualLRP(key, evacuating1, domain)
						_, messagesToEmit = table.AddEndpoint(logger, lrp1)
						Expect(messagesToEmit).To(BeZero())
					})
				})

				Context("when an instance endpoint is updated for an evacuating that already exists", func() {
					BeforeEach(func() {
						lrp1 := createActualLRP(key, evacuating1, domain)
						_, messagesToEmit = table.AddEndpoint(logger, lrp1)
						table.AddEndpoint(logger, lrp1)
					})

					It("emits nothing", func() {
						lrp2 := createActualLRP(key, endpoint1, domain)
						_, messagesToEmit = table.AddEndpoint(logger, lrp2)
						Expect(messagesToEmit).To(BeZero())
					})
				})

				Context("when there are internal routes", func() {
					var internalHostname string
					BeforeEach(func() {
						tempTable := routingtable.NewRoutingTable(false, false, fakeMetronClient)
						internalHostname = "internal"
						routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{internalHostname}, "", []uint32{}, "")

						beforeLRP = createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
						tempTable.SetRoutes(logger, nil, beforeLRP)
						lrp1 := createActualLRP(key, endpoint1, domain)
						tempTable.AddEndpoint(logger, lrp1)
						lrp2 := createActualLRP(key, endpoint2, domain)
						tempTable.AddEndpoint(logger, lrp2)

						table.Swap(logger, tempTable, domains)
					})

					It("emits registrations when adding an endpoint", func() {
						lrp3 := createActualLRP(key, endpoint3, domain)
						_, messagesToEmit = table.AddEndpoint(logger, lrp3)

						expected := []routingtable.RegistryMessage{
							{
								Host:                 endpoint3.ContainerIP,
								URIs:                 []string{internalHostname, fmt.Sprintf("%d.%s", 2, internalHostname)},
								PrivateInstanceIndex: "2",
								App:                  logGuid,
								EndpointUpdatedAtNs:  endpoint3.Since,
								Tags: map[string]string{
									"component": "route-emitter",
								},
							},
						}

						Expect(messagesToEmit.InternalRegistrationMessages).To(Equal(expected))
					})

					Context("when the instance container port changes", func() {
						var changedEndpoint routingtable.Endpoint
						BeforeEach(func() {
							changedEndpoint = endpoint2
							changedEndpoint.ContainerPort = 1234
							changedEndpoint.ModificationTag = newerTag
						})

						It("emits nothing", func() {
							lrp := createActualLRP(key, changedEndpoint, domain)
							_, messagesToEmit := table.AddEndpoint(logger, lrp)

							Expect(messagesToEmit.InternalRegistrationMessages).To(BeEmpty())
							Expect(messagesToEmit.InternalUnregistrationMessages).To(BeEmpty())
						})
					})

					Context("when the instance host port changes", func() {
						var changedEndpoint routingtable.Endpoint
						BeforeEach(func() {
							changedEndpoint = endpoint2
							changedEndpoint.Port = 1234
							changedEndpoint.ModificationTag = newerTag
						})

						It("emits nothing", func() {
							lrp := createActualLRP(key, changedEndpoint, domain)
							_, messagesToEmit := table.AddEndpoint(logger, lrp)

							Expect(messagesToEmit.InternalRegistrationMessages).To(BeEmpty())
							Expect(messagesToEmit.InternalUnregistrationMessages).To(BeEmpty())
						})
					})

					Context("when an evacuating endpoint is added for an instance that already exists", func() {
						It("emits nothing", func() {
							lrp1 := createActualLRP(key, evacuating1, domain)
							_, messagesToEmit = table.AddEndpoint(logger, lrp1)
							Expect(messagesToEmit).To(BeZero())
						})
					})

					Context("when an instance endpoint is updated for an evacuating that already exists", func() {
						BeforeEach(func() {
							lrp1 := createActualLRP(key, evacuating1, domain)
							_, messagesToEmit = table.AddEndpoint(logger, lrp1)
							table.AddEndpoint(logger, lrp1)
						})

						It("emits nothing", func() {
							lrp2 := createActualLRP(key, endpoint1, domain)
							_, messagesToEmit = table.AddEndpoint(logger, lrp2)
							Expect(messagesToEmit).To(BeZero())
						})
					})
				})
			})

			Context("RemoveEndpoint", func() {
				It("emits unregistrations with the same tag", func() {
					lrp1 := createActualLRP(key, endpoint2, domain)
					_, messagesToEmit = table.RemoveEndpoint(logger, lrp1)

					expected := routingtable.MessagesToEmit{
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
						},
						InternalUnregistrationMessages: []routingtable.RegistryMessage{
							{
								Host:                 endpoint2.ContainerIP,
								URIs:                 []string{internalHostname1, fmt.Sprintf("%d.%s", 1, internalHostname1)},
								PrivateInstanceIndex: "1",
								App:                  logGuid,
								Tags: map[string]string{
									"component": "route-emitter",
								},
							},
						},
					}
					Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
				})

				It("emits unregistrations when the tag is newer", func() {
					newerEndpoint := endpoint2
					newerEndpoint.ModificationTag = newerTag
					lrp1 := createActualLRP(key, newerEndpoint, domain)
					_, messagesToEmit = table.RemoveEndpoint(logger, lrp1)

					expected := routingtable.MessagesToEmit{
						UnregistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid}, false),
						},
						InternalUnregistrationMessages: []routingtable.RegistryMessage{
							{
								Host:                 endpoint2.ContainerIP,
								URIs:                 []string{internalHostname1, fmt.Sprintf("%d.%s", 1, internalHostname1)},
								PrivateInstanceIndex: "1",
								App:                  logGuid,
								Tags: map[string]string{
									"component": "route-emitter",
								},
							},
						},
					}
					Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
				})

				Context("when the instance has multiple ports, one of which has no routes", func() {
					var (
						lrp *models.ActualLRP
					)

					BeforeEach(func() {
						table = routingtable.NewRoutingTable(false, false, fakeMetronClient)
						routes := createRoutingInfo(key.ContainerPort, []string{hostname1}, []string{internalHostname1}, "", []uint32{}, "")

						beforeLRP := createDesiredLRPWithRoutes(key.ProcessGUID, 3, routes, logGuid, *currentTag, runInfo)
						table.SetRoutes(logger, nil, beforeLRP)
						lrp = createActualLRPWithPortMappings(key, endpoint1, domain,
							models.NewPortMapping(endpoint1.Port+1, 2222),
							models.NewPortMapping(endpoint1.Port, 8080),
						)
						table.AddEndpoint(logger, lrp)
					})

					It("emits unregistration message", func() {
						_, messages := table.RemoveEndpoint(logger, lrp)
						expected := routingtable.MessagesToEmit{
							UnregistrationMessages: []routingtable.RegistryMessage{
								routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid}, false),
							},
							InternalUnregistrationMessages: []routingtable.RegistryMessage{
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
						Expect(messages).To(MatchMessagesToEmit(expected))
					})
				})

				It("emits nothing when the tag is older", func() {
					olderEndpoint := endpoint2
					olderEndpoint.ModificationTag = olderTag
					lrp1 := createActualLRP(key, olderEndpoint, domain)
					_, messagesToEmit = table.RemoveEndpoint(logger, lrp1)
					Expect(messagesToEmit).To(BeZero())
				})

				Context("when an instance endpoint is removed for an instance that already exists", func() {
					BeforeEach(func() {
						lrp1 := createActualLRP(key, evacuating1, domain)
						_, messagesToEmit := table.AddEndpoint(logger, lrp1)

						Expect(messagesToEmit).To(BeZero())
					})

					It("emits nothing", func() {
						lrp2 := createActualLRP(key, endpoint1, domain)
						_, messagesToEmit = table.RemoveEndpoint(logger, lrp2)
						Expect(messagesToEmit).To(BeZero())
					})
				})

				Context("when a collision is avoided because the endpoint has already been removed", func() {
					It("does not log the collision", func() {
						lrp := createActualLRP(key, endpoint1, domain)
						table.RemoveEndpoint(logger, lrp)
						lrp = createActualLRP(key, collisionEndpoint, domain)
						table.AddEndpoint(logger, lrp)
						Consistently(logger).ShouldNot(Say("collision-detected-with-endpoint"))
					})
				})

				Context("when removing an endpoint that has a collision", func() {
					It("does logs the collision", func() {
						lrp := createActualLRP(key, collisionEndpoint, domain)
						table.RemoveEndpoint(logger, lrp)
						Eventually(logger).Should(Say("collision-detected-with-endpoint"))
					})
				})
			})
		})

		Context("when there are only routes in the table", func() {
			var beforeDesiredLRP *models.DesiredLRP

			BeforeEach(func() {
				beforeDesiredLRP = createDesiredLRPWithFixtures("https://rs.example.com")
				table.SetRoutes(logger, nil, beforeDesiredLRP)
			})

			Context("When setting routes", func() {
				It("emits nothing", func() {
					afterDesiredLRP := createDesiredLRP(key.ProcessGUID, int32(3), key.ContainerPort, logGuid, "", *currentTag, runInfo, hostname1, hostname3)

					table.SetRoutes(logger, nil, beforeDesiredLRP)
					_, messagesToEmit = table.SetRoutes(logger, beforeDesiredLRP, afterDesiredLRP)
					Expect(messagesToEmit).To(BeZero())
				})
			})

			Context("when removing routes", func() {
				It("emits nothing", func() {
					_, messagesToEmit = table.RemoveRoutes(logger, beforeDesiredLRP)
					Expect(messagesToEmit).To(BeZero())
				})
			})

			Context("when adding/updating endpoints", func() {
				It("emits registrations", func() {
					lrp1 := createActualLRP(key, endpoint1, domain)
					_, messagesToEmit = table.AddEndpoint(logger, lrp1)

					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, true),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, true),
						},
					}
					Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
				})
			})
		})

		Context("when there are only endpoints in the table", func() {
			var beforeLRP *models.DesiredLRP
			var lrp1, lrp2 *models.ActualLRP
			BeforeEach(func() {
				lrp1 = createActualLRP(key, endpoint1, domain)
				lrp2 = createActualLRP(key, endpoint2, domain)
				table.AddEndpoint(logger, lrp1)
				table.AddEndpoint(logger, lrp2)
				beforeLRP = createDesiredLRPWithFixtures("https://rs.example.com")
			})

			Context("When setting routes", func() {
				It("emits registrations", func() {
					_, messagesToEmit = table.SetRoutes(logger, nil, beforeLRP)

					expected := routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, false),
							routingtable.RegistryMessageFor(endpoint1, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname1, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, false),
							routingtable.RegistryMessageFor(endpoint2, routingtable.Route{Hostname: hostname2, LogGUID: logGuid, RouteServiceUrl: "https://rs.example.com"}, false),
						},
					}
					Expect(messagesToEmit).To(MatchMessagesToEmit(expected))
				})
			})

			Context("when removing routes", func() {
				It("emits nothing", func() {
					_, messagesToEmit = table.RemoveRoutes(logger, beforeLRP)
					Expect(messagesToEmit).To(BeZero())
				})
			})

			Context("when adding/updating endpoints", func() {
				It("emits nothing", func() {
					_, messagesToEmit = table.AddEndpoint(logger, lrp2)
					Expect(messagesToEmit).To(BeZero())
				})
			})

			Context("when removing endpoints", func() {
				It("emits nothing", func() {
					_, messagesToEmit = table.RemoveEndpoint(logger, lrp1)
					Expect(messagesToEmit).To(BeZero())
				})
			})
		})
	})

	Describe("HasExternalRoutes", func() {
		It("returns true if the actual lrp has external routes ", func() {
			beforeLRP := createDesiredLRP(key.ProcessGUID, int32(3), key.ContainerPort, logGuid, "", *currentTag, runInfo, hostname1, hostname2)
			table.SetRoutes(logger, nil, beforeLRP)
			lrp1 := createActualLRP(key, endpoint1, domain)
			table.AddEndpoint(logger, lrp1)
			Expect(table.HasExternalRoutes(lrp1)).To(BeTrue())
		})
	})

	Describe("RouteCount", func() {
		It("returns 0 on a new routing table", func() {
			Expect(table.HTTPAssociationsCount()).To(Equal(0))
		})

		It("returns 1 after adding a route to a single process", func() {
			desiredLRP := createDesiredLRP("fake-process-guid", int32(3), 0, logGuid, "", *currentTag, runInfo, "fake-route-url")
			table.SetRoutes(logger, nil, desiredLRP)
			lrp := createActualLRP(routingtable.RoutingKey{ProcessGUID: "fake-process-guid"}, routingtable.Endpoint{InstanceGUID: "fake-instance-guid", ModificationTag: currentTag}, domain)
			table.AddEndpoint(logger, lrp)

			Expect(table.HTTPAssociationsCount()).To(Equal(1))
		})

		It("returns 2 after associating 2 urls with a single process", func() {
			desiredLRP := createDesiredLRP("fake-process-guid", int32(3), 0, logGuid, "", *currentTag, runInfo, "fake-route-url-1", "fake-route-url-2")
			table.SetRoutes(logger, nil, desiredLRP)
			lrp := createActualLRP(routingtable.RoutingKey{ProcessGUID: "fake-process-guid"}, routingtable.Endpoint{InstanceGUID: "fake-instance-guid-1", ModificationTag: currentTag}, domain)
			table.AddEndpoint(logger, lrp)

			Expect(table.HTTPAssociationsCount()).To(Equal(2))
		})

		It("returns 8 after associating 2 urls with 2 processes with 2 instances each", func() {
			desiredLRP := createDesiredLRP("fake-process-guid-a", int32(3), 0, logGuid, "", *currentTag, runInfo, "fake-route-url-a1", "fake-route-url-a2")
			table.SetRoutes(logger, nil, desiredLRP)
			lrp := createActualLRP(routingtable.RoutingKey{ProcessGUID: "fake-process-guid-a"}, routingtable.Endpoint{InstanceGUID: "fake-instance-guid-a1", ModificationTag: currentTag}, domain)
			table.AddEndpoint(logger, lrp)
			lrp = createActualLRP(routingtable.RoutingKey{ProcessGUID: "fake-process-guid-a"}, routingtable.Endpoint{InstanceGUID: "fake-instance-guid-a2", ModificationTag: currentTag}, domain)
			table.AddEndpoint(logger, lrp)

			desiredLRP = createDesiredLRP("fake-process-guid-b", int32(3), 0, logGuid, "", *currentTag, runInfo, "fake-route-url-b1", "fake-route-url-b2")
			table.SetRoutes(logger, nil, desiredLRP)
			lrp = createActualLRP(routingtable.RoutingKey{ProcessGUID: "fake-process-guid-b"}, routingtable.Endpoint{InstanceGUID: "fake-instance-guid-b1", ModificationTag: currentTag}, domain)
			table.AddEndpoint(logger, lrp)
			lrp = createActualLRP(routingtable.RoutingKey{ProcessGUID: "fake-process-guid-b"}, routingtable.Endpoint{InstanceGUID: "fake-instance-guid-b2", ModificationTag: currentTag}, domain)
			table.AddEndpoint(logger, lrp)

			Expect(table.HTTPAssociationsCount()).To(Equal(8))
		})
	})
})
