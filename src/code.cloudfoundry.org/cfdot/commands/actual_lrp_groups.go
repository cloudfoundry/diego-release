package commands

import (
	"encoding/json"
	"io"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/cfdot/commands/helpers"
	"github.com/spf13/cobra"
)

// flags
var (
	actualLRPGroupsDomainFlag, actualLRPGroupsCellIdFlag string
)

var actualLRPGroupsCmd = &cobra.Command{
	Use:        "actual-lrp-groups",
	Short:      `[DEPRECATED] use "actual-lrps" command instead. List actual LRP groups`,
	Long:       "List actual LRP groups from the BBS",
	RunE:       actualLRPGroups,
	Deprecated: `use "actual-lrps" instead.`,
}

func init() {
	AddBBSAndTimeoutFlags(actualLRPGroupsCmd)

	actualLRPGroupsCmd.Flags().StringVarP(&actualLRPGroupsDomainFlag, "domain", "d", "", "retrieve only actual lrps for the given domain")
	actualLRPGroupsCmd.Flags().StringVarP(&actualLRPGroupsCellIdFlag, "cell-id", "c", "", "retrieve only actual lrps for the given cell id")

	RootCmd.AddCommand(actualLRPGroupsCmd)
}

func actualLRPGroups(cmd *cobra.Command, args []string) error {
	err := ValidateActualLRPGroupsArguments(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	bbsClient, err := helpers.NewBBSClient(cmd, Config)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	err = ActualLRPGroups(
		cmd.OutOrStdout(),
		cmd.OutOrStderr(),
		bbsClient,
		actualLRPGroupsDomainFlag,
		actualLRPGroupsCellIdFlag,
	)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	return nil
}

func ValidateActualLRPGroupsArguments(args []string) error {
	if len(args) > 0 {
		return errExtraArguments
	}
	return nil
}

func ActualLRPGroups(stdout, stderr io.Writer, bbsClient bbs.Client, domain, cellID string) error {
	traceID := trace.GenerateTraceID()
	logger := trace.LoggerWithTraceInfo(globalLogger.Session("actual-lrp-groups"), traceID)

	encoder := json.NewEncoder(stdout)

	actualLRPFilter := models.ActualLRPFilter{
		CellID: cellID,
		Domain: domain,
	}

	actualLRPGroups, err := bbsClient.ActualLRPGroups(logger, traceID, actualLRPFilter)
	if err != nil {
		return err
	}

	for _, actualLRPGroup := range actualLRPGroups {
		err = encoder.Encode(actualLRPGroup)
		if err != nil {
			logger.Error("failed-to-marshal", err)
		}
	}

	return nil
}
