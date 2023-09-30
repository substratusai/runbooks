package cli

import (
	"github.com/spf13/cobra"
)

var Version string

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sub",
		Short: "Substratus CLI",
	}

	cmd.AddCommand(notebookCommand())
	cmd.AddCommand(runCommand())
	cmd.AddCommand(getCommand())
	// cmd.AddCommand(inferCommand())
	cmd.AddCommand(deleteCommand())
	cmd.AddCommand(serveCommand())

	return cmd
}
