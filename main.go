package main

import (
	"org.openappstack/singularity/commands"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// make the release version and commit info available to the version command
	commands.MercurialCommit = MercurialCommit
	commands.ReleaseVersion = ReleaseVersion

	commands.Execute()
}
