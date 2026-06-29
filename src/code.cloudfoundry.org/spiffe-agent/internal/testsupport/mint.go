// Package testsupport provides certificate-minting helpers shared across
// spiffe-agent tests. It is imported only by _test.go files; production
// packages such as cfattestor must never depend on it.
package testsupport

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	"code.cloudfoundry.org/spiffe-agent/cfattestor"
)

// MintInstanceCert builds a self-signed RSA-2048 certificate whose CN is the
// container handle and whose OU entries encode the CF selectors in the order
// organization, space, app. It returns the PEM-encoded cert, the PKCS8
// PEM-encoded key, and the signer. Errors are fatal helper bugs and panic.
func MintInstanceCert(handle string, s cfattestor.Selectors) (certPEM, keyPEM string, key crypto.Signer) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: handle,
			OrganizationalUnit: []string{
				"organization:" + s.OrgID,
				"space:" + s.SpaceID,
				"app:" + s.AppID,
			},
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		panic(err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		panic(err)
	}

	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}))
	return certPEM, keyPEM, priv
}
