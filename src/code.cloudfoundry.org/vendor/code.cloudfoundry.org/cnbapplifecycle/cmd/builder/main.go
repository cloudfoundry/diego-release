package main

import (
	"os"

	"code.cloudfoundry.org/cnbapplifecycle/cmd/builder/cli"
	"code.cloudfoundry.org/cnbapplifecycle/pkg/errors"
)

func main() {
	err := cli.Execute()

	if err != nil {
		os.Exit(errors.ExitCodeFromError(err))
	}
}
