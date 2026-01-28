package config_test

import (
	"os"
	"time"

	"code.cloudfoundry.org/auctioneer/cmd/auctioneer/config"
	"code.cloudfoundry.org/debugserver"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/locket"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AuctioneerConfig", func() {
	var configFilePath, configData string

	BeforeEach(func() {
		configData = `{
			"auction_runner_workers": 10,
			"bbs_address": "1.1.1.1:9091",
			"bbs_ca_cert_file": "/tmp/bbs_ca_cert",
			"bbs_client_cert_file": "/tmp/bbs_client_cert",
			"bbs_client_key_file": "/tmp/bbs_client_key",
			"bbs_client_session_cache_size": 100,
			"bbs_max_idle_conns_per_host": 10,
			"ca_cert_file": "/path-to-cert",
			"bin_pack_first_fit_weight": 0.1,
			"cell_state_timeout": "2s",
			"communication_timeout": "15s",
			"debug_address": "127.0.0.1:17017",
			"listen_address": "0.0.0.0:9090",
			"lock_retry_interval": "1m",
			"lock_ttl": "20s",
			"locks_locket_enabled": true,
			"locket_address": "laksdjflksdajflkajsdf",
			"locket_ca_cert_file": "locket-ca-cert",
			"locket_client_cert_file": "locket-client-cert",
			"locket_client_key_file": "locket-client-key",
			"log_level": "debug",
			"loggregator": {
				"loggregator_use_v2_api": true,
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
			"rep_ca_cert": "/var/vcap/jobs/auctioneer/config/rep.ca",
			"rep_client_cert": "/var/vcap/jobs/auctioneer/config/rep.crt",
			"rep_client_key": "/var/vcap/jobs/auctioneer/config/rep.key",
			"rep_client_session_cache_size": 10,
			"report_interval": "1m",
			"server_cert_file": "/path-to-server-cert",
			"server_key_file": "/path-to-server-key",
			"starting_container_count_maximum": 10,
			"starting_container_weight": 0.5,
			"uuid": "bosh-boshy-bosh-bosh"
    }`
	})

	JustBeforeEach(func() {
		configFile, err := os.CreateTemp("", "auctioneer-config-file")
		Expect(err).NotTo(HaveOccurred())

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
		auctioneerConfig, err := config.NewAuctioneerConfig(configFilePath)
		Expect(err).NotTo(HaveOccurred())

		expectedConfig := config.AuctioneerConfig{
			AuctionRunnerWorkers:      10,
			BBSAddress:                "1.1.1.1:9091",
			BBSCACertFile:             "/tmp/bbs_ca_cert",
			BBSClientCertFile:         "/tmp/bbs_client_cert",
			BBSClientKeyFile:          "/tmp/bbs_client_key",
			BBSClientSessionCacheSize: 100,
			BBSMaxIdleConnsPerHost:    10,
			CACertFile:                "/path-to-cert",
			BinPackFirstFitWeight:     0.1,
			CellStateTimeout:          durationjson.Duration(2 * time.Second),
			ClientLocketConfig: locket.ClientLocketConfig{
				LocketAddress:        "laksdjflksdajflkajsdf",
				LocketCACertFile:     "locket-ca-cert",
				LocketClientCertFile: "locket-client-cert",
				LocketClientKeyFile:  "locket-client-key",
			},
			CommunicationTimeout: durationjson.Duration(15 * time.Second),
			DebugServerConfig: debugserver.DebugServerConfig{
				DebugAddress: "127.0.0.1:17017",
			},
			LagerConfig: lagerflags.LagerConfig{
				LogLevel: "debug",
			},
			ListenAddress:     "0.0.0.0:9090",
			LockRetryInterval: durationjson.Duration(1 * time.Minute),
			LockTTL:           durationjson.Duration(20 * time.Second),
			LoggregatorConfig: loggingclient.Config{
				UseV2API:      true,
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
			RepCACert:                     "/var/vcap/jobs/auctioneer/config/rep.ca",
			RepClientCert:                 "/var/vcap/jobs/auctioneer/config/rep.crt",
			RepClientKey:                  "/var/vcap/jobs/auctioneer/config/rep.key",
			RepClientSessionCacheSize:     10,
			ReportInterval:                durationjson.Duration(1 * time.Minute),
			ServerCertFile:                "/path-to-server-cert",
			ServerKeyFile:                 "/path-to-server-key",
			StartingContainerCountMaximum: 10,
			StartingContainerWeight:       .5,
			UUID:                          "bosh-boshy-bosh-bosh",
		}

		Expect(auctioneerConfig).To(Equal(expectedConfig))
	})

	Context("when the file does not exist", func() {
		It("returns an error", func() {
			_, err := config.NewAuctioneerConfig("foobar")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the file does not contain valid json", func() {
		BeforeEach(func() {
			configData = "{{"
		})

		It("returns an error", func() {
			_, err := config.NewAuctioneerConfig(configFilePath)
			Expect(err).To(HaveOccurred())
		})
	})
})
