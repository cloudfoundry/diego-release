package globalrequest

import (
	"net"
	"strconv"

	"code.cloudfoundry.org/diego-ssh/handlers/globalrequest/internal"
	"code.cloudfoundry.org/diego-ssh/helpers"
	"code.cloudfoundry.org/lager/v3"
	"golang.org/x/crypto/ssh"
)

const CancelTCPIPForward = "cancel-tcpip-forward"

type CancelTCPIPForwardHandler struct{}

func (h *CancelTCPIPForwardHandler) HandleRequest(logger lager.Logger, request *ssh.Request, conn ssh.Conn, lnStore *helpers.ListenerStore) {
	logger = logger.Session("cancel-tcpip-forward", lager.Data{
		"type":       request.Type,
		"want-reply": request.WantReply,
	})
	logger.Info("start")
	defer logger.Info("done")

	var tcpipForwardMessage internal.TCPIPForwardRequest

	err := ssh.Unmarshal(request.Payload, &tcpipForwardMessage)
	if err != nil {
		logger.Error("unmarshal-failed", err)
		err = request.Reply(false, nil)
		if err != nil {
			logger.Debug("failed-to-reply", lager.Data{"error": err})
		}
	}

	address := net.JoinHostPort(tcpipForwardMessage.Address, strconv.FormatUint(uint64(tcpipForwardMessage.Port), 10))

	logger.Info("recieved-payload", lager.Data{
		"message-address": tcpipForwardMessage.Address,
		"message-port":    tcpipForwardMessage.Port,
		"listen-address":  address,
	})

	if err = lnStore.RemoveListener(address); err != nil {
		logger.Error("failed-to-cancel", err)
		_ = request.Reply(false, nil)
		return
	}

	logger.Info("successfully-canceled-tcpip-forward")
	_ = request.Reply(true, nil)
}
