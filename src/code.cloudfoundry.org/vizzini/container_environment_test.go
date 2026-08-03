package vizzini_test

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("The container environment", func() {
	var lrp *models.DesiredLRP
	var url string

	BeforeEach(func() {
		url = "http://" + RouteForGuid(guid) + "/env?json=true"
		lrp = DesiredLRPWithGuid(guid)
		lrp.Ports = []uint32{8080, 5000}

		Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
		Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
	})

	getEnvs := func(url string) [][]string {
		response, err := http.Get(url)
		Expect(err).NotTo(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK))
		envs := [][]string{}
		err = json.NewDecoder(response.Body).Decode(&envs)
		Expect(err).NotTo(HaveOccurred())
		response.Body.Close()
		return envs
	}

	Describe("INSTANCE_INDEX and INSTANCE_GUID", func() {
		It("matches the ActualLRP's index and instance guid", func() {
			actualLRP, err := ActualLRPByProcessGuidAndIndex(logger, guid, 0)
			Expect(err).NotTo(HaveOccurred())

			envs := getEnvs(url)

			Expect(envs).To(ContainElement([]string{"INSTANCE_INDEX", "0"}))
			Expect(envs).To(ContainElement([]string{"INSTANCE_GUID", actualLRP.InstanceGuid}))
		})
	})

	Describe("networking environment variables", func() {
		It("matches the network info on the ActualLRP", func() {
			actualLRP, err := ActualLRPByProcessGuidAndIndex(logger, guid, 0)
			Expect(err).NotTo(HaveOccurred())

			type cfPortMapping struct {
				External    uint32 `json:"external,omitempty"`
				Internal    uint32 `json:"internal"`
				ExternalTLS uint32 `json:"external_tls_proxy,omitempty"`
				InternalTLS uint32 `json:"internal_tls_proxy,omitempty"`
			}

			cfPortMappingPayload, err := json.Marshal([]cfPortMapping{
				{Internal: actualLRP.Ports[0].ContainerPort, ExternalTLS: actualLRP.Ports[0].HostTlsProxyPort, InternalTLS: actualLRP.Ports[0].ContainerTlsProxyPort},
				{Internal: actualLRP.Ports[1].ContainerPort, ExternalTLS: actualLRP.Ports[1].HostTlsProxyPort, InternalTLS: actualLRP.Ports[1].ContainerTlsProxyPort},
				{Internal: actualLRP.Ports[2].ContainerPort, ExternalTLS: actualLRP.Ports[2].HostTlsProxyPort, InternalTLS: actualLRP.Ports[2].ContainerTlsProxyPort},
			})
			Expect(err).NotTo(HaveOccurred())

			envs := getEnvs(url)
			Expect(envs).To(ContainElement([]string{"CF_INSTANCE_IP", actualLRP.Address}), "If this fails, then your executor may not be configured to expose ip:port to the container")
			Expect(envs).To(ContainElement([]string{"CF_INSTANCE_PORTS", string(cfPortMappingPayload)}))
		})

		It("includes CF_INSTANCE_INTERNAL_IP", func() {
			envs := getEnvs(url)
			Expect(envs).To(ContainElement(ContainElement("CF_INSTANCE_INTERNAL_IP")))
		})
	})
})
