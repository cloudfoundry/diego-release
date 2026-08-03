package commands

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
)

var (
	locketApiLocation string
)

// errors
var (
	errMissingLocketUrl = errors.New("Locket API Location not set. Please specify one with the '--locketAPILocation' flag or the 'LOCKET_API_LOCATION' environment variable.")
)

func AddLocketFlags(cmd *cobra.Command) {
	AddTLSFlags(cmd)
	cmd.Flags().StringVar(&locketApiLocation, "locketAPILocation", "", "Hostname:Port of Locket server to target [environment variable equivalent: LOCKET_API_LOCATION]")
	cmd.PreRunE = LocketPrehook
}

func LocketPrehook(cmd *cobra.Command, args []string) error {
	if err := setLocketFlags(cmd, args); err != nil {
		return err
	}
	return tlsPreHook(cmd, args)
}

func setLocketFlags(cmd *cobra.Command, args []string) error {
	if locketApiLocation == "" {
		locketApiLocation = os.Getenv("LOCKET_API_LOCATION")
	}

	Config.LocketApiLocation = locketApiLocation
	if Config.LocketApiLocation == "" {
		return NewCFDotValidationError(cmd, errMissingLocketUrl)
	}
	return nil
}
