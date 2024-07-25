package commands

import (
	"context"
	"encoding/json"
	"io"

	"code.cloudfoundry.org/cfdot/commands/helpers"
	"code.cloudfoundry.org/locket/models"

	"github.com/spf13/cobra"
)

var locksCmd = &cobra.Command{
	Use:   "locks",
	Short: "List Locket locks",
	Long:  "List locks from Locket",
	RunE:  locks,
}

func init() {
	AddLocketFlags(locksCmd)
	RootCmd.AddCommand(locksCmd)
}

func locks(cmd *cobra.Command, args []string) error {
	err := ValidateLocksArguments(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	logger := globalLogger.Session("locket-client")
	locketClient, err := helpers.NewLocketClient(logger, cmd, Config)
	if err != nil {
		return NewCFDotComponentError(cmd, err)
	}

	err = Locks(
		cmd.OutOrStdout(),
		cmd.OutOrStderr(),
		locketClient,
	)
	if err != nil {
		return NewCFDotComponentError(cmd, err)
	}

	return nil
}

func ValidateLocksArguments(args []string) error {
	if len(args) > 0 {
		return errExtraArguments
	}
	return nil
}

func Locks(stdout, stderr io.Writer, locketClient models.LocketClient) error {
	logger := globalLogger.Session("locks")

	encoder := json.NewEncoder(stdout)

	req := &models.FetchAllRequest{TypeCode: models.TypeCode_LOCK}
	resp, err := locketClient.FetchAll(context.Background(), req)
	if err != nil {
		return err
	}

	for _, lock := range resp.Resources {
		err = encoder.Encode(lock)
		if err != nil {
			logger.Error("failed-to-marshal", err)
			return err
		}
	}

	return nil
}
