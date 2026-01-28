package voldocker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"code.cloudfoundry.org/dockerdriver"
	"code.cloudfoundry.org/dockerdriver/driverhttp"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/volman"
)

type DockerDriverPlugin struct {
	DockerDriver interface{}
	PluginSpec   volman.PluginSpec
}

func NewVolmanPluginWithDockerDriver(driver dockerdriver.Driver, pluginSpec volman.PluginSpec) volman.Plugin {
	return &DockerDriverPlugin{
		DockerDriver: driver,
		PluginSpec:   pluginSpec,
	}
}

func (dw *DockerDriverPlugin) Matches(logger lager.Logger, pluginSpec volman.PluginSpec) bool {
	logger = logger.Session("matches")
	logger.Info("start")
	defer logger.Info("end")

	var matches bool
	matchableDriver, ok := dw.DockerDriver.(dockerdriver.MatchableDriver)
	logger.Info("matches", lager.Data{"is-matchable": ok})
	if ok {
		var tlsConfig *dockerdriver.TLSConfig
		if pluginSpec.TLSConfig != nil {
			tlsConfig = &dockerdriver.TLSConfig{
				InsecureSkipVerify: pluginSpec.TLSConfig.InsecureSkipVerify,
				CAFile:             pluginSpec.TLSConfig.CAFile,
				CertFile:           pluginSpec.TLSConfig.CertFile,
				KeyFile:            pluginSpec.TLSConfig.KeyFile,
			}
		}
		matches = matchableDriver.Matches(logger, pluginSpec.Address, tlsConfig)
	}
	logger.Info("matches", lager.Data{"matches": matches})
	return matches
}

func (d *DockerDriverPlugin) ListVolumes(logger lager.Logger) ([]string, error) {
	logger = logger.Session("list-volumes")
	logger.Info("start")
	defer logger.Info("end")

	volumes := []string{}
	env := driverhttp.NewHttpDriverEnv(logger, context.TODO())

	response := d.DockerDriver.(dockerdriver.Driver).List(env)
	if response.Err != "" {
		return volumes, errors.New(response.Err)
	}

	for _, volumeInfo := range response.Volumes {
		volumes = append(volumes, volumeInfo.Name)
	}

	return volumes, nil
}

func (d *DockerDriverPlugin) Mount(logger lager.Logger, volumeId string, opts map[string]interface{}) (volman.MountResponse, error) {
	logger = logger.Session("mount")
	logger.Info("start")
	defer logger.Info("end")

	env := driverhttp.NewHttpDriverEnv(logger, context.TODO())

	logger.Debug("creating-volume", lager.Data{"volumeId": volumeId})
	response := d.DockerDriver.(dockerdriver.Driver).Create(env, dockerdriver.CreateRequest{Name: volumeId, Opts: opts})
	if response.Err != "" {
		return volman.MountResponse{}, errors.New(response.Err)
	}

	mountRequest := dockerdriver.MountRequest{Name: volumeId}
	logger.Debug("calling-docker-driver-with-mount-request", lager.Data{"mountRequest": mountRequest})
	mountResponse := d.DockerDriver.(dockerdriver.Driver).Mount(env, mountRequest)
	logger.Debug("response-from-docker-driver", lager.Data{"response": mountResponse})

	if !strings.HasPrefix(mountResponse.Mountpoint, "/var/vcap/data") {
		logger.Info("invalid-mountpath", lager.Data{"detail": fmt.Sprintf("Invalid or dangerous mountpath %s outside of /var/vcap/data", mountResponse.Mountpoint)})
	}

	if mountResponse.Err != "" {
		safeError := dockerdriver.SafeError{}
		err := json.Unmarshal([]byte(mountResponse.Err), &safeError)
		if err == nil {
			return volman.MountResponse{}, safeError
		} else {
			return volman.MountResponse{}, errors.New(mountResponse.Err)
		}
	}

	return volman.MountResponse{Path: mountResponse.Mountpoint}, nil
}

func (d *DockerDriverPlugin) Unmount(logger lager.Logger, volumeId string) error {
	logger = logger.Session("unmount")
	logger.Info("start")
	defer logger.Info("end")

	env := driverhttp.NewHttpDriverEnv(logger, context.TODO())

	if response := d.DockerDriver.(dockerdriver.Driver).Unmount(env, dockerdriver.UnmountRequest{Name: volumeId}); response.Err != "" {

		safeError := dockerdriver.SafeError{}
		err := json.Unmarshal([]byte(response.Err), &safeError)
		if err == nil {
			err = safeError
		} else {
			err = errors.New(response.Err)
		}

		logger.Error("unmount-failed", err)
		return err
	}
	return nil
}

func (d *DockerDriverPlugin) GetPluginSpec() volman.PluginSpec {
	return d.PluginSpec
}
