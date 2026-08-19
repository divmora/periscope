package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync [project_iid]",
	Short: "Start a background collector to sync a project's telemetry data",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectIID := args[0]
		fmt.Printf("Starting Periscope sync worker for project IID: %s\n", projectIID)
		// TODO: Establish gRPC connection to central server and start collector polling
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
