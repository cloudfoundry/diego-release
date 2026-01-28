package commands

import (
	"os"
	"strconv"

	"code.cloudfoundry.org/cfdot/commands/helpers"
	"github.com/spf13/cobra"
)

var (
	timeoutConfig   helpers.TLSConfig
	timeoutPreHooks = []func(cmd *cobra.Command, args []string) error{}
)

func AddBBSAndTimeoutFlags(cmd *cobra.Command) {
	AddBBSFlags(cmd)
	cmd.Flags().IntVar(&timeoutConfig.Timeout, "timeout", 0, "timeout for BBS requests in seconds [environment variable equivalent: CFDOT_TIMEOUT]")
	timeoutPreHooks = append(timeoutPreHooks, cmd.PreRunE)
	cmd.PreRunE = TimeoutPrehook
}

func TimeoutPrehook(cmd *cobra.Command, args []string) error {
	var err error
	for _, f := range timeoutPreHooks {
		if f == nil {
			continue
		}
		err = f(cmd, args)
		if err != nil {
			return err
		}
	}

	timeoutConfig.Merge(Config)
	err = setTimeoutFlag(cmd, args)
	if err != nil {
		return err
	}

	Config = timeoutConfig
	return nil
}

func setTimeoutFlag(cmd *cobra.Command, args []string) error {
	if timeoutConfig.Timeout == 0 && os.Getenv("CFDOT_TIMEOUT") != "" {
		timeout, err := strconv.ParseInt(os.Getenv("CFDOT_TIMEOUT"), 10, 16)
		if err != nil {
			return err
		}
		timeoutConfig.Timeout = int(timeout)
	}

	return nil
}
