package commands

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "velocity",
		Short:         "Velocity Cache CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newInitCommand())
	root.AddCommand(newRunCommand())
	root.AddCommand(newCleanCommand())
	root.AddCommand(newLoginCommand())
	root.AddCommand(newLinkCommand())

	return root
}

func Execute() error {
	return NewRootCommand().Execute()
}
