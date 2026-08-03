package internal

type TCPIPForwardRequest struct {
	Address string
	Port    uint32
}

type TCPIPForwardResponse struct {
	Port uint32
}
