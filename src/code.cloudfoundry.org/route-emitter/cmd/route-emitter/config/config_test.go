package config_test

import (
	"os"
	"time"

	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/debugserver"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/durationjson"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"code.cloudfoundry.org/locket"
	"code.cloudfoundry.org/route-emitter/cmd/route-emitter/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var configPath, configData string

	BeforeEach(func() {
		configData = `{
			"healthcheck_address": "127.0.0.1:8090",
			"cell_id": "cellID",
			"uuid": "bosh-boshy-bosh-bosh",
			"communication_timeout":"2s",
			"sync_interval": "4s",
			"bbs_address": "1.1.1.1:9091",
			"bbs_ca_cert_file": "/tmp/bbs_ca_cert",
			"bbs_client_cert_file": "/tmp/bbs_client_cert",
			"bbs_client_key_file": "/tmp/bbs_client_key",
			"bbs_client_session_cache_size": 100,
			"bbs_max_idle_conns_per_host": 10,
			"route_emitting_workers": 18,
			"jitter_factor": 0.3,
			"nats_addresses": "http://127.0.0.2:4222",
			"nats_username": "user",
			"nats_password": "password",
			"nats_tls_enabled": true,
			"nats_ca_cert_file": "/tmp/nats_ca_cert",
			"nats_client_cert_file": "/tmp/nats_client_cert",
			"nats_client_key_file": "/tmp/nats_client_key",
			"lock_retry_interval": "15s",
			"lock_ttl": "20s",
			"tcp_route_ttl": "2m",
			"tcp_enable_tls": true,
			"log_level": "debug",
			"debug_address": "127.0.0.1:9999",
			"enable_tcp_emitter": true,
			"enable_internal_emitter": true,
			"register_direct_instance_routes": true,
			"routing_api": {
				"url": "https://routing-api.cf.service.internal",
				"port": 443,
				"ca_cert_file": "/tmp/routing_api_ca_cert_file",
				"client_cert_file": "/tmp/routing_api_client_cert_file",
				"client_key_file": "/tmp/routing_api_client_key_file"
			},
			"locket_enabled": true,
			"locket_address": "127.0.0.1:18018",
			"locket_ca_cert_file": "locket-ca-cert",
			"report_interval": "1m",
			"locket_client_cert_file": "locket-client-cert",
			"locket_client_key_file": "locket-client-key",
			"oauth": {
				"uaa_url": "https://uaa.cf.service.internal:8443",
				"uaa_request_timeout": "4s",
				"client_name": "someclient",
				"client_secret": "somesecret",
				"ca_certs": "some-cert",
				"skip_cert_verify": true
			},
			"loggregator": {
			  "loggregator_use_v2_api": true,
			  "loggregator_api_port": 1234,
			  "loggregator_ca_path": "/var/ca_cert",
			  "loggregator_cert_path": "/var/cert_path",
			  "loggregator_key_path": "/var/key_path",
				"loggregator_job_deployment": "job1",
				"loggregator_job_name": "myjob",
				"loggregator_job_index": "1",
				"loggregator_job_ip": "1.1.1.1",
				"loggregator_job_origin": "Earth"
		}
	}`
	})

	JustBeforeEach(func() {
		configFile, err := os.CreateTemp("", "route-emitter-config")
		Expect(err).NotTo(HaveOccurred())

		defer configFile.Close()

		configPath = configFile.Name()

		n, err := configFile.WriteString(configData)
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(len(configData)))
	})

	AfterEach(func() {
		err := os.RemoveAll(configPath)
		Expect(err).NotTo(HaveOccurred())
	})

	It("correctly parses the config file", func() {
		routeEmitterConfig, err := config.NewRouteEmitterConfig(configPath)
		Expect(err).NotTo(HaveOccurred())

		expectedConfig := config.RouteEmitterConfig{
			HealthCheckAddress:           "127.0.0.1:8090",
			CellID:                       "cellID",
			UUID:                         "bosh-boshy-bosh-bosh",
			CommunicationTimeout:         durationjson.Duration(2 * time.Second),
			SyncInterval:                 durationjson.Duration(4 * time.Second),
			BBSAddress:                   "1.1.1.1:9091",
			BBSCACertFile:                "/tmp/bbs_ca_cert",
			BBSClientCertFile:            "/tmp/bbs_client_cert",
			BBSClientKeyFile:             "/tmp/bbs_client_key",
			BBSClientSessionCacheSize:    100,
			BBSMaxIdleConnsPerHost:       10,
			JitterFactor:                 0.3,
			NATSAddresses:                "http://127.0.0.2:4222",
			NATSUsername:                 "user",
			NATSPassword:                 "password",
			NATSTLSEnabled:               true,
			NATSCACertFile:               "/tmp/nats_ca_cert",
			NATSClientCertFile:           "/tmp/nats_client_cert",
			NATSClientKeyFile:            "/tmp/nats_client_key",
			LockRetryInterval:            durationjson.Duration(15 * time.Second),
			LockTTL:                      durationjson.Duration(20 * time.Second),
			RouteEmittingWorkers:         18,
			TCPRouteTTL:                  durationjson.Duration(2 * time.Minute),
			TCPEnableTLS:                 true,
			ReportInterval:               durationjson.Duration(1 * time.Minute),
			EnableTCPEmitter:             true,
			EnableInternalEmitter:        true,
			RegisterDirectInstanceRoutes: true,
			LocketEnabled:                true,
			RoutingAPI: config.RoutingAPIConfig{
				URL:            "https://routing-api.cf.service.internal",
				Port:           443,
				CACertFile:     "/tmp/routing_api_ca_cert_file",
				ClientCertFile: "/tmp/routing_api_client_cert_file",
				ClientKeyFile:  "/tmp/routing_api_client_key_file",
			},
			DebugServerConfig: debugserver.DebugServerConfig{
				DebugAddress: "127.0.0.1:9999",
			},
			LagerConfig: lagerflags.LagerConfig{
				LogLevel: "debug",
			},
			OAuth: config.OAuthConfig{
				UaaURL:            "https://uaa.cf.service.internal:8443",
				UaaRequestTimeout: durationjson.Duration(time.Second * 4),
				ClientName:        "someclient",
				ClientSecret:      "somesecret",
				CACerts:           "some-cert",
				SkipCertVerify:    true,
			},
			LoggregatorConfig: loggingclient.Config{
				UseV2API:      true,
				APIPort:       1234,
				CACertPath:    "/var/ca_cert",
				CertPath:      "/var/cert_path",
				KeyPath:       "/var/key_path",
				JobDeployment: "job1",
				JobName:       "myjob",
				JobIndex:      "1",
				JobIP:         "1.1.1.1",
				JobOrigin:     "Earth",
			},
			ClientLocketConfig: locket.ClientLocketConfig{
				LocketAddress:        "127.0.0.1:18018",
				LocketCACertFile:     "locket-ca-cert",
				LocketClientCertFile: "locket-client-cert",
				LocketClientKeyFile:  "locket-client-key",
			},
		}

		Expect(routeEmitterConfig).To(test_helpers.DeepEqual(expectedConfig))
	})

	Context("when the file does not exist", func() {
		It("returns an error", func() {
			_, err := config.NewRouteEmitterConfig("foobar")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the file does not contain valid json", func() {
		BeforeEach(func() {
			configData = "{{"
		})

		It("returns an error", func() {
			_, err := config.NewRouteEmitterConfig(configPath)
			Expect(err).To(HaveOccurred())
		})
	})
})
