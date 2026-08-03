package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/routing-api/models"
)

const (
	DefaultLockResourceKey = "routing_api_lock"
)

type MetronConfig struct {
	Address string
	Port    string
}

type OAuthConfig struct {
	TokenEndpoint     string `yaml:"token_endpoint"`
	Port              uint16 `yaml:"port"`
	SkipSSLValidation bool   `yaml:"-"`
	ClientName        string `yaml:"client_name"`
	ClientSecret      string `yaml:"client_secret"`
	CACerts           string `yaml:"ca_certs"`
}

type SqlDB struct {
	Host                   string `yaml:"host"`
	Port                   uint16 `yaml:"port"`
	Schema                 string `yaml:"schema"`
	Type                   string `yaml:"type"`
	Username               string `yaml:"username"`
	Password               string `yaml:"password"`
	CACert                 string `yaml:"ca_cert"`
	SkipSSLValidation      bool   `yaml:"-"`
	SkipHostnameValidation bool   `yaml:"skip_hostname_validation"`
	MaxIdleConns           int    `yaml:"max_idle_connections"`
	MaxOpenConns           int    `yaml:"max_open_connections"`
	ConnMaxLifetime        int    `yaml:"connections_max_lifetime_seconds"`
}

type APIConfig struct {
	ListenPort  uint16 `yaml:"listen_port"`
	HTTPEnabled bool   `yaml:"http_enabled"`

	MTLSListenPort     uint16 `yaml:"mtls_listen_port"`
	MTLSClientCAPath   string `yaml:"mtls_client_ca_file"`
	MTLSServerCertPath string `yaml:"mtls_server_cert_file"`
	MTLSServerKeyPath  string `yaml:"mtls_server_key_file"`
}

type Config struct {
	API                             APIConfig                 `yaml:"api"`
	AdminPort                       uint16                    `yaml:"admin_port"`
	DebugAddress                    string                    `yaml:"debug_address"`
	LogGuid                         string                    `yaml:"log_guid"`
	MetronConfig                    MetronConfig              `yaml:"metron_config"`
	MaxTTL                          time.Duration             `yaml:"max_ttl"`
	SystemDomain                    string                    `yaml:"system_domain"`
	MetricsReportingIntervalString  string                    `yaml:"metrics_reporting_interval"`
	MetricsReportingInterval        time.Duration             `yaml:"-"`
	StatsdEndpoint                  string                    `yaml:"statsd_endpoint"`
	StatsdClientFlushIntervalString string                    `yaml:"statsd_client_flush_interval"`
	StatsdClientFlushInterval       time.Duration             `yaml:"-"`
	OAuth                           OAuthConfig               `yaml:"oauth"`
	RouterGroups                    models.RouterGroups       `yaml:"router_groups"`
	ReservedSystemComponentPorts    []uint16                  `yaml:"reserved_system_component_ports"`
	FailOnRouterPortConflicts       bool                      `yaml:"fail_on_router_port_conflicts"`
	SqlDB                           SqlDB                     `yaml:"sqldb"`
	Locket                          locket.ClientLocketConfig `yaml:"locket"`
	UUID                            string                    `yaml:"uuid"`
	SkipSSLValidation               bool                      `yaml:"skip_ssl_validation"`
	LockTTL                         time.Duration             `yaml:"lock_ttl"`
	RetryInterval                   time.Duration             `yaml:"retry_interval"`
	LockResouceKey                  string                    `yaml:"lock_resource_key"`
}

func NewConfigFromFile(configFile string, authDisabled bool) (Config, error) {
	c, err := os.ReadFile(configFile)
	if err != nil {
		return Config{}, err
	}
	return NewConfigFromBytes(c, authDisabled)
}

func NewConfigFromBytes(bytes []byte, authDisabled bool) (Config, error) {
	config := Config{}
	err := yaml.Unmarshal(bytes, &config)
	if err != nil {
		return config, err
	}

	err = config.validate(authDisabled)
	if err != nil {
		return config, err
	}

	err = config.process()
	if err != nil {
		return config, err
	}

	return config, nil
}

func (cfg *Config) validate(authDisabled bool) error {
	if cfg.SystemDomain == "" {
		return errors.New("no system_domain specified")
	}

	if cfg.LogGuid == "" {
		return errors.New("no log_guid specified")
	}

	if !authDisabled && cfg.OAuth.TokenEndpoint == "" {
		return errors.New("no token endpoint specified")
	}

	if !authDisabled && cfg.OAuth.TokenEndpoint != "" && cfg.OAuth.Port == 0 {
		return errors.New("routing API requires TLS enabled to get OAuth token")
	}

	if cfg.UUID == "" {
		return errors.New("no UUID is specified")
	}

	if err := validatePort(cfg.AdminPort); err != nil {
		return fmt.Errorf("invalid admin port: %s", err)
	}

	if err := validatePort(cfg.API.ListenPort); err != nil {
		return fmt.Errorf("invalid API listen port: %s", err)
	}

	if err := validatePort(cfg.API.MTLSListenPort); err != nil {
		return fmt.Errorf("invalid API mTLS listen port: %s", err)
	}

	models.ReservedSystemComponentPorts = cfg.ReservedSystemComponentPorts
	models.FailOnRouterPortConflicts = cfg.FailOnRouterPortConflicts

	if cfg.Locket.LocketAddress == "" {
		return errors.New("locket address is required")
	}

	if err := cfg.RouterGroups.Validate(); err != nil {
		return err
	}

	return nil
}

func validatePort(port uint16) error {
	if port < 1 {
		return fmt.Errorf("port number is invalid: %d (1-65535)", port)
	}

	return nil
}

func (cfg *Config) process() error {
	if cfg.LockTTL == 0 {
		cfg.LockTTL = locket.DefaultSessionTTL
	}

	if cfg.RetryInterval == 0 {
		cfg.RetryInterval = locket.RetryInterval
	}

	if cfg.LockResouceKey == "" {
		cfg.LockResouceKey = DefaultLockResourceKey
	}

	cfg.SqlDB.SkipSSLValidation = cfg.SkipSSLValidation
	cfg.OAuth.SkipSSLValidation = cfg.SkipSSLValidation

	metricsReportingInterval, err := time.ParseDuration(cfg.MetricsReportingIntervalString)
	if err != nil {
		return err
	}
	cfg.MetricsReportingInterval = metricsReportingInterval

	statsdClientFlushInterval, err := time.ParseDuration(cfg.StatsdClientFlushIntervalString)
	if err != nil {
		return err
	}
	cfg.StatsdClientFlushInterval = statsdClientFlushInterval

	if cfg.MaxTTL == 0 {
		cfg.MaxTTL = 2 * time.Minute
	}

	return nil
}
