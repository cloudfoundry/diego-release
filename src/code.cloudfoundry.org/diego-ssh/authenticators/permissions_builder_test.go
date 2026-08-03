package authenticators_test

import (
	"encoding/json"
	"net"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/diego-ssh/authenticators"
	"code.cloudfoundry.org/diego-ssh/routes"
	"code.cloudfoundry.org/diego-ssh/test_helpers/fake_ssh"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"
)

var _ = Describe("PermissionsBuilder", func() {
	Describe("Build", func() {
		var (
			logger        *lagertest.TestLogger
			expectedRoute routes.SSHRoute
			desiredLRP    *models.DesiredLRP
			actualLRP     *models.ActualLRP
			bbsClient     *fake_bbs.FakeInternalClient
			metadata      *fake_ssh.FakeConnMetadata

			permissionsBuilder authenticators.PermissionsBuilder
			permissions        *ssh.Permissions
			buildErr           error
			processGuid        string
			index              int
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")

			expectedRoute = routes.SSHRoute{
				ContainerPort:   1111,
				PrivateKey:      "fake-pem-encoded-key",
				HostFingerprint: "host-fingerprint",
				User:            "user",
				Password:        "password",
			}

			diegoSSHRoutePayload, err := json.Marshal(expectedRoute)
			Expect(err).NotTo(HaveOccurred())

			diegoSSHRouteMessage := json.RawMessage(diegoSSHRoutePayload)

			desiredLRP = &models.DesiredLRP{
				ProcessGuid: "some-guid",
				Instances:   2,
				Routes: &models.Routes{
					routes.DIEGO_SSH: &diegoSSHRouteMessage,
				},
				LogGuid: "log-guid",
				MetricTags: map[string]*models.MetricTagValue{
					"some_static_key":        &models.MetricTagValue{Static: "some_value"},
					"some_dynamic_key":       &models.MetricTagValue{Dynamic: models.MetricTagDynamicValueIndex},
					"some_other_dynamic_key": &models.MetricTagValue{Dynamic: models.MetricTagDynamicValueInstanceGuid},
				},
			}

			actualLRP = &models.ActualLRP{
				ActualLRPKey:         models.NewActualLRPKey("some-guid", 1, "some-domain"),
				ActualLRPInstanceKey: models.NewActualLRPInstanceKey("some-instance-guid", "some-cell-id"),
				ActualLRPNetInfo:     models.NewActualLRPNetInfo("1.2.3.4", "2.2.2.2", models.ActualLRPNetInfo_PreferredAddressUnknown, models.NewPortMappingWithTLSProxy(3333, 1111, 2222, 4444)),
			}

			bbsClient = new(fake_bbs.FakeInternalClient)
			bbsClient.ActualLRPsReturns([]*models.ActualLRP{actualLRP}, nil)
			bbsClient.DesiredLRPByProcessGuidReturns(desiredLRP, nil)

			permissionsBuilder = authenticators.NewPermissionsBuilder(bbsClient, false)

			remoteAddr, err := net.ResolveIPAddr("ip", "1.1.1.1")
			Expect(err).NotTo(HaveOccurred())
			metadata = &fake_ssh.FakeConnMetadata{}
			metadata.RemoteAddrReturns(remoteAddr)

			processGuid = "some-guid"
			index = 1
		})

		JustBeforeEach(func() {
			permissions, buildErr = permissionsBuilder.Build(logger, processGuid, index, metadata)
		})

		It("gets information about the desired lrp referenced in the username", func() {
			Expect(bbsClient.DesiredLRPByProcessGuidCallCount()).To(Equal(1))
			_, traceId, guid := bbsClient.DesiredLRPByProcessGuidArgsForCall(0)
			Expect(traceId).To(BeEmpty())
			Expect(guid).To(Equal("some-guid"))
		})

		It("gets information about the the actual lrp from the username", func() {
			Expect(bbsClient.ActualLRPsCallCount()).To(Equal(1))

			_, traceId, filter := bbsClient.ActualLRPsArgsForCall(0)
			Expect(traceId).To(BeEmpty())
			Expect(filter.ProcessGuid).To(Equal("some-guid"))
			Expect(*filter.Index).To(BeEquivalentTo(1))
		})

		Context("ssh-proxy's connect-to-instance-address and rep's advertise-preference-for-instance-address interaction", func() {
			var preferredAddress models.ActualLRPNetInfo_PreferredAddress
			var connectToInstanceAddress bool

			JustBeforeEach(func() {
				actualLRP.ActualLRPNetInfo =
					models.NewActualLRPNetInfo("external-ip", "instance-address", preferredAddress, models.NewPortMappingWithTLSProxy(3333, 1111, 2222, 4444))

				permissionsBuilder = authenticators.NewPermissionsBuilder(bbsClient, connectToInstanceAddress)
				permissions, buildErr = permissionsBuilder.Build(logger, processGuid, index, metadata)
			})

			Context("when ssh-proxy is configured to connect to instance address, not Diego cell (external) address", func() {
				BeforeEach(func() {
					connectToInstanceAddress = true
				})

				Context("when the rep advertises preference for instance address", func() {
					BeforeEach(func() {
						preferredAddress = models.ActualLRPNetInfo_PreferredAddressInstance
					})

					It("saves the instance address in the critical options of the permissions", func() {
						Expect(permissions).NotTo(BeNil())
						Expect(permissions.CriticalOptions).NotTo(BeNil())
						Expect(permissions.CriticalOptions["proxy-target-config"]).To(ContainSubstring(`"address":"instance-address:1111"`))
						Expect(permissions.CriticalOptions["proxy-target-config"]).To(ContainSubstring(`"tls_address":"instance-address:4444"`))
					})
				})

				Context("when the rep advertises preference for host address", func() {
					BeforeEach(func() {
						preferredAddress = models.ActualLRPNetInfo_PreferredAddressHost
					})

					It("saves the Diego cell (external) address in the critical options of the permissions", func() {
						Expect(permissions).NotTo(BeNil())
						Expect(permissions.CriticalOptions).NotTo(BeNil())
						Expect(permissions.CriticalOptions["proxy-target-config"]).To(ContainSubstring(`"address":"external-ip:3333"`))
						Expect(permissions.CriticalOptions["proxy-target-config"]).To(ContainSubstring(`"tls_address":"external-ip:2222"`))
					})
				})

				Context("when the rep does not have preferrence for address", func() {
					BeforeEach(func() {
						preferredAddress = models.ActualLRPNetInfo_PreferredAddressUnknown
					})

					It("saves the instance address in the critical options of the permissions", func() {
						Expect(permissions).NotTo(BeNil())
						Expect(permissions.CriticalOptions).NotTo(BeNil())
						Expect(permissions.CriticalOptions["proxy-target-config"]).To(ContainSubstring(`"address":"instance-address:1111"`))
						Expect(permissions.CriticalOptions["proxy-target-config"]).To(ContainSubstring(`"tls_address":"instance-address:4444"`))
					})
				})
			})

			Context("when ssh-proxy is NOT configured to connect to instance address", func() {
				BeforeEach(func() {
					connectToInstanceAddress = false
				})

				Context("when the rep advertises preference for instance address", func() {
					BeforeEach(func() {
						preferredAddress = models.ActualLRPNetInfo_PreferredAddressInstance
					})

					It("saves the instance address in the critical options of the permissions", func() {
						Expect(permissions).NotTo(BeNil())
						Expect(permissions.CriticalOptions).NotTo(BeNil())
						Expect(permissions.CriticalOptions["proxy-target-config"]).To(ContainSubstring(`"address":"instance-address:1111"`))
						Expect(permissions.CriticalOptions["proxy-target-config"]).To(ContainSubstring(`"tls_address":"instance-address:4444"`))
					})
				})

				Context("when the rep does NOT advertise preference for instance address", func() {
					BeforeEach(func() {
						preferredAddress = models.ActualLRPNetInfo_PreferredAddressHost
					})

					It("saves the Diego cell (external) address in the critical options of the permissions", func() {
						Expect(permissions).NotTo(BeNil())
						Expect(permissions.CriticalOptions).NotTo(BeNil())
						Expect(permissions.CriticalOptions["proxy-target-config"]).To(ContainSubstring(`"address":"external-ip:3333"`))
						Expect(permissions.CriticalOptions["proxy-target-config"]).To(ContainSubstring(`"tls_address":"external-ip:2222"`))
					})
				})

				Context("when the rep does not have preferrence for address", func() {
					BeforeEach(func() {
						preferredAddress = models.ActualLRPNetInfo_PreferredAddressUnknown
					})

					It("saves the Diego cell (external) address in the critical options of the permissions", func() {
						Expect(permissions).NotTo(BeNil())
						Expect(permissions.CriticalOptions).NotTo(BeNil())
						Expect(permissions.CriticalOptions["proxy-target-config"]).To(ContainSubstring(`"address":"external-ip:3333"`))
						Expect(permissions.CriticalOptions["proxy-target-config"]).To(ContainSubstring(`"tls_address":"external-ip:2222"`))
					})
				})
			})
		})

		Context("when the tls port isn't set", func() {
			BeforeEach(func() {
				actualLRP.ActualLRPNetInfo =
					models.NewActualLRPNetInfo("1.2.3.4", "2.2.2.2", models.ActualLRPNetInfo_PreferredAddressUnknown, models.NewPortMapping(3333, 1111))
			})

			It("does not include a tls address in the permissions", func() {
				expectedConfig := `{
				"address": "1.2.3.4:3333",
				"tls_address": "",
				"server_cert_domain_san": "some-instance-guid",
				"host_fingerprint": "host-fingerprint",
				"private_key": "fake-pem-encoded-key",
				"user": "user",
				"password": "password"
			}`

				Expect(permissions).NotTo(BeNil())
				Expect(permissions.CriticalOptions).NotTo(BeNil())
				Expect(permissions.CriticalOptions["proxy-target-config"]).To(MatchJSON(expectedConfig))
			})
		})

		It("saves container information in the critical options of the permissions", func() {
			expectedConfig := `{
				"address": "1.2.3.4:3333",
				"tls_address": "1.2.3.4:2222",
				"server_cert_domain_san": "some-instance-guid",
				"host_fingerprint": "host-fingerprint",
				"private_key": "fake-pem-encoded-key",
				"user": "user",
				"password": "password"
			}`

			Expect(permissions).NotTo(BeNil())
			Expect(permissions.CriticalOptions).NotTo(BeNil())
			Expect(permissions.CriticalOptions["proxy-target-config"]).To(MatchJSON(expectedConfig))
		})

		It("saves log message information in the critical options of the permissions", func() {
			expectedConfig := `{
				"tags": {
				  "some_static_key": "some_value",
					"some_dynamic_key": "1",
					"some_other_dynamic_key": "some-instance-guid",
					"source_id": "log-guid",
					"instance_id": "1"
				},
				"message": "Successful remote access by 1.1.1.1"
			}`

			Expect(permissions).NotTo(BeNil())
			Expect(permissions.CriticalOptions).NotTo(BeNil())
			Expect(permissions.CriticalOptions["log-message"]).To(MatchJSON(expectedConfig))
		})

		Context("when getting the desired LRP information fails", func() {
			BeforeEach(func() {
				bbsClient.DesiredLRPByProcessGuidReturns(nil, &models.Error{})
			})

			It("returns the error", func() {
				Expect(buildErr).To(Equal(&models.Error{}))
			})
		})

		Context("when getting the actual LRP information fails", func() {
			BeforeEach(func() {
				bbsClient.ActualLRPsReturns(nil, &models.Error{})
			})

			It("returns the error", func() {
				Expect(buildErr).To(Equal(&models.Error{}))
			})
		})

		Context("when the container port cannot be found", func() {
			BeforeEach(func() {
				actualLRP.Ports = []*models.PortMapping{}
				bbsClient.ActualLRPsReturns([]*models.ActualLRP{actualLRP}, nil)
			})

			It("returns an empty permission reference", func() {
				Expect(permissions).To(Equal(&ssh.Permissions{}))
			})
		})

		Context("when the desired LRP does not include routes", func() {
			BeforeEach(func() {
				desiredLRP.Routes = nil
				bbsClient.DesiredLRPByProcessGuidReturns(desiredLRP, nil)
			})

			It("fails the authentication", func() {
				Expect(buildErr).To(Equal(authenticators.RouteNotFoundErr))
			})
		})

		Context("when the desired LRP does not include an SSH route", func() {
			BeforeEach(func() {
				r := *desiredLRP.Routes
				delete(r, routes.DIEGO_SSH)
				bbsClient.DesiredLRPByProcessGuidReturns(desiredLRP, nil)
			})

			It("fails the authentication", func() {
				Expect(buildErr).To(Equal(authenticators.RouteNotFoundErr))
			})
		})

		Context("when the ssh route fails to unmarshal", func() {
			BeforeEach(func() {
				message := json.RawMessage([]byte(`{,:`))
				(*desiredLRP.Routes)[routes.DIEGO_SSH] = &message
				bbsClient.DesiredLRPByProcessGuidReturns(desiredLRP, nil)
			})

			It("fails the authentication", func() {
				Expect(buildErr).To(HaveOccurred())
			})
		})
	})
})
