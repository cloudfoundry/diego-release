package main

import (
	"os"
	"strings"

	"code.cloudfoundry.org/cfdot/commands"
)

func main() {
	if err := commands.RootCmd.Execute(); err != nil {
		if cfDotError, ok := err.(commands.CFDotError); ok {
			os.Exit(cfDotError.ExitCode())
		}

		if strings.Contains(err.Error(), "invalid argument") {
			os.Exit(3)
		}

		os.Exit(-1)
	}
}
