package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/diego-ssh/authenticators"
	"code.cloudfoundry.org/diego-ssh/daemon"
	"code.cloudfoundry.org/diego-ssh/handlers"
	"code.cloudfoundry.org/diego-ssh/handlers/globalrequest"
	"code.cloudfoundry.org/diego-ssh/keys"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagerflags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
	"golang.org/x/crypto/ssh"
)

var address = flag.String(
	"address",
	"127.0.0.1:2222",
	"listen address for ssh daemon",
)

var hostKey = flag.String(
	"hostKey",
	"",
	"PEM encoded RSA host key",
)

var authorizedKey = flag.String(
	"authorizedKey",
	"",
	"Public key in the OpenSSH authorized_keys format",
)

var allowUnauthenticatedClients = flag.Bool(
	"allowUnauthenticatedClients",
	false,
	"Allow access to unauthenticated clients",
)

var inheritDaemonEnv = flag.Bool(
	"inheritDaemonEnv",
	false,
	"Inherit daemon's environment",
)

var allowedCiphers = flag.String(
	"allowedCiphers",
	"",
	"Limit cipher algorithms to those provided (comma separated)",
)

var allowedMACs = flag.String(
	"allowedMACs",
	"",
	"Limit MAC algorithms to those provided (comma separated)",
)

var allowedKeyExchanges = flag.String(
	"allowedKeyExchanges",
	"",
	"Limit key exchanges algorithms to those provided (comma separated)",
)

var hostKeyPEM string
var authorizedKeyValue string

func runServer() error {
	debugserver.AddFlags(flag.CommandLine)
	lagerflags.AddFlags(flag.CommandLine)
	flag.Parse()
	exec := false

	logger, reconfigurableSink := lagerflags.New("sshd")

	hostKeyPEM = os.Getenv("SSHD_HOSTKEY")
	if hostKeyPEM != "" {
		authorizedKeyValue = os.Getenv("SSHD_AUTHKEY")

		// unset the variables so child processes don't inherit them
		os.Unsetenv("SSHD_HOSTKEY")
		os.Unsetenv("SSHD_AUTHKEY")
	} else {
		hostKeyPEM = *hostKey
		if hostKeyPEM == "" {
			var err error
			hostKeyPEM, err = generateNewHostKey()
			if err != nil {
				logger.Error("failed-to-generate-host-key", err)
				return err
			}
		}
		authorizedKeyValue = *authorizedKey
		exec = true
	}

	if exec && runtime.GOOS != "windows" {
		err := os.Setenv("SSHD_HOSTKEY", hostKeyPEM)
		if err != nil {
			logger.Error("failed-to-set-environment-variable", err, lager.Data{"environment-variable": "SSHD_HOSTKEY"})
			return err
		}

		err = os.Setenv("SSHD_AUTHKEY", authorizedKeyValue)
		if err != nil {
			logger.Error("failed-to-set-environment-variable", err, lager.Data{"environment-variable": "SSHD_AUTHKEY"})
			return err
		}

		logLevel := "info"
		flag.CommandLine.Lookup("logLevel")
		logLevelFlag := flag.CommandLine.Lookup("logLevel")
		if logLevelFlag != nil {
			logLevel = logLevelFlag.Value.String()
		}

		runtime.GOMAXPROCS(1)
		err = syscall.Exec(os.Args[0], []string{
			os.Args[0],
			fmt.Sprintf("--allowedKeyExchanges=%s", *allowedKeyExchanges),
			fmt.Sprintf("--address=%s", *address),
			fmt.Sprintf("--allowUnauthenticatedClients=%t", *allowUnauthenticatedClients),
			fmt.Sprintf("--inheritDaemonEnv=%t", *inheritDaemonEnv),
			fmt.Sprintf("--allowedCiphers=%s", *allowedCiphers),
			fmt.Sprintf("--allowedMACs=%s", *allowedMACs),
			fmt.Sprintf("--logLevel=%s", logLevel),
			fmt.Sprintf("--debugAddr=%s", debugserver.DebugAddress(flag.CommandLine)),
		}, os.Environ())
		if err != nil {
			logger.Error("failed-exec", err)
			return err
		}
	}

	serverConfig, err := configure(logger)
	if err != nil {
		logger.Error("configure-failed", err)
		return err
	}

	runner := handlers.NewCommandRunner()
	shellLocator := handlers.NewShellLocator()
	dialer := &net.Dialer{}

	sshDaemon := daemon.New(
		logger,
		serverConfig,
		map[string]handlers.GlobalRequestHandler{
			globalrequest.TCPIPForward:       new(globalrequest.TCPIPForwardHandler),
			globalrequest.CancelTCPIPForward: new(globalrequest.CancelTCPIPForwardHandler),
		},
		map[string]handlers.NewChannelHandler{
			"session":      handlers.NewSessionChannelHandler(runner, shellLocator, getDaemonEnvironment(), 15*time.Second),
			"direct-tcpip": handlers.NewDirectTcpipChannelHandler(dialer),
		},
	)
	server, err := createServer(logger, *address, sshDaemon)
	if err != nil {
		logger.Error("create-server-failure", err)
		return err
	}

	members := grouper.Members{
		{Name: "sshd", Runner: server},
	}

	if dbgAddr := debugserver.DebugAddress(flag.CommandLine); dbgAddr != "" {
		members = append(grouper.Members{
			{Name: "debug-server", Runner: debugserver.Runner(dbgAddr, reconfigurableSink)},
		}, members...)
	}

	group := grouper.NewOrdered(os.Interrupt, members)
	monitor := ifrit.Invoke(sigmon.New(group))

	logger.Info("started")

	if err := <-monitor.Wait(); err != nil {
		logger.Error("exited-with-failure", err)
		return err
	}

	logger.Info("exited")
	return nil
}

func main() {
	if err := runServer(); err != nil {
		os.Exit(1)
	}
}

func getDaemonEnvironment() map[string]string {
	daemonEnv := map[string]string{}

	if *inheritDaemonEnv {
		envs := os.Environ()
		for _, env := range envs {
			nvp := strings.SplitN(env, "=", 2)
			// account for windows "Path" environment variable!
			if len(nvp) == 2 && strings.ToUpper(nvp[0]) != "PATH" {
				daemonEnv[nvp[0]] = nvp[1]
			}
		}
	}
	return daemonEnv
}

func configure(logger lager.Logger) (*ssh.ServerConfig, error) {
	errorStrings := []string{}
	sshConfig := &ssh.ServerConfig{ServerVersion: "SSH-2.0-diego-sshd"}
	sshConfig.SetDefaults()

	key, err := acquireHostKey(logger)
	if err != nil {
		logger.Error("failed-to-acquire-host-key", err)
		errorStrings = append(errorStrings, err.Error())
	}

	sshConfig.AddHostKey(key)
	sshConfig.NoClientAuth = *allowUnauthenticatedClients

	if authorizedKeyValue == "" && !*allowUnauthenticatedClients {
		logger.Error("authorized-key-required", nil)
		errorStrings = append(errorStrings, "Public user key is required")
	}

	if authorizedKeyValue != "" {
		decodedPublicKey, err := decodeAuthorizedKey(logger)
		if err == nil {
			authenticator := authenticators.NewPublicKeyAuthenticator(decodedPublicKey)
			sshConfig.PublicKeyCallback = authenticator.Authenticate
		} else {
			errorStrings = append(errorStrings, err.Error())
		}
	}

	if *allowedCiphers != "" {
		sshConfig.Config.Ciphers = strings.Split(*allowedCiphers, ",")
	} else {
		sshConfig.Config.Ciphers = []string{"aes128-gcm@openssh.com", "aes256-ctr", "aes192-ctr", "aes128-ctr"}
	}

	if *allowedMACs != "" {
		sshConfig.Config.MACs = strings.Split(*allowedMACs, ",")
	} else {
		sshConfig.Config.MACs = []string{"hmac-sha2-256-etm@openssh.com", "hmac-sha2-256"}
	}

	if *allowedKeyExchanges != "" {
		sshConfig.Config.KeyExchanges = strings.Split(*allowedKeyExchanges, ",")
	} else {
		sshConfig.Config.KeyExchanges = []string{"curve25519-sha256@libssh.org"}
	}

	err = nil
	if len(errorStrings) > 0 {
		err = errors.New(strings.Join(errorStrings, ", "))
	}

	return sshConfig, err
}

func decodeAuthorizedKey(logger lager.Logger) (ssh.PublicKey, error) {
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(authorizedKeyValue))
	return publicKey, err
}

func acquireHostKey(logger lager.Logger) (ssh.Signer, error) {
	var encoded []byte
	if hostKeyPEM == "" {
		return nil, errors.New("empty-host-key")
	} else {
		encoded = []byte(hostKeyPEM)
	}

	key, err := ssh.ParsePrivateKey(encoded)
	if err != nil {
		logger.Error("failed-to-parse-host-key", err)
		return nil, err
	}
	return key, nil
}

func generateNewHostKey() (string, error) {
	hostKeyPair, err := keys.RSAKeyPairFactory.NewKeyPair(1024)

	if err != nil {
		return "", err
	}
	return hostKeyPair.PEMEncodedPrivateKey(), nil
}
