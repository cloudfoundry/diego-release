package commands

import (
	"context"
	"errors"
	"io"

	"code.cloudfoundry.org/cfdot/commands/helpers"
	"code.cloudfoundry.org/locket/models"
	"github.com/spf13/cobra"
)

// flags
var (
	lockKey      string
	lockOwner    string
	lockValue    string
	ttlInSeconds int
)

var claimLockCmd = &cobra.Command{
	Use:   "claim-lock",
	Short: "Claim Locket lock",
	Long:  "Claims a Locket lock with the given key, owner, and optional value",
	RunE:  claimLock,
}

func init() {
	AddLocketFlags(claimLockCmd)
	claimLockCmd.Flags().StringVarP(&lockKey, "key", "k", "", "the key of the lock being claimed")
	claimLockCmd.Flags().StringVarP(&lockOwner, "owner", "o", "", "the lock owner")
	claimLockCmd.Flags().StringVarP(&lockValue, "value", "v", "", "the value associated with the key")
	claimLockCmd.Flags().IntVarP(&ttlInSeconds, "ttl", "t", 0, "the TTL for the lock")
	RootCmd.AddCommand(claimLockCmd)
}

func claimLock(cmd *cobra.Command, args []string) error {
	err := ValidateClaimLocksArguments(cmd, args, lockKey, lockOwner, lockValue, ttlInSeconds)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	logger := globalLogger.Session("locket-client")
	locketClient, err := helpers.NewLocketClient(logger, cmd, Config)
	if err != nil {
		return NewCFDotComponentError(cmd, err)
	}

	err = ClaimLock(
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

func ValidateClaimLocksArguments(cmd *cobra.Command, args []string, lockKey, lockOwner, lockValue string, ttlInSeconds int) error {
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

func ClaimLock(
	stdout, stderr io.Writer,
	locketClient models.LocketClient,
	lockKey, lockOwner, lockValue string,
	ttlInSeconds int64) error {
	logger := globalLogger.Session("claim-lock")

	req := &models.LockRequest{
		Resource: &models.Resource{
			Key:      lockKey,
			Owner:    lockOwner,
			Value:    lockValue,
			TypeCode: models.LOCK,
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
