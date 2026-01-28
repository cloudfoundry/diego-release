package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func ValidateConflictingShortAndLongFlag(short string, long string, cmd *cobra.Command) error {
	errorConflictingShortAndLongFlagPassed := fmt.Errorf("Only one of %s and %s should be passed", short, long)

	if contains(os.Args, short) && contains(os.Args, long) {
		return NewCFDotValidationError(cmd, errorConflictingShortAndLongFlagPassed)
	}

	return nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}

	return false
}
