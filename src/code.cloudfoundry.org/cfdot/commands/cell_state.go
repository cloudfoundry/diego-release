package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/cfdot/commands/helpers"
	cfhttp "code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/rep"
	"github.com/spf13/cobra"
)

var cellStateCmd = &cobra.Command{
	Use:   "cell-state CELL_ID",
	Short: "Show the specified cell state",
	Long:  "Show the cell state specified by the given cell id",
	RunE:  cellState,
}

func init() {
	AddBBSAndTimeoutFlags(cellStateCmd)
	RootCmd.AddCommand(cellStateCmd)
}

func cellState(cmd *cobra.Command, args []string) error {
	err := ValidateCellStateArguments(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	bbsClient, err := helpers.NewBBSClient(cmd, Config)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	traceID := trace.GenerateTraceID()
	cellRegistration, err := FetchCellRegistration(bbsClient, traceID, args[0])
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	httpClient := cfhttp.NewClient()
	stateClient := cfhttp.NewClient(
		cfhttp.WithRequestTimeout(10 * time.Second),
	)

	repTLSConfig := &rep.TLSConfig{
		CaCertFile: Config.CACertFile,
		CertFile:   Config.CertFile,
		KeyFile:    Config.KeyFile,
	}
	repClientFactory, err := rep.NewClientFactory(httpClient, stateClient, repTLSConfig)
	if err != nil {
		return NewCFDotComponentError(cmd, fmt.Errorf("Failed creating rep client factory: %s", err))
	}

	err = FetchCellState(
		cmd.OutOrStdout(),
		cmd.OutOrStderr(),
		repClientFactory,
		cellRegistration,
		traceID,
	)
	if err != nil {
		return NewCFDotComponentError(cmd, fmt.Errorf("Rep error: Failed to get cell state for cell %s: %s", args[0], err.Error()))
	}

	return nil
}

func ValidateCellStateArguments(args []string) error {
	switch {
	case len(args) > 1:
		return errExtraArguments
	case len(args) < 1:
		return errMissingArguments
	default:
		return nil
	}
}

func FetchCellRegistration(bbsClient bbs.Client, traceID string, cellId string) (*models.CellPresence, error) {
	logger := trace.LoggerWithTraceInfo(globalLogger.Session("fetch-cell-presence"), traceID)

	cells, err := bbsClient.Cells(logger, traceID)
	if err != nil {
		return nil, err
	}

	for _, cell := range cells {
		if cell.CellId == cellId {
			return cell, nil
		}
	}

	return nil, errors.New("Cell not found")
}

func FetchCellState(stdout, stderr io.Writer, clientFactory rep.ClientFactory, registration *models.CellPresence, traceID string) error {
	repClient, err := clientFactory.CreateClient(registration.RepAddress, registration.RepUrl, traceID)
	if err != nil {
		return err
	}

	logger := trace.LoggerWithTraceInfo(globalLogger.Session("cell-state"), traceID)
	encoder := json.NewEncoder(stdout)

	state, err := repClient.State(logger)
	if err != nil {
		logger.Error("failed-to-fetch-cell-state", err)
		return err
	}

	err = encoder.Encode(state)
	if err != nil {
		logger.Error("failed-to-marshal", err)
		return err
	}
	return nil
}
