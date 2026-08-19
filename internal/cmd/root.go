package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "periscope",
	Short: "Periscope - Enterprise Telemetry Lakehouse",
	Long:  `Enterprise Telemetry Lakehouse for Laravel Telescope logs. A fast and robust telemetry collector.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
