package cfattestor

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// mintInstanceCert mirrors testsupport.MintInstanceCert; inlined here because a
// white-box test cannot import testsupport (which imports this package).
func mintInstanceCert(handle string, s Selectors) (certPEM, keyPEM string, key crypto.Signer) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:         handle,
			OrganizationalUnit: []string{"organization:" + s.OrgID, "space:" + s.SpaceID, "app:" + s.AppID},
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	Expect(err).NotTo(HaveOccurred())
	pkcs8, err := x509.MarshalPKCS8PrivateKey(priv)
	Expect(err).NotTo(HaveOccurred())
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}))
	return certPEM, keyPEM, priv
}

var _ = Describe("readInstanceCredentials", func() {
	const pid = 7777
	var procRoot, credDir string

	BeforeEach(func() {
		procRoot = GinkgoT().TempDir()
		credDir = filepath.Join(procRoot, strconv.Itoa(pid), "root", "etc", "cf-instance-credentials")
		Expect(os.MkdirAll(credDir, 0755)).To(Succeed())
	})

	writeCreds := func(certPEM, keyPEM string) {
		Expect(os.WriteFile(filepath.Join(credDir, "instance.crt"), []byte(certPEM), 0644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(credDir, "instance.key"), []byte(keyPEM), 0600)).To(Succeed())
	}

	It("returns the cert PEM, parsed cert, signer, and decodable selectors", func() {
		certPEM, keyPEM, _ := mintInstanceCert("handle-abc", Selectors{OrgID: "org-1", SpaceID: "space-2", AppID: "app-3"})
		writeCreds(certPEM, keyPEM)

		gotPEM, cert, key, err := readInstanceCredentials(procRoot, pid)
		Expect(err).NotTo(HaveOccurred())
		Expect(gotPEM).To(Equal(certPEM))
		Expect(cert.Subject.CommonName).To(Equal("handle-abc"))
		Expect(key).NotTo(BeNil())

		parsed, err := parseSelectors(cert)
		Expect(err).NotTo(HaveOccurred())
		Expect(parsed).To(Equal(Selectors{OrgID: "org-1", SpaceID: "space-2", AppID: "app-3"}))
		Expect(parsed.ProcessType).To(BeEmpty())
	})

	It("errors when the cert file is missing", func() {
		_, _, _, err := readInstanceCredentials(procRoot, pid)
		Expect(err).To(HaveOccurred())
	})

	It("errors when the key is malformed", func() {
		certPEM, _, _ := mintInstanceCert("h", Selectors{OrgID: "o", SpaceID: "s", AppID: "a"})
		writeCreds(certPEM, "-----BEGIN PRIVATE KEY-----\nbogus\n-----END PRIVATE KEY-----\n")
		_, _, _, err := readInstanceCredentials(procRoot, pid)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("parsePrivateKey", func() {
	It("parses the PKCS8 key", func() {
		_, keyPEM, _ := mintInstanceCert("h", Selectors{})
		signer, err := parsePrivateKey([]byte(keyPEM))
		Expect(err).NotTo(HaveOccurred())
		Expect(signer).NotTo(BeNil())
	})

	It("errors on garbage input", func() {
		_, err := parsePrivateKey([]byte("not a key"))
		Expect(err).To(HaveOccurred())
	})
})
