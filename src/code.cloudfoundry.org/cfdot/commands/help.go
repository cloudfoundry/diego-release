package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var helpCmd = &cobra.Command{
	Use:   "help CMD",
	Short: "Get help on [command]",
	Long:  "Get help on using cfdot commands",
	RunE:  help,
}

func init() {
	RootCmd.AddCommand(helpCmd)
}

func help(cmd *cobra.Command, args []string) error {
	if len(args) == 0 || args[0] == "" {
		cmd.HelpFunc()(cmd, args)
		return nil
	}

	for _, c := range cmd.Root().Commands() {
		if c.Name() == args[0] {
			c.HelpFunc()(c, args)
			return nil
		}
	}

	return NewCFDotValidationError(cmd, fmt.Errorf("'%s' is not a valid command", args[0]))
}
