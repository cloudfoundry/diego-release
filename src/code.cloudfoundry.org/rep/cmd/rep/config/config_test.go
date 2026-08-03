package config_test

import (
	"os"
	"time"

	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/debugserver"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/durationjson"
	executorinit "code.cloudfoundry.org/executor/initializer"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/rep/cmd/rep/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RepConfig", func() {
	var configFilePath, configData string

	BeforeEach(func() {
		configData = `{
			"proxy_memory_allocation_mb": 6,
			"proxy_enable_http2": true,
			"advertise_domain": "test-domain",
			"bbs_address": "1.1.1.1:9091",
			"bbs_client_session_cache_size": 100,
			"bbs_max_idle_conns_per_host": 10,
			"ca_cert_file": "/tmp/ca_cert",
			"cache_path": "/tmp/cache",
			"cell_id" : "cell_z1/10",
			"cell_index": 10,
			"communication_timeout": "11s",
			"container_inode_limit": 1000,
			"container_max_cpu_shares": 4,
			"container_metrics_report_interval": "16s",
			"container_owner_name": "vcap",
			"container_reap_interval": "11s",
			"create_work_pool_size": 15,
			"debug_address": "5.5.5.5:9090",
			"delete_work_pool_size": 10,
			"disk_mb": "20000",
			"declarative_healthcheck_path": "/var/vcap/packages/healthcheck",
			"enable_legacy_api_endpoints": true,
			"evacuation_polling_interval" : "13s",
			"evacuation_timeout" : "12s",
			"enable_container_proxy": true,
			"container_proxy_ads_addresses": ["10.0.0.2:15010", "10.0.0.3:15010"],
			"enable_unproxied_port_mappings": true,
			"envoy_config_refresh_delay": "1s",
			"envoy_config_reload_duration": "5s",
			"envoy_drain_timeout": "15m",
			"extra_root_fs_dir": "/var/vcap/data/rootfses",
			"garden_addr": "100.0.0.1",
			"garden_healthcheck_command_retry_pause": "15s",
			"garden_healthcheck_emission_interval": "13s",
			"garden_healthcheck_interval": "12s",
			"garden_healthcheck_process_args": ["arg1", "arg2"],
			"garden_healthcheck_process_dir": "/tmp/process",
			"garden_healthcheck_process_env": ["env1", "env2"],
			"garden_healthcheck_process_path": "/tmp/healthcheck-process",
			"garden_healthcheck_process_user": "vcap_health",
			"garden_healthcheck_timeout": "14s",
			"garden_network": "test-network",
			"graceful_shutdown_interval": "10s",
			"healthcheck_container_owner_name": "vcap_health",
			"healthcheck_work_pool_size": 10,
			"healthy_monitoring_interval": "5s",
			"healthy_monitoring_interval": "5s",
			"layering_mode": "single-layer",
			"listen_addr": "0.0.0.0:8080",
			"listen_addr_admin": "0.0.0.1:8081",
			"listen_addr_securable": "0.0.0.0:8081",
			"lock_retry_interval": "5s",
			"lock_ttl": "5s",
			"cell_registrations_locket_enabled": true,
			"locket_address": "0.0.0.0:909090909",
			"locket_ca_cert_file": "locket-ca-cert",
			"locket_client_cert_file": "locket-client-cert",
			"locket_client_key_file": "locket-client-key",
			"log_level": "debug",
			"loggregator": {
				"loggregator_api_port": 1234,
				"loggregator_ca_path": "ca-path",
				"loggregator_cert_path": "cert-path",
				"loggregator_key_path": "key-path",
				"loggregator_job_deployment": "job-deployment",
				"loggregator_job_name": "job-name",
				"loggregator_job_index": "job-index",
				"loggregator_job_ip": "job-ip",
				"loggregator_job_origin": "job-origin"
			},
			"log_rate_limit_exceeded_report_interval": "5m",
			"max_cache_size_in_bytes": 101,
			"max_concurrent_downloads": 11,
			"max_log_lines_per_second": 200,
			"memory_mb": "1000",
			"metrics_work_pool_size": 5,
			"optional_placement_tags": ["otag1", "otag2"],
			"path_to_ca_certs_for_downloads": "/tmp/ca-certs",
			"placement_tags": ["tag1", "tag2"],
			"polling_interval": "10s",
			"post_setup_hook": "post_setup_hook",
			"post_setup_user": "post_setup_user",
			"preloaded_root_fs": ["test:value", "test2:value2"],
			"read_work_pool_size": 15,
			"rep_url": "https://custom-rep-url:8443",
			"reserved_expiration_time": "10s",
			"sidecar_root_fs_path": "/var/vcap/packages/cflinuxfs4/rootfs.tar",
			"sidecar_root_fs": "cflinuxfs4",
			"cert_file": "/tmp/server_cert",
			"key_file": "/tmp/server_key",
			"session_name": "test",
			"skip_cert_verify": true,
			"supported_providers": ["provider1", "provider2"],
			"temp_dir": "/tmp/test",
			"trusted_system_certificates_path": "/tmp/trusted",
			"unhealthy_monitoring_interval": "10s",
			"volman_driver_paths": "/tmp/volman1:/tmp/volman2",
			"zone": "test-zone",
			"report_interval": "2m",
			"disk_health_check_paths": ["/var/vcap/data/rep", "/var/vcap/store"],
			"disk_health_check_interval": "15s",
			"disk_health_check_failure_threshold": 3
		}`
	})

	JustBeforeEach(func() {
		configFile, err := os.CreateTemp("", "config-file")
		Expect(err).NotTo(HaveOccurred())

		defer configFile.Close()

		n, err := configFile.WriteString(configData)
		Expect(err).NotTo(HaveOccurred())

		Expect(n).To(Equal(len(configData)))

		configFilePath = configFile.Name()
	})

	AfterEach(func() {
		err := os.RemoveAll(configFilePath)
		Expect(err).NotTo(HaveOccurred())
	})

	It("correctly parses the config file", func() {
		repConfig, err := config.NewRepConfig(configFilePath)
		Expect(err).NotTo(HaveOccurred())

		Expect(repConfig).To(test_helpers.DeepEqual(config.RepConfig{
			AdvertiseDomain:           "test-domain",
			BBSAddress:                "1.1.1.1:9091",
			BBSClientSessionCacheSize: 100,
			BBSMaxIdleConnsPerHost:    10,
			CaCertFile:                "/tmp/ca_cert",
			CellID:                    "cell_z1/10",
			CellIndex:                 10,
			ClientLocketConfig: locket.ClientLocketConfig{
				LocketAddress:        "0.0.0.0:909090909",
				LocketCACertFile:     "locket-ca-cert",
				LocketClientCertFile: "locket-client-cert",
				LocketClientKeyFile:  "locket-client-key",
			},
			CommunicationTimeout: durationjson.Duration(11 * time.Second),
			DebugServerConfig: debugserver.DebugServerConfig{
				DebugAddress: "5.5.5.5:9090",
			},
			EvacuationPollingInterval: durationjson.Duration(13 * time.Second),
			EvacuationTimeout:         durationjson.Duration(12 * time.Second),
			ExecutorConfig: executorinit.ExecutorConfig{
				ProxyMemoryAllocationMB:            6,
				ProxyEnableHttp2:                   true,
				CachePath:                          "/tmp/cache",
				ContainerInodeLimit:                1000,
				ContainerMaxCpuShares:              4,
				ContainerMetricsReportInterval:     16000000000,
				ContainerOwnerName:                 "vcap",
				ContainerReapInterval:              11000000000,
				CreateWorkPoolSize:                 15,
				DeleteWorkPoolSize:                 10,
				DiskMB:                             "20000",
				DeclarativeHealthcheckPath:         "/var/vcap/packages/healthcheck",
				EnableContainerProxy:               true,
				ContainerProxyADSServers:           []string{"10.0.0.2:15010", "10.0.0.3:15010"},
				EnableUnproxiedPortMappings:        true,
				EnvoyConfigRefreshDelay:            durationjson.Duration(time.Second),
				EnvoyConfigReloadDuration:          durationjson.Duration(5 * time.Second),
				EnvoyDrainTimeout:                  durationjson.Duration(15 * time.Minute),
				GardenAddr:                         "100.0.0.1",
				GardenHealthcheckCommandRetryPause: 15000000000,
				GardenHealthcheckEmissionInterval:  13000000000,
				GardenHealthcheckInterval:          12000000000,
				GardenHealthcheckProcessArgs:       []string{"arg1", "arg2"},
				GardenHealthcheckProcessDir:        "/tmp/process",
				GardenHealthcheckProcessEnv:        []string{"env1", "env2"},
				GardenHealthcheckProcessPath:       "/tmp/healthcheck-process",
				GardenHealthcheckProcessUser:       "vcap_health",
				GardenHealthcheckTimeout:           14000000000,
				GardenNetwork:                      "test-network",
				GracefulShutdownInterval:           durationjson.Duration(10 * time.Second),
				HealthCheckContainerOwnerName:      "vcap_health",
				HealthCheckWorkPoolSize:            10,
				HealthyMonitoringInterval:          5000000000,
				MaxCacheSizeInBytes:                101,
				MaxConcurrentDownloads:             11,
				MaxLogLinesPerSecond:               200,
				MemoryMB:                           "1000",
				MetricsWorkPoolSize:                5,
				PathToCACertsForDownloads:          "/tmp/ca-certs",
				PostSetupHook:                      "post_setup_hook",
				PostSetupUser:                      "post_setup_user",
				ReadWorkPoolSize:                   15,
				ReservedExpirationTime:             10000000000,
				SkipCertVerify:                     true,
				TempDir:                            "/tmp/test",
				TrustedSystemCertificatesPath:      "/tmp/trusted",
				UnhealthyMonitoringInterval:        10000000000,
				VolmanDriverPaths:                  "/tmp/volman1:/tmp/volman2",
			},
			LagerConfig: lagerflags.LagerConfig{
				LogLevel: lagerflags.DEBUG,
			},
			LayeringMode:                    "single-layer",
			ListenAddr:                      "0.0.0.0:8080",
			ListenAddrSecurable:             "0.0.0.0:8081",
			LockRetryInterval:               durationjson.Duration(5 * time.Second),
			LockTTL:                         durationjson.Duration(5 * time.Second),
			OptionalPlacementTags:           []string{"otag1", "otag2"},
			PlacementTags:                   []string{"tag1", "tag2"},
			PollingInterval:                 durationjson.Duration(10 * time.Second),
			PreloadedRootFS:                 []config.RootFS{{"test", "value"}, {"test2", "value2"}},
			RepURL:                          "https://custom-rep-url:8443",
			ExtraRootfsDir:                  "/var/vcap/data/rootfses",
			SidecarRootFSPath:               "/var/vcap/packages/cflinuxfs4/rootfs.tar",
			SidecarRootFS:                   "cflinuxfs4",
			CertFile:                        "/tmp/server_cert",
			KeyFile:                         "/tmp/server_key",
			SessionName:                     "test",
			SupportedProviders:              []string{"provider1", "provider2"},
			Zone:                            "test-zone",
			ReportInterval:                  durationjson.Duration(2 * time.Minute),
			DiskHealthCheckPaths:            []string{"/var/vcap/data/rep", "/var/vcap/store"},
			DiskHealthCheckInterval:         durationjson.Duration(15 * time.Second),
			DiskHealthCheckFailureThreshold: 3,
			LoggregatorConfig: loggingclient.Config{
				APIPort:       1234,
				CACertPath:    "ca-path",
				CertPath:      "cert-path",
				KeyPath:       "key-path",
				JobDeployment: "job-deployment",
				JobName:       "job-name",
				JobIndex:      "job-index",
				JobIP:         "job-ip",
				JobOrigin:     "job-origin",
			},
		}))
	})

	Context("when the file does not exist", func() {
		It("returns an error", func() {
			_, err := config.NewRepConfig("foobar")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the file does not contain valid json", func() {
		BeforeEach(func() {
			configData = "{{"
		})

		It("returns an error", func() {
			_, err := config.NewRepConfig(configFilePath)
			Expect(err).To(HaveOccurred())
		})

		Context("because the communication_timeout is not valid", func() {
			BeforeEach(func() {
				configData = `{"communication_timeout": 4234342342}`
			})

			It("returns an error", func() {
				_, err := config.NewRepConfig(configFilePath)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("when rep_url is not provided in config", func() {
		BeforeEach(func() {
			configData = `{
				"advertise_domain": "test-domain",
				"bbs_address": "1.1.1.1:9091",
				"ca_cert_file": "/tmp/ca_cert",
				"cell_id" : "cell_z1/10",
				"listen_addr_securable": "0.0.0.0:8081",
				"preloaded_root_fs": ["test:value"],
				"supported_providers": ["provider1"]
			}`
		})

		It("parses config without rep_url field", func() {
			repConfig, err := config.NewRepConfig(configFilePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(repConfig.RepURL).To(BeEmpty())
		})
	})

	Context("when rep_url is empty in config", func() {
		BeforeEach(func() {
			configData = `{
				"advertise_domain": "test-domain",
				"bbs_address": "1.1.1.1:9091",
				"ca_cert_file": "/tmp/ca_cert",
				"cell_id" : "cell_z1/10",
				"listen_addr_securable": "0.0.0.0:8081",
				"preloaded_root_fs": ["test:value"],
				"rep_url": "",
				"supported_providers": ["provider1"]
			}`
		})

		It("parses config with empty rep_url field", func() {
			repConfig, err := config.NewRepConfig(configFilePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(repConfig.RepURL).To(BeEmpty())
		})
	})
})
