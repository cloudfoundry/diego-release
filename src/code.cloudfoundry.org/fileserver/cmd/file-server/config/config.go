package config

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/debugserver"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager/v3/lagerflags"
)

type FileServerConfig struct {
	ServerAddress   string `json:"server_address,omitempty"`
	StaticDirectory string `json:"static_directory,omitempty"`

	HTTPSServerEnabled bool   `json:"https_server_enabled"`
	HTTPSListenAddr    string `json:"https_listen_addr"`
	CertFile           string `json:"cert_file"`
	KeyFile            string `json:"key_file"`

	LoggregatorConfig loggingclient.Config `json:"loggregator"`
	debugserver.DebugServerConfig
	lagerflags.LagerConfig
}

func NewFileServerConfig(configPath string) (FileServerConfig, error) {
	fileServerConfig := FileServerConfig{}

	configFile, err := os.Open(configPath)
	if err != nil {
		return FileServerConfig{}, err
	}

	defer configFile.Close()

	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&fileServerConfig)
	if err != nil {
		return FileServerConfig{}, err
	}

	return fileServerConfig, nil
}
