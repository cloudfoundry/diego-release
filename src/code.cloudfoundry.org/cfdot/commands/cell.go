package commands

import (
	"encoding/json"
	"errors"
	"io"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/cfdot/commands/helpers"

	"github.com/spf13/cobra"
)

var cellCmd = &cobra.Command{
	Use:   "cell CELL_ID",
	Short: "Show the specified cell presence",
	Long:  "Show the cell presence specified by the given cell id",
	RunE:  cell,
}

func init() {
	AddBBSAndTimeoutFlags(cellCmd)
	RootCmd.AddCommand(cellCmd)
}

func cell(cmd *cobra.Command, args []string) error {
	err := ValidateCellArguments(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	bbsClient, err := helpers.NewBBSClient(cmd, Config)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	err = Cell(
		cmd.OutOrStdout(),
		cmd.OutOrStderr(),
		bbsClient,
		args[0],
	)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	return nil
}

func ValidateCellArguments(args []string) error {
	switch {
	case len(args) > 1:
		return errExtraArguments
	case len(args) < 1:
		return errMissingArguments
	default:
		return nil
	}
}

func Cell(stdout, stderr io.Writer, bbsClient bbs.Client, cellId string) error {
	traceID := trace.GenerateTraceID()
	logger := trace.LoggerWithTraceInfo(globalLogger.Session("cell-presence"), traceID)

	encoder := json.NewEncoder(stdout)

	cells, err := bbsClient.Cells(logger, traceID)
	if err != nil {
		return err
	}

	for _, cell := range cells {
		if cell.CellId == cellId {
			err = encoder.Encode(cell)
			if err != nil {
				logger.Error("failed-to-marshal", err)
			}

			return err
		}
	}

	return errors.New("Cell not found")
}
