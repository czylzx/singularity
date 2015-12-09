package commands

import (
	"github.com/spf13/cobra"
)

//MainCmd is Singularity's root command. Every other command is it's child.
var MainCmd = &cobra.Command{
	Use:   "singularity",
	Short: "singularity manages the sdn controllers",
	Long:  `singularity is the main command.`,
}

//Execute adds all subcommands to the root command MainCmd
func Execute() {
	AddSubcommands()
	MainCmd.Execute()
}

//AddSubcommands adds child commands to the root command MainCmd.
func AddSubcommands() {
	MainCmd.AddCommand(versionCmd)
	MainCmd.AddCommand(startCmd)
}
