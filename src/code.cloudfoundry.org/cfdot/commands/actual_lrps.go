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
	actualLRPsDomainFlag, actualLRPsCellIdFlag, actualLRPsProcessGuidFlag string
	actualLRPsIndexFlag                                                   int32
)

var actualLRPsCmd = &cobra.Command{
	Use:   "actual-lrps",
	Short: "List actual LRPs",
	Long:  "List actual LRPs from the BBS",
	RunE:  actualLRPs,
}

func init() {
	AddBBSAndTimeoutFlags(actualLRPsCmd)

	actualLRPsCmd.Flags().StringVarP(&actualLRPsDomainFlag, "domain", "d", "", "retrieve only actual lrps for the given domain")
	actualLRPsCmd.Flags().StringVarP(&actualLRPsCellIdFlag, "cell-id", "c", "", "retrieve only actual lrps for the given cell id")
	actualLRPsCmd.Flags().StringVarP(&actualLRPsProcessGuidFlag, "process-guid", "p", "", "retrieve only actual lrps for the given process guid")
	actualLRPsCmd.Flags().Int32VarP(&actualLRPsIndexFlag, "index", "i", 0, "retrieve only actual lrps for the given index")

	RootCmd.AddCommand(actualLRPsCmd)
}

func actualLRPs(cmd *cobra.Command, args []string) error {
	err := ValidateActualLRPsArguments(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	bbsClient, err := helpers.NewBBSClient(cmd, Config)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	var index *int32
	if cmd.Flag("index").Changed {
		index = &actualLRPsIndexFlag
	}

	err = ActualLRPs(
		cmd.OutOrStdout(),
		cmd.OutOrStderr(),
		bbsClient,
		actualLRPsDomainFlag,
		actualLRPsCellIdFlag,
		actualLRPsProcessGuidFlag,
		index,
	)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	return nil
}

func ValidateActualLRPsArguments(args []string) error {
	if len(args) > 0 {
		return errExtraArguments
	}
	return nil
}

func ActualLRPs(stdout, stderr io.Writer, bbsClient bbs.Client, domain, cellID, processGuid string, index *int32) error {
	traceID := trace.GenerateTraceID()
	logger := trace.LoggerWithTraceInfo(globalLogger.Session("actual-lrps"), traceID)

	encoder := json.NewEncoder(stdout)

	actualLRPFilter := models.ActualLRPFilter{
		CellID:      cellID,
		Domain:      domain,
		ProcessGuid: processGuid,
		Index:       index,
	}

	actualLRPs, err := bbsClient.ActualLRPs(logger, traceID, actualLRPFilter)
	if err != nil {
		return err
	}

	for _, actualLRP := range actualLRPs {
		err = encoder.Encode(actualLRP)
		if err != nil {
			logger.Error("failed-to-marshal", err)
		}
	}

	return nil
}
