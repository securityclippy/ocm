package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit  = "none"
)

var rootCmd = &cobra.Command{
	Use:   "ocm",
	Short: "OpenClaw Credential Manager",
	Long: `OCM (OpenClaw Credential Manager) is a secure credential management sidecar.

It stores credentials outside the agent's process and requires human approval
for sensitive operations, injecting credentials via environment variables.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(keygenCmd)
}
