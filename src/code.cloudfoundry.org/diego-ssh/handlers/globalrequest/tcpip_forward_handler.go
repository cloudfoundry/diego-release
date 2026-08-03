package globalrequest

import (
	"net"
	"strconv"
	"sync"

	"code.cloudfoundry.org/diego-ssh/handlers/globalrequest/internal"
	"code.cloudfoundry.org/diego-ssh/helpers"
	"code.cloudfoundry.org/lager/v3"
	"golang.org/x/crypto/ssh"
)

const TCPIPForward = "tcpip-forward"

type TCPIPForwardHandler struct{}

func (h *TCPIPForwardHandler) HandleRequest(logger lager.Logger, request *ssh.Request, conn ssh.Conn, lnStore *helpers.ListenerStore) {
	logger = logger.Session("tcpip-forward", lager.Data{
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

	address := net.JoinHostPort(tcpipForwardMessage.Address, strconv.Itoa(int(tcpipForwardMessage.Port)))

	logger.Info("new-tcpip-forward", lager.Data{
		"message-address": tcpipForwardMessage.Address,
		"message-port":    tcpipForwardMessage.Port,
		"listen-address":  address,
	})

	listener, err := net.Listen("tcp", address)
	if err != nil {
		logger.Error("failed-to-listen", err)
		err = request.Reply(false, nil)
		if err != nil {
			logger.Debug("failed-to-reply", lager.Data{"error": err})
		}
		return
	}

	var listenerAddr string
	var listenerPort uint32
	if addr, ok := listener.Addr().(*net.TCPAddr); ok {
		address = addr.String()
		listenerAddr = addr.IP.String()
		listenerPort = uint32(addr.Port)
	}
	logger.Info("actual-listener-address", lager.Data{
		"addr": listenerAddr,
		"port": listenerPort,
	})

	lnStore.AddListener(address, listener)

	go h.forwardAcceptLoop(listener, logger, conn, tcpipForwardMessage.Address, listenerPort)

	var tcpipForwardResponse internal.TCPIPForwardResponse
	tcpipForwardResponse.Port = listenerPort

	var replyPayload []byte

	if tcpipForwardMessage.Port == 0 {
		// See RFC 4254, section 7.1
		replyPayload = ssh.Marshal(tcpipForwardResponse)
	}

	// Reply() will only send something when WantReply is true
	_ = request.Reply(true, replyPayload)
}

// See RFC 4254, section 7.2
type forwardedTCPPayload struct {
	Addr       string
	Port       uint32
	OriginAddr string
	OriginPort uint32
}

func (h *TCPIPForwardHandler) forwardAcceptLoop(listener net.Listener, logger lager.Logger, sshConn ssh.Conn, lnAddr string, lnPort uint32) {
	logger = logger.Session("forward-accept-loop")
	logger.Info("start")
	defer logger.Info("done")

	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Error("failed-to-accept", err)
			return
		}

		go func(conn net.Conn) {
			defer conn.Close()

			payload := forwardedTCPPayload{
				Addr: lnAddr,
				Port: lnPort,
			}
			if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
				payload.OriginAddr = addr.IP.String()
				payload.OriginPort = uint32(addr.Port)
			}

			channel, requests, err := sshConn.OpenChannel("forwarded-tcpip", ssh.Marshal(payload))
			if err != nil {
				logger.Error("failed-to-open channel", err)
				return
			}
			defer channel.Close()

			logger.Info("opened-channel", lager.Data{"payload": payload})
			go ssh.DiscardRequests(requests)

			var wg sync.WaitGroup
			wg.Add(2)

			go helpers.CopyAndClose(logger.Session("to-target"), &wg, conn, channel, func() {
				err := conn.Close()
				if err != nil {
					logger.Debug("failed-to-close-connection", lager.Data{"error": err})
				}
			})
			go helpers.CopyAndClose(logger.Session("to-channel"), &wg, channel, conn, func() {
				err := channel.CloseWrite()
				if err != nil {
					logger.Debug("failed-to-close-channel", lager.Data{"error": err})
				}
			})

			wg.Wait()
		}(conn)

		logger.Info("accepted-connection", lager.Data{"Address": listener.Addr().String()})
	}
}
