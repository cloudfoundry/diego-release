package certauthority_test

import (
	"crypto/x509"
	"encoding/pem"
	"os"

	"code.cloudfoundry.org/inigo/helpers/certauthority"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cert Allocator", func() {
	var (
		authority certauthority.CertAuthority
		depotDir  string
		err       error
	)

	Context("when commonName and depotDir are provided", func() {
		BeforeEach(func() {
			depotDir, err = os.MkdirTemp("", "depot")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err = os.RemoveAll(depotDir)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates a cert authority, and generates ca cert and ca key", func() {
			authority, err = certauthority.NewCertAuthority(depotDir, "some-name")
			Expect(err).NotTo(HaveOccurred())

			key, cert := authority.CAAndKey()
			Expect(cert).To(BeAnExistingFile())
			Expect(key).To(BeAnExistingFile())
		})

		It("the generated CA certificate to have the correct CommonName", func() {
			authority, err = certauthority.NewCertAuthority(depotDir, "some-name")
			Expect(err).NotTo(HaveOccurred())

			_, cert := authority.CAAndKey()
			parsedCert, _ := parseCert(cert)
			Expect(parsedCert.Subject.CommonName).To(Equal("some-name"))
		})

		It("successfully generates self signed certificates", func() {
			authority, err = certauthority.NewCertAuthority(depotDir, "some-name")
			Expect(err).NotTo(HaveOccurred())

			key, cert, err := authority.GenerateSelfSignedCertAndKey("some-component", []string{"some-component"}, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(cert).To(BeAnExistingFile())
			Expect(key).To(BeAnExistingFile())
		})

		It("successfully generates intermediate certificate authorities", func() {
			authority, err = certauthority.NewCertAuthority(depotDir, "some-name")
			Expect(err).NotTo(HaveOccurred())

			key, cert, err := authority.GenerateSelfSignedCertAndKey("some-intermediate", []string{"some-intermediate"}, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(cert).To(BeAnExistingFile())
			Expect(key).To(BeAnExistingFile())
		})

		It("the generated certificate has the correct CommonName", func() {
			authority, err = certauthority.NewCertAuthority(depotDir, "some-name")
			Expect(err).NotTo(HaveOccurred())

			_, cert, err := authority.GenerateSelfSignedCertAndKey("some-component", []string{"some-component"}, false)
			Expect(err).NotTo(HaveOccurred())
			parsedCert, _ := parseCert(cert)
			Expect(parsedCert.Subject.CommonName).To(Equal("some-component"))
		})
	})

	Context("when depotDir is invalid", func() {
		BeforeEach(func() {
			depotDir = "/random"
		})

		It("fails to initialize the authority", func() {
			_, err = certauthority.NewCertAuthority(depotDir, "some-name")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no such file or directory"))
		})
	})
})

func parseCert(certPath string) (*x509.Certificate, []byte) {
	var block *pem.Block
	var rest []byte
	certBytes, err := os.ReadFile(certPath)
	Expect(err).NotTo(HaveOccurred())
	block, rest = pem.Decode(certBytes)
	Expect(block).NotTo(BeNil())
	Expect(block.Type).To(Equal("CERTIFICATE"))
	certs, err := x509.ParseCertificates(block.Bytes)
	Expect(err).NotTo(HaveOccurred())
	return certs[0], rest
}
