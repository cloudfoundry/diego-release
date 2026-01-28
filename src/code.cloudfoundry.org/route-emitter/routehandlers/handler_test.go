package routehandlers_test

import (
	"encoding/json"
	"fmt"

	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	loggregator "code.cloudfoundry.org/go-loggregator/v9"
	"code.cloudfoundry.org/lager/v3"
	ufakes "code.cloudfoundry.org/route-emitter/unregistration/fakes"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/route-emitter/emitter/fakes"
	"code.cloudfoundry.org/route-emitter/routehandlers"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/routingtable/fakeroutingtable"
	tcpmodels "code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/routing-info/cfroutes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

const (
	logGuid = "some-log-guid"
	traceId = "7f461654-74d1-1ee5-8367-77d85df2cdab"
)

var _ = Describe("Handler", func() {
	type counter struct {
		name  string
		delta uint64
	}
	type metric struct {
		name  string
		value int
	}

	const (
		expectedDomain                  = "domain"
		expectedProcessGuid             = "process-guid"
		expectedInstanceGUID            = "instance-guid"
		expectedIndex                   = 0
		expectedHost                    = "1.1.1.1"
		expectedInstanceAddress         = "2.2.2.2"
		expectedExternalPort            = 11000
		expectedAdditionalExternalPort  = 22000
		expectedContainerPort           = 11
		expectedAdditionalContainerPort = 22
		expectedRouteServiceUrl         = "https://so.good.com"
	)

	var (
		fakeTable             *fakeroutingtable.FakeRoutingTable
		natsEmitter           *fakes.FakeNATSEmitter
		fakeRoutingAPIEmitter *fakes.FakeRoutingAPIEmitter

		expectedRoutes  []string
		expectedCFRoute cfroutes.CFRoute

		dummyMessagesToEmit routingtable.MessagesToEmit
		fakeMetronClient    *mfakes.FakeIngressClient

		logger *lagertest.TestLogger

		routeHandler *routehandlers.Handler

		fakeUnregistrationCache *ufakes.FakeCache

		emptyTCPRouteMappings routingtable.TCPRouteMappings

		counterChan chan counter
		metricChan  chan metric

		dummyMessageFoo, dummyMessageBar, dummyMessageBaz routingtable.RegistryMessage

		routableTrue, routableFalse bool
	)

	BeforeEach(func() {
		fakeTable = &fakeroutingtable.FakeRoutingTable{}
		natsEmitter = &fakes.FakeNATSEmitter{}
		fakeRoutingAPIEmitter = new(fakes.FakeRoutingAPIEmitter)
		logger = lagertest.NewTestLogger("test")

		dummyEndpoint := routingtable.Endpoint{
			InstanceGUID: expectedInstanceGUID,
			Index:        expectedIndex,
			Host:         expectedHost,
			Port:         expectedContainerPort,
		}
		dummyMessageFoo = routingtable.RegistryMessageFor(dummyEndpoint, routingtable.Route{Hostname: "foo.com", LogGUID: logGuid}, true)
		dummyMessageBar = routingtable.RegistryMessageFor(dummyEndpoint, routingtable.Route{Hostname: "bar.com", LogGUID: logGuid}, true)
		dummyMessageBaz = routingtable.RegistryMessageFor(dummyEndpoint, routingtable.Route{Hostname: "baz.com", LogGUID: logGuid}, true)
		dummyMessagesToEmit = routingtable.MessagesToEmit{
			RegistrationMessages:   []routingtable.RegistryMessage{dummyMessageFoo, dummyMessageBar},
			UnregistrationMessages: []routingtable.RegistryMessage{dummyMessageBaz},
		}

		expectedRoutes = []string{"route-1", "route-2"}
		expectedCFRoute = cfroutes.CFRoute{Hostnames: expectedRoutes, Port: expectedContainerPort, RouteServiceUrl: expectedRouteServiceUrl}

		fakeMetronClient = &mfakes.FakeIngressClient{}
		counterChan = make(chan counter, 10)
		fakeMetronClient.IncrementCounterWithDeltaStub = func(name string, delta uint64) error {
			counterChan <- counter{name: name, delta: delta}
			return nil
		}
		metricChan = make(chan metric, 10)
		fakeMetronClient.SendMetricStub = func(name string, value int, opts ...loggregator.EmitGaugeOption) error {
			metricChan <- metric{name: name, value: value}
			return nil
		}

		fakeUnregistrationCache = &ufakes.FakeCache{}

		routeHandler = routehandlers.NewHandler(fakeTable, natsEmitter, fakeRoutingAPIEmitter, false, false, fakeMetronClient, fakeUnregistrationCache)
		routableTrue = true
		routableFalse = false
	})

	Context("when an unrecognized event is received", func() {
		It("logs an error", func() {
			routeHandler.HandleEvent(logger, &models.FakeEvent{})
			Expect(logger).To(gbytes.Say("did-not-handle-unrecognizable-event"))
		})
	})

	Describe("DesiredLRP Event", func() {
		Context("DesiredLRPCreated Event", func() {
			var (
				desiredLRP *models.DesiredLRP
			)

			BeforeEach(func() {
				routes := cfroutes.CFRoutes{expectedCFRoute}.RoutingInfo()
				desiredLRP = &models.DesiredLRP{
					Action: models.WrapAction(&models.RunAction{
						User: "me",
						Path: "ls",
					}),
					Domain:      "tests",
					ProcessGuid: expectedProcessGuid,
					Ports:       []uint32{expectedContainerPort},
					Routes:      &routes,
					LogGuid:     logGuid,
				}
				fakeTable.SetRoutesReturns(emptyTCPRouteMappings, dummyMessagesToEmit)
			})

			JustBeforeEach(func() {
				routeHandler.HandleEvent(logger, models.NewDesiredLRPCreatedEvent(desiredLRP, traceId))
			})

			It("should set the routes on the table", func() {
				Expect(fakeTable.SetRoutesCallCount()).To(Equal(1))
				_, before, after := fakeTable.SetRoutesArgsForCall(0)
				Expect(before).To(BeNil())
				Expect(after).To(Equal(desiredLRP))
			})

			It("sends a 'routes registered' metric", func() {
				Eventually(counterChan).Should(Receive(Equal(counter{
					name:  "RoutesRegistered",
					delta: 2,
				})))
			})

			It("sends a 'routes unregistered' metric", func() {
				Eventually(counterChan).Should(Receive(Equal(counter{
					name:  "RoutesUnregistered",
					delta: 1,
				})))
			})

			It("should emit whatever the table tells it to emit", func() {
				Expect(natsEmitter.EmitCallCount()).To(Equal(1))
				messagesToEmit := natsEmitter.EmitArgsForCall(0)
				Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
			})

			Context("when there are diego ssh-keys on the route", func() {
				BeforeEach(func() {
					diegoSSHInfo := json.RawMessage([]byte(`{"ssh-key": "ssh-value"}`))

					routes := cfroutes.CFRoutes{expectedCFRoute}.RoutingInfo()
					routes["diego-ssh"] = &diegoSSHInfo

					desiredLRP.Routes = &routes
				})

				It("does not log anything", func() {
					Expect(fakeTable.SetRoutesCallCount()).To(Equal(1))
					Expect(logger.Buffer()).NotTo(gbytes.Say("diego-ssh"))
				})
			})
		})

		Context("DesiredLRPChanged Event", func() {
			type metric struct {
				name  string
				delta uint64
			}

			var (
				originalDesiredLRP, changedDesiredLRP *models.DesiredLRP
				metricChan                            chan metric
			)

			BeforeEach(func() {
				fakeTable.SetRoutesReturns(emptyTCPRouteMappings, dummyMessagesToEmit)
				routes := cfroutes.CFRoutes{{Hostnames: expectedRoutes, Port: expectedContainerPort}}.RoutingInfo()

				originalDesiredLRP = &models.DesiredLRP{
					Action: models.WrapAction(&models.RunAction{
						User: "me",
						Path: "ls",
					}),
					Domain:      "tests",
					ProcessGuid: expectedProcessGuid,
					LogGuid:     logGuid,
					Routes:      &routes,
					Instances:   3,
				}
				changedDesiredLRP = &models.DesiredLRP{
					Action: models.WrapAction(&models.RunAction{
						User: "me",
						Path: "ls",
					}),
					Domain:          "tests",
					ProcessGuid:     expectedProcessGuid,
					LogGuid:         logGuid,
					Routes:          &routes,
					ModificationTag: &models.ModificationTag{Epoch: "abcd", Index: 1},
					Instances:       3,
				}
				metricChan = make(chan metric, 10)
				fakeMetronClient.IncrementCounterWithDeltaStub = func(name string, delta uint64) error {
					metricChan <- metric{name: name, delta: delta}
					return nil
				}
			})

			JustBeforeEach(func() {
				routeHandler.HandleEvent(logger, models.NewDesiredLRPChangedEvent(originalDesiredLRP, changedDesiredLRP, traceId))
			})

			It("should set the routes on the table", func() {
				Expect(fakeTable.SetRoutesCallCount()).To(Equal(1))
				_, before, after := fakeTable.SetRoutesArgsForCall(0)
				Expect(before).To(Equal(originalDesiredLRP))
				Expect(after).To(Equal(changedDesiredLRP))
			})

			It("sends a 'routes registered' metric", func() {
				Eventually(metricChan).Should(Receive(Equal(metric{
					name:  "RoutesRegistered",
					delta: 2,
				})))
			})

			It("sends a 'routes unregistered' metric", func() {
				Eventually(metricChan).Should(Receive(Equal(metric{
					name:  "RoutesUnregistered",
					delta: 1,
				})))
			})

			It("should emit whatever the table tells it to emit", func() {
				Expect(natsEmitter.EmitCallCount()).To(Equal(1))
				messagesToEmit := natsEmitter.EmitArgsForCall(0)
				Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
			})

			Context("when messages to emit contain unregistrations", func() {
				BeforeEach(func() {
					messagesToEmit := routingtable.MessagesToEmit{
						UnregistrationMessages: []routingtable.RegistryMessage{dummyMessageFoo, dummyMessageBar},
					}
					fakeTable.SetRoutesReturns(emptyTCPRouteMappings, messagesToEmit)
				})

				It("adds unregistration messages to unregistration cache", func() {
					Eventually(fakeUnregistrationCache.AddCallCount).Should(Equal(1))
					Expect(fakeUnregistrationCache.AddArgsForCall(0)).Should(ConsistOf(dummyMessageFoo, dummyMessageBar))
				})
			})

			Context("when messages to emit contain unregistrations and registrations", func() {
				BeforeEach(func() {
					messagesToEmit := routingtable.MessagesToEmit{
						UnregistrationMessages: []routingtable.RegistryMessage{dummyMessageFoo, dummyMessageBar},
						RegistrationMessages:   []routingtable.RegistryMessage{dummyMessageFoo},
					}
					fakeTable.SetRoutesReturns(emptyTCPRouteMappings, messagesToEmit)
				})

				It("adds unregistration messages to unregistration cache and removes registration messages from unregistraion cache", func() {
					Eventually(fakeUnregistrationCache.AddCallCount).Should(Equal(1))
					Eventually(fakeUnregistrationCache.RemoveCallCount).Should(Equal(1))
					Expect(fakeUnregistrationCache.AddArgsForCall(0)).Should(ConsistOf(dummyMessageFoo, dummyMessageBar))
					Expect(fakeUnregistrationCache.RemoveArgsForCall(0)).Should(ConsistOf(dummyMessageFoo))
				})
			})

			Context("when there are diego ssh-keys on the route", func() {
				BeforeEach(func() {
					diegoSSHInfo := json.RawMessage([]byte(`{"ssh-key": "ssh-value"}`))

					routes := cfroutes.CFRoutes{expectedCFRoute}.RoutingInfo()
					routes["diego-ssh"] = &diegoSSHInfo

					changedDesiredLRP.Routes = &routes
				})

				It("does not log them", func() {
					Expect(fakeTable.SetRoutesCallCount()).To(Equal(1))
					Expect(logger.Buffer()).NotTo(gbytes.Say("diego-ssh"))
				})
			})
		})

		Context("when a delete event occurs", func() {
			var desiredLRP *models.DesiredLRP

			BeforeEach(func() {
				fakeTable.RemoveRoutesReturns(emptyTCPRouteMappings, dummyMessagesToEmit)
				routes := cfroutes.CFRoutes{expectedCFRoute}.RoutingInfo()
				desiredLRP = &models.DesiredLRP{
					Action: models.WrapAction(&models.RunAction{
						User: "me",
						Path: "ls",
					}),
					Domain:          "tests",
					ProcessGuid:     expectedProcessGuid,
					Ports:           []uint32{expectedContainerPort},
					Routes:          &routes,
					LogGuid:         logGuid,
					ModificationTag: &models.ModificationTag{Epoch: "defg", Index: 2},
				}
			})

			JustBeforeEach(func() {
				routeHandler.HandleEvent(logger, models.NewDesiredLRPRemovedEvent(desiredLRP, "some-trace-id"))
			})

			It("should remove the routes from the table", func() {
				Expect(fakeTable.RemoveRoutesCallCount()).To(Equal(1))
				_, lrp := fakeTable.RemoveRoutesArgsForCall(0)
				Expect(lrp).To(Equal(desiredLRP))
			})

			It("should emit whatever the table tells it to emit", func() {
				Expect(natsEmitter.EmitCallCount()).To(Equal(1))

				messagesToEmit := natsEmitter.EmitArgsForCall(0)
				Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
			})

			Context("when there are diego ssh-keys on the route", func() {
				BeforeEach(func() {
					diegoSSHInfo := json.RawMessage([]byte(`{"ssh-key": "ssh-value"}`))

					routes := cfroutes.CFRoutes{expectedCFRoute}.RoutingInfo()
					routes["diego-ssh"] = &diegoSSHInfo

					desiredLRP.Routes = &routes
				})

				It("does not log them", func() {
					Expect(fakeTable.RemoveRoutesCallCount()).To(Equal(1))
					Expect(logger.Buffer()).NotTo(gbytes.Say("diego-ssh"))
				})
			})
		})
	})

	Describe("Actual LRP changes", func() {
		Context("when a create event occurs", func() {
			var (
				actualLRP *models.ActualLRP
			)

			Context("when the resulting LRP is nil", func() {
				BeforeEach(func() {
					actualLRP = nil
				})

				JustBeforeEach(func() {
					routeHandler.HandleEvent(logger, models.NewActualLRPInstanceCreatedEvent(actualLRP, traceId))
				})

				It("logs an error", func() {
					routeHandler.HandleEvent(logger, models.NewActualLRPInstanceCreatedEvent(actualLRP, traceId))
					Expect(logger).To(gbytes.Say("nil-actual-lrp"))
				})
			})

			Context("when the resulting LRP is in the RUNNING state", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRP{
						ActualLrpKey:         models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
						ActualLrpInstanceKey: models.NewActualLRPInstanceKey(expectedInstanceGUID, "cell-id"),
						ActualLrpNetInfo: models.NewActualLRPNetInfo(
							expectedHost,
							expectedInstanceAddress,
							models.ActualLRPNetInfo_PreferredAddressHost,
							models.NewPortMapping(expectedExternalPort, expectedContainerPort),
							models.NewPortMapping(expectedExternalPort, expectedAdditionalContainerPort),
						),
						State: models.ActualLRPStateRunning,
					}

					fakeTable.AddEndpointReturns(emptyTCPRouteMappings, dummyMessagesToEmit)
				})

				JustBeforeEach(func() {
					routeHandler.HandleEvent(logger, models.NewActualLRPInstanceCreatedEvent(actualLRP, "some-trace-id"))
				})

				It("should add/update the endpoints on the table", func() {
					Expect(fakeTable.AddEndpointCallCount()).To(Equal(1))
					_, lrp := fakeTable.AddEndpointArgsForCall(0)
					Expect(lrp).To(Equal(actualLRP))
				})

				It("should emit whatever the table tells it to emit", func() {
					Expect(natsEmitter.EmitCallCount()).To(Equal(1))

					messagesToEmit := natsEmitter.EmitArgsForCall(0)
					Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
				})

				It("sends a 'routes registered' metric", func() {
					Eventually(counterChan).Should(Receive(Equal(counter{
						name:  "RoutesRegistered",
						delta: 2,
					})))
				})

				It("sends a 'routes unregistered' metric", func() {
					Eventually(counterChan).Should(Receive(Equal(counter{
						name:  "RoutesUnregistered",
						delta: 1,
					})))
				})
			})

			Context("when the resulting LRP is not in the RUNNING state", func() {
				JustBeforeEach(func() {
					actualLRP = &models.ActualLRP{
						ActualLrpKey:         models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
						ActualLrpInstanceKey: models.NewActualLRPInstanceKey(expectedInstanceGUID, "cell-id"),
						ActualLrpNetInfo: models.NewActualLRPNetInfo(
							expectedHost,
							expectedInstanceAddress,
							models.ActualLRPNetInfo_PreferredAddressHost,
							models.NewPortMapping(expectedExternalPort, expectedContainerPort),
							models.NewPortMapping(expectedExternalPort, expectedAdditionalContainerPort),
						),
						State: models.ActualLRPStateUnclaimed,
					}
				})

				It("should NOT log the net info", func() {
					Expect(logger).ToNot(gbytes.Say(
						fmt.Sprintf(
							`"net_info":\{"address":"%s","ports":\[\{"container_port":%d,"host_port":%d\},\{"container_port":%d,"host_port":%d\}\]\}`,
							expectedHost,
							expectedContainerPort,
							expectedExternalPort,
							expectedAdditionalContainerPort,
							expectedExternalPort,
						),
					))
				})

				It("doesn't add/update the endpoint on the table", func() {
					Expect(fakeTable.AddEndpointCallCount()).Should(Equal(0))
				})

				It("doesn't emit", func() {
					Expect(natsEmitter.EmitCallCount()).To(Equal(0))
				})
			})
		})

		Context("when a change event occurs", func() {
			Context("when the resulting LRP is in the RUNNING state", func() {
				var (
					afterActualLRP, beforeActualLRP *models.ActualLRP
				)

				BeforeEach(func() {
					fakeTable.AddEndpointReturns(emptyTCPRouteMappings, dummyMessagesToEmit)
					fakeTable.RemoveEndpointReturns(emptyTCPRouteMappings, dummyMessagesToEmit)

					beforeActualLRP = &models.ActualLRP{
						ActualLrpKey:         models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
						ActualLrpInstanceKey: models.NewActualLRPInstanceKey(expectedInstanceGUID, "cell-id"),
						State:                models.ActualLRPStateClaimed,
					}
					afterActualLRP = &models.ActualLRP{
						ActualLrpKey:         models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
						ActualLrpInstanceKey: models.NewActualLRPInstanceKey(expectedInstanceGUID, "cell-id"),
						ActualLrpNetInfo: models.NewActualLRPNetInfo(
							expectedHost,
							expectedInstanceAddress,
							models.ActualLRPNetInfo_PreferredAddressHost,
							models.NewPortMapping(expectedExternalPort, expectedContainerPort),
							models.NewPortMapping(expectedAdditionalExternalPort, expectedAdditionalContainerPort),
						),
						State: models.ActualLRPStateRunning,
					}
				})

				JustBeforeEach(func() {
					routeHandler.HandleEvent(logger, models.NewActualLRPInstanceChangedEvent(beforeActualLRP, afterActualLRP, "some-trace-id"))
				})

				Context("when one or all the LRP are nil", func() {
					Context("when the before is nil", func() {
						JustBeforeEach(func() {
							beforeActualLRP = nil
						})
						It("logs an error", func() {
							routeHandler.HandleEvent(logger, models.NewActualLRPInstanceChangedEvent(beforeActualLRP, afterActualLRP, "some-trace-id"))
							Expect(logger).To(gbytes.Say("nil-actual-lrp"))
						})
					})

					Context("when the after is nil", func() {
						JustBeforeEach(func() {
							afterActualLRP = nil
						})

						It("logs an error", func() {
							routeHandler.HandleEvent(logger, models.NewActualLRPInstanceChangedEvent(beforeActualLRP, afterActualLRP, "some-trace-id"))
							Expect(logger).To(gbytes.Say("nil-actual-lrp"))
						})
					})
				})

				Context("when after state Routable is not provided (old BBS)", func() {
					It("should add/update the endpoint on the table", func() {
						Expect(fakeTable.AddEndpointCallCount()).To(Equal(1))

						_, actualLRP := fakeTable.AddEndpointArgsForCall(0)
						Expect(actualLRP).To(Equal(afterActualLRP))
					})

					It("should emit whatever the table tells it to emit", func() {
						Expect(natsEmitter.EmitCallCount()).Should(Equal(1))

						messagesToEmit := natsEmitter.EmitArgsForCall(0)
						Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
					})

					It("sends a 'routes registered' metric", func() {
						Eventually(counterChan).Should(Receive(Equal(counter{
							name:  "RoutesRegistered",
							delta: 2,
						})))
					})

					It("sends a 'routes unregistered' metric", func() {
						Eventually(counterChan).Should(Receive(Equal(counter{
							name:  "RoutesUnregistered",
							delta: 1,
						})))
					})
				})

				Context("when after state Routable is true", func() {
					BeforeEach(func() {
						afterActualLRP.SetRoutable(&routableTrue)
					})

					It("should add/update the endpoint on the table", func() {
						Expect(fakeTable.AddEndpointCallCount()).To(Equal(1))

						_, actualLRP := fakeTable.AddEndpointArgsForCall(0)
						Expect(actualLRP).To(Equal(afterActualLRP))
					})

					It("should emit whatever the table tells it to emit", func() {
						Expect(natsEmitter.EmitCallCount()).Should(Equal(1))

						messagesToEmit := natsEmitter.EmitArgsForCall(0)
						Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
					})

					It("sends a 'routes registered' metric", func() {
						Eventually(counterChan).Should(Receive(Equal(counter{
							name:  "RoutesRegistered",
							delta: 2,
						})))
					})

					It("sends a 'routes unregistered' metric", func() {
						Eventually(counterChan).Should(Receive(Equal(counter{
							name:  "RoutesUnregistered",
							delta: 1,
						})))
					})
				})

				Context("when after state Routable is false and before state Routable is true", func() {
					BeforeEach(func() {
						beforeActualLRP.SetRoutable(&routableTrue)
						afterActualLRP.SetRoutable(&routableFalse)
					})

					It("should not add/update the endpoint on the table", func() {
						Expect(fakeTable.AddEndpointCallCount()).To(Equal(0))
					})

					It("should remove the endpoint from the table", func() {
						Expect(fakeTable.RemoveEndpointCallCount()).To(Equal(1))

						_, actualLRP := fakeTable.RemoveEndpointArgsForCall(0)
						Expect(actualLRP).To(Equal(afterActualLRP))
					})

					It("should emit whatever the table tells it to emit", func() {
						Expect(natsEmitter.EmitCallCount()).To(Equal(1))

						messagesToEmit := natsEmitter.EmitArgsForCall(0)
						Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
					})

					It("should add the messages to the unregistration cache", func() {
						Eventually(fakeUnregistrationCache.AddCallCount).Should(Equal(1))
						Expect(fakeUnregistrationCache.AddArgsForCall(0)).Should(ConsistOf([]routingtable.RegistryMessage{dummyMessageBaz}))
					})

					It("sends a 'routes registered' metric", func() {
						Eventually(counterChan).Should(Receive(Equal(counter{
							name:  "RoutesRegistered",
							delta: 2,
						})))
					})

					It("sends a 'routes unregistered' metric", func() {
						Eventually(counterChan).Should(Receive(Equal(counter{
							name:  "RoutesUnregistered",
							delta: 1,
						})))
					})
				})

				Context("when after state Routable is false and before state Routable is false", func() {
					BeforeEach(func() {
						beforeActualLRP.SetRoutable(&routableFalse)
						afterActualLRP.SetRoutable(&routableFalse)
					})

					It("should not add/update the endpoint on the table", func() {
						Expect(fakeTable.AddEndpointCallCount()).To(Equal(0))
					})

					It("should not remove the endpoint from the table", func() {
						Expect(fakeTable.RemoveEndpointCallCount()).To(Equal(0))
					})

					It("should call emit with empty messages", func() {
						Expect(natsEmitter.EmitCallCount()).To(Equal(1))

						messagesToEmit := natsEmitter.EmitArgsForCall(0)
						Expect(messagesToEmit).To(Equal(routingtable.MessagesToEmit{RegistrationMessages: nil}))
					})

					It("sends a 'routes registered' metric", func() {
						Eventually(counterChan).Should(Receive(Equal(counter{
							name:  "RoutesRegistered",
							delta: 0,
						})))
					})

					It("sends a 'routes unregistered' metric", func() {
						Eventually(counterChan).Should(Receive(Equal(counter{
							name:  "RoutesUnregistered",
							delta: 0,
						})))
					})
				})

				Context("when messages to emit contain registraions", func() {
					BeforeEach(func() {
						messagesToEmit := routingtable.MessagesToEmit{
							RegistrationMessages: []routingtable.RegistryMessage{dummyMessageFoo},
						}
						fakeTable.AddEndpointReturns(emptyTCPRouteMappings, messagesToEmit)
					})

					It("removes registration messages from unregistraion cache", func() {
						Eventually(fakeUnregistrationCache.RemoveCallCount).Should(Equal(1))
						Expect(fakeUnregistrationCache.RemoveArgsForCall(0)).Should(ConsistOf(dummyMessageFoo))
					})
				})
			})

			Context("when the resulting LRP transitions away from the RUNNING state", func() {
				var (
					beforeActualLRP, afterActualLRP *models.ActualLRP
				)

				BeforeEach(func() {
					beforeActualLRP = &models.ActualLRP{
						ActualLrpKey:         models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
						ActualLrpInstanceKey: models.NewActualLRPInstanceKey(expectedInstanceGUID, "cell-id"),
						ActualLrpNetInfo: models.NewActualLRPNetInfo(
							expectedHost,
							expectedInstanceAddress,
							models.ActualLRPNetInfo_PreferredAddressHost,
							models.NewPortMapping(expectedExternalPort, expectedContainerPort),
							models.NewPortMapping(expectedAdditionalExternalPort, expectedAdditionalContainerPort),
						),
						State: models.ActualLRPStateRunning,
					}
					afterActualLRP = &models.ActualLRP{
						ActualLrpKey: models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
						State:        models.ActualLRPStateUnclaimed,
					}
					fakeTable.RemoveEndpointReturns(emptyTCPRouteMappings, dummyMessagesToEmit)
				})

				JustBeforeEach(func() {
					routeHandler.HandleEvent(logger, models.NewActualLRPInstanceChangedEvent(beforeActualLRP, afterActualLRP, "some-trace-id"))
				})

				It("should remove the endpoint from the table", func() {
					Expect(fakeTable.RemoveEndpointCallCount()).To(Equal(1))

					_, actualLRP := fakeTable.RemoveEndpointArgsForCall(0)
					Expect(actualLRP).To(Equal(beforeActualLRP))
				})

				It("should emit whatever the table tells it to emit", func() {
					Expect(natsEmitter.EmitCallCount()).To(Equal(1))

					messagesToEmit := natsEmitter.EmitArgsForCall(0)
					Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
				})
			})

			Context("when the LRP remains in RUNNING state", func() {
				var (
					afterActualLRP, beforeActualLRP *models.ActualLRP

					addMessagesToEmit, removeMessagesToEmit, combinedMessagesToEmit routingtable.MessagesToEmit
					addRouteMapping, removeRouteMapping, combinedRouteMapping       routingtable.TCPRouteMappings
				)

				BeforeEach(func() {
					addMessagesToEmit = routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{dummyMessageFoo},
					}
					removeMessagesToEmit = routingtable.MessagesToEmit{
						UnregistrationMessages: []routingtable.RegistryMessage{dummyMessageBar},
					}
					combinedMessagesToEmit = routingtable.MessagesToEmit{
						RegistrationMessages:   []routingtable.RegistryMessage{dummyMessageFoo},
						UnregistrationMessages: []routingtable.RegistryMessage{dummyMessageBar},
					}
					addRouteMapping = routingtable.TCPRouteMappings{
						Registrations: []tcpmodels.TcpRouteMapping{
							{Model: tcpmodels.Model{Guid: "add-route-mapping"}},
						},
					}
					removeRouteMapping = routingtable.TCPRouteMappings{
						Unregistrations: []tcpmodels.TcpRouteMapping{
							{Model: tcpmodels.Model{Guid: "remove-route-mapping"}},
						},
					}
					combinedRouteMapping = routingtable.TCPRouteMappings{
						Registrations: []tcpmodels.TcpRouteMapping{
							{Model: tcpmodels.Model{Guid: "add-route-mapping"}},
						},
						Unregistrations: []tcpmodels.TcpRouteMapping{
							{Model: tcpmodels.Model{Guid: "remove-route-mapping"}},
						},
					}
					fakeTable.AddEndpointReturns(addRouteMapping, addMessagesToEmit)
					fakeTable.RemoveEndpointReturns(removeRouteMapping, removeMessagesToEmit)

					beforeActualLRP = &models.ActualLRP{
						ActualLrpKey:         models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
						ActualLrpInstanceKey: models.NewActualLRPInstanceKey(expectedInstanceGUID, "cell-id"),
						State:                models.ActualLRPStateRunning,
						Presence:             models.ActualLRP_Ordinary,
					}
					beforeActualLRP.SetRoutable(&routableTrue)
					afterActualLRP = &models.ActualLRP{
						ActualLrpKey:         models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
						ActualLrpInstanceKey: models.NewActualLRPInstanceKey(expectedInstanceGUID, "cell-id"),
						State:                models.ActualLRPStateRunning,
					}
					afterActualLRP.SetRoutable(&routableTrue)
				})

				JustBeforeEach(func() {
					routeHandler.HandleEvent(logger, models.NewActualLRPInstanceChangedEvent(beforeActualLRP, afterActualLRP, "some-trace-id"))
				})

				Context("when the resulting LRP presence does not change", func() {
					BeforeEach(func() {
						afterActualLRP.Presence = models.ActualLRP_Ordinary
					})

					It("adds a new endpoint and does not remove an old endpoint", func() {
						Expect(fakeTable.AddEndpointCallCount()).To(Equal(1))
						_, actualLRP := fakeTable.AddEndpointArgsForCall(0)
						Expect(actualLRP).To(Equal(afterActualLRP))

						Expect(fakeTable.RemoveEndpointCallCount()).To(Equal(0))

						Expect(natsEmitter.EmitCallCount()).Should(Equal(1))

						messagesToEmit := natsEmitter.EmitArgsForCall(0)
						Expect(messagesToEmit).To(Equal(addMessagesToEmit))

						Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))

						routesToEmit := fakeRoutingAPIEmitter.EmitArgsForCall(0)
						Expect(routesToEmit).To(Equal(addRouteMapping))
					})
				})

				Context("when the resulting LRP changes its presence to SUSPECT", func() {
					BeforeEach(func() {
						afterActualLRP.Presence = models.ActualLRP_Suspect
					})

					It("adds a new endpoint and does not remove an old endpoint", func() {
						Expect(fakeTable.AddEndpointCallCount()).To(Equal(1))
						_, actualLRP := fakeTable.AddEndpointArgsForCall(0)
						Expect(actualLRP).To(Equal(afterActualLRP))

						Expect(fakeTable.RemoveEndpointCallCount()).To(Equal(0))

						Expect(natsEmitter.EmitCallCount()).Should(Equal(1))

						messagesToEmit := natsEmitter.EmitArgsForCall(0)
						Expect(messagesToEmit).To(Equal(addMessagesToEmit))

						Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))

						routesToEmit := fakeRoutingAPIEmitter.EmitArgsForCall(0)
						Expect(routesToEmit).To(Equal(addRouteMapping))
					})
				})

				Context("when the resulting LRP changes its presence to EVACUATING", func() {
					BeforeEach(func() {
						afterActualLRP.Presence = models.ActualLRP_Evacuating
					})

					It("adds a new endpoint and removes an old endpoint", func() {
						Expect(fakeTable.AddEndpointCallCount()).To(Equal(1))
						_, actualLRP := fakeTable.AddEndpointArgsForCall(0)
						Expect(actualLRP).To(Equal(afterActualLRP))

						Expect(fakeTable.RemoveEndpointCallCount()).To(Equal(1))
						_, actualLRP = fakeTable.RemoveEndpointArgsForCall(0)
						Expect(actualLRP).To(Equal(beforeActualLRP))

						Expect(natsEmitter.EmitCallCount()).Should(Equal(1))

						messagesToEmit := natsEmitter.EmitArgsForCall(0)
						Expect(messagesToEmit).To(Equal(combinedMessagesToEmit))

						Expect(fakeRoutingAPIEmitter.EmitCallCount()).Should(Equal(1))

						routesToEmit := fakeRoutingAPIEmitter.EmitArgsForCall(0)
						Expect(routesToEmit).To(Equal(combinedRouteMapping))
					})
				})
			})

			Context("when the endpoint neither starts nor ends in the RUNNING state", func() {
				JustBeforeEach(func() {
					beforeActualLRP := &models.ActualLRP{
						ActualLrpKey: models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
						State:        models.ActualLRPStateUnclaimed,
					}
					afterActualLRP := &models.ActualLRP{
						ActualLrpKey:         models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
						ActualLrpInstanceKey: models.NewActualLRPInstanceKey(expectedInstanceGUID, "cell-id"),
						ActualLrpNetInfo: models.NewActualLRPNetInfo(
							expectedHost,
							expectedInstanceAddress,
							models.ActualLRPNetInfo_PreferredAddressHost,
							models.NewPortMapping(expectedExternalPort, expectedContainerPort),
							models.NewPortMapping(expectedAdditionalExternalPort, expectedAdditionalContainerPort),
						),
						State: models.ActualLRPStateClaimed,
					}
					routeHandler.HandleEvent(logger, models.NewActualLRPInstanceChangedEvent(beforeActualLRP, afterActualLRP, "some-trace-id"))
				})

				It("should NOT log the net info", func() {
					Expect(logger).ToNot(gbytes.Say(
						fmt.Sprintf(
							`"net_info":\{"address":"%s","ports":\[\{"container_port":%d,"host_port":%d\},\{"container_port":%d,"host_port":%d\}\],"instance_address":"%s"\}`,
							expectedHost,
							expectedContainerPort,
							expectedExternalPort,
							expectedAdditionalContainerPort,
							expectedExternalPort,
							expectedInstanceAddress,
						),
					))
				})

				It("should not remove the endpoint", func() {
					Expect(fakeTable.RemoveEndpointCallCount()).To(BeZero())
				})

				It("should not add or update the endpoint", func() {
					Expect(fakeTable.AddEndpointCallCount()).To(BeZero())
				})
			})

		})

		Context("when a delete event occurs", func() {
			var (
				actualLRP *models.ActualLRP
			)
			Context("when the actual is in the RUNNING state", func() {

				BeforeEach(func() {
					fakeTable.RemoveEndpointReturns(emptyTCPRouteMappings, dummyMessagesToEmit)

					actualLRP = &models.ActualLRP{
						ActualLrpKey:         models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
						ActualLrpInstanceKey: models.NewActualLRPInstanceKey(expectedInstanceGUID, "cell-id"),
						ActualLrpNetInfo: models.NewActualLRPNetInfo(
							expectedHost,
							expectedInstanceAddress,
							models.ActualLRPNetInfo_PreferredAddressHost,
							models.NewPortMapping(expectedExternalPort, expectedContainerPort),
							models.NewPortMapping(expectedAdditionalExternalPort, expectedAdditionalContainerPort),
						),
						State: models.ActualLRPStateRunning,
					}
				})

				JustBeforeEach(func() {
					routeHandler.HandleEvent(logger, models.NewActualLRPInstanceRemovedEvent(actualLRP, "some-trace-id"))
				})

				It("should remove the endpoint from the table", func() {
					Expect(fakeTable.RemoveEndpointCallCount()).To(Equal(1))

					_, lrp := fakeTable.RemoveEndpointArgsForCall(0)
					Expect(lrp).To(Equal(actualLRP))
				})

				It("should emit whatever the table tells it to emit", func() {
					Expect(natsEmitter.EmitCallCount()).To(Equal(1))

					messagesToEmit := natsEmitter.EmitArgsForCall(0)
					Expect(messagesToEmit).To(Equal(dummyMessagesToEmit))
				})
			})

			Context("when the actual is not in the RUNNING state", func() {
				JustBeforeEach(func() {
					actualLRP := &models.ActualLRP{
						ActualLrpKey: models.NewActualLRPKey(expectedProcessGuid, expectedIndex, "domain"),
						ActualLrpNetInfo: models.NewActualLRPNetInfo(
							expectedHost,
							expectedInstanceAddress,
							models.ActualLRPNetInfo_PreferredAddressHost,
							models.NewPortMapping(expectedExternalPort, expectedContainerPort),
							models.NewPortMapping(expectedAdditionalExternalPort, expectedAdditionalContainerPort),
						),
						State: models.ActualLRPStateCrashed,
					}

					routeHandler.HandleEvent(logger, models.NewActualLRPInstanceRemovedEvent(actualLRP, "some-trace-id"))
				})

				It("should NOT log the net info", func() {
					Expect(logger).ToNot(gbytes.Say(
						fmt.Sprintf(
							`"net_info":\{"address":"%s","ports":\[\{"container_port":%d,"host_port":%d\},\{"container_port":%d,"host_port":%d\}\],"instance_address":"%s"\}`,
							expectedHost,
							expectedContainerPort,
							expectedExternalPort,
							expectedAdditionalContainerPort,
							expectedExternalPort,
							expectedInstanceAddress,
						),
					))
				})

				It("doesn't remove the endpoint from the table", func() {
					Expect(fakeTable.RemoveEndpointCallCount()).To(Equal(0))
				})

				It("doesn't emit", func() {
					Expect(natsEmitter.EmitCallCount()).To(Equal(0))
				})
			})
			Context("when the actualLRP is nil", func() {
				JustBeforeEach(func() {
					actualLRP = nil
				})

				It("logs an error", func() {
					routeHandler.HandleEvent(logger, models.NewActualLRPInstanceRemovedEvent(actualLRP, "some-trace-id"))
					Expect(logger).To(gbytes.Say("nil-actual-lrp"))
				})
			})
		})
	})

	Describe("Sync", func() {
		Context("when bbs server returns desired and actual lrps", func() {
			var (
				desiredLRPs []*models.DesiredLRP
				actualLRPs  []*models.ActualLRP
				domains     models.DomainSet

				endpoint1, endpoint2, endpoint3, endpoint4 routingtable.Endpoint
			)

			BeforeEach(func() {
				currentTag := &models.ModificationTag{Epoch: "abc", Index: 1}
				hostname1 := "foo.example.com"
				hostname2 := "bar.example.com"
				hostname3 := "baz.example.com"

				endpoint1 = routingtable.Endpoint{
					InstanceGUID:    "ig-1",
					Host:            "1.1.1.1",
					Index:           0,
					Port:            11,
					ContainerPort:   8080,
					Presence:        models.ActualLRP_Ordinary,
					ModificationTag: currentTag,
				}
				endpoint2 = routingtable.Endpoint{
					InstanceGUID:    "ig-2",
					Host:            "2.2.2.2",
					Index:           0,
					Port:            22,
					ContainerPort:   8080,
					Presence:        models.ActualLRP_Ordinary,
					ModificationTag: currentTag,
				}
				endpoint3 = routingtable.Endpoint{
					InstanceGUID:    "ig-3",
					Host:            "2.2.2.2",
					Index:           1,
					Port:            23,
					ContainerPort:   8080,
					Presence:        models.ActualLRP_Ordinary,
					ModificationTag: currentTag,
				}
				r1 := cfroutes.CFRoutes{
					cfroutes.CFRoute{
						Hostnames:       []string{hostname1},
						Port:            8080,
						RouteServiceUrl: "https://rs.example.com",
						Options:         json.RawMessage(`{"lb_algo":"least-connection"}`),
					},
				}.RoutingInfo()

				desiredLRP1 := &models.DesiredLRP{
					ProcessGuid: "pg-1",
					Domain:      "tests",
					LogGuid:     "lg1",
					Routes:      &r1,
					Instances:   1,
				}
				r2 := cfroutes.CFRoutes{
					cfroutes.CFRoute{
						Hostnames: []string{hostname2},
						Port:      8080,
					},
				}.RoutingInfo()

				desiredLRP2 := &models.DesiredLRP{
					ProcessGuid: "pg-2",
					Domain:      "tests",
					LogGuid:     "lg2",
					Routes:      &r2,
					Instances:   1,
				}
				r3 := cfroutes.CFRoutes{
					cfroutes.CFRoute{
						Hostnames: []string{hostname3},
						Port:      8080,
					},
				}.RoutingInfo()

				desiredLRP3 := &models.DesiredLRP{
					ProcessGuid: "pg-3",
					Domain:      "tests",
					LogGuid:     "lg3",
					Routes:      &r3,
					Instances:   2,
				}

				actualLRP1 := &models.ActualLRP{
					ActualLrpKey:         models.NewActualLRPKey("pg-1", 0, "domain"),
					ActualLrpInstanceKey: models.NewActualLRPInstanceKey(endpoint1.InstanceGUID, "cell-id"),
					ActualLrpNetInfo:     models.NewActualLRPNetInfo(endpoint1.Host, "container-ip-1", models.ActualLRPNetInfo_PreferredAddressHost, models.NewPortMapping(endpoint1.Port, endpoint1.ContainerPort)),
					State:                models.ActualLRPStateRunning,
				}

				actualLRP2 := &models.ActualLRP{
					ActualLrpKey:         models.NewActualLRPKey("pg-2", 0, "domain"),
					ActualLrpInstanceKey: models.NewActualLRPInstanceKey(endpoint2.InstanceGUID, "cell-id"),
					ActualLrpNetInfo:     models.NewActualLRPNetInfo(endpoint2.Host, "container-ip-2", models.ActualLRPNetInfo_PreferredAddressHost, models.NewPortMapping(endpoint2.Port, endpoint2.ContainerPort)),
					State:                models.ActualLRPStateRunning,
				}

				actualLRP3 := &models.ActualLRP{
					ActualLrpKey:         models.NewActualLRPKey("pg-3", 1, "domain"),
					ActualLrpInstanceKey: models.NewActualLRPInstanceKey(endpoint3.InstanceGUID, "cell-id"),
					ActualLrpNetInfo:     models.NewActualLRPNetInfo(endpoint3.Host, "container-ip-3", models.ActualLRPNetInfo_PreferredAddressHost, models.NewPortMapping(endpoint3.Port, endpoint3.ContainerPort)),
					State:                models.ActualLRPStateRunning,
				}

				desiredLRPs = []*models.DesiredLRP{
					desiredLRP1, desiredLRP2, desiredLRP3,
				}

				actualLRPs = []*models.ActualLRP{
					actualLRP1,
					actualLRP2,
					actualLRP3,
				}

				domains = models.NewDomainSet([]string{"domain"})

				routesByRoutingKey := func(desiredLRPs []*models.DesiredLRP) map[routingtable.RoutingKey][]routingtable.Route {
					byRoutingKey := map[routingtable.RoutingKey][]routingtable.Route{}
					for _, desired := range desiredLRPs {
						routes, err := cfroutes.CFRoutesFromRoutingInfo(*desired.Routes)
						if err == nil && len(routes) > 0 {
							for _, cfRoute := range routes {
								key := routingtable.RoutingKey{ProcessGUID: desired.ProcessGuid, ContainerPort: cfRoute.Port}
								var routeEntries []routingtable.Route
								for _, hostname := range cfRoute.Hostnames {
									routeEntries = append(routeEntries, routingtable.Route{
										Hostname:         hostname,
										LogGUID:          desired.LogGuid,
										RouteServiceUrl:  cfRoute.RouteServiceUrl,
										IsolationSegment: cfRoute.IsolationSegment,
									})
								}
								byRoutingKey[key] = append(byRoutingKey[key], routeEntries...)
							}
						}
					}

					return byRoutingKey
				}

				fakeTable.SwapStub = func(l lager.Logger, t routingtable.RoutingTable, d models.DomainSet) (routingtable.TCPRouteMappings, routingtable.MessagesToEmit) {

					routes := routesByRoutingKey(desiredLRPs)
					routesList := make([]routingtable.Route, 3)
					for _, route := range routes {
						routesList = append(routesList, route[0])
					}

					return emptyTCPRouteMappings, routingtable.MessagesToEmit{
						RegistrationMessages: []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, routesList[0], true),
							routingtable.RegistryMessageFor(endpoint2, routesList[1], true),
							routingtable.RegistryMessageFor(endpoint3, routesList[2], true),
						},
					}
				}
			})

			It("updates the routing table", func() {
				routeHandler.Sync(logger, desiredLRPs, actualLRPs, domains, nil)
				Expect(fakeTable.SwapCallCount()).Should(Equal(1))
				_, tempRoutingTable, swapDomains := fakeTable.SwapArgsForCall(0)
				Expect(tempRoutingTable.HTTPAssociationsCount()).To(Equal(3))
				Expect(swapDomains).To(Equal(domains))

				Expect(natsEmitter.EmitCallCount()).Should(Equal(1))
			})

			Context("swapping the new route table", func() {
				var (
					registrationMessages, unregistrationMessages []routingtable.RegistryMessage
					route                                        = routingtable.Route{
						Hostname: "test-hostname",
						LogGUID:  "test-guid",
					}
				)

				JustBeforeEach(func() {
					fakeTable.SwapStub = func(l lager.Logger, t routingtable.RoutingTable, d models.DomainSet) (routingtable.TCPRouteMappings, routingtable.MessagesToEmit) {
						return emptyTCPRouteMappings, routingtable.MessagesToEmit{
							RegistrationMessages:   registrationMessages,
							UnregistrationMessages: unregistrationMessages,
						}
					}
				})

				Context("Swapping the route tables removes routes", func() {
					BeforeEach(func() {
						registrationMessages = nil
						unregistrationMessages = []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, route, true),
						}
					})

					It("removes unregistration messages for new routes", func() {
						routeHandler.Sync(logger, desiredLRPs, actualLRPs, domains, nil)
						Eventually(fakeUnregistrationCache.RemoveCallCount).Should(Equal(1))
						Expect(fakeUnregistrationCache.RemoveArgsForCall(0)).Should(BeEmpty())
					})

					It("add unregistration messages for removed routes", func() {
						routeHandler.Sync(logger, desiredLRPs, actualLRPs, domains, nil)
						Eventually(fakeUnregistrationCache.AddCallCount).Should(Equal(1))
						Expect(fakeUnregistrationCache.AddArgsForCall(0)).Should(HaveLen(1))
					})
				})

				Context("Swapping the route tables adds routes", func() {
					BeforeEach(func() {
						registrationMessages = []routingtable.RegistryMessage{
							routingtable.RegistryMessageFor(endpoint1, route, true),
						}
						unregistrationMessages = nil
					})

					It("removes unregistration messages for new routes from the unregistration cache", func() {
						routeHandler.Sync(logger, desiredLRPs, actualLRPs, domains, nil)
						Eventually(fakeUnregistrationCache.RemoveCallCount).Should(Equal(1))
						Expect(fakeUnregistrationCache.RemoveArgsForCall(0)).Should(HaveLen(1))
					})

					It("add no unregistration messages to the unregistration cache", func() {
						routeHandler.Sync(logger, desiredLRPs, actualLRPs, domains, nil)
						Eventually(fakeUnregistrationCache.AddCallCount).Should(Equal(1))
						Expect(fakeUnregistrationCache.AddArgsForCall(0)).Should(BeEmpty())
					})
				})
			})

			Context("when emitting metrics in localMode", func() {
				BeforeEach(func() {
					routeHandler = routehandlers.NewHandler(fakeTable, natsEmitter, nil, true, false, fakeMetronClient, fakeUnregistrationCache)
					fakeTable.HTTPAssociationsCountReturns(5)
				})

				It("emits the HTTPRouteCount", func() {
					routeHandler.Sync(logger, desiredLRPs, actualLRPs, domains, nil)
					Eventually(metricChan).Should(Receive(Equal(metric{
						name:  "HTTPRouteCount",
						value: 5,
					})))
				})
			})

			Context("when NATS events are cached", func() {
				BeforeEach(func() {
					routes := cfroutes.CFRoutes{
						cfroutes.CFRoute{
							Hostnames: []string{"anungunrama.example.com"},
							Port:      8080,
						},
					}.RoutingInfo()
					desiredLRPEvent := models.NewDesiredLRPCreatedEvent(&models.DesiredLRP{
						ProcessGuid: "pg-4",
						Routes:      &routes,
						Instances:   1,
					}, "some-trace-id")

					endpoint4 = routingtable.Endpoint{
						InstanceGUID:    "ig-4",
						Host:            "3.3.3.3",
						Index:           1,
						Port:            23,
						ContainerPort:   8080,
						Presence:        models.ActualLRP_Ordinary,
						ModificationTag: &models.ModificationTag{Epoch: "abc", Index: 1},
					}

					actualLRPEvent := models.NewActualLRPInstanceCreatedEvent(&models.ActualLRP{
						ActualLrpKey:         models.NewActualLRPKey("pg-4", 0, "domain"),
						ActualLrpInstanceKey: models.NewActualLRPInstanceKey(endpoint4.InstanceGUID, "cell-id"),
						ActualLrpNetInfo:     models.NewActualLRPNetInfo(endpoint4.Host, "container-ip-4", models.ActualLRPNetInfo_PreferredAddressHost, models.NewPortMapping(endpoint4.Port, endpoint4.ContainerPort)),
						State:                models.ActualLRPStateRunning,
					}, "some-trace-id")

					cachedEvents := map[string]models.Event{
						desiredLRPEvent.Key(): desiredLRPEvent,
						actualLRPEvent.Key():  actualLRPEvent,
					}
					routeHandler.Sync(
						logger,
						desiredLRPs,
						actualLRPs,
						domains,
						cachedEvents,
					)
				})

				It("updates the routing table and emit cached events", func() {
					Expect(fakeTable.SwapCallCount()).Should(Equal(1))
					_, tempRoutingTable, _ := fakeTable.SwapArgsForCall(0)
					Expect(tempRoutingTable.HTTPAssociationsCount()).Should(Equal(4))
					Expect(natsEmitter.EmitCallCount()).Should(Equal(1))
				})
			})
		})
	})

	Describe("EmitExternal", func() {
		var registrationMsgs routingtable.MessagesToEmit
		BeforeEach(func() {
			currentTag := &models.ModificationTag{Epoch: "abc", Index: 1}
			endpoint1 := routingtable.Endpoint{
				InstanceGUID:    "ig-1",
				Host:            "1.1.1.1",
				Index:           0,
				Port:            11,
				ContainerPort:   8080,
				Presence:        models.ActualLRP_Ordinary,
				ModificationTag: currentTag,
			}
			endpoint2 := routingtable.Endpoint{
				InstanceGUID:    "ig-2",
				Host:            "2.2.2.2",
				Index:           0,
				Port:            22,
				ContainerPort:   8080,
				Presence:        models.ActualLRP_Ordinary,
				ModificationTag: currentTag,
			}
			endpoint3 := routingtable.Endpoint{
				InstanceGUID:    "ig-3",
				Host:            "2.2.2.2",
				Index:           1,
				Port:            23,
				ContainerPort:   8080,
				Presence:        models.ActualLRP_Ordinary,
				ModificationTag: currentTag,
			}
			route := routingtable.Route{}
			registrationMsgs = routingtable.MessagesToEmit{
				RegistrationMessages: []routingtable.RegistryMessage{
					routingtable.RegistryMessageFor(endpoint1, route, true),
					routingtable.RegistryMessageFor(endpoint2, route, true),
					routingtable.RegistryMessageFor(endpoint3, route, true),
				},
			}

			fakeTable.GetExternalRoutingEventsReturns(emptyTCPRouteMappings, registrationMsgs)
			fakeTable.HTTPAssociationsCountReturns(3)
		})
		It("emits all registration events", func() {
			routeHandler.EmitExternal(logger)
			Expect(fakeTable.GetExternalRoutingEventsCallCount()).To(Equal(1))
			Expect(natsEmitter.EmitCallCount()).To(Equal(1))
			Expect(natsEmitter.EmitArgsForCall(0)).To(Equal(registrationMsgs))
		})

		It("sends a 'routes total' metric", func() {
			routeHandler.EmitExternal(logger)
			Eventually(metricChan).Should(Receive(Equal(metric{
				name:  "RoutesTotal",
				value: 3,
			})))
		})

		It("sends a 'synced routes' metric", func() {
			routeHandler.EmitExternal(logger)
			Eventually(counterChan).Should(Receive(Equal(counter{
				name:  "RoutesSynced",
				delta: 3,
			})))
		})
	})

	Describe("EmitInternal", func() {
		var registrationMsgs routingtable.MessagesToEmit
		BeforeEach(func() {
			endpoint1 := routingtable.Endpoint{
				ContainerIP: "1.2.3.4",
				Since:       1,
			}
			endpoint2 := routingtable.Endpoint{
				ContainerIP: "1.2.3.5",
				Since:       2,
			}
			endpoint3 := routingtable.Endpoint{
				ContainerIP: "1.2.3.6",
				Since:       3,
			}
			registrationMsgs = routingtable.MessagesToEmit{
				InternalRegistrationMessages: []routingtable.RegistryMessage{
					{
						URIs:                 []string{"internal", "0.internal"},
						Host:                 endpoint1.ContainerIP,
						Tags:                 map[string]string{"component": "route-emitter"},
						App:                  logGuid,
						EndpointUpdatedAtNs:  endpoint1.Since,
						PrivateInstanceIndex: "0",
					},
					{
						URIs:                 []string{"internal", "0.internal"},
						Host:                 endpoint2.ContainerIP,
						Tags:                 map[string]string{"component": "route-emitter"},
						App:                  logGuid,
						EndpointUpdatedAtNs:  endpoint2.Since,
						PrivateInstanceIndex: "0",
					},
					{
						URIs:                 []string{"internal", "0.internal"},
						Host:                 endpoint3.ContainerIP,
						Tags:                 map[string]string{"component": "route-emitter"},
						App:                  logGuid,
						EndpointUpdatedAtNs:  endpoint3.Since,
						PrivateInstanceIndex: "0",
					},
				},
			}

			fakeTable.GetInternalRoutingEventsReturns(emptyTCPRouteMappings, registrationMsgs)
			fakeTable.HTTPAssociationsCountReturns(3)
		})

		It("emits all internal registration events", func() {
			routeHandler.EmitInternal(logger)
			Expect(fakeTable.GetInternalRoutingEventsCallCount()).To(Equal(1))
			Expect(natsEmitter.EmitCallCount()).To(Equal(1))
			Expect(natsEmitter.EmitArgsForCall(0)).To(Equal(registrationMsgs))
		})
	})

	Describe("RefreshDesired", func() {
		BeforeEach(func() {
			fakeTable.SetRoutesReturns(emptyTCPRouteMappings, routingtable.MessagesToEmit{})
		})

		It("adds the desired info to the routing table", func() {
			r1 := cfroutes.CFRoutes{
				cfroutes.CFRoute{
					Hostnames:       []string{"foo.example.com"},
					Port:            8080,
					RouteServiceUrl: "https://rs.example.com",
				},
			}.RoutingInfo()
			desiredLRPs := &models.DesiredLRP{
				ProcessGuid: "pg-1",
				Domain:      "tests",
				LogGuid:     "lg1",
				Routes:      &r1,
				Instances:   1,
			}
			routeHandler.RefreshDesired(logger, []*models.DesiredLRP{desiredLRPs})

			Expect(fakeTable.SetRoutesCallCount()).To(Equal(1))
			_, before, after := fakeTable.SetRoutesArgsForCall(0)
			Expect(before).To(BeNil())
			Expect(after).To(Equal(desiredLRPs))
			Expect(natsEmitter.EmitCallCount()).Should(Equal(1))
		})
	})

	Describe("ShouldRefreshDesired", func() {
		var (
			actualLRP *models.ActualLRP
		)
		BeforeEach(func() {
			currentTag := models.ModificationTag{Epoch: "abc", Index: 1}
			endpoint1 := routingtable.Endpoint{
				InstanceGUID:    "ig-1",
				Host:            "1.1.1.1",
				Index:           0,
				Port:            11,
				ContainerPort:   8080,
				Presence:        models.ActualLRP_Ordinary,
				ModificationTag: &currentTag,
			}

			actualLRP = &models.ActualLRP{
				ActualLrpKey:         models.NewActualLRPKey("pg-1", 0, "domain"),
				ActualLrpInstanceKey: models.NewActualLRPInstanceKey(endpoint1.InstanceGUID, "cell-id"),
				ActualLrpNetInfo: models.NewActualLRPNetInfo(
					endpoint1.Host,
					"container-ip-1",
					models.ActualLRPNetInfo_PreferredAddressHost,
					models.NewPortMapping(endpoint1.Port, endpoint1.ContainerPort),
					models.NewPortMapping(12, endpoint1.ContainerPort+1),
				),
				State:           models.ActualLRPStateRunning,
				ModificationTag: currentTag,
			}
		})

		Context("when corresponding desired state exists in the table", func() {
			BeforeEach(func() {
				fakeTable.HasExternalRoutesReturns(false)
			})

			It("returns false", func() {
				Expect(routeHandler.ShouldRefreshDesired(actualLRP)).To(BeTrue())
			})
		})

		Context("when corresponding desired state does not exist in the table", func() {
			BeforeEach(func() {
				fakeTable.HasExternalRoutesReturns(true)
			})

			It("returns true", func() {
				Expect(routeHandler.ShouldRefreshDesired(actualLRP)).To(BeFalse())
			})
		})
	})
})
