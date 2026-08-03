package daemon

import (
	"net"

	"code.cloudfoundry.org/diego-ssh/handlers"
	"code.cloudfoundry.org/diego-ssh/helpers"
	"code.cloudfoundry.org/lager/v3"
	"golang.org/x/crypto/ssh"
)

type Daemon struct {
	logger                lager.Logger
	serverConfig          *ssh.ServerConfig
	globalRequestHandlers map[string]handlers.GlobalRequestHandler
	newChannelHandlers    map[string]handlers.NewChannelHandler
}

func New(
	logger lager.Logger,
	serverConfig *ssh.ServerConfig,
	globalRequestHandlers map[string]handlers.GlobalRequestHandler,
	newChannelHandlers map[string]handlers.NewChannelHandler,
) *Daemon {
	return &Daemon{
		logger:                logger,
		serverConfig:          serverConfig,
		globalRequestHandlers: globalRequestHandlers,
		newChannelHandlers:    newChannelHandlers,
	}
}

func (d *Daemon) HandleConnection(netConn net.Conn) {
	logger := d.logger.Session("handle-connection")

	logger.Info("started")
	defer logger.Info("completed")
	defer netConn.Close()

	serverConn, serverChannels, serverRequests, err := ssh.NewServerConn(netConn, d.serverConfig)
	if err != nil {
		logger.Error("handshake-failed", err)
		return
	}

	lnStore := helpers.NewListenerStore()
	go d.handleGlobalRequests(logger, serverRequests, serverConn, lnStore)
	go d.handleNewChannels(logger, serverChannels)

	err = serverConn.Wait()
	if err != nil {
		logger.Debug("failed-to-wait-for-server", lager.Data{"error": err})
	}
	lnStore.RemoveAll()
}

func (d *Daemon) handleGlobalRequests(logger lager.Logger, requests <-chan *ssh.Request, conn ssh.Conn, lnStore *helpers.ListenerStore) {
	logger = logger.Session("handle-global-requests")
	logger.Info("starting")
	defer logger.Info("finished")

	for req := range requests {
		logger.Debug("request", lager.Data{
			"request-type": req.Type,
			"want-reply":   req.WantReply,
		})

		handler, ok := d.globalRequestHandlers[req.Type]
		if ok {
			handler.HandleRequest(logger, req, conn, lnStore)
			continue
		}

		if req.WantReply {
			err := req.Reply(false, nil)
			if err != nil {
				logger.Debug("failed-to-reply", lager.Data{"error": err})
			}
		}
	}
}

func (d *Daemon) handleNewChannels(logger lager.Logger, newChannelRequests <-chan ssh.NewChannel) {
	logger = logger.Session("handle-new-channels")
	logger.Info("starting")
	defer logger.Info("finished")

	for newChannel := range newChannelRequests {
		logger.Info("new-channel", lager.Data{
			"channelType": newChannel.ChannelType(),
			"extraData":   newChannel.ExtraData(),
		})

		if handler, ok := d.newChannelHandlers[newChannel.ChannelType()]; ok {
			go handler.HandleNewChannel(logger, newChannel)
			continue
		}

		logger.Info("rejecting-channel", lager.Data{"reason": "unkonwn-channel-type"})
		err := newChannel.Reject(ssh.UnknownChannelType, newChannel.ChannelType())
		if err != nil {
			logger.Debug("failed-to-reject", lager.Data{"error": err})
		}
	}
}
