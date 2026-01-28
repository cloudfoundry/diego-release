//go:build !windows2012R2

package main

import (
	"time"

	"code.cloudfoundry.org/diego-ssh/server"
	"code.cloudfoundry.org/lager/v3"
)

func createServer(
	logger lager.Logger,
	address string,
	sshDaemon server.ConnectionHandler,
) (*server.Server, error) {
	return server.NewServer(logger, address, sshDaemon, 5*time.Minute), nil
}
