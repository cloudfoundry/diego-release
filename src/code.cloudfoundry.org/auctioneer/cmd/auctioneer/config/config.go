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

type AuctioneerConfig struct {
	AuctionRunnerWorkers          int                   `json:"auction_runner_workers,omitempty"`
	BBSAddress                    string                `json:"bbs_address,omitempty"`
	BBSCACertFile                 string                `json:"bbs_ca_cert_file,omitempty"`
	BBSClientCertFile             string                `json:"bbs_client_cert_file,omitempty"`
	BBSClientKeyFile              string                `json:"bbs_client_key_file,omitempty"`
	BBSClientSessionCacheSize     int                   `json:"bbs_client_session_cache_size,omitempty"`
	BBSMaxIdleConnsPerHost        int                   `json:"bbs_max_idle_conns_per_host,omitempty"`
	CACertFile                    string                `json:"ca_cert_file,omitempty"`
	BinPackFirstFitWeight         float64               `json:"bin_pack_first_fit_weight,omitempty"`
	CellStateTimeout              durationjson.Duration `json:"cell_state_timeout,omitempty"`
	CommunicationTimeout          durationjson.Duration `json:"communication_timeout,omitempty"`
	ListenAddress                 string                `json:"listen_address,omitempty"`
	LockRetryInterval             durationjson.Duration `json:"lock_retry_interval,omitempty"`
	LockTTL                       durationjson.Duration `json:"lock_ttl,omitempty"`
	LoggregatorConfig             loggingclient.Config  `json:"loggregator"`
	RepCACert                     string                `json:"rep_ca_cert,omitempty"`
	RepClientCert                 string                `json:"rep_client_cert,omitempty"`
	RepClientKey                  string                `json:"rep_client_key,omitempty"`
	RepClientSessionCacheSize     int                   `json:"rep_client_session_cache_size,omitempty"`
	ReportInterval                durationjson.Duration `json:"report_interval,omitempty"`
	ServerCertFile                string                `json:"server_cert_file,omitempty"`
	ServerKeyFile                 string                `json:"server_key_file,omitempty"`
	StartingContainerCountMaximum int                   `json:"starting_container_count_maximum,omitempty"`
	StartingContainerWeight       float64               `json:"starting_container_weight,omitempty"`
	UUID                          string                `json:"uuid,omitempty"`
	debugserver.DebugServerConfig
	lagerflags.LagerConfig
	locket.ClientLocketConfig
}

func NewAuctioneerConfig(configPath string) (AuctioneerConfig, error) {
	cfg := AuctioneerConfig{}

	configFile, err := os.Open(configPath)
	if err != nil {
		return AuctioneerConfig{}, err
	}

	defer configFile.Close()

	decoder := json.NewDecoder(configFile)

	err = decoder.Decode(&cfg)
	if err != nil {
		return AuctioneerConfig{}, err
	}

	return cfg, nil
}
