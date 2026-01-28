package commands

import (
	"errors"
	"fmt"
	"io"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/trace"
	"code.cloudfoundry.org/cfdot/commands/helpers"
	cfhttp "code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/rep"
	"github.com/spf13/cobra"
)

var cellStatesCmd = &cobra.Command{
	Use:   "cell-states",
	Short: "Show cell states for all cells",
	Long:  "Show the cell state for all the cells",
	RunE:  cellStates,
}

func init() {
	AddBBSAndTimeoutFlags(cellStatesCmd)
	RootCmd.AddCommand(cellStatesCmd)
}

func cellStates(cmd *cobra.Command, args []string) error {
	err := ValidateCellStatesArguments(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	bbsClient, err := helpers.NewBBSClient(cmd, Config)
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

	return FetchCellStates(cmd, cmd.OutOrStdout(), cmd.OutOrStderr(), repClientFactory, bbsClient)
}

func ValidateCellStatesArguments(args []string) error {
	switch {
	case len(args) > 0:
		return errExtraArguments
	default:
		return nil
	}
}

func FetchCellStates(cmd *cobra.Command, stdout, stderr io.Writer, clientFactory rep.ClientFactory, bbsClient bbs.Client) error {
	traceID := trace.GenerateTraceID()
	logger := trace.LoggerWithTraceInfo(globalLogger.Session("cell-states"), traceID)

	registrations, err := bbsClient.Cells(logger, traceID)
	if err != nil {
		return NewCFDotComponentError(cmd, fmt.Errorf("BBS error: Failed to get cell registrations from BBS: %s", err))
	}
	errs := ""
	for _, registration := range registrations {
		err := FetchCellState(stdout, stderr, clientFactory, registration, traceID)
		if err != nil {
			errs += fmt.Sprintf("Rep error: Failed to get cell state for cell %s: %s\n", registration.CellId, err)
		}
	}

	if errs != "" {
		return NewCFDotComponentError(cmd, errors.New(errs))
	}
	return nil
}
