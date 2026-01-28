package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/trace"

	"code.cloudfoundry.org/cfdot/commands/helpers"
	"github.com/spf13/cobra"
)

// flags
var (
	actualLRPGroupsGuidIndexFlag string
)

var actualLRPGroupsByProcessGuidCmd = &cobra.Command{
	Use:        "actual-lrp-groups-for-guid PROCESS_GUID",
	Short:      `[DEPRECATED] use "actual-lrps" command with the "--process-guid" flag instead. List actual LRP groups for a process guid`,
	Long:       fmt.Sprintf("List actual LRP groups from the BBS for a given process guid. Process guids can be obtained by running %s actual-lrp-groups", os.Args[0]),
	RunE:       actualLRPGroupsByProcessGuid,
	Deprecated: `use "actual-lrps" with the "--process-guid" flag instead.`,
}

func init() {
	AddBBSAndTimeoutFlags(actualLRPGroupsByProcessGuidCmd)

	// String flag because logic for optional int flag is not clear
	actualLRPGroupsByProcessGuidCmd.Flags().StringVarP(&actualLRPGroupsGuidIndexFlag, "index", "i", "", "retrieve actual lrp for the given index")

	RootCmd.AddCommand(actualLRPGroupsByProcessGuidCmd)
}

func actualLRPGroupsByProcessGuid(cmd *cobra.Command, args []string) error {
	processGuid, index, err := ValidateActualLRPGroupsForGuidArgs(args, actualLRPGroupsGuidIndexFlag)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	bbsClient, err := helpers.NewBBSClient(cmd, Config)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	err = ActualLRPGroupsForGuid(cmd.OutOrStdout(), cmd.OutOrStderr(), bbsClient, processGuid, index)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	return nil
}

func ValidateActualLRPGroupsForGuidArgs(args []string, indexFlag string) (string, int, error) {
	if len(args) < 1 {
		return "", 0, errMissingArguments
	}

	if len(args) > 1 {
		return "", 0, errExtraArguments
	}

	if args[0] == "" {
		return "", 0, errInvalidProcessGuid
	}

	index := -1
	if indexFlag != "" {
		var err error
		index, err = strconv.Atoi(indexFlag)
		if err != nil || index < 0 {
			return "", 0, errInvalidIndex
		}
	}

	return args[0], index, nil
}

func ActualLRPGroupsForGuid(stdout, stderr io.Writer, bbsClient bbs.Client, processGuid string, index int) error {
	traceID := trace.GenerateTraceID()
	logger := trace.LoggerWithTraceInfo(globalLogger.Session("actual-lrp-groups-for-guid"), traceID)

	encoder := json.NewEncoder(stdout)
	if index < 0 {
		actualLRPGroups, err := bbsClient.ActualLRPGroupsByProcessGuid(logger, traceID, processGuid)
		if err != nil {
			return err
		}

		for _, group := range actualLRPGroups {
			err = encoder.Encode(group)
			if err != nil {
				logger.Error("failed-to-marshal", err)
			}
		}

		return nil
	} else {
		actualLRPGroup, err := bbsClient.ActualLRPGroupByProcessGuidAndIndex(logger, traceID, processGuid, index)
		if err != nil {
			return err
		}

		return encoder.Encode(actualLRPGroup)
	}
}
