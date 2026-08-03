package vollocal

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/volman"
	"github.com/tedsuo/ifrit"
)

type MountPurger interface {
	Runner() ifrit.Runner
	PurgeMounts(logger lager.Logger) error
}

type mountPurger struct {
	logger   lager.Logger
	registry volman.PluginRegistry
}

func NewMountPurger(logger lager.Logger, registry volman.PluginRegistry) MountPurger {
	return &mountPurger{
		logger,
		registry,
	}
}

func (p *mountPurger) Runner() ifrit.Runner {
	return p
}

func (p *mountPurger) Run(signals <-chan os.Signal, ready chan<- struct{}) error {

	if err := p.PurgeMounts(p.logger); err != nil {
		return err
	}

	close(ready)
	<-signals
	return nil
}

func (p *mountPurger) PurgeMounts(logger lager.Logger) error {
	logger = logger.Session("purge-mounts")
	logger.Info("start")
	defer logger.Info("end")

	plugins := p.registry.Plugins()

	for _, plugin := range plugins {
		volumes, err := plugin.ListVolumes(logger)
		if err != nil {
			logger.Error("failed-listing-volume-mount", err)
			continue
		}

		for _, volume := range volumes {
			err = plugin.Unmount(logger, volume)
			if err != nil {
				logger.Error(fmt.Sprintf("failed-unmounting-volume-mount %s", volume), err)
			}
		}
	}
	return nil
}
