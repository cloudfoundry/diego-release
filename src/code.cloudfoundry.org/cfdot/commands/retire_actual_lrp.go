package commands

import (
	"io"
	"strconv"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/cfdot/commands/helpers"
	"github.com/spf13/cobra"
)

var retireActualLRPCmd = &cobra.Command{
	Use:   "retire-actual-lrp PROCESS_GUID INDEX",
	Short: "Retire actual LRP by index and process guid",
	Long:  "Retire actual LRP by index and process guid",
	RunE:  retireActualLRP,
}

func init() {
	AddBBSAndTimeoutFlags(retireActualLRPCmd)
	RootCmd.AddCommand(retireActualLRPCmd)
}

func retireActualLRP(cmd *cobra.Command, args []string) error {
	processGuid, index, err := ValidateRetireActualLRPArgs(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	bbsClient, err := helpers.NewBBSClient(cmd, Config)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	err = RetireActualLRP(cmd.OutOrStdout(), cmd.OutOrStderr(), bbsClient, processGuid, int32(index))
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	return nil
}

func ValidateRetireActualLRPArgs(args []string) (string, int, error) {
	if len(args) < 2 {
		return "", 0, errMissingArguments
	}

	if len(args) > 2 {
		return "", 0, errExtraArguments
	}

	if args[0] == "" {
		return "", 0, errInvalidProcessGuid
	}

	index, err := strconv.Atoi(args[1])
	if err != nil || index < 0 {
		return "", 0, errInvalidIndex
	}

	return args[0], index, nil
}

func RetireActualLRP(stdout, stderr io.Writer, bbsClient bbs.Client, processGuid string, index int32) error {
	traceID := trace.GenerateTraceID()
	logger := trace.LoggerWithTraceInfo(globalLogger.Session("retire-actual-lrp"), traceID)

	desiredLRP, err := bbsClient.DesiredLRPByProcessGuid(logger, traceID, processGuid)
	if err != nil {
		return err
	}

	actualLRPKey := models.ActualLRPKey{ProcessGuid: processGuid, Index: index, Domain: desiredLRP.Domain}
	err = bbsClient.RetireActualLRP(logger, traceID, &actualLRPKey)
	if err != nil {
		return err
	}

	return nil
}
