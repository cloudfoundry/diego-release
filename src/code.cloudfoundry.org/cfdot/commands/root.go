package commands

import (
	"errors"

	"code.cloudfoundry.org/lager/v3"
	"github.com/spf13/cobra"
)

var globalLogger = lager.NewLogger("cfdot")

var RootCmd = &cobra.Command{
	Use:   "cfdot",
	Short: "Diego operator tooling",
	Long:  "A command-line tool to interact with a Cloud Foundry Diego deployment",
}

var (
	errMissingArguments   = errors.New("Missing arguments")
	errExtraArguments     = errors.New("Too many arguments specified")
	errInvalidProcessGuid = errors.New("Process guid should be non empty string")
	errInvalidIndex       = errors.New("Index must be a non-negative integer")
)
