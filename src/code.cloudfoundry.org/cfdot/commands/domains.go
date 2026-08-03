package commands

import (
	"encoding/json"
	"io"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/trace"

	"code.cloudfoundry.org/cfdot/commands/helpers"
	"github.com/spf13/cobra"
)

var domainsCmd = &cobra.Command{
	Use:   "domains",
	Short: "List domains",
	Long:  "List fresh domains from the BBS",
	RunE:  domains,
}

func init() {
	AddBBSAndTimeoutFlags(domainsCmd)
	RootCmd.AddCommand(domainsCmd)
}

func domains(cmd *cobra.Command, args []string) error {
	err := ValidateDomainsArguments(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	bbsClient, err := helpers.NewBBSClient(cmd, Config)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	err = Domains(cmd.OutOrStdout(), cmd.OutOrStderr(), bbsClient)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	return nil
}

func ValidateDomainsArguments(args []string) error {
	if len(args) > 0 {
		return errExtraArguments
	}
	return nil
}

func Domains(stdout, stderr io.Writer, bbsClient bbs.Client) error {
	traceID := trace.GenerateTraceID()
	logger := trace.LoggerWithTraceInfo(globalLogger.Session("domains"), traceID)

	encoder := json.NewEncoder(stdout)

	domains, err := bbsClient.Domains(logger, traceID)
	if err != nil {
		return err
	}

	for _, domain := range domains {
		err = encoder.Encode(domain)
		if err != nil {
			logger.Error("failed-to-marshal", err)
		}
	}

	return nil
}
