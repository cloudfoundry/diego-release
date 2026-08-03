package commands

import (
	"encoding/json"
	"io"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/cfdot/commands/helpers"
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task TASK_GUID",
	Short: "Display task",
	Long:  "Retrieve the specified task",
	RunE:  task,
}

func init() {
	AddBBSAndTimeoutFlags(taskCmd)
	RootCmd.AddCommand(taskCmd)
}

func task(cmd *cobra.Command, args []string) error {
	guid, err := ValidateTaskArgs(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	bbsClient, err := helpers.NewBBSClient(cmd, Config)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	if err := TaskByGuid(cmd.OutOrStdout(), cmd.OutOrStderr(), bbsClient, guid); err != nil {
		return NewCFDotError(cmd, err)
	}

	return nil
}

func TaskByGuid(stdout, _ io.Writer, bbsClient bbs.Client, taskGuid string) error {
	traceID := trace.GenerateTraceID()
	logger := trace.LoggerWithTraceInfo(globalLogger.Session("task-by-guid"), traceID)

	task, err := bbsClient.TaskByGuid(logger, traceID, taskGuid)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(stdout)
	err = encoder.Encode(task)
	if err != nil {
		logger.Error("failed-to-marshal", err)
	}

	return nil
}

func ValidateTaskArgs(args []string) (string, error) {
	if len(args) == 0 || args[0] == "" {
		return "", errMissingArguments
	}

	if len(args) > 1 {
		return "", errExtraArguments
	}

	return args[0], nil
}
