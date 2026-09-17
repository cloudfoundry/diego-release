package k8sgarden

import (
	"fmt"
	"math"
	"sync"
)

const startPort uint32 = 62000

type portManager struct {
	allocated map[uint32]struct{}
	mu        sync.Mutex
}

func newPortManager() *portManager {
	return &portManager{
		allocated: make(map[uint32]struct{}),
		mu:        sync.Mutex{},
	}
}

func (p *portManager) Release(port uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.allocated, port)
}

func (p *portManager) Next() (uint32, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for port := startPort; port < math.MaxUint16; port++ {
		if _, allocated := p.allocated[port]; !allocated {
			p.allocated[port] = struct{}{}
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports")
}
