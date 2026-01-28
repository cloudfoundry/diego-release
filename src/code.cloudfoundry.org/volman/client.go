package volman

import "code.cloudfoundry.org/lager/v3"

//go:generate counterfeiter -o volmanfakes/fake_manager_client.go . Manager

type Manager interface {
	ListDrivers(logger lager.Logger) (ListDriversResponse, error)
	Mount(logger lager.Logger, driverId string, volumeId string, containerId string, config map[string]interface{}) (MountResponse, error)
	Unmount(logger lager.Logger, driverId string, volumeId string, containerId string) error
}
