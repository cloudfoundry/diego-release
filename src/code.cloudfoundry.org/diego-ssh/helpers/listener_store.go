package helpers

import (
	"fmt"
	"net"
	"sync"
)

type ListenerStore struct {
	store map[string]net.Listener
	lock  sync.Mutex
}

func NewListenerStore() *ListenerStore {
	return &ListenerStore{
		store: make(map[string]net.Listener),
	}
}

func (t *ListenerStore) AddListener(addr string, ln net.Listener) {
	t.lock.Lock()
	t.store[addr] = ln
	t.lock.Unlock()
}

func (t *ListenerStore) RemoveListener(addr string) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	if ln, ok := t.store[addr]; ok {
		delete(t.store, addr)
		return ln.Close()
	}
	return fmt.Errorf("RemoveListener error: addr %s doesn't exist", addr)
}

func (t *ListenerStore) ListAll() []string {
	t.lock.Lock()
	a := make([]string, 0, len(t.store))
	for k := range t.store {
		a = append(a, k)
	}
	t.lock.Unlock()
	return a
}

func (t *ListenerStore) RemoveAll() {
	t.lock.Lock()
	for k, ln := range t.store {
		delete(t.store, k)
		// #nosec G104 - no logging here, ignoring close errors
		ln.Close()
	}
	t.lock.Unlock()
}
