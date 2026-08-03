package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"code.cloudfoundry.org/debugserver"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/durationjson"
	executorinit "code.cloudfoundry.org/executor/initializer"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/rep"
)

type RootFS struct {
	Name, Path string
}

type RootFSes []RootFS

func (m *RootFSes) UnmarshalJSON(data []byte) error {
	*m = make(RootFSes, 0)
	arr := []string{}
	err := json.Unmarshal(data, &arr)
	if err != nil {
		return err
	}

	for _, s := range arr {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) != 2 {
			return errors.New("Invalid preloaded RootFS value: not of the form 'stack-name:path'")
		}

		if parts[0] == "" {
			return errors.New("Invalid preloaded RootFS value: blank stack")
		}

		if parts[1] == "" {
			return errors.New("Invalid preloaded RootFS value: blank path")
		}

		*m = append(*m, RootFS{parts[0], parts[1]})
	}

	return nil
}

func (rootFSes RootFSes) Names() []string {
	names := make([]string, len(rootFSes))
	for i, rootFS := range rootFSes {
		names[i] = rootFS.Name
	}
	return names
}

func (rootFSes RootFSes) StackPathMap() rep.StackPathMap {
	m := make(rep.StackPathMap)
	for _, rootFS := range rootFSes {
		m[rootFS.Name] = rootFS.Path
	}
	return m
}

func (m RootFSes) MarshalJSON() (b []byte, err error) {
	arr := make([]string, len(m))
	for i, rootFS := range m {
		arr[i] = fmt.Sprintf("%s:%s", rootFS.Name, rootFS.Path)
	}
	return json.Marshal(arr)
}

type RepConfig struct {
	AdvertiseDomain                 string                `json:"advertise_domain,omitempty"`
	BBSAddress                      string                `json:"bbs_address"`
	BBSClientSessionCacheSize       int                   `json:"bbs_client_session_cache_size,omitempty"`
	BBSMaxIdleConnsPerHost          int                   `json:"bbs_max_idle_conns_per_host,omitempty"`
	BBSCACertFile                   string                `json:"bbs_ca_cert_file"`     // DEPRECATED. Kept around for dusts compatability
	BBSClientCertFile               string                `json:"bbs_client_cert_file"` // DEPRECATED. Kept around for dusts compatability
	BBSClientKeyFile                string                `json:"bbs_client_key_file"`  // DEPRECATED. Kept around for dusts compatability
	CaCertFile                      string                `json:"ca_cert_file"`
	CellAnnotations                 map[string]string     `json:"cell_annotations,omitempty"`
	CellID                          string                `json:"cell_id"`
	CellIndex                       int                   `json:"cell_index"`
	CommunicationTimeout            durationjson.Duration `json:"communication_timeout,omitempty"`
	EvacuationPollingInterval       durationjson.Duration `json:"evacuation_polling_interval,omitempty"`
	EvacuationTimeout               durationjson.Duration `json:"evacuation_timeout,omitempty"`
	ExtraRootfsDir                  string                `json:"extra_root_fs_dir"`
	LayeringMode                    string                `json:"layering_mode,omitempty"`
	ListenAddr                      string                `json:"listen_addr,omitempty"`
	ListenAddrSecurable             string                `json:"listen_addr_securable,omitempty"`
	LockRetryInterval               durationjson.Duration `json:"lock_retry_interval,omitempty"`
	LockTTL                         durationjson.Duration `json:"lock_ttl,omitempty"`
	OptionalPlacementTags           []string              `json:"optional_placement_tags"`
	PlacementTags                   []string              `json:"placement_tags"`
	PollingInterval                 durationjson.Duration `json:"polling_interval,omitempty"`
	PreloadedRootFS                 RootFSes              `json:"preloaded_root_fs"`
	RepURL                          string                `json:"rep_url,omitempty"`
	SidecarRootFSPath               string                `json:"sidecar_root_fs_path"`
	SidecarRootFS                   string                `json:"sidecar_root_fs"`
	ServerCertFile                  string                `json:"server_cert_file"` // DEPRECATED. Kept around for dusts compatability
	ServerKeyFile                   string                `json:"server_key_file"`  // DEPRECATED. Kept around for dusts compatability
	CertFile                        string                `json:"cert_file"`
	KeyFile                         string                `json:"key_file"`
	SessionName                     string                `json:"session_name,omitempty"`
	SupportedProviders              []string              `json:"supported_providers"`
	Zone                            string                `json:"zone"`
	ReportInterval                  durationjson.Duration `json:"report_interval,omitempty"`
	DiskHealthCheckPaths            []string              `json:"disk_health_check_paths,omitempty"`
	DiskHealthCheckInterval         durationjson.Duration `json:"disk_health_check_interval,omitempty"`
	DiskHealthCheckFailureThreshold int                   `json:"disk_health_check_failure_threshold,omitempty"`
	LoggregatorConfig               loggingclient.Config  `json:"loggregator"`
	debugserver.DebugServerConfig
	executorinit.ExecutorConfig
	lagerflags.LagerConfig
	locket.ClientLocketConfig
}

func NewRepConfig(configPath string) (RepConfig, error) {
	repConfig := RepConfig{}
	configFile, err := os.Open(configPath)
	if err != nil {
		return RepConfig{}, err
	}

	defer configFile.Close()

	decoder := json.NewDecoder(configFile)

	err = decoder.Decode(&repConfig)
	if err != nil {
		return RepConfig{}, err
	}

	return repConfig, nil
}
