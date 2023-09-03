package cli

import (
	"github.com/spf13/cobra"

	"github.com/substratusai/substratus/internal/cli/notebook"
)

var Version string

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "Substratus CLI",
	}
	cmd.AddCommand(notebook.Command())

	return cmd
}
