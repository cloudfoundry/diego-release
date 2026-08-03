package voldiscoverers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"net/url"

	"code.cloudfoundry.org/dockerdriver"
	"code.cloudfoundry.org/dockerdriver/driverhttp"
	"code.cloudfoundry.org/goshims/osshim"
	"code.cloudfoundry.org/lager/v3"
)

//go:generate counterfeiter -o ../volmanfakes/fake_docker_driver_factory.go . DockerDriverFactory

// DriverFactories are responsible for instantiating remote client implementations of the dockerdriver.Driver interface.
type DockerDriverFactory interface {
	// Given a driver id, path and config filename returns a remote client implementation of the dockerdriver.Driver interface
	DockerDriver(logger lager.Logger, driverId string, driverPath, driverFileName string) (dockerdriver.Driver, error)
}

type dockerDriverFactory struct {
	Factory driverhttp.RemoteClientFactory
	useOs   osshim.Os
}

func NewDockerDriverFactory() DockerDriverFactory {
	remoteClientFactory := driverhttp.NewRemoteClientFactory()
	return NewDockerDriverFactoryWithRemoteClientFactory(remoteClientFactory)
}

func NewDockerDriverFactoryWithRemoteClientFactory(remoteClientFactory driverhttp.RemoteClientFactory) DockerDriverFactory {
	return &dockerDriverFactory{remoteClientFactory, &osshim.OsShim{}}
}

func NewDockerDriverFactoryWithOs(useOs osshim.Os) DockerDriverFactory {
	remoteClientFactory := driverhttp.NewRemoteClientFactory()
	return &dockerDriverFactory{remoteClientFactory, useOs}
}

func (r *dockerDriverFactory) DockerDriver(logger lager.Logger, driverId string, driverPath string, driverFileName string) (dockerdriver.Driver, error) {
	logger = logger.Session("driver", lager.Data{"driverId": driverId, "driverFileName": driverFileName})
	logger.Info("start")
	defer logger.Info("end")

	var address string
	var tls *dockerdriver.TLSConfig
	if strings.Contains(driverFileName, ".") {
		extension := strings.Split(driverFileName, ".")[1]
		switch extension {
		case "sock":
			address = path.Join(driverPath, driverFileName)
		case "spec":
			configFile, err := r.useOs.Open(path.Join(driverPath, driverFileName))
			if err != nil {
				logger.Error("error-opening-config", err, lager.Data{"DriverFileName": driverFileName})
				return nil, err
			}
			reader := bufio.NewReader(configFile)
			addressBytes, _, err := reader.ReadLine()
			if err != nil { // no real value in faking this as bigger problems exist when this fails
				logger.Error("error-reading-driver-file", err, lager.Data{"DriverFileName": driverFileName})
				return nil, err
			}
			address = string(addressBytes)
		case "json":
			// extract url from json file
			var driverJsonSpec dockerdriver.DriverSpec
			configFile, err := r.useOs.Open(path.Join(driverPath, driverFileName))
			if err != nil {
				logger.Error("error-opening-config", err, lager.Data{"DriverFileName": driverFileName})
				return nil, err
			}
			jsonParser := json.NewDecoder(configFile)
			if err = jsonParser.Decode(&driverJsonSpec); err != nil {
				logger.Error("parsing-config-file-error", err)
				return nil, err
			}
			address = driverJsonSpec.Address
			tls = driverJsonSpec.TLSConfig
		default:
			err := fmt.Errorf("unknown-driver-extension: %s", extension)
			logger.Error("driver", err)
			return nil, err

		}
		var err error

		address, err = r.canonicalize(logger, address)
		if err != nil {
			logger.Error("invalid-address", err, lager.Data{"address": address})
			return nil, err
		}

		logger.Info("getting-driver", lager.Data{"address": address})
		driver, err := r.Factory.NewRemoteClient(address, tls)
		if err != nil {
			logger.Error("error-building-driver", err, lager.Data{"address": address})
			return nil, err
		}

		return driver, nil
	}

	return nil, fmt.Errorf("Driver '%s' not found in list of known drivers", driverId)
}

func (r *dockerDriverFactory) canonicalize(logger lager.Logger, address string) (string, error) {
	logger = logger.Session("canonicalize", lager.Data{"address": address})
	logger.Debug("start")
	defer logger.Debug("end")

	url, err := url.Parse(address)
	if err != nil {
		return address, err
	}

	switch url.Scheme {
	case "http", "https":
		return address, nil
	case "tcp":
		return fmt.Sprintf("http://%s%s", url.Host, url.Path), nil
	case "unix":
		return address, nil
	default:
		if strings.HasSuffix(url.Path, ".sock") {
			return fmt.Sprintf("%s%s", url.Host, url.Path), nil
		}
	}
	return fmt.Sprintf("http://%s", address), nil
}

func driverImplements(protocol string, activateResponseProtocols []string) bool {
	for _, nextProtocol := range activateResponseProtocols {
		if protocol == nextProtocol {
			return true
		}
	}
	return false
}
