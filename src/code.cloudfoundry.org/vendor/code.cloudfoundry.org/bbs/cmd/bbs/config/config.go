package config

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/debugserver"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/locket"
)

type RouteConfiguration struct {
	RequestCountRoutes   []string `json:"request_count"`
	RequestLatencyRoutes []string `json:"request_latency"`
}

type AdvancedMetrics struct {
	Enabled     bool               `json:"enabled"`
	RouteConfig RouteConfiguration `json:"route_config"`
}

type BBSConfig struct {
	AccessLogPath                 string                `json:"access_log_path,omitempty"`
	AdvertiseURL                  string                `json:"advertise_url,omitempty"`
	AuctioneerAddress             string                `json:"auctioneer_address,omitempty"`
	AuctioneerCACert              string                `json:"auctioneer_ca_cert,omitempty"`
	AuctioneerClientCert          string                `json:"auctioneer_client_cert,omitempty"`
	AuctioneerClientKey           string                `json:"auctioneer_client_key,omitempty"`
	AuctioneerRequireTLS          bool                  `json:"auctioneer_require_tls,omitempty"`
	UUID                          string                `json:"uuid,omitempty"`
	CaFile                        string                `json:"ca_file,omitempty"`
	CertFile                      string                `json:"cert_file,omitempty"`
	CommunicationTimeout          durationjson.Duration `json:"communication_timeout,omitempty"`
	ConvergeRepeatInterval        durationjson.Duration `json:"converge_repeat_interval,omitempty"`
	ConvergenceWorkers            int                   `json:"convergence_workers,omitempty"`
	DatabaseConnectionString      string                `json:"database_connection_string"`
	DatabaseDriver                string                `json:"database_driver,omitempty"`
	DebugLRPStartHeartbeats       bool                  `json:"debug_lrp_start_heartbeats,omitempty"`
	DesiredLRPCreationTimeout     durationjson.Duration `json:"desired_lrp_creation_timeout,omitempty"`
	ExpireCompletedTaskDuration   durationjson.Duration `json:"expire_completed_task_duration,omitempty"`
	ExpirePendingTaskDuration     durationjson.Duration `json:"expire_pending_task_duration,omitempty"`
	HealthAddress                 string                `json:"health_address,omitempty"`
	HealthCheckTimeout            durationjson.Duration `json:"health_check_timeout,omitempty"`
	HealthCheckFailureThreshold   int                   `json:"health_check_failure_threshold,omitempty"`
	HealthCheckInterval           durationjson.Duration `json:"health_check_interval,omitempty"`
	EnableDBHealthCheck           bool                  `json:"enable_db_health_check,omitempty"`
	KeyFile                       string                `json:"key_file,omitempty"`
	KickTaskDuration              durationjson.Duration `json:"kick_task_duration,omitempty"`
	ListenAddress                 string                `json:"listen_address,omitempty"`
	LockRetryInterval             durationjson.Duration `json:"lock_retry_interval,omitempty"`
	LockTTL                       durationjson.Duration `json:"lock_ttl,omitempty"`
	MaxIdleDatabaseConnections    int                   `json:"max_idle_database_connections,omitempty"`
	DBConnectionTimeout           durationjson.Duration `json:"db_connection_timeout,omitempty"`
	DBReadTimeout                 durationjson.Duration `json:"db_read_timeout,omitempty"`
	DBWriteTimeout                durationjson.Duration `json:"db_write_timeout,omitempty"`
	MaxOpenDatabaseConnections    int                   `json:"max_open_database_connections,omitempty"`
	MaxDatabaseConnectionLifetime durationjson.Duration `json:"max_database_connection_lifetime,omitempty"`
	MaxTaskRetries                int                   `json:"max_task_retries,omitempty"`
	RepCACert                     string                `json:"rep_ca_cert,omitempty"`
	RepClientCert                 string                `json:"rep_client_cert,omitempty"`
	RepClientKey                  string                `json:"rep_client_key,omitempty"`
	RepClientSessionCacheSize     int                   `json:"rep_client_session_cache_size,omitempty"`
	ReportInterval                durationjson.Duration `json:"report_interval,omitempty"`
	RequireSSL                    bool                  `json:"require_ssl,omitempty"`
	SQLCACertFile                 string                `json:"sql_ca_cert_file,omitempty"`
	SQLEnableIdentityVerification bool                  `json:"sql_enable_identity_verification,omitempty"`
	SessionName                   string                `json:"session_name,omitempty"`
	TaskCallbackWorkers           int                   `json:"task_callback_workers,omitempty"`
	UpdateWorkers                 int                   `json:"update_workers,omitempty"`
	LoggregatorConfig             loggingclient.Config  `json:"loggregator"`
	AdvancedMetricsConfig         AdvancedMetrics       `json:"advanced_metrics"`
	debugserver.DebugServerConfig
	encryption.EncryptionConfig
	lagerflags.LagerConfig
	locket.ClientLocketConfig
}

func NewBBSConfig(configPath string) (BBSConfig, error) {
	bbsConfig := BBSConfig{}
	configFile, err := os.Open(configPath)
	if err != nil {
		return BBSConfig{}, err
	}

	defer configFile.Close()

	decoder := json.NewDecoder(configFile)

	err = decoder.Decode(&bbsConfig)
	if err != nil {
		return BBSConfig{}, err
	}

	return bbsConfig, nil
}
