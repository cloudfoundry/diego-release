package portauthority

import "errors"

type PortAllocator interface {
	ClaimPorts(int) (uint16, error)
}

type portAllocator struct {
	nextPort   uint16
	endingPort uint16
}

// New creates a new port allocator
// startingPort indicates the first port that will be assigned by the ClaimPorts() function.
// endingPort indicates the maximum port number that this allocator may assign.
//
// returns a non-nil error if the ending port exceeds the IANA maximum of 65535.
func New(startingPort, endingPort int) (PortAllocator, error) {
	if endingPort > 65535 {
		return nil, errors.New("Invalid port range requested. Ports can only be numbers between 0-65535")
	}
	return &portAllocator{
		nextPort:   uint16(startingPort),
		endingPort: uint16(endingPort),
	}, nil
}

// ClaimPorts returns a new uint16 port to be used for testing processes.
//
// No guarantees are made that something is not already listening on that port.
// If running multiple processes, you should initialize the portAllocator with different ranges.
// If ports are also allocated by another method, the portAllocator should be
// provided with a range that skips those other ports.
//
// numPorts indicates the number of ports that will be claimed. The first claimed
// port is returned, and the next numPorts-1 ports sequentially after that are yours
// to use.
//
// returns a non-nil error if there are not enough ports in the range compared to
// the number requested.
func (p *portAllocator) ClaimPorts(numPorts int) (uint16, error) {
	port := p.nextPort
	if p.endingPort < port+uint16(numPorts-1) {
		return 0, errors.New("insufficient ports available")
	}

	p.nextPort = p.nextPort + uint16(numPorts)
	return uint16(port), nil
}
