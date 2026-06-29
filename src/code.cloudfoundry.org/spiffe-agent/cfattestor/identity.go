package cfattestor

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Selectors are the CF workload identity attributes parsed from a container's
// instance certificate. ProcessType is filled in by the resolver, not here.
type Selectors struct {
	OrgID       string
	SpaceID     string
	AppID       string
	ProcessType string
}

// readInstanceCredentials reads the CF instance certificate and key from the
// container's view at <procRoot>/<pid>/root/etc/cf-instance-credentials and
// returns the raw cert PEM, the parsed certificate, and the private key signer.
func readInstanceCredentials(procRoot string, pid int) (certPEM string, cert *x509.Certificate, key crypto.Signer, err error) {
	base := filepath.Join(procRoot, strconv.Itoa(pid), "root", "etc", "cf-instance-credentials")

	crtBytes, err := os.ReadFile(filepath.Join(base, "instance.crt"))
	if err != nil {
		return "", nil, nil, err
	}
	block, _ := pem.Decode(crtBytes)
	if block == nil {
		return "", nil, nil, fmt.Errorf("instance.crt: no PEM data")
	}
	cert, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", nil, nil, fmt.Errorf("instance.crt: %w", err)
	}

	keyBytes, err := os.ReadFile(filepath.Join(base, "instance.key"))
	if err != nil {
		return "", nil, nil, err
	}
	key, err = parsePrivateKey(keyBytes)
	if err != nil {
		return "", nil, nil, fmt.Errorf("instance.key: %w", err)
	}

	return string(crtBytes), cert, key, nil
}

// parseSelectors extracts CF selectors from the certificate's OU entries, which
// are encoded as "organization:<id>", "space:<id>", "app:<id>". OU order is not
// significant (x509 stores them as a SET). ProcessType is left empty.
func parseSelectors(cert *x509.Certificate) (Selectors, error) {
	var s Selectors
	for _, ou := range cert.Subject.OrganizationalUnit {
		key, val, ok := strings.Cut(ou, ":")
		if !ok {
			continue
		}
		switch key {
		case "organization":
			s.OrgID = val
		case "space":
			s.SpaceID = val
		case "app":
			s.AppID = val
		}
	}
	if s.OrgID == "" && s.SpaceID == "" && s.AppID == "" {
		return Selectors{}, fmt.Errorf("no CF selectors in certificate OUs")
	}
	return s, nil
}

// parsePrivateKey decodes a PEM private key, trying PKCS8, then PKCS1, then EC.
func parsePrivateKey(pemBytes []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("no PEM data")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		signer, ok := k.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("PKCS8 key is not a signer")
		}
		return signer, nil
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	return nil, fmt.Errorf("unsupported or malformed private key")
}
