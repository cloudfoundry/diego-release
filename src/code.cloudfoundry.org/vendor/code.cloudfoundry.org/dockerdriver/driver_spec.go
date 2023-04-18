package dockerdriver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

	"code.cloudfoundry.org/lager/v3"
)

func WriteDriverSpec(logger lager.Logger, pluginsDirectory string, driver string, extension string, contents []byte) error {
	err := os.MkdirAll(pluginsDirectory, 0666)
	if err != nil {
		logger.Error("error-creating-directory", err)
		return err
	}

	f, err := os.Create(path.Join(pluginsDirectory, driver+"."+extension))
	if err != nil {
		logger.Error("error-creating-file ", err)
		return err
	}
	defer f.Close()
	_, err = f.Write(contents)
	if err != nil {
		logger.Error("error-writing-file ", err)
		return err
	}
	f.Sync()
	return nil
}

func ReadDriverSpec(logger lager.Logger, specName string, driverPath string, specFile string) (*DriverSpec, error) {
	logger = logger.Session("read-driver-spec", lager.Data{"spec-name": specName, "spec-file": specFile})
	logger.Info("start")
	defer logger.Info("end")

	var driverSpec DriverSpec

	if strings.Contains(specFile, ".") {
		extension := strings.Split(specFile, ".")[1]
		switch extension {
		case "sock":
			driverSpec = DriverSpec{
				Name:    specName,
				Address: path.Join(driverPath, specFile),
			}
		case "spec":
			configFile, err := os.Open(path.Join(driverPath, specFile))
			if err != nil {
				logger.Error("error-opening-config", err, lager.Data{"DriverFileName": specFile})
				return nil, err
			}
			reader := bufio.NewReader(configFile)
			addressBytes, _, err := reader.ReadLine()
			if err != nil { // no real value in faking this as bigger problems exist when this fails
				logger.Error("error-reading-driver-file", err, lager.Data{"DriverFileName": specFile})
				return nil, err
			}
			driverSpec = DriverSpec{
				Name:    specName,
				Address: string(addressBytes),
			}
		case "json":
			// extract url from json file
			var driverJsonSpec DriverSpec
			configFile, err := os.Open(path.Join(driverPath, specFile))
			if err != nil {
				logger.Error("error-opening-config", err, lager.Data{"DriverFileName": specFile})
				return nil, err
			}
			jsonParser := json.NewDecoder(configFile)
			if err = jsonParser.Decode(&driverJsonSpec); err != nil {
				logger.Error("parsing-config-file-error", err)
				return nil, err
			}
			driverSpec = DriverSpec{
				Name:            specName,
				Address:         driverJsonSpec.Address,
				TLSConfig:       driverJsonSpec.TLSConfig,
				UniqueVolumeIds: driverJsonSpec.UniqueVolumeIds,
			}
		default:
			err := fmt.Errorf("unknown-driver-extension: %s", extension)
			logger.Error("driver", err)
			return nil, err
		}
	}

	return &driverSpec, nil
}
