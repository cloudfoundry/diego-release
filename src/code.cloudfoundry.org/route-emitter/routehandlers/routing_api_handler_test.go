package routehandlers_test

import (
	"code.cloudfoundry.org/bbs/models"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	loggregator "code.cloudfoundry.org/go-loggregator/v9"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	emitterfakes "code.cloudfoundry.org/route-emitter/emitter/fakes"
	"code.cloudfoundry.org/route-emitter/routehandlers"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/routingtable/fakeroutingtable"
	"code.cloudfoundry.org/route-emitter/unregistration"
	ufakes "code.cloudfoundry.org/route-emitter/unregistration/fakes"
	tcpmodels "code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/routing-info/tcp_routes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RoutingAPIHandler", func() {
	var (
		logger                  lager.Logger
		fakeRoutingTable        *fakeroutingtable.FakeRoutingTable
		fakeRoutingAPIEmitter   *emitterfakes.FakeRoutingAPIEmitter
		routeHandler            *routehandlers.Handler
		emptyNatsMessages       routingtable.MessagesToEmit
		fakeMetronClient        *mfakes.FakeIngressClient
		fakeUnregistrationCache unregistration.Cache
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		emptyNatsMessages = routingtable.MessagesToEmit{}
		fakeRoutingTable = new(fakeroutingtable.FakeRoutingTable)
		fakeRoutingAPIEmitter = new(emitterfakes.FakeRoutingAPIEmitter)
		fakeMetronClient = &mfakes.FakeIngressClient{}
		fakeUnregistrationCache = &ufakes.FakeCache{}
		routeHandler = routehandlers.NewHandler(fakeRoutingTable, nil, fakeRoutingAPIEmitter, false, false, fakeMetronClient, fakeUnregistrationCache)
	})

	Describe("DesiredLRP Event", func() {
		var (
			desiredLRP    *models.DesiredLRP
			routingEvents routingtable.TCPRouteMappings
		)

		BeforeEach(func() {
			externalPort := uint32(61000)
			containerPort := uint32(5222)
			tcpRoutes := tcp_routes.TCPRoutes{
				tcp_routes.TCPRoute{
					ExternalPort:  externalPort,
					ContainerPort: containerPort,
				},
			}
			desiredLRP = &models.DesiredLRP{
				ProcessGuid: "process-guid-1",
				Ports:       []uint32{containerPort},
				LogGuid:     "log-guid",
				Routes:      tcpRoutes.RoutingInfo(),
			}
			routingEvents = routingtable.TCPRouteMappings{
				Registrations: []tcpmodels.TcpRouteMapping{},
			}
		})

		Describe("HandleDesiredCreate", func() {
			JustBeforeEach(func() {
				routeHandler.HandleEvent(logger, models.NewDesiredLRPCreatedEvent(desiredLRP, "some-trace-id"))
			})

			It("invokes AddRoutes on RoutingTable", func() {
				Expect(fakeRoutingTable.SetRoutesCallCount()).Should(Equal(1))
				_, before, after := fakeRoutingTable.SetRoutesArgsForCall(0)
				Expect(before).To(BeNil())
				Expect(after).Should(Equal(desiredLRP))
			})

			Context("when there are routing events", func() {
				BeforeEach(func() {
					fakeRoutingTable.SetRoutesReturns(routingEvents, emptyNatsMessages)
				})

				It("invokes Emit on Emitter", func() {
					Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
					events := fakeRoutingAPIEmitter.EmitArgsForCall(0)
					Expect(events).Should(Equal(routingEvents))
				})
			})
		})

		Describe("HandleDesiredUpdate", func() {
			var after *models.DesiredLRP

			BeforeEach(func() {
				externalPort := uint32(62000)
				containerPort := uint32(5222)
				tcpRoutes := tcp_routes.TCPRoutes{
					tcp_routes.TCPRoute{
						ExternalPort:  externalPort,
						ContainerPort: containerPort,
					},
				}
				after = &models.DesiredLRP{
					ProcessGuid: "process-guid-1",
					Ports:       []uint32{containerPort},
					LogGuid:     "log-guid",
					Routes:      tcpRoutes.RoutingInfo(),
				}
			})

			JustBeforeEach(func() {
				routeHandler.HandleEvent(logger, models.NewDesiredLRPChangedEvent(desiredLRP, after, "some-trace-id"))
			})

			It("invokes UpdateRoutes on RoutingTable", func() {
				Expect(fakeRoutingTable.SetRoutesCallCount()).Should(Equal(1))
				_, beforeLrp, afterLrp := fakeRoutingTable.SetRoutesArgsForCall(0)
				Expect(beforeLrp).Should(Equal(desiredLRP))
				Expect(afterLrp).Should(Equal(after))
			})

			Context("when there are routing events", func() {
				BeforeEach(func() {
					fakeRoutingTable.SetRoutesReturns(routingEvents, emptyNatsMessages)
				})

				It("invokes Emit on Emitter", func() {
					Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
					events := fakeRoutingAPIEmitter.EmitArgsForCall(0)
					Expect(events).Should(Equal(routingEvents))
				})
			})
		})

		Describe("HandleDesiredDelete", func() {
			BeforeEach(func() {
				unregistrationEvent := routingtable.TCPRouteMappings{
					Unregistrations: []tcpmodels.TcpRouteMapping{},
				}
				fakeRoutingTable.RemoveRoutesReturns(unregistrationEvent, emptyNatsMessages)
			})
			JustBeforeEach(func() {
				routeHandler.HandleEvent(logger, models.NewDesiredLRPRemovedEvent(desiredLRP, "some-trace-id"))
			})

			It("does not invoke AddRoutes on RoutingTable", func() {
				Expect(fakeRoutingTable.RemoveRoutesCallCount()).Should(Equal(1))
				Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
				_, lrp := fakeRoutingTable.RemoveRoutesArgsForCall(0)
				Expect(lrp).Should(Equal(desiredLRP))
			})
		})
	})

	Describe("ActualLRP Event", func() {
		var (
			actualLRP     *models.ActualLRP
			routingEvents routingtable.TCPRouteMappings
		)

		BeforeEach(func() {
			routingEvents = routingtable.TCPRouteMappings{
				Registrations: []tcpmodels.TcpRouteMapping{},
			}
		})

		Describe("HandleActualCreate", func() {
			JustBeforeEach(func() {
				routeHandler.HandleEvent(logger, models.NewActualLRPInstanceCreatedEvent(actualLRP, "some-trace-id"))
			})

			Context("when state is Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"some-ip",
							"container-ip",
							models.ActualLRPNetInfo_PreferredAddressHost,
							models.NewPortMapping(611006, 5222),
						),
						State: models.ActualLRPStateRunning,
					}
				})

				It("invokes AddEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.AddEndpointCallCount()).Should(Equal(1))
					_, lrp := fakeRoutingTable.AddEndpointArgsForCall(0)
					Expect(lrp).Should(Equal(actualLRP))
				})

				Context("when there are routing events", func() {
					BeforeEach(func() {
						fakeRoutingTable.AddEndpointReturns(routingEvents, emptyNatsMessages)
					})

					It("invokes Emit on Emitter", func() {
						Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
						events := fakeRoutingAPIEmitter.EmitArgsForCall(0)
						Expect(events).Should(Equal(routingEvents))
					})
				})
			})

			Context("when state is not in Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"some-ip",
							"container-ip",
							models.ActualLRPNetInfo_PreferredAddressHost,
							models.NewPortMapping(611006, 5222),
						),
						State: models.ActualLRPStateClaimed,
					}
				})

				It("does not invoke AddEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.AddEndpointCallCount()).Should(Equal(0))
				})

				It("does not invoke Emit on Emitter", func() {
					Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(0))
				})
			})
		})

		Describe("HandleActualUpdate", func() {
			var (
				afterLRP *models.ActualLRP
			)

			JustBeforeEach(func() {
				routeHandler.HandleEvent(logger, models.NewActualLRPInstanceChangedEvent(actualLRP, afterLRP, "some-trace-id"))
			})

			Context("when after state is Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"",
							"",
							models.ActualLRPNetInfo_PreferredAddressHost,
						),
						State: models.ActualLRPStateClaimed,
					}

					afterLRP = &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"some-ip",
							"container-ip",
							models.ActualLRPNetInfo_PreferredAddressHost,
							models.NewPortMapping(611006, 5222),
						),
						State: models.ActualLRPStateRunning,
					}
				})

				Context("when Routable is true", func() {
					BeforeEach(func() {
						afterLRP.SetRoutable(true)
					})

					It("invokes AddEndpoint on RoutingTable", func() {
						Expect(fakeRoutingTable.AddEndpointCallCount()).Should(Equal(1))
						_, lrp := fakeRoutingTable.AddEndpointArgsForCall(0)
						Expect(lrp).Should(Equal(afterLRP))
					})

					Context("when there are routing events", func() {
						BeforeEach(func() {
							fakeRoutingTable.AddEndpointReturns(routingEvents, emptyNatsMessages)
						})

						It("invokes Emit on Emitter", func() {
							Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
							events := fakeRoutingAPIEmitter.EmitArgsForCall(0)
							Expect(events).Should(Equal(routingEvents))
						})
					})
				})
			})

			Context("when after state is not Running and before state is Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"some-ip",
							"container-ip",
							models.ActualLRPNetInfo_PreferredAddressHost,
							models.NewPortMapping(611006, 5222),
						),
						State: models.ActualLRPStateRunning,
					}

					afterLRP = &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"",
							"",
							models.ActualLRPNetInfo_PreferredAddressHost,
						),
						State: models.ActualLRPStateCrashed,
					}
				})

				It("invokes RemoveEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.RemoveEndpointCallCount()).Should(Equal(1))
					_, lrp := fakeRoutingTable.RemoveEndpointArgsForCall(0)
					Expect(lrp).Should(Equal(actualLRP))
				})

				Context("when there are routing events", func() {
					BeforeEach(func() {
						fakeRoutingTable.RemoveEndpointReturns(routingEvents, emptyNatsMessages)
					})

					It("invokes Emit on Emitter", func() {
						Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
						events := fakeRoutingAPIEmitter.EmitArgsForCall(0)
						Expect(events).Should(Equal(routingEvents))
					})
				})
			})

			Context("when both after and before state is not Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", ""),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"",
							"",
							models.ActualLRPNetInfo_PreferredAddressHost,
						),
						State: models.ActualLRPStateUnclaimed,
					}

					afterLRP = &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"",
							"",
							models.ActualLRPNetInfo_PreferredAddressHost,
						),
						State: models.ActualLRPStateClaimed,
					}
				})

				It("does not invoke AddEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.AddEndpointCallCount()).Should(Equal(0))
				})

				It("does not invoke RemoveEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.RemoveEndpointCallCount()).Should(Equal(0))
				})
			})
		})

		Describe("HandleActualDelete", func() {
			JustBeforeEach(func() {
				routeHandler.HandleEvent(logger, models.NewActualLRPInstanceRemovedEvent(actualLRP, "some-trace-id"))
			})

			Context("when state is Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"some-ip",
							"container-ip",
							models.ActualLRPNetInfo_PreferredAddressHost,
							models.NewPortMapping(611006, 5222),
						),
						State: models.ActualLRPStateRunning,
					}
				})

				It("invokes RemoveEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.RemoveEndpointCallCount()).Should(Equal(1))
					_, lrp := fakeRoutingTable.RemoveEndpointArgsForCall(0)
					Expect(lrp).Should(Equal(actualLRP))
				})

				Context("when there are routing events", func() {
					BeforeEach(func() {
						fakeRoutingTable.RemoveEndpointReturns(routingEvents, emptyNatsMessages)
					})

					It("invokes Emit on Emitter", func() {
						Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
						events := fakeRoutingAPIEmitter.EmitArgsForCall(0)
						Expect(events).Should(Equal(routingEvents))
					})
				})
			})

			Context("when state is not in Running", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"",
							"",
							models.ActualLRPNetInfo_PreferredAddressHost,
						),
						State: models.ActualLRPStateClaimed,
					}
				})

				It("does not invoke RemoveEndpoint on RoutingTable", func() {
					Expect(fakeRoutingTable.RemoveEndpointCallCount()).Should(Equal(0))
				})

				It("does not invoke Emit on Emitter", func() {
					Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(0))
				})
			})
		})
	})

	Describe("ShouldRefreshDesired", func() {
		var actualLRP *models.ActualLRP
		BeforeEach(func() {
			actualLRP = &models.ActualLRP{
				ActualLRPKey:         models.NewActualLRPKey("process-guid-1", 0, "domain"),
				ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
				ActualLRPNetInfo: models.NewActualLRPNetInfo(
					"some-ip",
					"container-ip",
					models.ActualLRPNetInfo_PreferredAddressHost,
					models.NewPortMapping(61006, 5222),
					models.NewPortMapping(61007, 5223),
				),
				State:           models.ActualLRPStateRunning,
				ModificationTag: models.ModificationTag{Epoch: "abc", Index: 1},
			}
		})

		Context("when corresponding desired state exists in the table", func() {
			BeforeEach(func() {
				fakeRoutingTable.HasExternalRoutesReturns(false)
			})

			It("returns false", func() {
				Expect(routeHandler.ShouldRefreshDesired(actualLRP)).To(BeTrue())
			})
		})

		Context("when corresponding desired state does not exist in the table", func() {
			BeforeEach(func() {
				fakeRoutingTable.HasExternalRoutesReturns(true)
			})

			It("returns true", func() {
				Expect(routeHandler.ShouldRefreshDesired(actualLRP)).To(BeFalse())
			})
		})
	})

	Describe("RefreshDesired", func() {
		BeforeEach(func() {
			fakeRoutingTable.SetRoutesReturns(routingtable.TCPRouteMappings{}, emptyNatsMessages)
		})

		It("adds the desired info to the routing table", func() {
			modificationTag := models.ModificationTag{Epoch: "abc", Index: 1}
			externalPort := uint32(61000)
			containerPort := uint32(5222)
			tcpRoutes := tcp_routes.TCPRoutes{
				tcp_routes.TCPRoute{
					RouterGroupGuid: "router-group-guid",
					ExternalPort:    externalPort,
					ContainerPort:   containerPort,
				},
			}
			desiredLRP := &models.DesiredLRP{
				ProcessGuid:     "process-guid-1",
				LogGuid:         "log-guid",
				Routes:          tcpRoutes.RoutingInfo(),
				ModificationTag: &modificationTag,
			}
			routeHandler.RefreshDesired(logger, []*models.DesiredLRP{desiredLRP})

			Expect(fakeRoutingTable.SetRoutesCallCount()).To(Equal(1))
			_, _, after := fakeRoutingTable.SetRoutesArgsForCall(0)
			Expect(after).To(Equal(desiredLRP))
			Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
		})
	})

	Describe("Sync", func() {
		Context("when bbs server returns desired and actual lrps", func() {
			var (
				desiredLRPs     []*models.DesiredLRP
				actualLRPs      []*models.ActualLRP
				modificationTag models.ModificationTag
			)

			BeforeEach(func() {
				modificationTag = models.ModificationTag{Epoch: "abc", Index: 1}
				externalPort := uint32(61000)
				containerPort := uint32(5222)
				tcpRoutes := tcp_routes.TCPRoutes{
					tcp_routes.TCPRoute{
						RouterGroupGuid: "router-group-guid",
						ExternalPort:    externalPort,
						ContainerPort:   containerPort,
					},
				}

				desiredLRPs = []*models.DesiredLRP{
					&models.DesiredLRP{
						ProcessGuid:     "process-guid-1",
						LogGuid:         "log-guid",
						Routes:          tcpRoutes.RoutingInfo(),
						ModificationTag: &modificationTag,
					},
				}

				actualLRPs = []*models.ActualLRP{
					&models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid-1", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"some-ip",
							"container-ip",
							models.ActualLRPNetInfo_PreferredAddressHost,
							models.NewPortMapping(61006, containerPort),
						),
						State:           models.ActualLRPStateRunning,
						ModificationTag: modificationTag,
					},
				}

				fakeRoutingTable.SwapStub = func(l lager.Logger, t routingtable.RoutingTable, domains models.DomainSet) (routingtable.TCPRouteMappings, routingtable.MessagesToEmit) {
					return routingtable.TCPRouteMappings{}, emptyNatsMessages
				}
			})

			Context("when emitting metrics in localMode", func() {
				type metric struct {
					name  string
					value int
				}

				var (
					metricsChan chan metric
				)

				BeforeEach(func() {
					metricsChan = make(chan metric, 10)
					fakeMetronClient.SendMetricStub = func(name string, value int, opts ...loggregator.EmitGaugeOption) error {
						metricsChan <- metric{
							name:  name,
							value: value,
						}
						return nil
					}
					routeHandler = routehandlers.NewHandler(fakeRoutingTable, nil, fakeRoutingAPIEmitter, true, false, fakeMetronClient, fakeUnregistrationCache)
					fakeRoutingTable.TCPAssociationsCountReturns(1)
				})

				It("emits the TCPRouteCount", func() {
					routeHandler.Sync(logger, desiredLRPs, actualLRPs, nil, nil)
					Eventually(metricsChan).Should(Receive(Equal(metric{
						name:  "TCPRouteCount",
						value: 1,
					})))
				})
			})

			It("updates the routing table", func() {
				domains := models.DomainSet{}
				domains.Add("foo")
				routeHandler.Sync(logger, desiredLRPs, actualLRPs, domains, nil)
				Expect(fakeRoutingTable.SwapCallCount()).Should(Equal(1))
				_, tempRoutingTable, actualDomains := fakeRoutingTable.SwapArgsForCall(0)
				Expect(actualDomains).To(Equal(domains))
				Expect(tempRoutingTable.TCPAssociationsCount()).To(Equal(1))
				routingEvents, _ := tempRoutingTable.GetExternalRoutingEvents()
				ttl := 0
				Expect(routingEvents.Registrations).To(ConsistOf(tcpmodels.TcpRouteMapping{
					TcpMappingEntity: tcpmodels.TcpMappingEntity{
						RouterGroupGuid: "router-group-guid",
						ExternalPort:    61000,
						HostPort:        61006,
						HostTLSPort:     -1,
						HostIP:          "some-ip",
						TTL:             &ttl,
					},
				}))

				Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))
			})

			Context("when events are cached", func() {
				BeforeEach(func() {
					tcpRoutes := tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61007,
							ContainerPort:   5222,
						},
					}
					desiredLRPEvent := models.NewDesiredLRPCreatedEvent(&models.DesiredLRP{
						ProcessGuid: "process-guid-2",
						Routes:      tcpRoutes.RoutingInfo(),
						Instances:   1,
					}, "some-trace-id")

					actualLRPEvent := models.NewActualLRPInstanceCreatedEvent(&models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey("process-guid-2", 0, "domain"),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid-1", "cell-id"),
						ActualLRPNetInfo: models.NewActualLRPNetInfo(
							"some-ip-2",
							"container-ip-2",
							models.ActualLRPNetInfo_PreferredAddressHost,
							models.NewPortMapping(61006, 5222),
						),
						State:           models.ActualLRPStateRunning,
						ModificationTag: modificationTag,
					}, "some-trace-id")

					cachedEvents := map[string]models.Event{
						desiredLRPEvent.Key(): desiredLRPEvent,
						actualLRPEvent.Key():  actualLRPEvent,
					}
					routeHandler.Sync(
						logger,
						desiredLRPs,
						actualLRPs,
						nil,
						cachedEvents,
					)
				})

				It("updates the routing table and emit cached events", func() {
					Expect(fakeRoutingTable.SwapCallCount()).Should(Equal(1))
					_, tempRoutingTable, _ := fakeRoutingTable.SwapArgsForCall(0)
					Expect(tempRoutingTable.TCPAssociationsCount()).Should(Equal(2))
					Expect(fakeRoutingAPIEmitter.EmitCallCount()).To(Equal(1))
				})
			})
		})
	})

	Describe("EmitExternal", func() {
		var events routingtable.TCPRouteMappings
		BeforeEach(func() {
			events = routingtable.TCPRouteMappings{}
			fakeRoutingTable.GetExternalRoutingEventsReturns(events, emptyNatsMessages)
		})

		It("emits all valid external registration events", func() {
			routeHandler.EmitExternal(logger)
			Expect(fakeRoutingTable.GetExternalRoutingEventsCallCount()).To(Equal(1))
			Expect(fakeRoutingAPIEmitter.EmitCallCount()).To(Equal(1))
			Expect(fakeRoutingAPIEmitter.EmitArgsForCall(0)).To(Equal(events))
		})
	})
})
