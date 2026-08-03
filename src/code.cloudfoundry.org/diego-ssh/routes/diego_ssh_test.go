package routes_test

import (
	"encoding/json"

	"code.cloudfoundry.org/diego-ssh/routes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Diego SSH Route", func() {
	var route routes.SSHRoute

	BeforeEach(func() {
		route = routes.SSHRoute{
			ContainerPort:   2222,
			HostFingerprint: "my-key-fingerprint",
			User:            "user",
			Password:        "password",
			PrivateKey:      "FAKE_PEM_ENCODED_KEY",
		}
	})

	Describe("JSON Marshalling", func() {
		Context("when the user and password are missing", func() {
			var expectedJson string

			BeforeEach(func() {
				route.User = ""
				route.Password = ""

				expectedJson = `{
					"container_port": 2222,
					"host_fingerprint": "my-key-fingerprint",
					"private_key": "FAKE_PEM_ENCODED_KEY"
				}`
			})

			It("marshals the structure correctly", func() {
				payload, err := json.Marshal(route)
				Expect(err).NotTo(HaveOccurred())

				Expect(payload).To(MatchJSON(expectedJson))
			})
		})

		Context("when the private key and host fingerprint are empty", func() {
			var expectedJson string

			BeforeEach(func() {
				route.PrivateKey = ""
				route.HostFingerprint = ""

				expectedJson = `{
					"container_port": 2222,
					"user": "user",
					"password": "password"
				}`
			})

			It("marshals the structure correctly", func() {
				payload, err := json.Marshal(route)
				Expect(err).NotTo(HaveOccurred())

				Expect(payload).To(MatchJSON(expectedJson))
			})
		})
		Context("when both fingerprints are present", func() {
			BeforeEach(func() {
				route = routes.SSHRoute{
					ContainerPort:      2222,
					HostFingerprint:    "24:2e:53:c3:72:4f:25:b8:72:29:2d:e3:56:63:4b:c8",
					Host256Fingerprint: "x+EcRzt7EfVuXTxnFt01lkxabPULguUgpvcpo52/Puc=",
					PrivateKey:         "FAKE_PEM_ENCODED_KEY",
				}
			})

			It("marshals both fingerprints correctly", func() {
				payload, err := json.Marshal(route)
				Expect(err).NotTo(HaveOccurred())

				var result routes.SSHRoute
				err = json.Unmarshal(payload, &result)
				Expect(err).NotTo(HaveOccurred())

				Expect(result.HostFingerprint).To(Equal(route.HostFingerprint))
				Expect(result.Host256Fingerprint).To(Equal(route.Host256Fingerprint))
			})
		})
	})

	Describe("Round Trip Marshalling", func() {
		It("successfully marshals and unmarshals", func() {
			payload, err := json.Marshal(route)
			Expect(err).NotTo(HaveOccurred())

			var result routes.SSHRoute
			err = json.Unmarshal(payload, &result)
			Expect(err).NotTo(HaveOccurred())

			Expect(result).To(Equal(route))
		})
	})
})
