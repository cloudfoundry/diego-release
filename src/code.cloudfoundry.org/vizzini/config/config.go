package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type VizziniConfig struct {
	BBSAddress                     string   `json:"bbs_address"`
	BBSClientCertPath              string   `json:"bbs_client_cert_path"`
	BBSClientKeyPath               string   `json:"bbs_client_key_path"`
	SSHAddress                     string   `json:"ssh_address"`
	SSHPassword                    string   `json:"ssh_password"`
	RoutableDomainSuffix           string   `json:"routable_domain_suffix"`
	HostAddress                    string   `json:"host_addresss"`
	EnableContainerProxyTests      bool     `json:"enable_container_proxy_tests"`
	ProxyCAPath                    string   `json:"proxy_ca_path"`
	ProxyClientCertPath            string   `json:"proxy_client_cert_path"`
	ProxyClientKeyPath             string   `json:"proxy_client_key_path"`
	EnablePrivilegedContainerTests bool     `json:"enable_privileged_container_tests"`
	RepPlacementTags               []string `json:"rep_placement_tags"`
	MaxTaskRetries                 int      `json:"max_task_retries"`
	DefaultRootFS                  string   `json:"default_rootfs"`
	GraceTarballURL                string   `json:"grace_tarball_url"`
	GraceTarballChecksum           string   `json:"grace_tarball_checksum"`
	GraceBusyboxImageURL           string   `json:"grace_busybox_image_url"`
	DiegoDockerOCIImageURL         string   `json:"diego_docker_oci_image_url"`
	FileServerAddress              string   `json:"file_server_address"`
}

func NewVizziniConfig() (VizziniConfig, error) {
	configPath, ok := os.LookupEnv("VIZZINI_CONFIG_PATH")
	if !ok {
		return VizziniConfig{}, fmt.Errorf("error loading Vizzini config: VIZZINI_CONFIG_PATH env var not set")
	}

	configFile, err := os.Open(configPath)
	if err != nil {
		return VizziniConfig{}, fmt.Errorf("error loading Vizzini config: %s", err.Error())
	}
	defer configFile.Close()

	vizziniConfig := VizziniConfig{}
	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&vizziniConfig)
	if err != nil {
		return VizziniConfig{}, fmt.Errorf("error unmarshalling Vizzini config: %s", err.Error())
	}

	return vizziniConfig, nil
}
