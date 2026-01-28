package runners

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"gopkg.in/yaml.v2"

	routing_api "code.cloudfoundry.org/routing-api"
	apiconfig "code.cloudfoundry.org/routing-api/config"
	"code.cloudfoundry.org/routing-api/models"
	"code.cloudfoundry.org/tlsconfig"
	ginkgomon "github.com/tedsuo/ifrit/ginkgomon_v2"
)

type Config struct {
	apiconfig.Config
	DevMode bool
	IP      string
}

type RoutingAPIRunner struct {
	Config              Config
	configPath, binPath string
}

type SQLConfig struct {
	Port       uint16
	DBName     string
	DriverName string
	Username   string
	Password   string
}

func NewRoutingAPIRunner(binPath string, adminPort uint16, sqlConfig SQLConfig, fs ...func(*Config)) (*RoutingAPIRunner, error) {
	cfg := Config{
		DevMode: true,
		Config: apiconfig.Config{
			AdminPort: uint16(adminPort),
			// required fields
			MetricsReportingIntervalString:  "500ms",
			StatsdClientFlushIntervalString: "10ms",
			SystemDomain:                    "example.com",
			LogGuid:                         "routing-api-logs",
			RouterGroups: models.RouterGroups{
				{
					Name:            "default-tcp",
					Type:            "tcp",
					ReservablePorts: "1024-65535",
				},
			},
			// end of required fields
			SqlDB: apiconfig.SqlDB{
				Host:     "localhost",
				Port:     uint16(sqlConfig.Port),
				Schema:   sqlConfig.DBName,
				Type:     sqlConfig.DriverName,
				Username: sqlConfig.Username,
				Password: sqlConfig.Password,
			},
			UUID: "routing-api-uuid",
		},
	}

	for _, f := range fs {
		f(&cfg)
	}

	f, err := os.CreateTemp(os.TempDir(), "routing-api-config")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	configBytes, err := yaml.Marshal(cfg.Config)
	if err != nil {
		return nil, err
	}
	_, err = f.Write(configBytes)
	if err != nil {
		return nil, err
	}

	return &RoutingAPIRunner{
		Config:     cfg,
		configPath: f.Name(),
		binPath:    binPath,
	}, nil
}

func (runner *RoutingAPIRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	// Create a new ginkgomon runner here instead in New() so that we can restart
	// the same runner without having to worry about messing the state of the
	// ginkgomon Runner
	args := []string{
		"-ip", "localhost",
		"-config", runner.configPath,
		"-logLevel=debug",
		"-devMode=" + strconv.FormatBool(runner.Config.DevMode),
	}
	r := ginkgomon.New(ginkgomon.Config{
		Name:              "routing-api",
		Command:           exec.Command(runner.binPath, args...),
		StartCheck:        "routing-api.started",
		StartCheckTimeout: 20 * time.Second,
	})
	return r.Run(signals, ready)
}

type RoutingAPIClientConfig struct {
	Port           uint16
	CACertFile     string
	ClientCertFile string
	ClientKeyFile  string
}

func NewRoutingAPIClient(config RoutingAPIClientConfig) (routing_api.Client, error) {
	if config.CACertFile != "" && config.ClientCertFile != "" && config.ClientKeyFile != "" {
		tlsConfig, err := tlsconfig.Build(
			tlsconfig.WithInternalServiceDefaults(),
			tlsconfig.WithIdentityFromFile(config.ClientCertFile, config.ClientKeyFile),
		).Client(
			tlsconfig.WithAuthorityFromFile(config.CACertFile),
		)
		if err != nil {
			return nil, err
		}
		return routing_api.NewClientWithTLSConfig(fmt.Sprintf("https://127.0.0.1:%d", config.Port), tlsConfig), nil
	}

	return routing_api.NewClient(fmt.Sprintf("http://127.0.0.1:%d", config.Port), false), nil
}
