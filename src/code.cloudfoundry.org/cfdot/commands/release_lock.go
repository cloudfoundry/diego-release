package commands

import (
	"context"
	"errors"
	"io"

	"code.cloudfoundry.org/cfdot/commands/helpers"
	"code.cloudfoundry.org/locket/models"
	"github.com/spf13/cobra"
)

var releaseLockCmd = &cobra.Command{
	Use:   "release-lock",
	Short: "Release Locket lock",
	Long:  "Releases a Locket lock with the given key and owner",
	RunE:  releaseLock,
}

func init() {
	AddLocketFlags(releaseLockCmd)
	releaseLockCmd.Flags().StringVarP(&lockKey, "key", "k", "", "the key of the lock being releaseed")
	releaseLockCmd.Flags().StringVarP(&lockOwner, "owner", "o", "", "the lock owner")
	RootCmd.AddCommand(releaseLockCmd)
}

func releaseLock(cmd *cobra.Command, args []string) error {
	err := ValidateReleaseLocksArguments(cmd, args, lockKey, lockOwner)
	if err != nil {
		return NewCFDotValidationError(cmd, err)
	}

	logger := globalLogger.Session("locket-client")
	locketClient, err := helpers.NewLocketClient(logger, cmd, Config)
	if err != nil {
		return NewCFDotComponentError(cmd, err)
	}

	err = ReleaseLock(
		cmd.OutOrStdout(),
		cmd.OutOrStderr(),
		locketClient,
		lockKey,
		lockOwner,
	)
	if err != nil {
		return NewCFDotComponentError(cmd, err)
	}

	return nil
}

func ValidateReleaseLocksArguments(cmd *cobra.Command, args []string, lockKey, lockOwner string) error {
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

	if lockKey == "" {
		return NewCFDotValidationError(cmd, errors.New("key cannot be empty"))
	}

	if lockOwner == "" {
		return NewCFDotValidationError(cmd, errors.New("owner cannot be empty"))
	}

	return nil
}

func ReleaseLock(
	stdout, stderr io.Writer,
	locketClient models.LocketClient,
	lockKey, lockOwner string,
) error {
	logger := globalLogger.Session("release-lock")

	req := &models.ReleaseRequest{
		Resource: &models.Resource{
			Key:   lockKey,
			Owner: lockOwner,
		},
	}
	_, err := locketClient.Release(context.Background(), req)
	if err != nil {
		return err
	}

	logger.Info("completed")
	return nil
}
