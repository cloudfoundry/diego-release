package properties

import (
	"encoding/json"
	"fmt"
	"sync"

	"code.cloudfoundry.org/garden"
)

type Manager struct {
	propMutex sync.RWMutex
	prop      map[string]map[string]string
}

func NewManager() *Manager {
	return &Manager{
		prop: make(map[string]map[string]string),
	}
}

func (m *Manager) DestroyKeySpace(handle string) error {
	m.propMutex.Lock()
	defer m.propMutex.Unlock()

	delete(m.prop, handle)

	return nil
}

func (m *Manager) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.prop)
}

func (m *Manager) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &(m.prop))
}

func (m *Manager) Set(handle string, name string, value string) {
	m.propMutex.Lock()
	defer m.propMutex.Unlock()

	if _, ok := m.prop[handle]; !ok {
		m.prop[handle] = make(map[string]string)
	}

	m.prop[handle][name] = value
}

func (m *Manager) All(handle string) (garden.Properties, error) {
	m.propMutex.RLock()
	defer m.propMutex.RUnlock()

	return m.prop[handle], nil
}

func (m *Manager) Get(handle string, name string) (string, bool) {
	var (
		prop   string
		exists bool
	)

	m.propMutex.RLock()
	defer m.propMutex.RUnlock()

	prop, exists = m.prop[handle][name]
	return prop, exists
}

func (m *Manager) Remove(handle string, name string) error {
	m.propMutex.Lock()
	defer m.propMutex.Unlock()

	if _, exists := m.prop[handle][name]; !exists {
		return NoSuchPropertyError{
			Message: fmt.Sprintf("cannot Remove %s:%s", handle, name),
		}
	}

	delete(m.prop[handle], name)

	return nil
}

func (m *Manager) MatchesAll(handle string, props garden.Properties) bool {
	m.propMutex.RLock()
	defer m.propMutex.RUnlock()

	for key, val := range props {
		if m.prop[handle][key] != val {
			return false
		}
	}

	return true
}

type NoSuchPropertyError struct {
	Message string
}

func (e NoSuchPropertyError) Error() string {
	return e.Message
}

type NoSuchKeySpaceError struct {
	Message string
}

func (e NoSuchKeySpaceError) Error() string {
	return e.Message
}
