package commands

import (
	"fmt"
	"github.com/spf13/cobra"
	"org.openappstack/singularity/agent"
	"os"
	"os/signal"
	"syscall"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Singularity",
	Long:  `All software needs starting. So does Singularity`,
	Run: func(cmd *cobra.Command, args []string) {

		shutdownChannel := makeShutdownChannel()

		agent.Start()
		fmt.Println("Singularity started successfully...")

		//we block on this channel
		<-shutdownChannel

		agent.Stop()

		fmt.Println("Singularity stopped ")
	},
}

func makeShutdownChannel() chan os.Signal {
	//channel for catching signals of interest
	signalCatchingChannel := make(chan os.Signal)

	//catch Ctrl-C and Kill -9 signals
	signal.Notify(signalCatchingChannel, syscall.SIGINT, syscall.SIGTERM)

	return signalCatchingChannel
}
