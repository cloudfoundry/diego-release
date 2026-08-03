package db

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"

	"code.cloudfoundry.org/routing-api/config"
)

//go:generate counterfeiter -o fakes/fake_mysql_adapter.go --fake-name MySQLAdapter . mySQLAdapter
type mySQLAdapter interface {
	RegisterTLSConfig(key string, config *tls.Config) error
}

type MySQLConnectionStringBuilder struct {
	MySQLAdapter mySQLAdapter
}

func (m *MySQLConnectionStringBuilder) Build(cfg *config.SqlDB) (string, error) {
	rootCA := x509.NewCertPool()
	queryString := "?parseTime=true"
	if cfg.SkipSSLValidation {
		tlsConfig := tls.Config{}
		tlsConfig.InsecureSkipVerify = cfg.SkipSSLValidation
		configKey := "dbTLSSkipVerify"
		err := m.MySQLAdapter.RegisterTLSConfig(configKey, &tlsConfig)
		if err != nil {
			return "", err
		}
		queryString = fmt.Sprintf("%s&tls=%s", queryString, configKey)

	} else if cfg.CACert != "" {
		tlsConfig := tls.Config{}
		rootCA.AppendCertsFromPEM([]byte(cfg.CACert))
		tlsConfig.ServerName = cfg.Host
		tlsConfig.RootCAs = rootCA
		if cfg.SkipHostnameValidation {
			tlsConfig.InsecureSkipVerify = true
			tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
				return VerifyCertificatesIgnoreHostname(rawCerts, rootCA)
			}
		}
		configKey := "dbTLSCertVerify"
		err := m.MySQLAdapter.RegisterTLSConfig(configKey, &tlsConfig)
		if err != nil {
			return "", err
		}
		queryString = fmt.Sprintf("%s&tls=%s", queryString, configKey)
	}
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s%s",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Schema,
		queryString,
	), nil
}

func VerifyCertificatesIgnoreHostname(rawCerts [][]byte, caCertPool *x509.CertPool) error {
	certs := make([]*x509.Certificate, len(rawCerts))
	for i, asn1Data := range rawCerts {
		cert, err := x509.ParseCertificate(asn1Data)
		if err != nil {
			return fmt.Errorf("tls: failed to parse certificate from server: %s", err)
		}
		certs[i] = cert
	}

	opts := x509.VerifyOptions{
		Roots:         caCertPool,
		CurrentTime:   time.Now(),
		Intermediates: x509.NewCertPool(),
	}

	for i, cert := range certs {
		if i == 0 {
			continue
		}
		opts.Intermediates.AddCert(cert)
	}

	_, err := certs[0].Verify(opts)
	if err != nil {
		return err
	}

	return nil
}
