package commands

import (
	"context"
	"encoding/json"
	"io"

	"code.cloudfoundry.org/cfdot/commands/helpers"
	"code.cloudfoundry.org/locket/models"

	"github.com/spf13/cobra"
)

var presencesCmd = &cobra.Command{
	Use:   "presences",
	Short: "List Locket presences",
	Long:  "List presences registered in Locket",
	RunE:  presences,
}

func init() {
	AddLocketFlags(presencesCmd)
	RootCmd.AddCommand(presencesCmd)
}

func presences(cmd *cobra.Command, args []string) error {
	err := ValidatePresencesArguments(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	logger := globalLogger.Session("locket-client")
	locketClient, err := helpers.NewLocketClient(logger, cmd, Config)
	if err != nil {
		return NewCFDotComponentError(cmd, err)
	}

	err = Presences(
		cmd.OutOrStdout(),
		cmd.OutOrStderr(),
		locketClient,
	)
	if err != nil {
		return NewCFDotComponentError(cmd, err)
	}

	return nil
}

func ValidatePresencesArguments(args []string) error {
	if len(args) > 0 {
		return errExtraArguments
	}
	return nil
}

func Presences(stdout, stderr io.Writer, locketClient models.LocketClient) error {
	logger := globalLogger.Session("presences")

	encoder := json.NewEncoder(stdout)

	req := &models.FetchAllRequest{TypeCode: models.TypeCode_PRESENCE}
	resp, err := locketClient.FetchAll(context.Background(), req)
	if err != nil {
		return err
	}

	for _, presence := range resp.Resources {
		err = encoder.Encode(presence)
		if err != nil {
			logger.Error("failed-to-marshal", err)
			return err
		}
	}

	return nil
}
