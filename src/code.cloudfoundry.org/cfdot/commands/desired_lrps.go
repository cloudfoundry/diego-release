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
	desiredLRPsDomainFlag string
)

var desiredLRPsCmd = &cobra.Command{
	Use:   "desired-lrps",
	Short: "List desired LRPs",
	Long:  "List desired LRPs from the BBS",
	RunE:  desiredLRPs,
}

func init() {
	AddBBSAndTimeoutFlags(desiredLRPsCmd)
	desiredLRPsCmd.Flags().StringVarP(&desiredLRPsDomainFlag, "domain", "d", "", "retrieve only desired lrps for the given domain")
	RootCmd.AddCommand(desiredLRPsCmd)
}

func desiredLRPs(cmd *cobra.Command, args []string) error {
	err := ValidateConflictingShortAndLongFlag("-d", "--domain", cmd)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	err = ValidateDesiredLRPsArguments(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	bbsClient, err := helpers.NewBBSClient(cmd, Config)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	err = DesiredLRPs(cmd.OutOrStdout(), cmd.OutOrStderr(), bbsClient, desiredLRPsDomainFlag)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	return nil
}

func ValidateDesiredLRPsArguments(args []string) error {
	if len(args) > 0 {
		return errExtraArguments
	}
	return nil
}

func DesiredLRPs(stdout, stderr io.Writer, bbsClient bbs.Client, domain string) error {
	traceID := trace.GenerateTraceID()
	logger := trace.LoggerWithTraceInfo(globalLogger.Session("desired-lrps"), traceID)

	desiredLRPFilter := models.DesiredLRPFilter{Domain: domain}

	desiredLRPs, err := bbsClient.DesiredLRPs(logger, traceID, desiredLRPFilter)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(stdout)
	for _, lrp := range desiredLRPs {
		err = encoder.Encode(lrp)
		if err != nil {
			logger.Error("failed-to-marshal", err)
		}
	}

	return nil
}
