package routingtable_test

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/route-emitter/routingtable"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RegistryMessage", func() {
	var expectedMessage routingtable.RegistryMessage

	BeforeEach(func() {
		expectedMessage = routingtable.RegistryMessage{
			Host:                 "1.1.1.1",
			Port:                 61001,
			URIs:                 []string{"host-1.example.com"},
			App:                  "app-guid",
			ServerCertDomainSAN:  "instance-guid",
			PrivateInstanceId:    "instance-guid",
			PrivateInstanceIndex: "0",
			RouteServiceUrl:      "https://hello.com",
			EndpointUpdatedAtNs:  1000,
			Tags:                 map[string]string{"component": "route-emitter", "foo": "bar", "doo": "0", "goo": "instance-guid"},
			AvailabilityZone:     "some-zone",
		}
	})

	Describe("serialization", func() {
		var expectedJSON string

		BeforeEach(func() {
			expectedJSON = `{
				"host": "1.1.1.1",
				"port": 61001,
				"uris": ["host-1.example.com"],
				"app" : "app-guid",
				"private_instance_id": "instance-guid",
				"server_cert_domain_san": "instance-guid",
				"private_instance_index": "0",
				"route_service_url": "https://hello.com",
				"endpoint_updated_at_ns": 1000,
				"tags": {"component":"route-emitter", "doo": "0", "foo": "bar", "goo": "instance-guid"},
				"availability_zone": "some-zone"
			}`
		})

		It("marshals correctly", func() {
			payload, err := json.Marshal(expectedMessage)
			Expect(err).NotTo(HaveOccurred())

			Expect(payload).To(MatchJSON(expectedJSON))
		})

		It("unmarshals correctly", func() {
			message := routingtable.RegistryMessage{}

			err := json.Unmarshal([]byte(expectedJSON), &message)
			Expect(err).NotTo(HaveOccurred())
			Expect(message).To(Equal(expectedMessage))
		})

		Context("when TLS port is set", func() {
			BeforeEach(func() {
				expectedMessage.TlsPort = 61007

				expectedJSON = `{
				"host": "1.1.1.1",
				"port": 61001,
				"tls_port": 61007,
				"uris": ["host-1.example.com"],
				"app" : "app-guid",
				"private_instance_id": "instance-guid",
				"private_instance_index": "0",
				"server_cert_domain_san": "instance-guid",
				"route_service_url": "https://hello.com",
				"endpoint_updated_at_ns": 1000,
				"tags": {"component":"route-emitter", "doo": "0", "foo": "bar", "goo": "instance-guid"},
				"availability_zone": "some-zone"
			}`
			})

			It("correctly marshals the TLS port", func() {
				message := routingtable.RegistryMessage{}

				err := json.Unmarshal([]byte(expectedJSON), &message)
				Expect(err).NotTo(HaveOccurred())
				Expect(message).To(Equal(expectedMessage))
			})
		})

		Context("when protocol is set", func() {
			BeforeEach(func() {
				expectedMessage.Protocol = "http2"

				expectedJSON = `{
				"host": "1.1.1.1",
				"port": 61001,
				"protocol": "http2",
				"uris": ["host-1.example.com"],
				"app" : "app-guid",
				"private_instance_id": "instance-guid",
				"private_instance_index": "0",
				"server_cert_domain_san": "instance-guid",
				"route_service_url": "https://hello.com",
				"endpoint_updated_at_ns": 1000,
				"tags": {"component":"route-emitter", "doo": "0", "foo": "bar", "goo": "instance-guid"},
				"availability_zone": "some-zone"
			}`
			})

			It("correctly marshals the protocol", func() {
				message := routingtable.RegistryMessage{}

				err := json.Unmarshal([]byte(expectedJSON), &message)
				Expect(err).NotTo(HaveOccurred())
				Expect(message).To(Equal(expectedMessage))
			})
		})

		Context("when a route option is provided", func() {
			BeforeEach(func() {
				expectedMessage.Options = json.RawMessage(`{"lb_algo":"least-connection"}`)
				expectedJSON = `{
				"host": "1.1.1.1",
				"port": 61001,
				"uris": ["host-1.example.com"],
				"app" : "app-guid",
				"private_instance_id": "instance-guid",
				"server_cert_domain_san": "instance-guid",
				"private_instance_index": "0",
				"route_service_url": "https://hello.com",
				"endpoint_updated_at_ns": 1000,
				"tags": {"component":"route-emitter", "doo": "0", "foo": "bar", "goo": "instance-guid"},
				"availability_zone": "some-zone",
				"options": {"lb_algo":"least-connection"}
			}`
			})

			It("correctly marshals the route option", func() {
				message := routingtable.RegistryMessage{}

				err := json.Unmarshal([]byte(expectedJSON), &message)
				Expect(err).NotTo(HaveOccurred())
				Expect(message).To(Equal(expectedMessage))
			})
		})
	})

	Describe("RegistryMessageFor", func() {
		var endpoint routingtable.Endpoint
		var route routingtable.Route

		BeforeEach(func() {
			endpoint = routingtable.Endpoint{
				InstanceGUID:     "instance-guid",
				Index:            0,
				Host:             "1.1.1.1",
				Port:             61001,
				ContainerPort:    11,
				Since:            2000,
				AvailabilityZone: "some-zone",
			}

			route = routingtable.Route{
				Hostname:        "host-1.example.com",
				LogGUID:         "app-guid",
				RouteServiceUrl: "https://hello.com",
				MetricTags: map[string]*models.MetricTagValue{
					"foo": &models.MetricTagValue{Static: "bar"},
					"doo": &models.MetricTagValue{Dynamic: models.MetricTagValue_MetricTagDynamicValueIndex},
					"goo": &models.MetricTagValue{Dynamic: models.MetricTagValue_MetricTagDynamicValueInstanceGuid},
				},
			}

			expectedMessage.EndpointUpdatedAtNs = 2000
		})

		It("omit EndpointUpdatedAtNs if emitEndpointUpdatedAt is set to false", func() {
			message := routingtable.RegistryMessageFor(endpoint, route, false)
			expectedMessage.EndpointUpdatedAtNs = 0
			Expect(message).To(Equal(expectedMessage))
		})

		It("creates a valid message from an endpoint and routes", func() {
			message := routingtable.RegistryMessageFor(endpoint, route, true)
			Expect(message).To(Equal(expectedMessage))
		})

		It("sets the TLS port if a TLS proxy port is provided", func() {
			expectedMessage.TlsPort = 61005
			endpoint.TlsProxyPort = 61005

			message := routingtable.RegistryMessageFor(endpoint, route, true)
			Expect(message).To(Equal(expectedMessage))
		})

		Context("when instance index is greater than 0", func() {
			BeforeEach(func() {
				expectedMessage.PrivateInstanceIndex = "2"
				expectedMessage.Tags["doo"] = "2"
				endpoint.Index = 2
			})

			It("creates a valid message when instance index is greater than 0", func() {
				message := routingtable.RegistryMessageFor(endpoint, route, true)
				Expect(message).To(Equal(expectedMessage))
			})
		})

		Context("when route options are provided", func() {
			BeforeEach(func() {
				route.Options = json.RawMessage(`{"lb_algo":"least-connection"}`)
				expectedMessage.Options = json.RawMessage(`{"lb_algo":"least-connection"}`)
			})

			It("creates a valid message when route options are provided", func() {
				message := routingtable.RegistryMessageFor(endpoint, route, true)
				Expect(message).To(Equal(expectedMessage))
			})
		})
	})

	Describe("InternalAddressRegistryMessageFor", func() {
		var endpoint routingtable.Endpoint
		var route routingtable.Route

		BeforeEach(func() {
			expectedMessage = routingtable.RegistryMessage{
				Host:                 "1.2.3.4",
				Port:                 11,
				URIs:                 []string{"host-1.example.com"},
				App:                  "app-guid",
				PrivateInstanceId:    "instance-guid",
				PrivateInstanceIndex: "0",
				ServerCertDomainSAN:  "instance-guid",
				RouteServiceUrl:      "https://hello.com",
				EndpointUpdatedAtNs:  2000,
				Tags:                 map[string]string{"component": "route-emitter", "foo": "bar", "doo": "0"},
				AvailabilityZone:     "some-zone",
			}

			endpoint = routingtable.Endpoint{
				InstanceGUID:     "instance-guid",
				Index:            0,
				Host:             "1.1.1.1",
				ContainerIP:      "1.2.3.4",
				Port:             61001,
				ContainerPort:    11,
				Since:            2000,
				AvailabilityZone: "some-zone",
			}
			route = routingtable.Route{
				Hostname:        "host-1.example.com",
				LogGUID:         "app-guid",
				RouteServiceUrl: "https://hello.com",
				MetricTags: map[string]*models.MetricTagValue{
					"foo": &models.MetricTagValue{Static: "bar"},
					"doo": &models.MetricTagValue{Dynamic: models.MetricTagValue_MetricTagDynamicValueIndex},
				},
			}

		})

		It("omit EndpointUpdatedAtNs if emitEndpointUpdatedAt is set to false", func() {
			message := routingtable.InternalAddressRegistryMessageFor(endpoint, route, false)
			expectedMessage.EndpointUpdatedAtNs = 0
			Expect(message).To(Equal(expectedMessage))
		})

		It("creates a valid message from an endpoint and routes", func() {
			message := routingtable.InternalAddressRegistryMessageFor(endpoint, route, true)
			Expect(message).To(Equal(expectedMessage))
		})

		It("sets the TLS port in the message if the container TLS proxy port is set", func() {
			expectedMessage.TlsPort = 61007
			endpoint.ContainerTlsProxyPort = 61007

			message := routingtable.InternalAddressRegistryMessageFor(endpoint, route, true)
			Expect(message).To(Equal(expectedMessage))
		})

		It("creates a valid message when instance index is greater than 0", func() {
			expectedMessage.PrivateInstanceIndex = "2"
			expectedMessage.Tags["doo"] = "2"
			endpoint.Index = 2

			message := routingtable.InternalAddressRegistryMessageFor(endpoint, route, true)
			Expect(message).To(Equal(expectedMessage))
		})
	})

	Describe("InternalEndpointRegistryMessageFor", func() {
		var endpoint routingtable.Endpoint
		var route routingtable.InternalRoute

		BeforeEach(func() {
			expectedMessage = routingtable.RegistryMessage{
				Host:                 "1.2.3.4",
				URIs:                 []string{"host-1.example.com", "0.host-1.example.com"},
				App:                  "app-guid",
				Tags:                 map[string]string{"component": "route-emitter"},
				EndpointUpdatedAtNs:  2000,
				PrivateInstanceIndex: "0",
				AvailabilityZone:     "some-zone",
			}

			endpoint = routingtable.Endpoint{
				InstanceGUID:     "instance-guid",
				Index:            0,
				Host:             "1.1.1.1",
				ContainerIP:      "1.2.3.4",
				Port:             61001,
				ContainerPort:    11,
				Since:            2000,
				AvailabilityZone: "some-zone",
			}

			route = routingtable.InternalRoute{
				Hostname:    "host-1.example.com",
				LogGUID:     "app-guid",
				ContainerIP: "5.6.7.8",
			}
		})

		It("omit EndpointUpdatedAtNs if emitEndpointUpdatedAt is set to false", func() {
			message := routingtable.InternalEndpointRegistryMessageFor(endpoint, route, false)
			expectedMessage.EndpointUpdatedAtNs = 0
			Expect(message).To(Equal(expectedMessage))
		})

		It("creates a valid message from an endpoint and routes", func() {
			message := routingtable.InternalEndpointRegistryMessageFor(endpoint, route, true)
			Expect(message).To(Equal(expectedMessage))
		})
	})
})
