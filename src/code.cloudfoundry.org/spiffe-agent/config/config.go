package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/lager/v3/lagerflags"
)

const signerHTTPTimeout = 30 * time.Second

type Config struct {
	lagerflags.LagerConfig
	SocketPath         string `json:"socket_path"`
	TrustDomain        string `json:"trust_domain"`
	CellID             string `json:"cell_id"`
	SignerURL          string `json:"signer_url"`
	SignerClientID     string `json:"signer_client_id"`
	SignerClientSecret string `json:"signer_client_secret"`
	SignerCACertFile   string `json:"signer_ca_cert_file"`
	BBSAddress         string `json:"bbs_address"`
	BBSCACertFile      string `json:"bbs_ca_cert_file"`
	BBSClientCertFile  string `json:"bbs_client_cert_file"`
	BBSClientKeyFile   string `json:"bbs_client_key_file"`
}

func defaultConfig() Config {
	return Config{
		LagerConfig: lagerflags.DefaultLagerConfig(),
		SocketPath:  "/var/vcap/data/spiffe-agent/run/workload.sock",
	}
}

func NewSpiffeAgentConfig(path string) (Config, error) {
	cfg := defaultConfig()

	contents, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	if err := json.Unmarshal(contents, &cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) SignerHTTPClient() (*http.Client, error) {
	if c.SignerCACertFile == "" {
		return &http.Client{Timeout: signerHTTPTimeout}, nil
	}

	caCert, err := os.ReadFile(c.SignerCACertFile)
	if err != nil {
		return nil, err
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("no valid certs in %s", c.SignerCACertFile)
	}

	return &http.Client{
		Timeout: signerHTTPTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool},
		},
	}, nil
}
