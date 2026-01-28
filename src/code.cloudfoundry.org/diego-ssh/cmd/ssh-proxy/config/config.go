package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"os"

	"code.cloudfoundry.org/debugserver"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/tlsconfig"
)

type SSHProxyConfig struct {
	lagerflags.LagerConfig
	debugserver.DebugServerConfig
	Address                   string                `json:"address,omitempty"`
	HealthCheckAddress        string                `json:"health_check_address,omitempty"`
	DisableHealthCheckServer  bool                  `json:"disable_health_check_server,omitempty"`
	HostKey                   string                `json:"host_key"`
	BBSAddress                string                `json:"bbs_address"`
	CCAPIURL                  string                `json:"cc_api_url"`
	CCAPICACert               string                `json:"cc_api_ca_cert"`
	UAATokenURL               string                `json:"uaa_token_url"`
	UAAPassword               string                `json:"uaa_password"`
	UAAUsername               string                `json:"uaa_username"`
	UAACACert                 string                `json:"uaa_ca_cert"`
	SkipCertVerify            bool                  `json:"skip_cert_verify"`
	EnableCFAuth              bool                  `json:"enable_cf_auth"`
	EnableDiegoAuth           bool                  `json:"enable_diego_auth"`
	DiegoCredentials          string                `json:"diego_credentials"`
	BBSCACert                 string                `json:"bbs_ca_cert"`
	BBSClientCert             string                `json:"bbs_client_cert"`
	BBSClientKey              string                `json:"bbs_client_key"`
	BBSClientSessionCacheSize int                   `json:"bbs_client_session_cache_size"`
	BBSMaxIdleConnsPerHost    int                   `json:"bbs_max_idle_conns_per_host"`
	AllowedCiphers            string                `json:"allowed_ciphers"`
	AllowedMACs               string                `json:"allowed_macs"`
	AllowedKeyExchanges       string                `json:"allowed_key_exchanges"`
	LoggregatorConfig         loggingclient.Config  `json:"loggregator"`
	CommunicationTimeout      durationjson.Duration `json:"communication_timeout,omitempty"`
	IdleConnectionTimeout     durationjson.Duration `json:"idle_connection_timeout,omitempty"`
	ConnectToInstanceAddress  bool                  `json:"connect_to_instance_address"`

	BackendsTLSEnabled    bool   `json:"backends_tls_enabled,omitempty"`
	BackendsTLSCACerts    string `json:"backends_tls_ca_certificates,omitempty"`
	BackendsTLSClientCert string `json:"backends_tls_client_certificate,omitempty"`
	BackendsTLSClientKey  string `json:"backends_tls_client_private_key,omitempty"`
}

func NewSSHProxyConfig(configPath string) (SSHProxyConfig, error) {
	proxyConfig := SSHProxyConfig{}

	configFile, err := os.Open(configPath)
	if err != nil {
		return SSHProxyConfig{}, err
	}

	defer configFile.Close()

	decoder := json.NewDecoder(configFile)

	err = decoder.Decode(&proxyConfig)
	if err != nil {
		return SSHProxyConfig{}, err
	}

	return proxyConfig, nil
}

func (c SSHProxyConfig) BackendsTLSConfig() (*tls.Config, error) {
	if !c.BackendsTLSEnabled {
		return nil, nil
	}

	if c.BackendsTLSCACerts == "" {
		return nil, errors.New("backend tls ca certificates must be specified if backend TLS is enabled")
	}

	rootCAs := x509.NewCertPool()
	ca, err := os.ReadFile(c.BackendsTLSCACerts)
	if err != nil {
		return nil, err
	}

	ok := rootCAs.AppendCertsFromPEM(ca)
	if !ok {
		return nil, errors.New("Failed to parse backends_tls_ca_certificates")
	}

	config := &tls.Config{
		RootCAs: rootCAs,
	}

	if c.BackendsTLSClientCert == "" || c.BackendsTLSClientKey == "" {
		return config, nil
	}

	return tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(c.BackendsTLSClientCert, c.BackendsTLSClientKey),
	).Client(tlsconfig.WithAuthority(rootCAs))
}
