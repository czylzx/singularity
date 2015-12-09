package commands

import (
	"fmt"
	"github.com/spf13/cobra"
)

// The git commit that was compiled
var MercurialCommit string

// The complete release version number
var ReleaseVersion string

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Singularity",
	Long:  `All software has versions. This is Singularity's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Singularity Version: %s, Commit: %s\n", ReleaseVersion, MercurialCommit)
	},
}
