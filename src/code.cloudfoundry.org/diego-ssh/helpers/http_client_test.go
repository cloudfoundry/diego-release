package helpers_test

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/diego-ssh/helpers"
	"code.cloudfoundry.org/inigo/helpers/certauthority"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NewHTTPSClient", func() {
	var (
		caCertFiles        []string
		insecureSkipVerify bool
		timeout            time.Duration
	)

	BeforeEach(func() {
		caCertFiles = []string{}
	})

	It("sets InsecureSkipVerify on the TLS config", func() {
		client, err := helpers.NewHTTPSClient(true, caCertFiles, timeout)
		Expect(err).NotTo(HaveOccurred())
		httpTrans, ok := client.Transport.(*http.Transport)
		Expect(ok).To(BeTrue())
		Expect(httpTrans.TLSClientConfig.InsecureSkipVerify).To(BeTrue())
	})

	It("sets the client timeout", func() {
		client, err := helpers.NewHTTPSClient(insecureSkipVerify, caCertFiles, 5*time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(client.Timeout).To(Equal(5 * time.Second))
	})

	Context("when a list of ca Cert files is provided", func() {
		var (
			certDepotDir string
			ca           certauthority.CertAuthority
		)

		BeforeEach(func() {
			var err error
			certDepotDir, err = os.MkdirTemp("", "cert-depot-dir")
			Expect(err).NotTo(HaveOccurred())

			ca, err = certauthority.NewCertAuthority(certDepotDir, "one")
			Expect(err).NotTo(HaveOccurred())

			_, cert := ca.CAAndKey()
			caCertFiles = []string{cert}
		})

		AfterEach(func() {
			Expect(os.RemoveAll(certDepotDir)).To(Succeed())
		})

		It("sets the RootCAs with a pool consisting of those CAs", func() {
			expectedPool := x509.NewCertPool()
			for _, caCert := range caCertFiles {
				certBytes, err2 := os.ReadFile(caCert)
				Expect(err2).NotTo(HaveOccurred())

				Expect(expectedPool.AppendCertsFromPEM(certBytes)).To(BeTrue())
			}

			client, err := helpers.NewHTTPSClient(insecureSkipVerify, caCertFiles, timeout)
			Expect(err).NotTo(HaveOccurred())
			httpTrans, ok := client.Transport.(*http.Transport)
			Expect(ok).To(BeTrue())

			caPool := httpTrans.TLSClientConfig.RootCAs

			//lint:ignore SA1019 - ignoring tlsCert.RootCAs.Subjects is deprecated ERR because cert does not come from SystemCertPool.
			Expect(expectedPool.Subjects()).To(Equal(caPool.Subjects()))
		})

		Context("when an invalid tls cert is provided", func() {
			var invalidCertPath string

			BeforeEach(func() {
				invalidCert, err := os.CreateTemp("", "invalid-cert-")
				Expect(err).NotTo(HaveOccurred())

				invalidCertPath = invalidCert.Name()

				Expect(invalidCert.Close()).To(Succeed())

				Expect(os.WriteFile(invalidCertPath, []byte("not valid pem"), 0644)).To(Succeed())

				caCertFiles = append(caCertFiles, invalidCertPath)
			})

			AfterEach(func() {
				Expect(os.Remove(invalidCertPath)).To(Succeed())
			})

			It("returns an error", func() {
				_, err := helpers.NewHTTPSClient(insecureSkipVerify, caCertFiles, timeout)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Unable to load caCert"))
			})
		})

		Context("when the UAA tls cert does not exist", func() {
			BeforeEach(func() {
				caCertFiles = append(caCertFiles, "doesntexist")
			})

			It("returns an error", func() {
				_, err := helpers.NewHTTPSClient(insecureSkipVerify, caCertFiles, timeout)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to read ca cert file"))
			})
		})
	})
})

var _ = Describe("NewMutualTLSClient", func() {
	var (
		certDepotDir string
		ca           certauthority.CertAuthority
		caCertFile   string
		clientCert   string
		clientKey    string
		timeout      time.Duration
	)

	BeforeEach(func() {
		var err error
		certDepotDir, err = os.MkdirTemp("", "mtls-cert-depot-")
		Expect(err).NotTo(HaveOccurred())

		ca, err = certauthority.NewCertAuthority(certDepotDir, "test-ca")
		Expect(err).NotTo(HaveOccurred())

		_, caCertFile = ca.CAAndKey()
		clientKey, clientCert, err = ca.GenerateSelfSignedCertAndKey("ssh-proxy-client", []string{"ssh-proxy-client"}, false)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(certDepotDir)).To(Succeed())
	})

	It("returns an error when clientCertFile is empty", func() {
		_, err := helpers.NewMutualTLSClient(false, []string{caCertFile}, "", clientKey, timeout)
		Expect(err).To(MatchError("client certificate and key are required for mutual TLS"))
	})

	It("returns an error when clientKeyFile is empty", func() {
		_, err := helpers.NewMutualTLSClient(false, []string{caCertFile}, clientCert, "", timeout)
		Expect(err).To(MatchError("client certificate and key are required for mutual TLS"))
	})

	It("returns an error when the client cert/key files do not exist", func() {
		_, err := helpers.NewMutualTLSClient(false, []string{caCertFile}, "no-such-cert", "no-such-key", timeout)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to load client cert/key pair"))
	})

	It("returns an error when the client cert and key do not match", func() {
		_, otherCert, err := ca.GenerateSelfSignedCertAndKey("other", []string{"other"}, false)
		Expect(err).NotTo(HaveOccurred())
		_, err = helpers.NewMutualTLSClient(false, []string{caCertFile}, otherCert, clientKey, timeout)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to load client cert/key pair"))
	})

	It("loads the client certificate into the TLS config", func() {
		client, err := helpers.NewMutualTLSClient(false, []string{caCertFile}, clientCert, clientKey, timeout)
		Expect(err).NotTo(HaveOccurred())

		httpTrans, ok := client.Transport.(*http.Transport)
		Expect(ok).To(BeTrue())
		Expect(httpTrans.TLSClientConfig.Certificates).To(HaveLen(1))

		expectedCert, err := tls.LoadX509KeyPair(clientCert, clientKey)
		Expect(err).NotTo(HaveOccurred())
		Expect(httpTrans.TLSClientConfig.Certificates[0].Certificate).To(Equal(expectedCert.Certificate))
	})

	It("sets the CA cert pool", func() {
		client, err := helpers.NewMutualTLSClient(false, []string{caCertFile}, clientCert, clientKey, timeout)
		Expect(err).NotTo(HaveOccurred())

		httpTrans, ok := client.Transport.(*http.Transport)
		Expect(ok).To(BeTrue())

		expectedPool := x509.NewCertPool()
		certBytes, err := os.ReadFile(caCertFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(expectedPool.AppendCertsFromPEM(certBytes)).To(BeTrue())

		//lint:ignore SA1019 - ignoring tlsCert.RootCAs.Subjects is deprecated ERR because cert does not come from SystemCertPool.
		Expect(httpTrans.TLSClientConfig.RootCAs.Subjects()).To(Equal(expectedPool.Subjects()))
	})

	It("sets InsecureSkipVerify on the TLS config", func() {
		client, err := helpers.NewMutualTLSClient(true, []string{caCertFile}, clientCert, clientKey, timeout)
		Expect(err).NotTo(HaveOccurred())

		httpTrans, ok := client.Transport.(*http.Transport)
		Expect(ok).To(BeTrue())
		Expect(httpTrans.TLSClientConfig.InsecureSkipVerify).To(BeTrue())
	})

	It("sets the client timeout", func() {
		client, err := helpers.NewMutualTLSClient(false, []string{caCertFile}, clientCert, clientKey, 7*time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(client.Timeout).To(Equal(7 * time.Second))
	})
})
