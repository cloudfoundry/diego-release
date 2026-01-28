package helpers

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

func NewHTTPSClient(insecureSkipVerify bool, caCertFiles []string, communicationTimeout time.Duration) (*http.Client, error) {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: insecureSkipVerify}

	caCertPool := x509.NewCertPool()
	for _, caCertFile := range caCertFiles {
		if caCertFile != "" {
			certBytes, err := os.ReadFile(caCertFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read ca cert file: %s", err.Error())
			}

			if ok := caCertPool.AppendCertsFromPEM(certBytes); !ok {
				return nil, errors.New("Unable to load caCert")
			}
		}
	}
	tlsConfig.RootCAs = caCertPool

	return &http.Client{
		Transport: &http.Transport{
			Dial:            dialer.Dial,
			TLSClientConfig: tlsConfig,
		},
		Timeout: communicationTimeout,
	}, nil
}
