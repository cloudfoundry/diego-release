package k8sgarden

import (
	"fmt"
	"sync"

	"code.cloudfoundry.org/garden"
)

type containerMap struct {
	containers map[string]*container
	lock       *sync.RWMutex
}

func newContainerMap() *containerMap {
	return &containerMap{
		containers: make(map[string]*container),
		lock:       &sync.RWMutex{},
	}
}

func (n *containerMap) Add(handle string, cntr *container) error {
	n.lock.Lock()
	defer n.lock.Unlock()

	if _, ok := n.containers[handle]; ok {
		return fmt.Errorf("Handle '%s' already in use", handle)
	}

	n.containers[handle] = cntr

	return nil
}

func (n *containerMap) Remove(handle string) {
	n.lock.Lock()
	defer n.lock.Unlock()

	_, ok := n.containers[handle]
	if !ok {
		return
	}

	delete(n.containers, handle)
}

func (n *containerMap) Get(handle string) (*container, error) {
	n.lock.RLock()
	defer n.lock.RUnlock()

	cntr, ok := n.containers[handle]
	if !ok {
		return nil, garden.ContainerNotFoundError{Handle: handle}
	}

	return cntr, nil
}

func (n *containerMap) List() []garden.Container {
	n.lock.RLock()
	defer n.lock.RUnlock()

	list := make([]garden.Container, 0, len(n.containers))
	for _, cntr := range n.containers {
		list = append(list, cntr)
	}

	return list
}

func (n *containerMap) Exists(handle string) bool {
	n.lock.RLock()
	defer n.lock.RUnlock()

	_, ok := n.containers[handle]
	return ok
}
