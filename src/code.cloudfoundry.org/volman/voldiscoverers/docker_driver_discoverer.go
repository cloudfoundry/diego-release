package voldiscoverers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"code.cloudfoundry.org/dockerdriver"
	"code.cloudfoundry.org/dockerdriver/driverhttp"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/volman"
	"code.cloudfoundry.org/volman/voldocker"
)

type dockerDriverDiscoverer struct {
	logger        lager.Logger
	driverFactory DockerDriverFactory

	driverRegistry volman.PluginRegistry
	driverPaths    []string
}

func NewDockerDriverDiscoverer(logger lager.Logger, driverRegistry volman.PluginRegistry, driverPaths []string) volman.Discoverer {
	return &dockerDriverDiscoverer{
		logger:        logger,
		driverFactory: NewDockerDriverFactory(),

		driverRegistry: driverRegistry,
		driverPaths:    driverPaths,
	}
}

func NewDockerDriverDiscovererWithDriverFactory(logger lager.Logger, driverRegistry volman.PluginRegistry, driverPaths []string, factory DockerDriverFactory) volman.Discoverer {
	return &dockerDriverDiscoverer{
		logger:        logger,
		driverFactory: factory,

		driverRegistry: driverRegistry,
		driverPaths:    driverPaths,
	}
}

func (r *dockerDriverDiscoverer) Discover(logger lager.Logger) (map[string]volman.Plugin, error) {
	logger = logger.Session("discover")
	logger.Debug("start")
	logger.Info("discovering-drivers", lager.Data{"driver-paths": r.driverPaths})
	defer logger.Debug("end")

	endpoints := make(map[string]volman.Plugin)

	for _, driverPath := range r.driverPaths {
		specTypes := [3]string{"json", "spec", "sock"}
		for _, specType := range specTypes {
			matchingDriverSpecs, err := r.getMatchingDriverSpecs(logger, driverPath, specType)

			if err != nil {
				// untestable on linux, does glob work differently on windows???
				return map[string]volman.Plugin{}, fmt.Errorf("Volman configured with an invalid driver path '%s', error occured list files (%s)", driverPath, err.Error())
			}
			if len(matchingDriverSpecs) > 0 {
				logger.Debug("driver-specs", lager.Data{"drivers": matchingDriverSpecs})
				var existing map[string]volman.Plugin
				if r.driverRegistry != nil {
					existing = r.driverRegistry.Plugins()
					logger.Debug("existing-drivers", lager.Data{"len": len(existing)})
				}

				endpoints = r.findAllPlugins(logger, endpoints, driverPath, matchingDriverSpecs, existing)
				endpoints = r.activatePlugins(logger, endpoints, driverPath, matchingDriverSpecs)
			}
		}
	}
	return endpoints, nil
}

func (r *dockerDriverDiscoverer) findAllPlugins(logger lager.Logger, newPlugins map[string]volman.Plugin, driverPath string, specs []string, existingPlugins map[string]volman.Plugin) map[string]volman.Plugin {
	logger = logger.Session("insert-if-not-found")
	logger.Debug("start")
	defer logger.Debug("end")
	var plugin volman.Plugin

	for _, spec := range specs {
		validSpecName, specName, specFile := specName(logger, spec)
		if !validSpecName {
			continue
		}

		_, newPluginFound := newPlugins[specName]
		if !newPluginFound {
			pluginSpec, err := r.getPluginSpec(logger, specName, driverPath, specFile)
			if err != nil {
				continue
			}

			var existingPluginFound bool
			plugin, existingPluginFound = existingPlugins[specName]
			if !existingPluginFound || r.pluginDoesNotMatch(logger, plugin, pluginSpec) {
				plugin, err = r.createPlugin(logger, specName, driverPath, specFile, pluginSpec)
				if err != nil {
					continue
				}
			}

			logger.Info("new-plugin", lager.Data{"name": specName})
			newPlugins[specName] = plugin
		}
	}
	return newPlugins
}

func (r *dockerDriverDiscoverer) activatePlugins(logger lager.Logger, plugins map[string]volman.Plugin, driverPath string, specs []string) map[string]volman.Plugin {

	activatedPlugins := map[string]volman.Plugin{}

	for k, plugin := range plugins {
		dockerPlugin := plugin.(*voldocker.DockerDriverPlugin)
		dockerDriver := dockerPlugin.DockerDriver.(dockerdriver.Driver)
		env := driverhttp.NewHttpDriverEnv(logger, context.Background())
		resp := dockerDriver.Activate(env)
		if resp.Err == "" {
			if implementVolumeDriver(resp) {
				activatedPlugins[k] = dockerPlugin
			} else {
				logger.Error("driver-invalid", fmt.Errorf("driver-implements: %#v, expecting: VolumeDriver", resp.Implements))
			}
		} else {
			logger.Error("existing-driver-unreachable", errors.New(resp.Err), lager.Data{"spec-name": dockerPlugin.GetPluginSpec().Name, "address": dockerPlugin.GetPluginSpec().Address, "tls": dockerPlugin.GetPluginSpec().TLSConfig})

			foundSpecFile, specFile := r.findDockerSpecFileByName(logger, dockerPlugin.GetPluginSpec().Name, driverPath, specs)
			if !foundSpecFile {
				logger.Info("error-spec-file-not-found", lager.Data{"spec-name": dockerPlugin.GetPluginSpec().Name})
				continue
			}

			logger.Info("updating-driver", lager.Data{"spec-name": plugin.GetPluginSpec().Name, "driver-path": driverPath})
			driver, err := r.driverFactory.DockerDriver(logger, plugin.GetPluginSpec().Name, driverPath, specFile)
			if err != nil {
				logger.Error("error-creating-driver", err)
			}
			env := driverhttp.NewHttpDriverEnv(logger, context.TODO())
			resp := driver.Activate(env)
			if resp.Err == "" {
				if implementVolumeDriver(resp) {
					activatedPlugins[k] = dockerPlugin
				} else {
					logger.Error("driver-invalid", fmt.Errorf("driver-implements: %#v, expecting: VolumeDriver", resp.Implements))
				}
			} else {
				logger.Info("updated-driver-unreachable", lager.Data{"spec-name": dockerPlugin.GetPluginSpec().Name, "address": dockerPlugin.GetPluginSpec().Address, "tls": dockerPlugin.GetPluginSpec().TLSConfig})
			}
		}

	}

	return activatedPlugins
}

func (r *dockerDriverDiscoverer) getMatchingDriverSpecs(logger lager.Logger, path string, pattern string) ([]string, error) {
	logger.Debug("binaries", lager.Data{"path": path, "pattern": pattern})
	matchingDriverSpecs, err := filepath.Glob(path + string(os.PathSeparator) + "*." + pattern)
	if err != nil { // untestable on linux, does glob work differently on windows???
		return nil, fmt.Errorf("Volman configured with an invalid driver path '%s', error occured list files (%s)", path, err.Error())
	}
	return matchingDriverSpecs, nil

}

func (r *dockerDriverDiscoverer) pluginDoesNotMatch(logger lager.Logger, plugin volman.Plugin, pluginSpec volman.PluginSpec) bool {
	if plugin == nil {
		return true
	}
	doesNotMatch := !plugin.Matches(logger, pluginSpec)
	if doesNotMatch {
		logger.Info("existing-plugin-mismatch", lager.Data{"specName": plugin.GetPluginSpec().Name, "existing-address": plugin.GetPluginSpec().Address, "new-adddress": pluginSpec.Address})
	}
	return doesNotMatch
}

func (r *dockerDriverDiscoverer) getPluginSpec(logger lager.Logger, specName string, driverPath string, specFile string) (volman.PluginSpec, error) {
	driverSpec, err := dockerdriver.ReadDriverSpec(logger, specName, driverPath, specFile)
	if err != nil {
		logger.Error("error-reading-driver-spec", err)
		return volman.PluginSpec{}, errors.New("error-reading-driver-spec")
	}

	pluginSpec := mapDriverSpecToPluginSpec(driverSpec)
	return pluginSpec, err
}

func (r *dockerDriverDiscoverer) findDockerSpecFileByName(logger lager.Logger, nameToFind string, driverPath string, specs []string) (bool, string) {
	for _, spec := range specs {
		found, specName, specFile := specName(logger, spec)

		if found && specName == nameToFind {
			return true, specFile
		}
	}
	return false, "'"
}

func (r *dockerDriverDiscoverer) createPlugin(logger lager.Logger, specName string, driverPath string, specFile string, pluginSpec volman.PluginSpec) (volman.Plugin, error) {
	logger.Info("creating-driver", lager.Data{"specName": specName, "driver-path": driverPath, "specFile": specFile})
	driver, err := r.driverFactory.DockerDriver(logger, specName, driverPath, specFile)
	if err != nil {
		logger.Error("error-creating-driver", err)
		return nil, err
	}

	return voldocker.NewVolmanPluginWithDockerDriver(driver, pluginSpec), nil
}

func specName(logger lager.Logger, spec string) (bool, string, string) {
	re := regexp.MustCompile(`([^/]*/)?([^/]*)\.(sock|spec|json)$`)

	segs2 := re.FindAllStringSubmatch(spec, 1)
	if len(segs2) <= 0 {
		return false, "", ""
	}
	specName := segs2[0][2]
	specFile := segs2[0][2] + "." + segs2[0][3]
	logger.Debug("insert-unique-spec", lager.Data{"specname": specName})
	return true, specName, specFile
}

func implementVolumeDriver(resp dockerdriver.ActivateResponse) bool {
	return len(resp.Implements) > 0 && driverImplements("VolumeDriver", resp.Implements)
}

func mapDriverSpecToPluginSpec(driverSpec *dockerdriver.DriverSpec) volman.PluginSpec {
	pluginSpec := volman.PluginSpec{
		Name:            driverSpec.Name,
		Address:         driverSpec.Address,
		UniqueVolumeIds: driverSpec.UniqueVolumeIds,
	}
	if driverSpec.TLSConfig != nil {
		pluginSpec.TLSConfig = &volman.TLSConfig{
			InsecureSkipVerify: driverSpec.TLSConfig.InsecureSkipVerify,
			CAFile:             driverSpec.TLSConfig.CAFile,
			CertFile:           driverSpec.TLSConfig.CertFile,
			KeyFile:            driverSpec.TLSConfig.KeyFile,
		}
	}
	return pluginSpec
}
