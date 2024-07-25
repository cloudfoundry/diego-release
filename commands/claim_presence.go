package commands

import (
	"context"
	"errors"
	"io"

	"code.cloudfoundry.org/cfdot/commands/helpers"
	"code.cloudfoundry.org/locket/models"
	"github.com/spf13/cobra"
)

var claimPresenceCmd = &cobra.Command{
	Use:   "claim-presence",
	Short: "Claim Locket presence",
	Long:  "Claims a Locket presence with the given key, owner, and optional value",
	RunE:  claimPresence,
}

func init() {
	AddLocketFlags(claimPresenceCmd)
	claimPresenceCmd.Flags().StringVarP(&lockKey, "key", "k", "", "the key of the presence being claimed")
	claimPresenceCmd.Flags().StringVarP(&lockOwner, "owner", "o", "", "the presence owner")
	claimPresenceCmd.Flags().StringVarP(&lockValue, "value", "v", "", "the value associated with the presence")
	claimPresenceCmd.Flags().IntVarP(&ttlInSeconds, "ttl", "t", 0, "the TTL for the presence")
	RootCmd.AddCommand(claimPresenceCmd)
}

func claimPresence(cmd *cobra.Command, args []string) error {
	err := ValidateClaimPresenceArguments(cmd, args, lockKey, lockOwner, lockValue, ttlInSeconds)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	logger := globalLogger.Session("locket-client")
	locketClient, err := helpers.NewLocketClient(logger, cmd, Config)
	if err != nil {
		return NewCFDotComponentError(cmd, err)
	}

	err = ClaimPresence(
		cmd.OutOrStdout(),
		cmd.OutOrStderr(),
		locketClient,
		lockKey,
		lockOwner,
		lockValue,
		int64(ttlInSeconds),
	)
	if err != nil {
		return NewCFDotComponentError(cmd, err)
	}

	return nil
}

func ValidateClaimPresenceArguments(cmd *cobra.Command, args []string, lockKey, lockOwner, lockValue string, ttlInSeconds int) error {
	if len(args) > 0 {
		return errExtraArguments
	}

	var err error

	err = ValidateConflictingShortAndLongFlag("-k", "--key", cmd)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	err = ValidateConflictingShortAndLongFlag("-o", "--owner", cmd)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	err = ValidateConflictingShortAndLongFlag("-v", "--value", cmd)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	err = ValidateConflictingShortAndLongFlag("-t", "--ttl", cmd)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	if lockKey == "" {
		return NewCFDotValidationError(cmd, errors.New("key cannot be empty"))
	}

	if lockOwner == "" {
		return NewCFDotValidationError(cmd, errors.New("owner cannot be empty"))
	}

	if ttlInSeconds <= 0 {
		return NewCFDotValidationError(cmd, errors.New("ttl should be an integer greater than zero"))
	}

	return nil
}

func ClaimPresence(
	stdout, stderr io.Writer,
	locketClient models.LocketClient,
	lockKey, lockOwner, lockValue string,
	ttlInSeconds int64) error {
	logger := globalLogger.Session("claim-presence")

	req := &models.LockRequest{
		Resource: &models.Resource{
			Key:      lockKey,
			Owner:    lockOwner,
			Value:    lockValue,
			TypeCode: models.TypeCode_PRESENCE,
		},
		TtlInSeconds: ttlInSeconds,
	}
	_, err := locketClient.Lock(context.Background(), req)
	if err != nil {
		return err
	}

	logger.Info("completed")
	return nil
}
