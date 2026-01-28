package vollocal

import (
	"sync"

	"code.cloudfoundry.org/volman"
)

type pluginRegistry struct {
	sync.RWMutex
	registryEntries map[string]volman.Plugin
}

func NewPluginRegistry() volman.PluginRegistry {
	return &pluginRegistry{
		registryEntries: map[string]volman.Plugin{},
	}
}

func NewPluginRegistryWith(initialMap map[string]volman.Plugin) volman.PluginRegistry {
	return &pluginRegistry{
		registryEntries: initialMap,
	}
}

func (d *pluginRegistry) Plugin(id string) (volman.Plugin, bool) {
	d.RLock()
	defer d.RUnlock()

	if !d.containsPlugin(id) {
		return nil, false
	}

	return d.registryEntries[id], true
}

func (d *pluginRegistry) Plugins() map[string]volman.Plugin {
	d.RLock()
	defer d.RUnlock()

	return d.registryEntries
}

func (d *pluginRegistry) Set(plugins map[string]volman.Plugin) {
	d.Lock()
	defer d.Unlock()

	d.registryEntries = plugins
}

func (d *pluginRegistry) Keys() []string {
	d.Lock()
	defer d.Unlock()

	var keys []string
	for k := range d.registryEntries {
		keys = append(keys, k)
	}

	return keys
}

func (d *pluginRegistry) containsPlugin(id string) bool {
	_, ok := d.registryEntries[id]
	return ok
}
