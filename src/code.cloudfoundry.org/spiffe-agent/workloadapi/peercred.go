package workloadapi

import (
	"context"
	"errors"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc/credentials"
)

// peerCredentials implements credentials.TransportCredentials by reading the
// SO_PEERCRED socket option of the connecting Unix-domain peer. The verified
// pid/uid/gid are exposed via peerCredAuthInfo on the connection context.
type peerCredentials struct{}

// peerCredAuthInfo carries the kernel-verified peer credentials for a
// connection. SecurityLevel is PrivacyAndIntegrity because a local Unix socket
// pair is not observable by other parties.
type peerCredAuthInfo struct {
	credentials.CommonAuthInfo
	PID int32
	UID int32
	GID int32
}

// AuthType identifies this AuthInfo so callers can type-assert it.
func (peerCredAuthInfo) AuthType() string { return "peercred" }

// ServerHandshake reads SO_PEERCRED from the underlying socket and attaches the
// peer's pid/uid/gid to the returned AuthInfo. The connection is passed through
// unchanged.
func (peerCredentials) ServerHandshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	sc, ok := conn.(syscall.Conn)
	if !ok {
		return nil, nil, errors.New("workloadapi: connection does not support SyscallConn")
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return nil, nil, err
	}

	var ucred *unix.Ucred
	var ctrlErr error
	if err := raw.Control(func(fd uintptr) {
		ucred, ctrlErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return nil, nil, err
	}
	if ctrlErr != nil {
		return nil, nil, ctrlErr
	}

	info := peerCredAuthInfo{
		CommonAuthInfo: credentials.CommonAuthInfo{SecurityLevel: credentials.PrivacyAndIntegrity},
		PID:            ucred.Pid,
		UID:            int32(ucred.Uid),
		GID:            int32(ucred.Gid),
	}
	return conn, info, nil
}

// ClientHandshake is unused: these credentials only authenticate local servers.
func (peerCredentials) ClientHandshake(context.Context, string, net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return nil, nil, errors.New("workloadapi: ClientHandshake not implemented")
}

// Info reports a plaintext protocol; security comes from the peer credentials.
func (peerCredentials) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{SecurityProtocol: "peercred"}
}

// Clone returns a copy; peerCredentials is stateless.
func (peerCredentials) Clone() credentials.TransportCredentials { return peerCredentials{} }

// OverrideServerName is a no-op for Unix-socket peer credentials.
func (peerCredentials) OverrideServerName(string) error { return nil }
