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
	return newHTTPSClient(insecureSkipVerify, caCertFiles, "", "", communicationTimeout)
}

// NewMutualTLSClient builds an HTTP client that, in addition to verifying the
// server against the provided CA cert files, presents the given client
// certificate/key for mutual TLS. clientCertFile and clientKeyFile must both be
// non-empty.
func NewMutualTLSClient(insecureSkipVerify bool, caCertFiles []string, clientCertFile, clientKeyFile string, communicationTimeout time.Duration) (*http.Client, error) {
	if clientCertFile == "" || clientKeyFile == "" {
		return nil, errors.New("client certificate and key are required for mutual TLS")
	}
	return newHTTPSClient(insecureSkipVerify, caCertFiles, clientCertFile, clientKeyFile, communicationTimeout)
}

func newHTTPSClient(insecureSkipVerify bool, caCertFiles []string, clientCertFile, clientKeyFile string, communicationTimeout time.Duration) (*http.Client, error) {
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

	if clientCertFile != "" && clientKeyFile != "" {
		clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert/key pair: %s", err.Error())
		}
		tlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	return &http.Client{
		Transport: &http.Transport{
			DialContext:     dialer.DialContext,
			TLSClientConfig: tlsConfig,
		},
		Timeout: communicationTimeout,
	}, nil
}
