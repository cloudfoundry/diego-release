package vollocal

import (
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/volman"
	"github.com/tedsuo/ifrit"
)

type Syncer struct {
	logger       lager.Logger
	registry     volman.PluginRegistry
	scanInterval time.Duration
	clock        clock.Clock
	discoverer   []volman.Discoverer
}

func NewSyncer(logger lager.Logger, registry volman.PluginRegistry, discoverer []volman.Discoverer, scanInterval time.Duration, clock clock.Clock) *Syncer {
	return &Syncer{
		logger:       logger,
		registry:     registry,
		scanInterval: scanInterval,
		clock:        clock,
		discoverer:   discoverer,
	}
}

func NewSyncerWithShims(logger lager.Logger, registry volman.PluginRegistry, discoverer []volman.Discoverer, scanInterval time.Duration, clock clock.Clock) *Syncer {
	return &Syncer{
		logger:       logger,
		registry:     registry,
		scanInterval: scanInterval,
		clock:        clock,
		discoverer:   discoverer,
	}
}

func (p *Syncer) Runner() ifrit.Runner {
	return p
}

func (p *Syncer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := p.logger.Session("sync-plugin")
	logger.Info("start")
	defer logger.Info("end")

	logger.Info("running-discovery")
	allPlugins, err := discoverAllplugins(logger, p.discoverer)
	if err != nil {
		return err
	}

	p.registry.Set(allPlugins)

	timer := p.clock.NewTimer(p.scanInterval)
	defer timer.Stop()

	close(ready)

	for {
		select {
		case <-timer.C():
			go func() {
				logger.Info("running-re-discovery")
				allPlugins, err := discoverAllplugins(logger, p.discoverer)
				if err != nil {
					logger.Error("failed-discover", err)
				}
				p.registry.Set(allPlugins)
				timer.Reset(p.scanInterval)
			}()
		case signal := <-signals:
			logger.Info("signalled", lager.Data{"signal": signal.String()})
			return nil
		}
	}
}

func discoverAllplugins(logger lager.Logger, discoverers []volman.Discoverer) (map[string]volman.Plugin, error) {
	allPlugins := map[string]volman.Plugin{}
	for _, discoverer := range discoverers {
		plugins, err := discoverer.Discover(logger)
		logger.Debug(fmt.Sprintf("plugins found: %#v", plugins))
		if err != nil {
			logger.Error("failed-discover", err)
			return map[string]volman.Plugin{}, err
		}
		for k, v := range plugins {
			allPlugins[k] = v
		}
	}
	return allPlugins, nil
}
