package commands

import (
	"encoding/json"
	"io"

	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/cfdot/commands/helpers"

	"code.cloudfoundry.org/bbs"
	"github.com/spf13/cobra"
)

var desiredLRPCmd = &cobra.Command{
	Use:   "desired-lrp PROCESS_GUID",
	Short: "Show the specified desired LRP",
	Long:  "Show the desired LRP specified by the given process GUID",
	RunE:  desiredLRP,
}

func init() {
	AddBBSAndTimeoutFlags(desiredLRPCmd)
	RootCmd.AddCommand(desiredLRPCmd)
}

func desiredLRP(cmd *cobra.Command, args []string) error {
	processGuid, err := ValidateDesiredLRPArguments(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	bbsClient, err := helpers.NewBBSClient(cmd, Config)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	err = DesiredLRP(cmd.OutOrStdout(), cmd.OutOrStderr(), bbsClient, processGuid)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	return nil
}

func ValidateDesiredLRPArguments(args []string) (string, error) {
	if len(args) == 0 {
		return "", errMissingArguments
	}

	if len(args) > 1 {
		return "", errExtraArguments
	}

	if args[0] == "" {
		return "", errInvalidProcessGuid
	}

	return args[0], nil
}

func DesiredLRP(stdout, stderr io.Writer, bbsClient bbs.Client, processGuid string) error {
	traceID := trace.GenerateTraceID()
	logger := trace.LoggerWithTraceInfo(globalLogger.Session("desired-lrp"), traceID)

	desiredLRP, err := bbsClient.DesiredLRPByProcessGuid(logger, traceID, processGuid)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(stdout)
	return encoder.Encode(desiredLRP)
}
