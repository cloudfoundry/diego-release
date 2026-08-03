package diegonats

import (
	"errors"
	"net/url"
	"os"
	"strings"

	"code.cloudfoundry.org/lager/v3"
)

type NATSClientRunner struct {
	addresses string
	username  string
	password  string
	logger    lager.Logger
	client    NATSClient
}

func NewClientRunner(addresses, username, password string, logger lager.Logger, client NATSClient) NATSClientRunner {
	return NATSClientRunner{
		addresses: addresses,
		username:  username,
		password:  password,
		logger:    logger.Session("nats-runner"),
		client:    client,
	}
}

func (runner NATSClientRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	natsMembers := []string{}
	for _, addr := range strings.Split(runner.addresses, ",") {
		uri := url.URL{
			Scheme: "nats",
			User:   url.UserPassword(runner.username, runner.password),
			Host:   addr,
		}
		natsMembers = append(natsMembers, uri.String())
	}

	unexpectedConnClosed, err := runner.client.Connect(natsMembers)
	if err != nil {
		runner.logger.Error("connecting-to-nats-failed", err)
		return err
	}

	runner.logger.Info("connecting-to-nats-succeeeded")
	close(ready)

	select {
	case <-signals:
		runner.client.Close()
		runner.logger.Info("shutting-down")
		return nil
	case <-unexpectedConnClosed:
		runner.logger.Error("unexpected-nats-close", nil)
		return errors.New("nats closed unexpectedly")
	}
}
