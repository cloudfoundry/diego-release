package tlshelpers

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"os"
	"time"

	"net"

	. "github.com/onsi/gomega"
)

func GenerateCa() (string, *rsa.PrivateKey) {
	caFileName, privKey, err := buildCaFile()
	Expect(err).NotTo(HaveOccurred())
	return caFileName, privKey
}

func GenerateCertAndKey(caFileName string, caPrivateKey *rsa.PrivateKey) (clientCertFileName string, clientPrivateKeyFileName string, cert tls.Certificate) {
	certPem, keyPem, err := buildCertPem(caPrivateKey, caFileName)
	Expect(err).NotTo(HaveOccurred())
	clientCertFileName = writeClientCredFile(certPem)
	clientPrivateKeyFileName = writeClientCredFile(keyPem)

	cert, err = tls.X509KeyPair(certPem, keyPem)
	Expect(err).NotTo(HaveOccurred())

	return
}

func GenerateCaAndMutualTlsCerts() (caFileName string, certFileName string, privateKeyFileName string, cert tls.Certificate) {
	var (
		err     error
		privKey *rsa.PrivateKey
	)

	caFileName, privKey = GenerateCa()
	Expect(err).NotTo(HaveOccurred())

	certFileName, privateKeyFileName, cert = GenerateCertAndKey(caFileName, privKey)

	return
}

func CertPool(certName string) *x509.CertPool {
	certPool := x509.NewCertPool()
	caCertificate := mapToX509Cert(certName)
	Expect(caCertificate).To(HaveLen(1))
	certPool.AddCert(caCertificate[0])
	return certPool
}

func mapToX509Cert(PemEncodedCertFilePath string) []*x509.Certificate {
	caFile, err := os.Open(PemEncodedCertFilePath)
	Expect(err).NotTo(HaveOccurred())

	caFileContents, err := ioutil.ReadAll(caFile)
	Expect(err).NotTo(HaveOccurred())
	caFileBlock, _ := pem.Decode(caFileContents)
	caCertificate, err := x509.ParseCertificates(caFileBlock.Bytes)
	Expect(err).NotTo(HaveOccurred())
	return caCertificate
}

func buildCaFile() (string, *rsa.PrivateKey, error) {
	privKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return "", nil, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return "", nil, err
	}

	now := time.Now()
	notAfter := now.Add(365 * 24 * time.Hour)

	ca := &x509.Certificate{
		SerialNumber:       serialNumber,
		SignatureAlgorithm: x509.SHA256WithRSA,
		Subject: pkix.Name{
			Country:      []string{"USA"},
			Organization: []string{"Cloud Foundry"},
			CommonName:   "CA",
		},
		NotBefore:             now,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,

		IsCA:     true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	caCert, err := x509.CreateCertificate(rand.Reader, ca, ca, &privKey.PublicKey, privKey)
	if err != nil {
		return "", nil, err
	}

	file, err := ioutil.TempFile(os.TempDir(), "bosh-dns-adapter-ca")
	if err != nil {
		return "", nil, err
	}

	err = pem.Encode(file, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCert,
	})

	if err != nil {
		return "", nil, err
	}

	return file.Name(), privKey, nil
}

func buildCertPem(privKey *rsa.PrivateKey, caFilePath string) (cert []byte, key []byte, err error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	notAfter := now.Add(365 * 24 * time.Hour)

	serverCertTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Country:      []string{"USA"},
			Organization: []string{"Cloud Foundry"},
		},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:             now,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,

		IsCA:               false,
		SignatureAlgorithm: x509.SHA256WithRSA,
		KeyUsage:           x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	caBytes, err := ioutil.ReadFile(caFilePath)
	if err != nil {
		return nil, nil, err
	}

	caBlockDer, _ := pem.Decode(caBytes)
	caCert, err := x509.ParseCertificate(caBlockDer.Bytes)
	if err != nil {
		return nil, nil, err
	}

	serverCert, err := x509.CreateCertificate(rand.Reader, serverCertTemplate, caCert, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, nil, err
	}

	certBuffer := &bytes.Buffer{}

	err = pem.Encode(certBuffer, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCert,
	})
	if err != nil {
		return nil, nil, err
	}

	keyBuffer := &bytes.Buffer{}

	derEncodedPrivateKey := x509.MarshalPKCS1PrivateKey(privKey)
	err = pem.Encode(keyBuffer, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: derEncodedPrivateKey,
	})
	if err != nil {
		return nil, nil, err
	}

	return certBuffer.Bytes(), keyBuffer.Bytes(), nil
}

func writeClientCredFile(data []byte) string {
	tempFile, err := ioutil.TempFile(os.TempDir(), "clientcredstest")
	Expect(err).NotTo(HaveOccurred())
	Expect(ioutil.WriteFile(tempFile.Name(), data, os.ModePerm)).To(Succeed())
	return tempFile.Name()
}
