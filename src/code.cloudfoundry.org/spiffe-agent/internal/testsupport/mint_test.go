package testsupport_test

import (
	"crypto/x509"
	"encoding/pem"

	"code.cloudfoundry.org/spiffe-agent/cfattestor"
	"code.cloudfoundry.org/spiffe-agent/internal/testsupport"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MintInstanceCert", func() {
	It("mints a self-signed RSA cert with CN=handle and selector OUs", func() {
		certPEM, keyPEM, key := testsupport.MintInstanceCert("habc", cfattestor.Selectors{OrgID: "o1", SpaceID: "s2", AppID: "a3"})
		Expect(key).NotTo(BeNil())

		block, _ := pem.Decode([]byte(certPEM))
		Expect(block).NotTo(BeNil())
		cert, err := x509.ParseCertificate(block.Bytes)
		Expect(err).NotTo(HaveOccurred())
		Expect(cert.Subject.CommonName).To(Equal("habc"))
		Expect(cert.Subject.OrganizationalUnit).To(ConsistOf("organization:o1", "space:s2", "app:a3"))

		kb, _ := pem.Decode([]byte(keyPEM))
		Expect(kb).NotTo(BeNil())
		_, err = x509.ParsePKCS8PrivateKey(kb.Bytes)
		Expect(err).NotTo(HaveOccurred())
	})
})
