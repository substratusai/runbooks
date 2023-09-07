package cli

import (
	"github.com/spf13/cobra"
)

var Version string

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "strat",
		Short: "Substratus CLI",
	}
	cmd.AddCommand(notebookCommand())
	cmd.AddCommand(runCommand())
	cmd.AddCommand(listCommand())

	return cmd
}
