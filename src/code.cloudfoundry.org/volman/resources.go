package volman

import (
	"code.cloudfoundry.org/lager/v3"
)

//go:generate counterfeiter -o volmanfakes/fake_plugin.go . Plugin
type Plugin interface {
	ListVolumes(logger lager.Logger) ([]string, error)
	Mount(logger lager.Logger, volumeId string, config map[string]interface{}) (MountResponse, error)
	Unmount(logger lager.Logger, volumeId string) error
	Matches(lager.Logger, PluginSpec) bool
	GetPluginSpec() PluginSpec
}

//go:generate counterfeiter -o volmanfakes/fake_discoverer.go . Discoverer
type Discoverer interface {
	Discover(logger lager.Logger) (map[string]Plugin, error)
}

type ListDriversResponse struct {
	Drivers []InfoResponse `json:"drivers"`
}

type MountRequest struct {
	DriverId string                 `json:"driverId"`
	VolumeId string                 `json:"volumeId"`
	Config   map[string]interface{} `json:"config"`
}

type MountResponse struct {
	Path string `json:"path"`
}

type InfoResponse struct {
	Name string `json:"name"`
}

type UnmountRequest struct {
	DriverId string `json:"driverId"`
	VolumeId string `json:"volumeId"`
}

type PluginSpec struct {
	Name            string     `json:"Name"`
	Address         string     `json:"Addr"`
	TLSConfig       *TLSConfig `json:"TLSConfig"`
	UniqueVolumeIds bool
}

type TLSConfig struct {
	InsecureSkipVerify bool   `json:"InsecureSkipVerify"`
	CAFile             string `json:"CAFile"`
	CertFile           string `json:"CertFile"`
	KeyFile            string `json:"KeyFile"`
}

type PluginRegistry interface {
	Plugin(id string) (Plugin, bool)
	Plugins() map[string]Plugin
	Set(plugins map[string]Plugin)
	Keys() []string
}

type SafeError struct {
	SafeDescription string `json:"SafeDescription"`
}

func (s SafeError) Error() string {
	return s.SafeDescription
}
