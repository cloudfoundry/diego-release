package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"code.cloudfoundry.org/bbs/models"
)

type CFDotError struct {
	err      error
	exitCode int
}

func (a CFDotError) Error() string {
	if err, ok := a.err.(*models.Error); ok {
		return fmt.Sprintf(`BBS error
Type %d: %s
Message: %s`,
			err.Type, err.Type.String(), err.Message)
	}

	return a.err.Error()
}

func (a CFDotError) ExitCode() int {
	return a.exitCode
}

func NewCFDotError(cmd *cobra.Command, err error) CFDotError {
	cmd.SilenceUsage = true

	if _, ok := err.(*models.Error); ok {
		return CFDotError{
			err:      err,
			exitCode: 4,
		}
	}

	return CFDotError{
		err:      err,
		exitCode: 5,
	}
}

func NewCFDotComponentError(cmd *cobra.Command, err error) CFDotError {
	cmd.SilenceUsage = true

	return CFDotError{
		err:      err,
		exitCode: 4,
	}
}

func NewCFDotValidationError(cmd *cobra.Command, err error) CFDotError {
	return CFDotError{
		err:      err,
		exitCode: 3,
	}
}
