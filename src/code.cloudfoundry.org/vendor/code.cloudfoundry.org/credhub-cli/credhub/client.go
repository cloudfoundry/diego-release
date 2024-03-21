package credhub

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"time"

	proxy "github.com/cloudfoundry/socks5-proxy"
)

// Client provides an unauthenticated http.Client to the CredHub server
func (ch *CredHub) Client() *http.Client {
	if ch.defaultClient == nil {
		ch.defaultClient = ch.client()
	}

	return ch.defaultClient
}

func (ch *CredHub) client() *http.Client {
	if ch.baseURL.Scheme == "https" {
		return httpsClient(ch.insecureSkipVerify, ch.caCerts, ch.clientCertificate, ch.httpTimeout)
	}

	return httpClient(ch.httpTimeout)
}

func httpClient(timeout *time.Duration) *http.Client {
	if timeout == nil {
		t := 45 * time.Second
		timeout = &t
	}

	return &http.Client{
		Timeout: *timeout,
	}
}

func httpsClient(insecureSkipVerify bool, rootCAs *x509.CertPool, cert *tls.Certificate, timeout *time.Duration) *http.Client {
	client := httpClient(timeout)
	var certs []tls.Certificate
	if cert != nil {
		certs = []tls.Certificate{*cert}
	}

	var dialer = SOCKS5DialFuncFromEnvironment((&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).Dial, proxy.NewSocks5Proxy(proxy.NewHostKey(), nil, 30*time.Second))

	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify:       insecureSkipVerify,
			PreferServerCipherSuites: true,
			Certificates:             certs,
			RootCAs:                  rootCAs,
			MinVersion:               tls.VersionTLS12,
		},
		Proxy:               http.ProxyFromEnvironment,
		Dial:                dialer,
		MaxIdleConnsPerHost: 100,
	}

	return client
}
