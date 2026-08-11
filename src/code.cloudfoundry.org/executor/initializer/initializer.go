package initializer

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/executor/depot/containerstore"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/tlsconfig"
)

//go:generate counterfeiter -o fakes/fake_cert_pool_retriever.go . CertPoolRetriever
type CertPoolRetriever interface {
	SystemCerts() (*x509.CertPool, error)
}

type systemcertsRetriever struct{}

func (s systemcertsRetriever) SystemCerts() (*x509.CertPool, error) {
	caCertPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	if caCertPool == nil {
		caCertPool = x509.NewCertPool()
	}
	return caCertPool, nil
}

type ExecutorConfig struct {
	AdvertisePreferenceForInstanceAddress bool                  `json:"advertise_preference_for_instance_address"`
	AutoDiskOverheadMB                    int                   `json:"auto_disk_capacity_overhead_mb"`
	CachePath                             string                `json:"cache_path,omitempty"`
	ContainerInodeLimit                   uint64                `json:"container_inode_limit,omitempty"`
	ContainerMaxCpuShares                 uint64                `json:"container_max_cpu_shares,omitempty"`
	ContainerMetricsReportInterval        durationjson.Duration `json:"container_metrics_report_interval,omitempty"`
	ContainerOwnerName                    string                `json:"container_owner_name,omitempty"`
	ContainerProxyADSServers              []string              `json:"container_proxy_ads_addresses,omitempty"`
	ContainerProxyConfigPath              string                `json:"container_proxy_config_path,omitempty"`
	ContainerProxyPath                    string                `json:"container_proxy_path,omitempty"`
	ContainerProxyRequireClientCerts      bool                  `json:"container_proxy_require_and_verify_client_certs"`
	ContainerProxyTrustedCACerts          []string              `json:"container_proxy_trusted_ca_certs"`
	ContainerProxyVerifySubjectAltName    []string              `json:"container_proxy_verify_subject_alt_name"`
	ContainerReapInterval                 durationjson.Duration `json:"container_reap_interval,omitempty"`
	CreateWorkPoolSize                    int                   `json:"create_work_pool_size,omitempty"`
	DeclarativeHealthcheckPath            string                `json:"declarative_healthcheck_path,omitempty"`
	DeleteWorkPoolSize                    int                   `json:"delete_work_pool_size,omitempty"`
	DiskMB                                string                `json:"disk_mb,omitempty"`
	EnableContainerProxy                  bool                  `json:"enable_container_proxy,omitempty"`
	EnableContainerProxyHealthChecks      bool                  `json:"enable_container_proxy_healthcheck,omitempty"`
	DeclarativeHealthCheckDefaultTimeout  durationjson.Duration `json:"declarative_healthcheck_default_timeout,omitempty"`
	EnableHealtcheckMetrics               bool                  `json:"enable_healthcheck_metrics,omitempty"`
	EnableUnproxiedPortMappings           bool                  `json:"enable_unproxied_port_mappings"`
	EnvoyConfigRefreshDelay               durationjson.Duration `json:"envoy_config_refresh_delay"`
	EnvoyConfigReloadDuration             durationjson.Duration `json:"envoy_config_reload_duration"`
	EnvoyDrainTimeout                     durationjson.Duration `json:"envoy_drain_timeout,omitempty"`
	ProxyHealthCheckInterval              durationjson.Duration `json:"proxy_healthcheck_interval,omitempty"`
	ExportNetworkEnvVars                  bool                  `json:"export_network_env_vars,omitempty"` // DEPRECATED. Kept around for dusts compatability
	GardenAddr                            string                `json:"garden_addr,omitempty"`
	GardenHealthcheckCommandRetryPause    durationjson.Duration `json:"garden_healthcheck_command_retry_pause,omitempty"`
	GardenHealthcheckEmissionInterval     durationjson.Duration `json:"garden_healthcheck_emission_interval,omitempty"`
	GardenHealthcheckInterval             durationjson.Duration `json:"garden_healthcheck_interval,omitempty"`
	GardenHealthcheckProcessArgs          []string              `json:"garden_healthcheck_process_args,omitempty"`
	GardenHealthcheckProcessDir           string                `json:"garden_healthcheck_process_dir"`
	GardenHealthcheckProcessEnv           []string              `json:"garden_healthcheck_process_env,omitempty"`
	GardenHealthcheckProcessPath          string                `json:"garden_healthcheck_process_path"`
	GardenHealthcheckProcessUser          string                `json:"garden_healthcheck_process_user"`
	GardenHealthcheckTimeout              durationjson.Duration `json:"garden_healthcheck_timeout,omitempty"`
	GardenNetwork                         string                `json:"garden_network,omitempty"`
	GracefulShutdownInterval              durationjson.Duration `json:"graceful_shutdown_interval,omitempty"`
	HealthCheckContainerOwnerName         string                `json:"healthcheck_container_owner_name,omitempty"`
	HealthCheckWorkPoolSize               int                   `json:"healthcheck_work_pool_size,omitempty"`
	HealthyMonitoringInterval             durationjson.Duration `json:"healthy_monitoring_interval,omitempty"`
	InstanceIdentityCAPath                string                `json:"instance_identity_ca_path,omitempty"`
	InstanceIdentityCredDir               string                `json:"instance_identity_cred_dir,omitempty"`
	InstanceIdentityPrivateKeyPath        string                `json:"instance_identity_private_key_path,omitempty"`
	InstanceIdentityValidityPeriod        durationjson.Duration `json:"instance_identity_validity_period,omitempty"`
	MaxCacheSizeInBytes                   uint64                `json:"max_cache_size_in_bytes,omitempty"`
	MinCachePartitionFreeBytes            uint64                `json:"min_cache_partition_free_bytes,omitempty"`
	MaxConcurrentDownloads                int                   `json:"max_concurrent_downloads,omitempty"`
	MaxLogLinesPerSecond                  int                   `json:"max_log_lines_per_second"`
	MemoryMB                              string                `json:"memory_mb,omitempty"`
	MetricsWorkPoolSize                   int                   `json:"metrics_work_pool_size,omitempty"`
	PathToCACertsForDownloads             string                `json:"path_to_ca_certs_for_downloads"`
	PathToTLSCACert                       string                `json:"path_to_tls_ca_cert"`
	PathToTLSCert                         string                `json:"path_to_tls_cert"`
	PathToTLSKey                          string                `json:"path_to_tls_key"`
	PostSetupHook                         string                `json:"post_setup_hook"`
	PostSetupUser                         string                `json:"post_setup_user"`
	ProxyEnableHttp2                      bool                  `json:"proxy_enable_http2"`
	ProxyMemoryAllocationMB               int                   `json:"proxy_memory_allocation_mb,omitempty"`
	ReadWorkPoolSize                      int                   `json:"read_work_pool_size,omitempty"`
	ReservedExpirationTime                durationjson.Duration `json:"reserved_expiration_time,omitempty"`
	SetCPUWeight                          bool                  `json:"set_cpu_weight,omitempty"`
	SkipCertVerify                        bool                  `json:"skip_cert_verify,omitempty"`
	TempDir                               string                `json:"temp_dir,omitempty"`
	TrustedSystemCertificatesPath         string                `json:"trusted_system_certificates_path"`
	UnhealthyMonitoringInterval           durationjson.Duration `json:"unhealthy_monitoring_interval,omitempty"`
	UseSchedulableDiskSize                bool                  `json:"use_schedulable_disk_size,omitempty"`
	VolmanDriverPaths                     string                `json:"volman_driver_paths"`
	VolumeMountedFiles                    string                `json:"volume_mounted_files"`
	UseNodePortForPodService              bool                  `json:"use_node_port_for_pod_service,omitempty"`
	UseKubernetesGardenClient             bool                  `json:"use_kubernetes_garden_client,omitempty"`
	WorkloadsNamespace                    string                `json:"workloads_namespace,omitempty"`
}

func TLSConfigFromConfig(logger lager.Logger, certsRetriever CertPoolRetriever, config ExecutorConfig) (*tls.Config, error) {
	var tlsConfig *tls.Config
	var err error

	caCertPool, err := certsRetriever.SystemCerts()
	if err != nil {
		return nil, err
	}
	if (config.PathToTLSKey != "" && config.PathToTLSCert == "") || (config.PathToTLSKey == "" && config.PathToTLSCert != "") {
		return nil, errors.New("The TLS certificate or key is missing")
	}

	if config.PathToTLSCACert != "" {
		caCertPool, err = appendCACerts(caCertPool, config.PathToTLSCACert)
		if err != nil {
			return nil, err
		}
	}

	if config.PathToCACertsForDownloads != "" {
		caCertPool, err = appendCACerts(caCertPool, config.PathToCACertsForDownloads)
		if err != nil {
			return nil, err
		}
	}

	if config.PathToTLSKey != "" && config.PathToTLSCert != "" {
		tlsConfig, err = tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(config.PathToTLSCert, config.PathToTLSKey),
		).Client(
			tlsconfig.WithAuthority(caCertPool),
		)
		if err != nil {
			logger.Error("failed-to-configure-tls", err)
			return nil, err
		}
		tlsConfig.InsecureSkipVerify = config.SkipCertVerify
		// Make the cipher suites less restrictive as we cannot control what cipher
		// suites asset servers support
		tlsConfig.CipherSuites = nil
	} else {
		tlsConfig = &tls.Config{
			RootCAs:            caCertPool,
			InsecureSkipVerify: config.SkipCertVerify,
			MinVersion:         tls.VersionTLS12,
		}
	}

	return tlsConfig, nil
}

func CredManagerFromConfig(logger lager.Logger, metronClient loggingclient.IngressClient, config ExecutorConfig, clock clock.Clock, handlers ...containerstore.CredentialHandler) (containerstore.CredManager, error) {
	if config.InstanceIdentityCredDir != "" {
		logger.Info("instance-identity-enabled")
		keyData, err := os.ReadFile(config.InstanceIdentityPrivateKeyPath)
		if err != nil {
			return nil, err
		}
		keyBlock, _ := pem.Decode(keyData)
		if keyBlock == nil {
			return nil, errors.New("instance ID key is not PEM-encoded")
		}
		privateKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, err
		}

		certData, err := os.ReadFile(config.InstanceIdentityCAPath)
		if err != nil {
			return nil, err
		}
		certBlock, _ := pem.Decode(certData)
		if certBlock == nil {
			return nil, errors.New("instance ID CA is not PEM-encoded")
		}
		certs, err := x509.ParseCertificates(certBlock.Bytes)
		if err != nil {
			return nil, err
		}

		if config.InstanceIdentityValidityPeriod <= 0 {
			return nil, errors.New("instance ID validity period needs to be set and positive")
		}

		return containerstore.NewCredManager(
			logger,
			metronClient,
			time.Duration(config.InstanceIdentityValidityPeriod),
			rand.Reader,
			clock,
			certs[0],
			privateKey,
			handlers,
		), nil
	}

	logger.Info("instance-identity-disabled")
	return containerstore.NewNoopCredManager(), nil
}

func (config *ExecutorConfig) Validate(logger lager.Logger) bool {
	valid := true

	if config.ContainerMaxCpuShares == 0 {
		logger.Error("max-cpu-shares-invalid", nil)
		valid = false
	}

	if config.HealthyMonitoringInterval <= 0 {
		logger.Error("healthy-monitoring-interval-invalid", nil)
		valid = false
	}

	if config.UnhealthyMonitoringInterval <= 0 {
		logger.Error("unhealthy-monitoring-interval-invalid", nil)
		valid = false
	}

	if config.GardenHealthcheckInterval <= 0 {
		logger.Error("garden-healthcheck-interval-invalid", nil)
		valid = false
	}

	if config.GardenHealthcheckProcessUser == "" {
		logger.Error("garden-healthcheck-process-user-invalid", nil)
		valid = false
	}

	if config.GardenHealthcheckProcessPath == "" {
		logger.Error("garden-healthcheck-process-path-invalid", nil)
		valid = false
	}

	if config.PostSetupHook != "" && config.PostSetupUser == "" {
		logger.Error("post-setup-hook-requires-a-user", nil)
		valid = false
	}

	if time.Duration(config.DeclarativeHealthCheckDefaultTimeout) <= 0 {
		logger.Error("declarative_healthcheck_default_timeout", nil)
		valid = false
	}

	return valid
}

func appendCACerts(caCertPool *x509.CertPool, pathToCA string) (*x509.CertPool, error) {
	certBytes, err := os.ReadFile(pathToCA)
	if err != nil {
		return nil, fmt.Errorf("Unable to open CA cert bundle '%s'", pathToCA)
	}

	certBytes = bytes.TrimSpace(certBytes)

	if len(certBytes) > 0 {
		if ok := caCertPool.AppendCertsFromPEM(certBytes); !ok {
			return nil, errors.New("unable to load CA certificate")
		}
	}

	return caCertPool, nil
}
