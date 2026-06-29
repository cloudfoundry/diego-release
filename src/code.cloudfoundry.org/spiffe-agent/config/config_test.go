package config_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/spiffe-agent/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// writeCAPEM generates a self-signed CA certificate and writes it as PEM to a
// temp file, returning the file path so RootCAs population can be exercised.
func writeCAPEM(dir string) string {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).NotTo(HaveOccurred())

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "spiffe-agent-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())

	caPath := filepath.Join(dir, "ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	Expect(os.WriteFile(caPath, pemBytes, 0644)).To(Succeed())
	return caPath
}

var _ = Describe("Config", func() {
	var dir string

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
	})

	Describe("NewSpiffeAgentConfig", func() {
		It("round-trips every field from JSON", func() {
			json := `{
				"socket_path": "/run/workload.sock",
				"trust_domain": "example.org",
				"cell_id": "cell-1",
				"signer_url": "https://uaa.example.org",
				"signer_client_id": "spiffe-signer",
				"signer_client_secret": "s3cr3t",
				"signer_ca_cert_file": "/etc/uaa/ca.pem",
				"bbs_address": "https://bbs.example.org",
				"bbs_ca_cert_file": "/etc/bbs/ca.pem",
				"bbs_client_cert_file": "/etc/bbs/cert.pem",
				"bbs_client_key_file": "/etc/bbs/key.pem",
				"log_level": "debug"
			}`
			path := filepath.Join(dir, "config.json")
			Expect(os.WriteFile(path, []byte(json), 0644)).To(Succeed())

			cfg, err := config.NewSpiffeAgentConfig(path)
			Expect(err).NotTo(HaveOccurred())

			Expect(cfg.SocketPath).To(Equal("/run/workload.sock"))
			Expect(cfg.TrustDomain).To(Equal("example.org"))
			Expect(cfg.CellID).To(Equal("cell-1"))
			Expect(cfg.SignerURL).To(Equal("https://uaa.example.org"))
			Expect(cfg.SignerClientID).To(Equal("spiffe-signer"))
			Expect(cfg.SignerClientSecret).To(Equal("s3cr3t"))
			Expect(cfg.SignerCACertFile).To(Equal("/etc/uaa/ca.pem"))
			Expect(cfg.BBSAddress).To(Equal("https://bbs.example.org"))
			Expect(cfg.BBSCACertFile).To(Equal("/etc/bbs/ca.pem"))
			Expect(cfg.BBSClientCertFile).To(Equal("/etc/bbs/cert.pem"))
			Expect(cfg.BBSClientKeyFile).To(Equal("/etc/bbs/key.pem"))
			Expect(cfg.LogLevel).To(Equal("debug"))
		})

		It("defaults SocketPath when omitted", func() {
			path := filepath.Join(dir, "config.json")
			Expect(os.WriteFile(path, []byte(`{"trust_domain":"example.org"}`), 0644)).To(Succeed())

			cfg, err := config.NewSpiffeAgentConfig(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.SocketPath).To(Equal("/var/vcap/data/spiffe-agent/run/workload.sock"))
		})

		It("defaults LagerConfig when omitted", func() {
			path := filepath.Join(dir, "config.json")
			Expect(os.WriteFile(path, []byte(`{"trust_domain":"example.org"}`), 0644)).To(Succeed())

			cfg, err := config.NewSpiffeAgentConfig(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.LagerConfig).To(Equal(lagerflags.DefaultLagerConfig()))
		})

		It("errors on a missing file", func() {
			_, err := config.NewSpiffeAgentConfig(filepath.Join(dir, "does-not-exist.json"))
			Expect(err).To(HaveOccurred())
		})

		It("errors on malformed JSON", func() {
			path := filepath.Join(dir, "bad.json")
			Expect(os.WriteFile(path, []byte("{not json"), 0644)).To(Succeed())

			_, err := config.NewSpiffeAgentConfig(path)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("SignerHTTPClient", func() {
		It("returns a non-nil client with a sane timeout when no CA is configured", func() {
			cfg := config.Config{}
			client, err := cfg.SignerHTTPClient()
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())
			Expect(client.Timeout).To(Equal(30 * time.Second))
		})

		It("populates RootCAs from the configured CA file", func() {
			cfg := config.Config{SignerCACertFile: writeCAPEM(dir)}
			client, err := cfg.SignerHTTPClient()
			Expect(err).NotTo(HaveOccurred())
			Expect(client).NotTo(BeNil())
			Expect(client.Timeout).To(Equal(30 * time.Second))

			transport, ok := client.Transport.(*http.Transport)
			Expect(ok).To(BeTrue())
			Expect(transport.TLSClientConfig).NotTo(BeNil())
			Expect(transport.TLSClientConfig.RootCAs).NotTo(BeNil())
			Expect(transport.TLSClientConfig.RootCAs.Subjects()).To(HaveLen(1))
		})

		It("errors when the CA file contains no valid certs", func() {
			path := filepath.Join(dir, "garbage.pem")
			Expect(os.WriteFile(path, []byte("not a pem"), 0644)).To(Succeed())

			cfg := config.Config{SignerCACertFile: path}
			_, err := cfg.SignerHTTPClient()
			Expect(err).To(MatchError(ContainSubstring("no valid certs")))
		})
	})
})
