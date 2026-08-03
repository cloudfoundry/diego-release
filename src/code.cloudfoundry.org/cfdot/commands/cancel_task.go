package commands

import (
	"io"

	"github.com/spf13/cobra"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/cfdot/commands/helpers"
)

var cancelTaskCmd = &cobra.Command{
	Use:   "cancel-task TASK_GUID",
	Short: "Cancel task",
	Long:  "Cancel the specified task",
	RunE:  cancelTask,
}

func init() {
	AddBBSAndTimeoutFlags(cancelTaskCmd)
	RootCmd.AddCommand(cancelTaskCmd)
}

func cancelTask(cmd *cobra.Command, args []string) error {
	guid, err := ValidateTaskArgs(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	bbsClient, err := helpers.NewBBSClient(cmd, Config)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	if err := CancelTaskByGuid(cmd.OutOrStdout(), cmd.OutOrStderr(), bbsClient, guid); err != nil {
		return NewCFDotError(cmd, err)
	}

	return nil
}

func CancelTaskByGuid(stdout, _ io.Writer, bbsClient bbs.Client, taskGuid string) error {
	traceID := trace.GenerateTraceID()
	logger := trace.LoggerWithTraceInfo(globalLogger.Session("cancel-task-by-guid"), traceID)

	err := bbsClient.CancelTask(logger, traceID, taskGuid)
	if err != nil {
		return err
	}

	return nil
}

func ValidateCancelTaskArgs(args []string) (string, error) {
	if len(args) == 0 || args[0] == "" {
		return "", errMissingArguments
	}

	if len(args) > 1 {
		return "", errExtraArguments
	}

	return args[0], nil
}
