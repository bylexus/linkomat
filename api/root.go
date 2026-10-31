package api

import (
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "linkomat-server",
		Short:         "Linkomat server",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newServeCmd())
	root.AddCommand(newUserCmd())
	root.AddCommand(newVersionCmd())
	return root
}
