package config

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/debugserver"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/locket"
)

type RoutingAPIConfig struct {
	URL            string `json:"url"`
	Port           uint16 `json:"port"`
	CACertFile     string `json:"ca_cert_file"`
	ClientCertFile string `json:"client_cert_file"`
	ClientKeyFile  string `json:"client_key_file"`
	AuthEnabled    bool   `json:"auth_enabled"`
}

type OAuthConfig struct {
	UaaURL            string                `json:"uaa_url"`
	UaaRequestTimeout durationjson.Duration `json:"uaa_request_timeout"`
	ClientName        string                `json:"client_name"`
	ClientSecret      string                `json:"client_secret"`
	CACerts           string                `json:"ca_certs"`
	SkipCertVerify    bool                  `json:"skip_cert_verify"`
}

type RouteEmitterConfig struct {
	BBSAddress                   string                `json:"bbs_address"`
	BBSCACertFile                string                `json:"bbs_ca_cert_file"`
	BBSClientCertFile            string                `json:"bbs_client_cert_file"`
	BBSClientKeyFile             string                `json:"bbs_client_key_file"`
	BBSClientSessionCacheSize    int                   `json:"bbs_client_session_cache_size,omitempty"`
	BBSMaxIdleConnsPerHost       int                   `json:"bbs_max_idle_conns_per_host,omitempty"`
	CellID                       string                `json:"cell_id,omitempty"`
	UUID                         string                `json:"uuid,omitempty"`
	RegisterDirectInstanceRoutes bool                  `json:"register_direct_instance_routes,omitempty"`
	CommunicationTimeout         durationjson.Duration `json:"communication_timeout,omitempty"`
	HealthCheckAddress           string                `json:"healthcheck_address,omitempty"`
	JitterFactor                 float64               `json:"jitter_factor,omitempty"`
	LockRetryInterval            durationjson.Duration `json:"lock_retry_interval,omitempty"`
	LockTTL                      durationjson.Duration `json:"lock_ttl,omitempty"`
	NATSAddresses                string                `json:"nats_addresses,omitempty"`
	NATSUsername                 string                `json:"nats_username,omitempty"`
	NATSPassword                 string                `json:"nats_password,omitempty"`
	NATSTLSEnabled               bool                  `json:"nats_tls_enabled"`
	NATSCACertFile               string                `json:"nats_ca_cert_file"`
	NATSClientCertFile           string                `json:"nats_client_cert_file"`
	NATSClientKeyFile            string                `json:"nats_client_key_file"`
	RouteEmittingWorkers         int                   `json:"route_emitting_workers,omitempty"`
	SyncInterval                 durationjson.Duration `json:"sync_interval,omitempty"`
	TCPRouteTTL                  durationjson.Duration `json:"tcp_route_ttl,omitempty"`
	TCPEnableTLS                 bool                  `json:"tcp_enable_tls"`
	OAuth                        OAuthConfig           `json:"oauth"`
	RoutingAPI                   RoutingAPIConfig      `json:"routing_api"`
	EnableTCPEmitter             bool                  `json:"enable_tcp_emitter"`
	LoggregatorConfig            loggingclient.Config  `json:"loggregator"`
	ReportInterval               durationjson.Duration `json:"report_interval,omitempty"`
	UnregistrationInterval       durationjson.Duration `json:"unregistration_interval,omitempty"`
	UnregistrationSendCount      int                   `json:"unregistration_send_count,omitempty"`
	EnableInternalEmitter        bool                  `json:"enable_internal_emitter"`
	LocketSessionName            string                `json:"locket_session_name"`

	lagerflags.LagerConfig
	debugserver.DebugServerConfig
	locket.ClientLocketConfig
}

func NewRouteEmitterConfig(configPath string) (RouteEmitterConfig, error) {
	routeEmitterConfig := RouteEmitterConfig{}

	configFile, err := os.Open(configPath)
	if err != nil {
		return RouteEmitterConfig{}, err
	}

	defer configFile.Close()

	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&routeEmitterConfig)
	if err != nil {
		return RouteEmitterConfig{}, err
	}

	return routeEmitterConfig, nil
}
