package vizzini_test

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/tlsconfig"
	. "code.cloudfoundry.org/vizzini/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TLS Proxy", func() {
	var (
		lrp       *models.DesiredLRP
		actualLRP models.ActualLRP
	)

	BeforeEach(func() {
		if !config.EnableContainerProxyTests {
			Skip("container proxy tests are disabled")
		}

		lrp = DesiredLRPWithGuid(guid)
		Expect(bbsClient.DesireLRP(logger, traceID, lrp)).To(Succeed())
		actualGetterFn := ActualGetter(logger, guid, 0)
		Eventually(actualGetterFn).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
		var err error
		actualLRP, err = actualGetterFn()
		Expect(err).NotTo(HaveOccurred())
	})

	It("proxies traffic to the application process inside the container", func() {
		directURL := "https://" + TLSDirectAddressFor(guid, 0, 8080)

		tlsConfig, err := containerProxyTLSConfig(actualLRP.InstanceGuid)
		Expect(err).NotTo(HaveOccurred())

		client := http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}
		resp, err := client.Get(directURL)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	Describe("has a valid certificate", func() {
		var (
			certs []*x509.Certificate
		)

		BeforeEach(func() {
			tlsConfig, err := containerProxyTLSConfig(actualLRP.InstanceGuid)
			Expect(err).NotTo(HaveOccurred())

			conn, err := tls.Dial("tcp", TLSDirectAddressFor(guid, 0, 8080), tlsConfig)
			Expect(err).NotTo(HaveOccurred())

			err = conn.Handshake()
			Expect(err).NotTo(HaveOccurred())

			connState := conn.ConnectionState()
			Expect(connState.HandshakeComplete).To(BeTrue())
			certs = connState.PeerCertificates
			Expect(certs).To(HaveLen(2)) // the instance identity cert + CA
		})

		It("has a common name that matches the instance guid", func() {
			Expect(certs[0].Subject.CommonName).To(Equal(actualLRP.InstanceGuid))
		})
	})
})

func containerProxyTLSConfig(instanceGuid string) (*tls.Config, error) {
	caCertPool := x509.NewCertPool()
	if config.ProxyCAPath == "" {
		return nil, errors.New("proxy CA file not provided")
	}
	certBytes, err := os.ReadFile(config.ProxyCAPath)
	if err != nil {
		return nil, fmt.Errorf("failed read ca cert file: %s", err.Error())
	}

	if ok := caCertPool.AppendCertsFromPEM(certBytes); !ok {
		return nil, errors.New("unable to load ca cert")
	}

	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}

	if config.ProxyClientCertPath != "" && config.ProxyClientKeyPath != "" {
		tlsConfig, err = tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(config.ProxyClientCertPath, config.ProxyClientKeyPath),
		).Server(
			tlsconfig.WithClientAuthentication(caCertPool),
		)
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = caCertPool // proxy operates as a client and server
	}
	tlsConfig.ServerName = instanceGuid

	return tlsConfig, nil
}
