package handlers

import (
	"fmt"
	"net"
	"sync"

	"code.cloudfoundry.org/diego-ssh/helpers"
	"code.cloudfoundry.org/lager/v3"
	"golang.org/x/crypto/ssh"
)

type DirectTcpipChannelHandler struct {
	dialer Dialer
}

func NewDirectTcpipChannelHandler(dialer Dialer) *DirectTcpipChannelHandler {
	return &DirectTcpipChannelHandler{
		dialer: dialer,
	}
}

func (handler *DirectTcpipChannelHandler) HandleNewChannel(logger lager.Logger, newChannel ssh.NewChannel) {
	logger = logger.Session("directtcip-handle-new-channel")
	logger.Debug("starting")
	defer logger.Debug("complete")

	// RFC 4254 Section 7.1
	type channelOpenDirectTcpipMsg struct {
		TargetAddr string
		TargetPort uint32
		OriginAddr string
		OriginPort uint32
	}
	var directTcpipMessage channelOpenDirectTcpipMsg

	err := ssh.Unmarshal(newChannel.ExtraData(), &directTcpipMessage)
	if err != nil {
		logger.Error("failed-unmarshalling-ssh-message", err)
		err := newChannel.Reject(ssh.ConnectionFailed, "Failed to parse open channel message")
		if err != nil {
			logger.Debug("failed-to-reject", lager.Data{"error": err})
		}
		return
	}

	destination := fmt.Sprintf("%s:%d", directTcpipMessage.TargetAddr, directTcpipMessage.TargetPort)
	logger.Debug("dialing-connection", lager.Data{"destination": destination})

	conn, err := handler.dialer.Dial("tcp", destination)
	if err != nil {
		logger.Error("failed-connecting-to-target", err)
		err := newChannel.Reject(ssh.ConnectionFailed, err.Error())
		if err != nil {
			logger.Debug("failed-to-reject", lager.Data{"error": err})
		}
		return
	}
	defer conn.Close()

	logger.Debug("dialed-connection", lager.Data{"destintation": destination})
	channel, requests, err := newChannel.Accept()
	if err != nil {
		logger.Error("failed-to-accept-channel", err)
		err := newChannel.Reject(ssh.ConnectionFailed, err.Error())
		if err != nil {
			logger.Debug("failed-to-reject", lager.Data{"error": err})
		}
		return
	}
	defer channel.Close()

	go ssh.DiscardRequests(requests)

	wg := &sync.WaitGroup{}

	wg.Add(2)

	logger.Debug("copying-channel-data")
	go helpers.CopyAndClose(logger.Session("to-target"), wg, conn, channel,
		func() {
			err := conn.(*net.TCPConn).CloseWrite()
			if err != nil {
				logger.Debug("failed-to-close-connection", lager.Data{"error": err})
			}
		},
	)
	go helpers.CopyAndClose(logger.Session("to-channel"), wg, channel, conn,
		func() {
			err := channel.CloseWrite()
			if err != nil {
				logger.Debug("failed-to-close-channel", lager.Data{"error": err})
			}
		},
	)

	wg.Wait()
}
