package helpers_test

import (
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
