package routingtable_test

import (
	"encoding/json"
	"time"

	"code.cloudfoundry.org/bbs/models"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/route-emitter/routingtable"
	"code.cloudfoundry.org/route-emitter/routingtable/fakeroutingtable"
	tcpmodels "code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/routing-info/tcp_routes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

const (
	DEFAULT_TIMEOUT          = 5 * time.Second
	DEFAULT_POLLING_INTERVAL = 5 * time.Millisecond
)

var _ = Describe("TCPRoutingTable", func() {

	var (
		routingTable     routingtable.RoutingTable
		modificationTag  *models.ModificationTag
		tcpRoutes        tcp_routes.TCPRoutes
		logger           lager.Logger
		fakeMetronClient *mfakes.FakeIngressClient
	)

	getDesiredLRP := func(
		processGuid, logGuid string,
		tcpRoutes tcp_routes.TCPRoutes,
		modificationTag *models.ModificationTag,
	) *models.DesiredLRP {
		var desiredLRP models.DesiredLRP
		portMap := map[uint32]struct{}{}
		for _, tcpRoute := range tcpRoutes {
			portMap[tcpRoute.ContainerPort] = struct{}{}
		}

		ports := []uint32{}
		for k := range portMap {
			ports = append(ports, k)
		}

		desiredLRP.ProcessGuid = processGuid
		desiredLRP.Ports = ports
		desiredLRP.LogGuid = logGuid
		desiredLRP.Instances = 3
		desiredLRP.ModificationTag = modificationTag
		desiredLRP.Routes = tcpRoutes.RoutingInfo()
		desiredLRP.Domain = "domain"

		// add 'diego-ssh' data for testing sanitize
		routingInfo := json.RawMessage([]byte(`{ "private_key": "fake-key" }`))
		(*desiredLRP.Routes)["diego-ssh"] = &routingInfo

		return &desiredLRP
	}

	getActualLRP := func(
		processGuid, instanceGuid, hostAddress, instanceAddress string,
		hostPort, containerPort, tlsHostPort, tlsContainerPort uint32,
		modificationTag *models.ModificationTag,
	) *models.ActualLRP {
		return &models.ActualLRP{
			ActualLrpKey:         models.NewActualLRPKey(processGuid, 0, "domain"),
			ActualLrpInstanceKey: models.NewActualLRPInstanceKey(instanceGuid, "cell-id-1"),
			ActualLrpNetInfo: models.NewActualLRPNetInfo(
				hostAddress,
				instanceAddress,
				models.ActualLRPNetInfo_PreferredAddressUnknown,
				models.NewPortMappingWithTLSProxy(hostPort, containerPort, tlsHostPort, tlsContainerPort),
			),
			Presence:        models.ActualLRP_Ordinary,
			State:           models.ActualLRPStateRunning,
			ModificationTag: *modificationTag,
		}
	}

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeMetronClient = &mfakes.FakeIngressClient{}
		tcpRoutes = tcp_routes.TCPRoutes{
			tcp_routes.TCPRoute{
				RouterGroupGuid: "router-group-guid",
				ExternalPort:    61000,
				ContainerPort:   5222,
			},
		}
	})

	Context("when no entry exist for route", func() {
		BeforeEach(func() {
			routingTable = routingtable.NewRoutingTable(false, false, fakeMetronClient)
			modificationTag = &models.ModificationTag{Epoch: "abc", Index: 0}
		})

		Describe("AddRoutes", func() {
			It("emits nothing", func() {
				desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingEvents, _ := routingTable.SetRoutes(logger, nil, desiredLRP)
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})

			It("does not emit any sensitive information", func() {
				desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingEvents, _ := routingTable.SetRoutes(logger, nil, desiredLRP)
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})

			It("logs required routing info", func() {
				desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingEvents, _ := routingTable.SetRoutes(logger, nil, desiredLRP)
				// for i := 0; i < 3; i++ {
				Eventually(logger, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(gbytes.Say("process-guid.*process-guid-1"))
				Eventually(logger, DEFAULT_TIMEOUT, DEFAULT_POLLING_INTERVAL).Should(gbytes.Say("routes.*tcp-router.*61000.*5222"))
				// }

				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})
		})

		Describe("RemoveRoutes", func() {
			It("emits nothing", func() {
				desiredLRP := getDesiredLRP("process-guid-10", "log-guid-10", tcpRoutes, modificationTag)
				routingEvents, _ := routingTable.RemoveRoutes(logger, desiredLRP)
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})

			It("does not log sensitive info", func() {
				desiredLRP := getDesiredLRP("process-guid-10", "log-guid-10", tcpRoutes, modificationTag)
				routingEvents, _ := routingTable.RemoveRoutes(logger, desiredLRP)
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})

			It("does not log any routing info", func() {
				Consistently(logger).ShouldNot(gbytes.Say("remove-routes"))
			})
		})

		Describe("AddEndpoint", func() {
			It("emits nothing", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61104, 5222, 61443, 5443, modificationTag)
				routingEvents, _ := routingTable.AddEndpoint(logger, actualLRP)
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})

			It("does not log sensitive info", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61104, 5222, 61443, 5443, modificationTag)
				routingEvents, _ := routingTable.AddEndpoint(logger, actualLRP)
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
				Consistently(logger).ShouldNot(gbytes.Say("private_key"))
			})

			It("logs required routing info", func() {
				Consistently(logger).ShouldNot(gbytes.Say("add-endpoint"))
			})
		})

		Describe("RemoveEndpoint", func() {
			It("emits nothing", func() {
				actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61104, 5222, 61443, 5443, modificationTag)
				routingEvents, _ := routingTable.RemoveEndpoint(logger, actualLRP)
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})

			It("does not log endpoint removal", func() {
				Consistently(logger).ShouldNot(gbytes.Say("remove-endpoint"))
			})

			It("logs required routing info", func() {
				Consistently(logger).ShouldNot(gbytes.Say("remove-endpoint"))
			})
		})

		Describe("Swap", func() {
			var (
				tempRoutingTable routingtable.RoutingTable
				logGuid          string
			)

			BeforeEach(func() {
				logGuid = "log-guid-1"
				tempRoutingTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
				beforeLRP := getDesiredLRP("process-guid-1", logGuid, tcpRoutes, modificationTag)
				tempRoutingTable.SetRoutes(logger, nil, beforeLRP)
				tempRoutingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, modificationTag))
				tempRoutingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-2", "some-ip-2", "container-ip-2", 62004, 5222, 61443, 5443, modificationTag))
				Expect(tempRoutingTable.TCPAssociationsCount()).Should(Equal(2))
			})

			It("emits routing events for new routes", func() {
				Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
				routingEvents, _ := routingTable.Swap(logger, tempRoutingTable, models.DomainSet{})
				Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))

				Expect(routingEvents.Registrations).Should(ConsistOf(
					tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
					tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-2", 62004, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
				))
			})

			Context("when the table is configured to emit direct instance route", func() {
				BeforeEach(func() {
					routingTable = routingtable.NewRoutingTable(true, false, fakeMetronClient)
				})

				It("emits routing events for new routes", func() {
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
					routingEvents, _ := routingTable.Swap(logger, tempRoutingTable, models.DomainSet{})
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))

					Expect(routingEvents.Registrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "container-ip-1", 5222, 5443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "container-ip-2", 5222, 5443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
					))
				})

				Context("when instance prefers host address", func() {
					It("emits routing events for new routes", func() {
						tempRoutingTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
						beforeLRP := getDesiredLRP("process-guid-1", logGuid, tcpRoutes, modificationTag)
						tempRoutingTable.SetRoutes(logger, nil, beforeLRP)
						actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, modificationTag)
						actualLRP.ActualLrpNetInfo.PreferredAddress = models.ActualLRPNetInfo_PreferredAddressHost
						tempRoutingTable.AddEndpoint(logger, actualLRP)
						Expect(tempRoutingTable.TCPAssociationsCount()).Should(Equal(1))
						Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
						routingEvents, _ := routingTable.Swap(logger, tempRoutingTable, models.DomainSet{})
						Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))

						Expect(routingEvents.Registrations).Should(ConsistOf(
							tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						))
					})
				})
			})
		})

		Describe("GetRoutingEvents", func() {
			It("returns empty routing events", func() {
				routingEvents, _ := routingTable.GetExternalRoutingEvents()
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
			})
		})

		Context("when the routing tables are of different type", func() {
			It("should not swap the tables", func() {
				routingTable = routingtable.NewRoutingTable(false, false, fakeMetronClient)
				fakeTable := &fakeroutingtable.FakeRoutingTable{}
				routingEvents, _ := routingTable.Swap(logger, fakeTable, models.DomainSet{})
				Expect(routingEvents.Registrations).To(HaveLen(0))
				Expect(routingEvents.Unregistrations).To(HaveLen(0))
				Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
			})
		})
	})

	Context("when there exists an entry for route", func() {
		var (
			logGuid string
		)

		BeforeEach(func() {
			logGuid = "log-guid-1"
			modificationTag = &models.ModificationTag{Epoch: "abc", Index: 1}
		})

		Describe("HasExternalRoutes", func() {
			It("returns the associated desired state", func() {
				routingTable = routingtable.NewRoutingTable(true, false, fakeMetronClient)
				beforeLRP := getDesiredLRP("process-guid-1", logGuid, tcpRoutes, modificationTag)
				routingTable.SetRoutes(logger, nil, beforeLRP)
				routingInfo := getActualLRP("process-guid-1", "instance-guid-2", "some-ip-2", "container-ip-2", 62004, 5222, 61443, 5443, modificationTag)
				routingTable.AddEndpoint(logger, routingInfo)
				Expect(routingTable.HasExternalRoutes(routingInfo)).To(BeTrue())
			})
		})

		Describe("AddRoutes", func() {
			var newTcpRoutes tcp_routes.TCPRoutes

			BeforeEach(func() {
				routingTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
				beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingTable.SetRoutes(logger, nil, beforeLRP)
				routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, modificationTag))
				routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-2", "some-ip-2", "container-ip-2", 62004, 5222, 61443, 5443, modificationTag))
				Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
			})

			Context("existing external port changes", func() {
				BeforeEach(func() {
					newTcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61001,
							ContainerPort:   5222,
						},
					}
				})

				It("emits routing event with modified external port, HostTLSPort and InstanceId", func() {
					newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
					routingEvents, _ := routingTable.SetRoutes(logger, nil, desiredLRP)

					Expect(routingEvents.Unregistrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-2", 62004, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
					))

					Expect(routingEvents.Registrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61001, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61001, "some-ip-2", 62004, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
					))

					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
				})

				Context("older modification tag", func() {
					It("emits nothing", func() {
						desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
						routingEvents, _ := routingTable.SetRoutes(logger, nil, desiredLRP)
						Expect(routingEvents.Registrations).To(HaveLen(0))
						Expect(routingEvents.Unregistrations).To(HaveLen(0))
					})

					It("does not change the routing table", func() {
						Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
					})
				})

			})

			Context("new external port is added", func() {
				BeforeEach(func() {
					tcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61000,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61001,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61002,
							ContainerPort:   5222,
						},
					}
				})

				It("emits routing event with both external ports", func() {
					modificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingEvents, _ := routingTable.SetRoutes(logger, nil, desiredLRP)

					Expect(routingEvents.Registrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61001, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61001, "some-ip-2", 62004, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61002, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61002, "some-ip-2", 62004, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
					))
					Expect(routingEvents.Unregistrations).Should(HaveLen(0))

					Expect(routingTable.TCPAssociationsCount()).Should(Equal(6))
				})

				Context("older modification tag", func() {
					It("emits nothing", func() {
						desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
						routingEvents, _ := routingTable.SetRoutes(logger, nil, desiredLRP)
						Expect(routingEvents.Registrations).To(HaveLen(0))
						Expect(routingEvents.Unregistrations).To(HaveLen(0))
					})

					It("does not change the routing table", func() {
						Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
					})
				})
			})

			Context("multiple external port added and multiple existing external ports deleted", func() {
				BeforeEach(func() {
					currentTcpRoutes := tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61000,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61001,
							ContainerPort:   5222,
						},
					}

					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", currentTcpRoutes, modificationTag)
					routingTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
					routingTable.SetRoutes(logger, nil, desiredLRP)
					routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, modificationTag))
					routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-2", "some-ip-2", "container-ip-2", 62004, 5222, 61443, 5443, modificationTag))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(4))

					newTcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61002,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61003,
							ContainerPort:   5222,
						},
					}
				})

				It("emits routing event with both external ports", func() {
					newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
					routingEvents, _ := routingTable.SetRoutes(logger, nil, desiredLRP)

					Expect(routingEvents.Registrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61002, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61002, "some-ip-2", 62004, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61003, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61003, "some-ip-2", 62004, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
					))

					Expect(routingEvents.Unregistrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-2", 62004, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61001, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61001, "some-ip-2", 62004, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
					))

					Expect(routingTable.TCPAssociationsCount()).Should(Equal(4))
				})
			})

			Context("when TLS is disabled", func() {
				BeforeEach(func() {
					routingTable = routingtable.NewRoutingTable(false, false, fakeMetronClient)
					beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingTable.SetRoutes(logger, nil, beforeLRP)
					routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, modificationTag))
					routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-2", "some-ip-2", "container-ip-2", 62004, 5222, 61443, 5443, modificationTag))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
				})

				It("emits empty InstanceId and 0 for HostTLSPort", func() {
					newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					newTcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61001,
							ContainerPort:   5222,
						},
					}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
					routingEvents, _ := routingTable.SetRoutes(logger, nil, desiredLRP)

					Expect(routingEvents.Unregistrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, -1, "", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-2", 62004, -1, "", nil, 0, tcpmodels.ModificationTag{}),
					))

					Expect(routingEvents.Registrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61001, "some-ip-1", 62004, -1, "", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61001, "some-ip-2", 62004, -1, "", nil, 0, tcpmodels.ModificationTag{}),
					))

					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
				})
			})

			Context("older modification tag", func() {
				It("emits nothing", func() {
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingEvents, _ := routingTable.SetRoutes(logger, nil, desiredLRP)
					Expect(routingEvents.Registrations).To(HaveLen(0))
					Expect(routingEvents.Unregistrations).To(HaveLen(0))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
				})
			})

			Context("no changes to external port", func() {
				It("emits nothing", func() {
					tag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, tag)
					routingEvents, _ := routingTable.SetRoutes(logger, nil, desiredLRP)
					Expect(routingEvents.Registrations).To(HaveLen(0))
					Expect(routingEvents.Unregistrations).To(HaveLen(0))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
				})
			})

			Context("when two disjoint (external port, container port) pairs are given", func() {
				BeforeEach(func() {
					beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
					routingTable.SetRoutes(logger, nil, beforeLRP)
					routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, modificationTag))
					routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 63004, 5223, 61443, 5443, modificationTag))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
				})

				It("emits two separate registration events with no overlap", func() {
					newTcpRoutes := tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61001,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61002,
							ContainerPort:   5223,
						},
					}
					newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					routingEvents, _ := routingTable.SetRoutes(logger, nil, getDesiredLRP("process-guid-1", logGuid, newTcpRoutes, newModificationTag))

					// Two registration and one unregistration events
					Expect(routingEvents.Registrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61001, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61002, "some-ip-1", 63004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
					))

					Expect(routingEvents.Unregistrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
					))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
				})
			})

			Context("when container ports don't match", func() {
				BeforeEach(func() {
					tcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61000,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							ExternalPort:  61000,
							ContainerPort: 5223,
						},
					}
				})

				It("emits nothing", func() {
					newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newTag)
					routingEvents, _ := routingTable.SetRoutes(logger, nil, desiredLRP)
					Expect(routingEvents.Registrations).To(HaveLen(0))
					Expect(routingEvents.Unregistrations).To(HaveLen(0))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
				})
			})
		})

		Describe("Updating routes", func() {
			var (
				newTcpRoutes       tcp_routes.TCPRoutes
				newModificationTag *models.ModificationTag
				beforeLRP          *models.DesiredLRP
			)

			BeforeEach(func() {
				newModificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
				routingTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
				beforeLRP = getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingTable.SetRoutes(logger, nil, beforeLRP)
				routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, modificationTag))
				Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
			})

			Context("when there is no change in container ports", func() {
				BeforeEach(func() {
					newTcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61001,
							ContainerPort:   5222,
						},
					}
				})

				Context("existing router group guid changes", func() {
					BeforeEach(func() {
						newTcpRoutes = tcp_routes.TCPRoutes{
							tcp_routes.TCPRoute{
								RouterGroupGuid: "new-router-group-guid",
								ExternalPort:    61000,
								ContainerPort:   5222,
							},
						}
					})

					It("emits unregistration and registration mappings", func() {
						newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
						beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
						afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
						routingEvents, _ := routingTable.SetRoutes(logger, beforeLRP, afterLRP)

						Expect(routingEvents.Registrations).Should(ConsistOf(
							tcpmodels.NewTcpRouteMapping("new-router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						))

						Expect(routingEvents.Unregistrations).Should(ConsistOf(
							tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						))
					})
				})

				Context("when there is change in external port", func() {
					It("emits registration and unregistration events", func() {
						afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
						routingEvents, _ := routingTable.SetRoutes(logger, nil, afterLRP)

						Expect(routingEvents.Registrations).Should(ConsistOf(
							tcpmodels.NewTcpRouteMapping("router-group-guid", 61001, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						))

						Expect(routingEvents.Unregistrations).Should(ConsistOf(
							tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						))

						Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
					})

					Context("with older modification tag", func() {
						It("emits nothing", func() {
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
							routingEvents, _ := routingTable.SetRoutes(logger, nil, afterLRP)
							Expect(routingEvents.Registrations).To(HaveLen(0))
							Expect(routingEvents.Unregistrations).To(HaveLen(0))
							Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
						})
					})
				})

				Context("when there is no change in external port", func() {
					It("emits nothing", func() {
						afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newModificationTag)
						routingEvents, _ := routingTable.SetRoutes(logger, beforeLRP, afterLRP)
						Expect(routingEvents.Registrations).To(HaveLen(0))
						Expect(routingEvents.Unregistrations).To(HaveLen(0))
						Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
					})
				})
			})

			Context("when new container port is added", func() {
				Context("when mapped to new external port", func() {
					BeforeEach(func() {
						newTcpRoutes = tcp_routes.TCPRoutes{
							tcp_routes.TCPRoute{
								RouterGroupGuid: "router-group-guid",
								ExternalPort:    61000,
								ContainerPort:   5222,
							},
							tcp_routes.TCPRoute{
								RouterGroupGuid: "router-group-guid",
								ExternalPort:    61001,
								ContainerPort:   5223,
							},
						}
					})

					Context("no backends for new container port", func() {
						It("emits no routing events and adds to routing table entry", func() {
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							routingEvents, _ := routingTable.SetRoutes(logger, beforeLRP, afterLRP)
							Expect(routingEvents.Registrations).To(HaveLen(0))
							Expect(routingEvents.Unregistrations).To(HaveLen(0))
							Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
						})

						Context("with older modification tag", func() {
							It("emits nothing but add the routing table entry", func() {
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								currentRoutesCount := routingTable.TCPAssociationsCount()
								routingEvents, _ := routingTable.SetRoutes(logger, beforeLRP, afterLRP)
								Expect(routingEvents.Registrations).To(HaveLen(0))
								Expect(routingEvents.Unregistrations).To(HaveLen(0))
								Expect(routingTable.TCPAssociationsCount()).Should(Equal(currentRoutesCount))
							})
						})
					})

					Context("existing backends for new container port", func() {
						BeforeEach(func() {
							routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-2", "some-ip-1", "container-ip-1", 62006, 5223, 61443, 5443, modificationTag))
						})

						It("emits registration events for new container port", func() {
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							currentRoutesCount := routingTable.TCPAssociationsCount()
							routingEvents, _ := routingTable.SetRoutes(logger, beforeLRP, afterLRP)
							Expect(routingEvents.Registrations).Should(ConsistOf(
								tcpmodels.NewTcpRouteMapping("router-group-guid", 61001, "some-ip-1", 62006, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
							))

							Expect(routingEvents.Unregistrations).To(HaveLen(0))

							Expect(routingTable.TCPAssociationsCount()).Should(Equal(currentRoutesCount + 1))
						})
					})
				})

				Context("when mapped to existing external port", func() {
					BeforeEach(func() {
						newTcpRoutes = tcp_routes.TCPRoutes{
							tcp_routes.TCPRoute{
								RouterGroupGuid: "router-group-guid",
								ExternalPort:    61000,
								ContainerPort:   5222,
							},
							tcp_routes.TCPRoute{
								RouterGroupGuid: "router-group-guid",
								ExternalPort:    61001,
								ContainerPort:   5223,
							},
						}
					})

					Context("no backends for new container port", func() {
						var (
							routingEvents routingtable.TCPRouteMappings
						)

						BeforeEach(func() {
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							routingEvents, _ = routingTable.SetRoutes(logger, beforeLRP, afterLRP)
						})

						It("emits no routing events and adds to routing table entry", func() {
							Expect(routingEvents.Registrations).To(HaveLen(0))
							Expect(routingEvents.Unregistrations).To(HaveLen(0))
						})

						Context("when the actual lrp is updated", func() {
							BeforeEach(func() {
								oldActualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, modificationTag)
								routingTable.RemoveEndpoint(logger, oldActualLRP)
								newActualLRP := oldActualLRP
								newActualLRP.ActualLrpNetInfo = models.NewActualLRPNetInfo(
									"some-ip-1",
									"container-ip-1",
									models.ActualLRPNetInfo_PreferredAddressHost,
									models.NewPortMappingWithTLSProxy(62004, 5222, 61443, 5443),
									models.NewPortMappingWithTLSProxy(62005, 5223, 62443, 5243),
								)
								routingEvents, _ = routingTable.AddEndpoint(logger, newActualLRP)
							})

							It("emits two registration events", func() {
								Expect(routingEvents.Registrations).Should(ConsistOf(
									tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
									tcpmodels.NewTcpRouteMapping("router-group-guid", 61001, "some-ip-1", 62005, 62443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
								))
							})
						})

						Context("with older modification tag", func() {
							It("emits nothing but add the routing table entry", func() {
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								currentRoutesCount := routingTable.TCPAssociationsCount()
								routingEvents, _ := routingTable.SetRoutes(logger, beforeLRP, afterLRP)
								Expect(routingEvents.Registrations).To(HaveLen(0))
								Expect(routingEvents.Unregistrations).To(HaveLen(0))
								Expect(routingTable.TCPAssociationsCount()).Should(Equal(currentRoutesCount))
							})
						})
					})

					Context("existing backends for new container port", func() {
						var numActualLRPs int

						BeforeEach(func() {
							routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-2", "some-ip-1", "container-ip-1", 62006, 5223, 61443, 5443, modificationTag))
							numActualLRPs++
						})

						It("emits registration events for new container port", func() {
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							expectedRoutesCount := len(newTcpRoutes) * numActualLRPs

							routingEvents, _ := routingTable.SetRoutes(logger, beforeLRP, afterLRP)

							Expect(routingEvents.Registrations).Should(ConsistOf(
								tcpmodels.NewTcpRouteMapping("router-group-guid", 61001, "some-ip-1", 62006, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
							))
							Expect(routingTable.TCPAssociationsCount()).Should(Equal(expectedRoutesCount))
						})
					})
				})
			})

			Context("when existing container port is removed", func() {
				Context("when there are no routes left", func() {
					BeforeEach(func() {
						newTcpRoutes = tcp_routes.TCPRoutes{}
					})

					It("emits only unregistration events", func() {
						afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
						routingEvents, _ := routingTable.SetRoutes(logger, beforeLRP, afterLRP)
						Expect(routingEvents.Unregistrations).Should(ConsistOf(
							tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						))
					})

					Context("with older modification tag", func() {
						It("emits nothing", func() {
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
							routingEvents, _ := routingTable.SetRoutes(logger, beforeLRP, afterLRP)
							Expect(routingEvents.Registrations).To(HaveLen(0))
							Expect(routingEvents.Unregistrations).To(HaveLen(0))
							Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
						})
					})
				})

				Context("when container port is switched", func() {
					BeforeEach(func() {
						newTcpRoutes = tcp_routes.TCPRoutes{
							tcp_routes.TCPRoute{
								RouterGroupGuid: "router-group-guid",
								ExternalPort:    61000,
								ContainerPort:   5223,
							},
						}
					})

					Context("no backends for new container port", func() {
						It("only emits unregistration events and adds to routing table entry", func() {
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							routingEvents, _ := routingTable.SetRoutes(logger, beforeLRP, afterLRP)
							Expect(routingEvents.Unregistrations).Should(ConsistOf(
								tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
							))
						})

						Context("with older modification tag", func() {
							It("emits nothing but add the routing table entry", func() {
								afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
								Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
								routingEvents, _ := routingTable.SetRoutes(logger, beforeLRP, afterLRP)
								Expect(routingEvents.Registrations).To(HaveLen(0))
								Expect(routingEvents.Unregistrations).To(HaveLen(0))
								Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
							})
						})
					})

					Context("existing backends for new container port", func() {
						BeforeEach(func() {
							routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62006, 5223, 61443, 5443, modificationTag))
							Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
						})

						It("emits registration events for new container port", func() {
							afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, newModificationTag)
							// currentRoutesCount := routingTable.RouteCount()
							routingEvents, _ := routingTable.SetRoutes(logger, beforeLRP, afterLRP)
							Expect(routingEvents.Registrations).To(HaveLen(1))
							Expect(routingEvents.Unregistrations).To(HaveLen(1))

							Expect(routingEvents.Registrations).Should(ConsistOf(
								tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62006, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
							))
							Expect(routingEvents.Unregistrations).Should(ConsistOf(
								tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
							))
						})
					})
				})
			})

			Context("when an existing route is removed", func() {
				BeforeEach(func() {
					newTcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61000,
							ContainerPort:   5222,
						},
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    62000,
							ContainerPort:   5222,
						},
					}
					routingTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
					beforeLRP = getDesiredLRP("process-guid-1", "log-guid-1", newTcpRoutes, modificationTag)
					routingTable.SetRoutes(logger, nil, beforeLRP)
					routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, modificationTag))
				})

				It("emits an unregistration event and keeps the other route", func() {
					afterLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newModificationTag)
					routingEvents, _ := routingTable.SetRoutes(logger, beforeLRP, afterLRP)
					Expect(routingEvents.Unregistrations).To(HaveLen(1))

					Expect(routingEvents.Unregistrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 62000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
					))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
				})
			})
		})

		Describe("RemoveRoutes", func() {
			Context("when entry does not have endpoints", func() {
				var (
					desiredLRP *models.DesiredLRP
				)

				BeforeEach(func() {
					routingTable = routingtable.NewRoutingTable(false, false, fakeMetronClient)
					desiredLRP = getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingTable.SetRoutes(logger, nil, desiredLRP)
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
				})

				It("emits nothing", func() {
					modificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingEvents, _ := routingTable.RemoveRoutes(logger, desiredLRP)
					Expect(routingEvents.Registrations).To(HaveLen(0))
					Expect(routingEvents.Unregistrations).To(HaveLen(0))
				})
			})

			Context("when entry does have endpoints", func() {
				BeforeEach(func() {
					tcpRoutes = tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "router-group-guid",
							ExternalPort:    61000,
							ContainerPort:   5222,
						},
					}
					routingTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
					modificationTag := &models.ModificationTag{Epoch: "abc", Index: 1}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingTable.SetRoutes(logger, nil, desiredLRP)
					routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61000, 5222, 61443, 5443, modificationTag))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
				})

				It("emits unregistration routing events", func() {
					newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
					desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, newModificationTag)
					routingEvents, _ := routingTable.RemoveRoutes(logger, desiredLRP)
					Expect(routingEvents.Unregistrations).To(HaveLen(1))
					Expect(routingEvents.Unregistrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 61000, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
					))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
				})

				Context("when there are no external endpoints", func() {
					BeforeEach(func() {
						routingTable = routingtable.NewRoutingTable(false, false, fakeMetronClient)
						modificationTag := &models.ModificationTag{Epoch: "abc", Index: 1}
						desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
						routingTable.SetRoutes(logger, nil, desiredLRP)
						routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61000, 5222, 61443, 5443, modificationTag))
						Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
					})

					It("does not emit any routing events", func() {
						newModificationTag := &models.ModificationTag{Epoch: "abc", Index: 2}
						desiredLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcp_routes.TCPRoutes{}, newModificationTag)
						routingEvents, _ := routingTable.RemoveRoutes(logger, desiredLRP)
						Expect(routingEvents.Registrations).To(HaveLen(0))
						Expect(routingEvents.Unregistrations).To(HaveLen(0))
					})
				})
			})
		})

		Describe("AddEndpoint", func() {
			Context("with no existing endpoints", func() {
				BeforeEach(func() {
					routingTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
					beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingTable.SetRoutes(logger, nil, beforeLRP)
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
				})

				It("emits routing events", func() {
					newTag := &models.ModificationTag{Epoch: "abc", Index: 1}
					actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61104, 5222, 61443, 5443, newTag)
					routingEvents, _ := routingTable.AddEndpoint(logger, actualLRP)

					Expect(routingEvents.Registrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 61104, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
					))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
				})
			})

			Context("with existing endpoints", func() {
				BeforeEach(func() {
					routingTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
					beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingTable.SetRoutes(logger, nil, beforeLRP)
					routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, modificationTag))
					routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-2", "some-ip-2", "container-ip-2", 62004, 5222, 61443, 5443, modificationTag))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
				})

				Context("with different instance guid", func() {
					It("emits routing events", func() {
						newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
						actualLRP := getActualLRP("process-guid-1", "instance-guid-3", "some-ip-3", "container-ip-3", 61104, 5222, 61443, 5443, newTag)
						routingEvents, _ := routingTable.AddEndpoint(logger, actualLRP)
						Expect(routingEvents.Registrations).Should(ConsistOf(
							tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-3", 61104, 61443, "instance-guid-3", nil, 0, tcpmodels.ModificationTag{}),
						))
						Expect(routingTable.TCPAssociationsCount()).Should(Equal(3))
					})
				})

				Context("with same instance guid", func() {
					Context("newer modification tag", func() {
						It("emits routing events", func() {
							newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61105, 5222, 61443, 5443, newTag)
							routingEvents, _ := routingTable.AddEndpoint(logger, actualLRP)
							Expect(routingEvents.Registrations).Should(ConsistOf(
								tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 61105, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
							))
							Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
						})
					})

					Context("older modification tag", func() {
						It("emits nothing", func() {
							olderTag := &models.ModificationTag{Epoch: "abc", Index: 0}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61105, 5222, 61443, 5443, olderTag)
							routingEvents, _ := routingTable.AddEndpoint(logger, actualLRP)
							Expect(routingEvents.Registrations).To(HaveLen(0))
							Expect(routingEvents.Unregistrations).To(HaveLen(0))
						})
					})

					Context("when TLS port of actual LRP is updated", func() {
						It("emits unregistration for old route and registration for new route", func() {
							newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
							newActualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 62443, 5243, newTag)
							routingEvents, _ := routingTable.AddEndpoint(logger, newActualLRP)

							Expect(routingEvents.Unregistrations).Should(ConsistOf(
								tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
							))

							Expect(routingEvents.Registrations).Should(ConsistOf(
								tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 62443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
							))
						})
					})
				})
			})
		})

		Describe("RemoveEndpoint", func() {
			Context("with no existing endpoints", func() {
				BeforeEach(func() {
					routingTable = routingtable.NewRoutingTable(false, false, fakeMetronClient)
					beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingTable.SetRoutes(logger, nil, beforeLRP)
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
				})

				It("emits nothing", func() {
					newTag := &models.ModificationTag{Epoch: "abc", Index: 1}
					actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 61104, 5222, 61443, 5443, newTag)
					routingEvents, _ := routingTable.RemoveEndpoint(logger, actualLRP)
					Expect(routingEvents.Registrations).To(HaveLen(0))
					Expect(routingEvents.Unregistrations).To(HaveLen(0))
				})
			})

			Context("with existing endpoints", func() {
				BeforeEach(func() {
					routingTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
					beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
					routingTable.SetRoutes(logger, nil, beforeLRP)
					routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, modificationTag))
					Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
				})

				Context("with instance guid not present in existing endpoints", func() {
					It("emits nothing", func() {
						newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
						actualLRP := getActualLRP("process-guid-1", "instance-guid-3", "some-ip-3", "container-ip-3", 62004, 5222, 61443, 5443, newTag)
						routingEvents, _ := routingTable.RemoveEndpoint(logger, actualLRP)
						Expect(routingEvents.Registrations).To(HaveLen(0))
						Expect(routingEvents.Unregistrations).To(HaveLen(0))
					})
				})

				Context("with same instance guid", func() {
					Context("newer modification tag", func() {
						It("emits routing events", func() {
							newTag := &models.ModificationTag{Epoch: "abc", Index: 2}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, newTag)
							routingEvents, _ := routingTable.RemoveEndpoint(logger, actualLRP)
							Expect(routingEvents.Unregistrations).Should(ConsistOf(
								tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
							))

							Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
						})
					})

					Context("same modification tag", func() {
						It("emits routing events", func() {
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, modificationTag)
							routingEvents, _ := routingTable.RemoveEndpoint(logger, actualLRP)
							Expect(routingEvents.Unregistrations).Should(ConsistOf(
								tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
							))
							Expect(routingTable.TCPAssociationsCount()).Should(Equal(0))
						})
					})

					Context("older modification tag", func() {
						It("emits nothing", func() {
							olderTag := &models.ModificationTag{Epoch: "abc", Index: 0}
							actualLRP := getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, olderTag)
							routingEvents, _ := routingTable.RemoveEndpoint(logger, actualLRP)
							Expect(routingEvents.Registrations).To(HaveLen(0))
							Expect(routingEvents.Unregistrations).To(HaveLen(0))
						})
					})
				})
			})
		})

		Describe("GetRoutingEvents", func() {
			BeforeEach(func() {
				routingTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
				beforeLRP := getDesiredLRP("process-guid-1", "log-guid-1", tcpRoutes, modificationTag)
				routingTable.SetRoutes(logger, nil, beforeLRP)
				routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, modificationTag))
				Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
			})

			It("returns routing events for entries in routing table", func() {
				routingEvents, _ := routingTable.GetExternalRoutingEvents()
				Expect(routingEvents.Registrations).Should(ConsistOf(
					tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
				))
				Expect(routingTable.TCPAssociationsCount()).Should(Equal(1))
			})
		})

		Describe("Swap", func() {
			var (
				tempRoutingTable   routingtable.RoutingTable
				newModificationTag *models.ModificationTag
				logGuid            string
				existingLogGuid    string
			)

			BeforeEach(func() {
				existingLogGuid = "log-guid-1"
				newModificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
				routingTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
				beforeLRP := getDesiredLRP("process-guid-1", existingLogGuid, tcpRoutes, modificationTag)
				routingTable.SetRoutes(logger, nil, beforeLRP)
				routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, modificationTag))
				routingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-2", "some-ip-2", "container-ip-2", 62004, 5222, 61443, 5443, modificationTag))
				Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
			})

			Context("when adding a new routing key (process-guid, container-port)", func() {
				BeforeEach(func() {
					logGuid = "log-guid-2"
					tempRoutingTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
					beforeLRP := getDesiredLRP("process-guid-2", logGuid, tcpRoutes, newModificationTag)
					tempRoutingTable.SetRoutes(logger, nil, beforeLRP)
					tempRoutingTable.AddEndpoint(logger, getActualLRP("process-guid-2", "instance-guid-1", "some-ip-3", "container-ip-3", 63004, 5222, 61443, 5443, newModificationTag))
					tempRoutingTable.AddEndpoint(logger, getActualLRP("process-guid-2", "instance-guid-2", "some-ip-4", "container-ip-4", 63004, 5222, 61443, 5443, newModificationTag))
					Expect(tempRoutingTable.TCPAssociationsCount()).Should(Equal(2))
				})

				It("overwrites the existing entries and emits registration and unregistration routing events", func() {
					routingEvents, _ := routingTable.Swap(logger, tempRoutingTable, models.DomainSet{})
					Expect(routingEvents.Unregistrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-2", 62004, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
					))

					Expect(routingEvents.Registrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-3", 63004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-4", 63004, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
					))
				})
			})

			Context("when updating an existing routing key (process-guid, container-port)", func() {
				BeforeEach(func() {
					logGuid = "log-guid-2"
					tempRoutingTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
					beforeLRP := getDesiredLRP("process-guid-1", logGuid, tcpRoutes, newModificationTag)
					tempRoutingTable.SetRoutes(logger, nil, beforeLRP)
					tempRoutingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-3", "container-ip-3", 63004, 5222, 61443, 5443, newModificationTag))
					tempRoutingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-2", "some-ip-4", "container-ip-4", 63004, 5222, 61443, 5443, newModificationTag))
					Expect(tempRoutingTable.TCPAssociationsCount()).Should(Equal(2))

				})

				It("overwrites the existing entries and emits registration and unregistration routing events", func() {
					routingEvents, _ := routingTable.Swap(logger, tempRoutingTable, models.DomainSet{})
					Expect(routingEvents.Unregistrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-2", 62004, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
					))
					Expect(routingEvents.Registrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-3", 63004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-4", 63004, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
					))

					Expect(routingTable.TCPAssociationsCount()).Should(Equal(2))
				})
			})

			Context("when updating the router group guid", func() {
				BeforeEach(func() {
					newModificationTag = &models.ModificationTag{Epoch: "abc", Index: 2}
					newTcpRoutes := tcp_routes.TCPRoutes{
						tcp_routes.TCPRoute{
							RouterGroupGuid: "new-router-group-guid",
							ExternalPort:    61000,
							ContainerPort:   5222,
						},
					}
					beforeLRP := getDesiredLRP("process-guid-1", existingLogGuid, newTcpRoutes, newModificationTag)
					tempRoutingTable = routingtable.NewRoutingTable(false, true, fakeMetronClient)
					tempRoutingTable.SetRoutes(logger, nil, beforeLRP)
					tempRoutingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-1", "some-ip-1", "container-ip-1", 62004, 5222, 61443, 5443, modificationTag))
					tempRoutingTable.AddEndpoint(logger, getActualLRP("process-guid-1", "instance-guid-2", "some-ip-2", "container-ip-2", 62004, 5222, 61443, 5443, modificationTag))

					Expect(tempRoutingTable.TCPAssociationsCount()).Should(Equal(2))
				})

				It("emits registration and unregistration events", func() {
					domains := models.DomainSet{}
					domains.Add("domain")
					routingEvents, _ := routingTable.Swap(logger, tempRoutingTable, domains)

					Expect(routingEvents.Registrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("new-router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("new-router-group-guid", 61000, "some-ip-2", 62004, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
					))
					Expect(routingEvents.Unregistrations).Should(ConsistOf(
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-1", 62004, 61443, "instance-guid-1", nil, 0, tcpmodels.ModificationTag{}),
						tcpmodels.NewTcpRouteMapping("router-group-guid", 61000, "some-ip-2", 62004, 61443, "instance-guid-2", nil, 0, tcpmodels.ModificationTag{}),
					))
				})
			})
		})
	})
})
