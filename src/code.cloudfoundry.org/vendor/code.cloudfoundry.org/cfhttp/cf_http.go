package cfhttp

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/cfhttp/unix_transport"
)

var SUPPORTED_CIPHER_SUITES = []uint16{
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
}

var config Config

// Deprecated: Use code.cloudfoundry.org/cfhttp/v2
type Config struct {
	Timeout time.Duration
}

// Deprecated: Use code.cloudfoundry.org/cfhttp/v2
func Initialize(timeout time.Duration) {
	atomic.StoreInt64((*int64)(&config.Timeout), int64(timeout))
}

// Deprecated: Use NewClient in code.cloudfoundry.org/cfhttp/v2
func NewClient() *http.Client {
	return newClient(5*time.Second, 0*time.Second, 90*time.Second, time.Duration(atomic.LoadInt64((*int64)(&config.Timeout))))
}

// Deprecated: Use NewClient in code.cloudfoundry.org/cfhttp/v2
func NewUnixClient(socketPath string) *http.Client {
	return &http.Client{
		Transport: unix_transport.NewWithDial(socketPath,
			(&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 0 * time.Second,
			}).Dial),
		Timeout: time.Duration(atomic.LoadInt64((*int64)(&config.Timeout))),
	}
}

// Deprecated: Use NewClient in code.cloudfoundry.org/cfhttp/v2
func NewCustomTimeoutClient(customTimeout time.Duration) *http.Client {
	return newClient(5*time.Second, 0*time.Second, 90*time.Second, customTimeout)
}

// Deprecated: Use NewClient in code.cloudfoundry.org/cfhttp/v2
func NewStreamingClient() *http.Client {
	return newClient(5*time.Second, 30*time.Second, 90*time.Second, 0*time.Second)
}

func newClient(dialTimeout, keepAliveTimeout, idleConnTimeout, timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: keepAliveTimeout,
			}).DialContext,
			IdleConnTimeout: idleConnTimeout,
		},
		Timeout: timeout,
	}
}

// Deprecated: use code.cloudfoundry.org/tlsconfig
func NewTLSConfig(certFile, keyFile, caCertFile string) (*tls.Config, error) {
	caCertPool := x509.NewCertPool()
	if caCertFile != "" {
		certBytes, err := ioutil.ReadFile(caCertFile)
		if err != nil {
			return nil, fmt.Errorf("failed read ca cert file: %s", err.Error())
		}

		if ok := caCertPool.AppendCertsFromPEM(certBytes); !ok {
			return nil, errors.New("Unable to load caCert")
		}
	}
	return NewTLSConfigWithCertPool(certFile, keyFile, caCertPool)
}

// Deprecated: use code.cloudfoundry.org/tlsconfig
func NewTLSConfigWithCertPool(certFile, keyFile string, caCertPool *x509.CertPool) (*tls.Config, error) {
	tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load keypair: %s", err.Error())
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: false,
		ClientAuth:         tls.RequireAndVerifyClientCert,
		CipherSuites:       SUPPORTED_CIPHER_SUITES,
		MinVersion:         tls.VersionTLS12,
	}

	if caCertPool == nil {
		return nil, fmt.Errorf("CaCertPool is nil")
	}

	tlsConfig.RootCAs = caCertPool
	tlsConfig.ClientCAs = caCertPool

	return tlsConfig, nil
}
