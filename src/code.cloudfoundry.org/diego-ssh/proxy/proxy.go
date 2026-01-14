package proxy

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"unicode/utf8"

	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/diego-ssh/helpers"
	"code.cloudfoundry.org/lager/v3"
	"golang.org/x/crypto/ssh"
)

const (
	sshConnectionsMetric = "ssh-connections"
)

type Waiter interface {
	Wait() error
}

type TargetConfig struct {
	Address             string `json:"address"`
	TLSAddress          string `json:"tls_address"`
	ServerCertDomainSAN string `json:"server_cert_domain_san"`
	HostFingerprint     string `json:"host_fingerprint"`
	User                string `json:"user,omitempty"`
	Password            string `json:"password,omitempty"`
	PrivateKey          string `json:"private_key,omitempty"`
}

type LogMessage struct {
	Message string            `json:"message"`
	Tags    map[string]string `json:"tags"`
}

type Proxy struct {
	logger       lager.Logger
	serverConfig *ssh.ServerConfig

	connectionLock *sync.Mutex
	connections    int
	metronClient   loggingclient.IngressClient

	tlsConfig *tls.Config
}

func New(
	logger lager.Logger,
	serverConfig *ssh.ServerConfig,
	metronClient loggingclient.IngressClient,
	tlsConfig *tls.Config,
) *Proxy {
	return &Proxy{
		logger:         logger,
		serverConfig:   serverConfig,
		connectionLock: &sync.Mutex{},
		metronClient:   metronClient,
		tlsConfig:      tlsConfig,
	}
}

func (p *Proxy) HandleConnection(netConn net.Conn) {
	logger := p.logger.Session("handle-connection")
	defer netConn.Close()

	serverConn, serverChannels, serverRequests, err := ssh.NewServerConn(netConn, p.serverConfig)
	if err != nil {
		return
	}
	defer serverConn.Close()

	clientConn, clientChannels, clientRequests, err := NewClientConn(logger, serverConn.Permissions, p.tlsConfig)
	if err != nil {
		return
	}

	logMessage := extractLogMessage(logger, serverConn.Permissions)

	defer func() {
		if logMessage != nil {
			endMessage := fmt.Sprintf("Remote access ended for %s", serverConn.RemoteAddr().String())
			err = p.metronClient.SendAppLog(endMessage, "SSH", logMessage.Tags)
			if err != nil {
				logger.Debug("failed-to-send-appLog", lager.Data{"error": err})
			}
		}
		err = clientConn.Close()
		if err != nil {
			logger.Debug("failed-to-close-connection", lager.Data{"error": err})
		}
	}()

	if logMessage != nil {
		err = p.metronClient.SendAppLog(logMessage.Message, "SSH", logMessage.Tags)
		if err != nil {
			logger.Debug("failed-to-send-appLog", lager.Data{"error": err})
		}
	}

	fromClientLogger := logger.Session("from-client")
	fromDaemonLogger := logger.Session("from-daemon")

	go ProxyGlobalRequests(fromClientLogger, clientConn, serverRequests)
	go ProxyGlobalRequests(fromDaemonLogger, serverConn, clientRequests)

	go ProxyChannels(fromClientLogger, clientConn, serverChannels)
	go ProxyChannels(fromDaemonLogger, serverConn, clientChannels)

	p.connectionLock.Lock()
	p.connections++
	err = p.metronClient.SendMetric(sshConnectionsMetric, p.connections)
	if err != nil {
		logger.Error("failed-to-send-ssh-connections-metric", err)
	}
	p.connectionLock.Unlock()

	defer func() {
		p.emitConnectionClosing(logger)
	}()

	Wait(logger, serverConn, clientConn)
}

func (p *Proxy) emitConnectionClosing(logger lager.Logger) {
	p.connectionLock.Lock()
	p.connections--
	err := p.metronClient.SendMetric(sshConnectionsMetric, p.connections)
	p.connectionLock.Unlock()

	if err != nil {
		logger.Error("failed-to-send-ssh-connections-metric", err)
	}
}

func extractLogMessage(logger lager.Logger, perms *ssh.Permissions) *LogMessage {
	logMessageJson := perms.CriticalOptions["log-message"]
	if logMessageJson == "" {
		return nil
	}

	logMessage := &LogMessage{}
	err := json.Unmarshal([]byte(logMessageJson), logMessage)
	if err != nil {
		logger.Error("json-unmarshal-failed", err)
		return nil
	}

	return logMessage
}

func ProxyGlobalRequests(logger lager.Logger, conn ssh.Conn, reqs <-chan *ssh.Request) {
	logger = logger.Session("proxy-global-requests")

	logger.Info("started")
	defer logger.Info("completed")

	for req := range reqs {
		logger.Info("request", lager.Data{
			"type":      req.Type,
			"wantReply": req.WantReply,
			"payload":   req.Payload,
		})

		success, reply, err := conn.SendRequest(req.Type, req.WantReply, req.Payload)
		if err != nil {
			logger.Error("send-request-failed", err)
			continue
		}

		if req.WantReply {
			err = req.Reply(success, reply)
			if err != nil {
				logger.Debug("failed-to-reply", lager.Data{"error": err})
			}
		}
	}
}

func ProxyChannels(logger lager.Logger, conn ssh.Conn, channels <-chan ssh.NewChannel) {
	logger = logger.Session("proxy-channels")

	logger.Info("started")
	defer func() {
		logger.Info("completed")
		err := conn.Close()
		logger.Debug("failed-to-close-channel", lager.Data{"error": err})
	}()

	for newChannel := range channels {
		handleNewChannel(logger, conn, newChannel)
	}
}

func handleNewChannel(logger lager.Logger, conn ssh.Conn, newChannel ssh.NewChannel) {
	logger.Info("new-channel", lager.Data{
		"channelType": newChannel.ChannelType(),
		"extraData":   newChannel.ExtraData(),
	})

	logger.Debug("openning-channel-to-daemon")

	targetChan, targetReqs, err := conn.OpenChannel(newChannel.ChannelType(), newChannel.ExtraData())
	if err != nil {
		logger.Error("failed-to-open-channel", err)
		if openErr, ok := err.(*ssh.OpenChannelError); ok {
			err = newChannel.Reject(openErr.Reason, openErr.Message)
			if err != nil {
				logger.Debug("failed-to-reject-channel-creation", lager.Data{"error": err})
			}
		} else {
			err = newChannel.Reject(ssh.ConnectionFailed, err.Error())
			if err != nil {
				logger.Debug("failed-to-reject-channel-creation", lager.Data{"error": err})
			}
		}
		return
	}
	logger.Debug("opened-channel-to-daemon")

	sourceChan, sourceReqs, err := newChannel.Accept()
	if err != nil {
		err = targetChan.Close()
		if err != nil {
			logger.Debug("failed-to-close-target-chan", lager.Data{"error": err})
		}
		return
	}
	logger.Debug("accepted-channel-from-client")

	toTargetLogger := logger.Session("to-target")
	toSourceLogger := logger.Session("to-source")

	targetWg := &sync.WaitGroup{}
	sourceWg := &sync.WaitGroup{}

	targetWg.Add(2)
	go helpers.Copy(toTargetLogger.Session("stdout"), targetWg, targetChan, sourceChan)
	go helpers.Copy(toTargetLogger.Session("stderr"), targetWg, targetChan.Stderr(), sourceChan.Stderr())
	go func() {
		targetWg.Wait()
		err := targetChan.CloseWrite()
		if err != nil {
			logger.Debug("failed-to-close-write-target-chan", lager.Data{"error": err})
		}
	}()

	sourceWg.Add(2)
	go helpers.Copy(toSourceLogger.Session("stdout"), sourceWg, sourceChan, targetChan)
	go helpers.Copy(toSourceLogger.Session("stderr"), sourceWg, sourceChan.Stderr(), targetChan.Stderr())
	go func() {
		sourceWg.Wait()
		err := sourceChan.CloseWrite()
		if err != nil {
			logger.Debug("failed-to-close-write-source-chan", lager.Data{"error": err})
		}
	}()

	go ProxyRequests(toTargetLogger, newChannel.ChannelType(), sourceReqs, targetChan, targetWg)
	go ProxyRequests(toSourceLogger, newChannel.ChannelType(), targetReqs, sourceChan, sourceWg)
}

func ProxyRequests(logger lager.Logger, channelType string, reqs <-chan *ssh.Request, channel ssh.Channel, wg *sync.WaitGroup) {
	logger = logger.Session("proxy-requests", lager.Data{
		"channel-type": channelType,
	})

	logger.Info("started")
	defer func() {
		logger.Info("completed")
		wg.Wait()
		err := channel.Close()
		if err != nil {
			logger.Debug("failed-to-close-channel", lager.Data{"error": err})
		}
	}()

	for req := range reqs {
		logger.Info("request", lager.Data{
			"type":      req.Type,
			"wantReply": req.WantReply,
			"payload":   req.Payload,
		})
		success, err := channel.SendRequest(req.Type, req.WantReply, req.Payload)
		if err != nil {
			logger.Error("send-request-failed", err)
			continue
		}

		if req.WantReply {
			err = req.Reply(success, nil)
			if err != nil {
				logger.Debug("failed-to-reply", lager.Data{"error": err})
			}
		}

		if req.Type == "exit-status" {
			return
		}
	}
}

func Wait(logger lager.Logger, waiters ...Waiter) {
	wg := &sync.WaitGroup{}
	for _, waiter := range waiters {
		wg.Add(1)
		go func(waiter Waiter) {
			err := waiter.Wait()
			if err != nil {
				logger.Debug("failed-to-wait", lager.Data{"error": err})
			}
			wg.Done()
		}(waiter)
	}
	wg.Wait()
}

func NewClientConn(logger lager.Logger, permissions *ssh.Permissions, tlsConfig *tls.Config) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	if permissions == nil || permissions.CriticalOptions == nil {
		err := errors.New("Invalid permissions from authentication")
		logger.Error("permissions-and-critical-options-required", err)
		return nil, nil, nil, err
	}

	targetConfigJson := permissions.CriticalOptions["proxy-target-config"]
	logger = logger.Session("new-client-conn")

	logger.Debug("creating-client-connection", lager.Data{
		"proxy-target-config": targetConfigJson,
	})

	var targetConfig TargetConfig
	err := json.Unmarshal([]byte(permissions.CriticalOptions["proxy-target-config"]), &targetConfig)
	if err != nil {
		logger.Error("unmarshal-failed", err)
		return nil, nil, nil, err
	}

	dialer := func() (net.Conn, error) {
		tlsConfig := tlsConfigWithServerName(tlsConfig, targetConfig.ServerCertDomainSAN)
		if tlsConfig != nil && targetConfig.TLSAddress != "" {
			nConn, err := tls.Dial("tcp", targetConfig.TLSAddress, tlsConfig)
			if err == nil {
				return nConn, nil
			}

			logger.Error("tls-dial-failed", err, lager.Data{
				"tcp_address":            targetConfig.TLSAddress,
				"server_cert_domain_san": targetConfig.ServerCertDomainSAN,
			})
		}

		nConn, err := net.Dial("tcp", targetConfig.Address)
		if err != nil {
			logger.Error("dial-failed", err, lager.Data{
				"address": targetConfig.Address,
			})
			return nil, err
		}

		return nConn, nil
	}

	nConn, err := dialer()
	if err != nil {
		return nil, nil, nil, err
	}

	logger.Info("connected-to-backend", lager.Data{
		"backend-address": nConn.RemoteAddr().String(),
	})

	clientConfig := &ssh.ClientConfig{}

	if targetConfig.User != "" {
		clientConfig.User = targetConfig.User
	}

	if targetConfig.PrivateKey != "" {
		key, err := ssh.ParsePrivateKey([]byte(targetConfig.PrivateKey))
		if err != nil {
			logger.Error("parsing-key-failed", err)
			return nil, nil, nil, err
		}
		clientConfig.Auth = append(clientConfig.Auth, ssh.PublicKeys(key))
	}

	if targetConfig.User != "" && targetConfig.Password != "" {
		clientConfig.Auth = append(clientConfig.Auth, ssh.Password(targetConfig.Password))
	}

	if targetConfig.HostFingerprint != "" {
		clientConfig.HostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			expectedFingerprint := targetConfig.HostFingerprint

			var actualFingerprint string
			switch utf8.RuneCountInString(expectedFingerprint) {
			case helpers.MD5_FINGERPRINT_LENGTH:
				actualFingerprint = helpers.MD5Fingerprint(key)
			case helpers.SHA1_FINGERPRINT_LENGTH:
				actualFingerprint = helpers.SHA1Fingerprint(key)
			}

			if expectedFingerprint != actualFingerprint {
				err := errors.New("Host fingerprint mismatch")
				logger.Error("host-key-fingerprint-mismatch", err)
				return err
			}

			return nil
		}
	} else {
		clientConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	conn, ch, req, err := ssh.NewClientConn(nConn, targetConfig.Address, clientConfig)
	if err != nil {
		logger.Error("handshake-failed", err)
		return nil, nil, nil, err
	}

	return conn, ch, req, nil
}

func tlsConfigWithServerName(original *tls.Config, serverName string) *tls.Config {
	if original == nil {
		return nil
	}
	new := original.Clone()
	new.ServerName = serverName
	return new
}
