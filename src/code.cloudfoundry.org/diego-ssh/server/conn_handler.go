package server

import (
	"net"
	"sync"
	"sync/atomic"
)

type serverState int32

const (
	stateDefault = int32(iota)
	stateStopped
)

func (s *serverState) StopOnce() bool {
	return atomic.CompareAndSwapInt32((*int32)(s), stateDefault, stateStopped)
}

func (s *serverState) Stopped() bool {
	return atomic.LoadInt32((*int32)(s)) == stateStopped
}

type connHandler struct {
	store map[net.Conn]struct{}
	mu    sync.Mutex
	wg    sync.WaitGroup
	state serverState
}

func (s *connHandler) remove(conn net.Conn) {
	s.mu.Lock()
	delete(s.store, conn)
	s.wg.Done()
	s.mu.Unlock()
}

func (s *connHandler) handle(handler ConnectionHandler, conn net.Conn) {
	defer s.remove(conn)
	handler.HandleConnection(conn)
}

func (s *connHandler) Handle(handler ConnectionHandler, conn net.Conn) {
	// fast exit: don't attempt to acquire the mutex or
	// handle the conn if shutdown
	if s.state.Stopped() {
		// #nosec G104 - ignore errors when closing SSH connections so we don't spam our logs during a DoS
		conn.Close()
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// recheck the state now that we've locked the mutex
	// as we may have been blocked on call to Shutdown()
	if s.state.Stopped() {
		// #nosec G104 - ignore errors when closing SSH connections so we don't spam our logs during a DoS
		conn.Close()
		return
	}
	// lazily initialize the store
	if s.store == nil {
		s.store = make(map[net.Conn]struct{})
	}
	s.store[conn] = struct{}{}
	s.wg.Add(1)
	go s.handle(handler, conn)
}

func (s *connHandler) Shutdown() {
	if s.state.StopOnce() {
		s.mu.Lock()
		for c := range s.store {
			// #nosec G104 - ignore errors when closing SSH connections so we don't spam our logs during a DoS
			c.Close()
		}
		s.mu.Unlock()
		s.wg.Wait()
	}
}
