package commands

import (
	"errors"
	"io"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/trace"

	"code.cloudfoundry.org/cfdot/commands/helpers"
	"github.com/spf13/cobra"
)

var (
	// errors
	errMissingDomain = errors.New("No domain given")
	errNegativeTTL   = errors.New("ttl is negative")

	// flags
	setDomainTTLFlag time.Duration
)

var setDomainCmd = &cobra.Command{
	Use:   "set-domain DOMAIN",
	Short: "Set domain",
	Long:  "Mark a domain as fresh for ttl seconds, where 0 or non-specified means keep fresh permanently",
	RunE:  setDomain,
}

func init() {
	AddBBSAndTimeoutFlags(setDomainCmd)
	setDomainCmd.Flags().DurationVarP(&setDomainTTLFlag, "ttl", "t", 0*time.Second, "ttl of domain")
	RootCmd.AddCommand(setDomainCmd)
}

func setDomain(cmd *cobra.Command, args []string) error {
	domain, err := ValidateSetDomainArgs(args)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	if setDomainTTLFlag < 0 {
		return NewCFDotValidationError(cmd, errNegativeTTL)
	}

	bbsClient, err := helpers.NewBBSClient(cmd, Config)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	err = SetDomain(cmd.OutOrStdout(), cmd.OutOrStderr(), bbsClient, domain, setDomainTTLFlag)
	if err != nil {
		return NewCFDotError(cmd, err)
	}

	return nil
}

func ValidateSetDomainArgs(args []string) (string, error) {
	if len(args) < 1 {
		return "", errMissingArguments
	}

	if len(args) > 1 {
		return "", errExtraArguments
	}

	if args[0] == "" {
		return "", errMissingDomain
	}

	return args[0], nil
}

func SetDomain(stdout, stderr io.Writer, bbsClient bbs.Client, domain string, ttlDuration time.Duration) error {
	traceID := trace.GenerateTraceID()
	logger := trace.LoggerWithTraceInfo(globalLogger.Session("set-domain"), traceID)

	err := bbsClient.UpsertDomain(logger, traceID, domain, ttlDuration)
	if err != nil {
		return err
	}

	return nil
}
