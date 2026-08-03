//go:build windows2012R2

package handlers

import (
	"time"

	"code.cloudfoundry.org/lager/v3"
	"golang.org/x/crypto/ssh"
)

type SessionChannelHandler struct {
}

func NewSessionChannelHandler(
	runner Runner,
	shellLocator ShellLocator,
	defaultEnv map[string]string,
	keepalive time.Duration,
) *SessionChannelHandler {
	return &SessionChannelHandler{}
}

func (handler *SessionChannelHandler) HandleNewChannel(logger lager.Logger, newChannel ssh.NewChannel) {
	err := newChannel.Reject(ssh.Prohibited, "SSH is not supported on windows2012R2 cells")
	if err != nil {
		logger.Error("handle-new-session-channel-failed", err)
	}

	return
}
