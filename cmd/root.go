package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/ui"
)

var rootCmd = &cobra.Command{
	Use:           "advncd",
	Short:         "Advncd — local-first developer platform for Google Cloud",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// Pretty print known errors
		if ae, ok := err.(*apperr.Error); ok {
			ui.PrintError(ae)
			os.Exit(1)
		}
		// Fallback
		ui.PrintPlainError(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logoutCmd)

	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(publishCmd)
	rootCmd.AddCommand(projectsCmd)
	rootCmd.AddCommand(n8nCmd)

	servicesCmd.AddCommand(servicesDescribeCmd)
	servicesCmd.AddCommand(servicesOpenCmd)
	servicesCmd.AddCommand(servicesLogsCmd)
	servicesCmd.AddCommand(servicesMetricsCmd)
	rootCmd.AddCommand(servicesCmd)

	llmCmd.AddCommand(llmStatusCmd)
	rootCmd.AddCommand(llmCmd)
	servicesCmd.AddCommand(servicesExplainCmd)

	rootCmd.AddCommand(dashboardCmd)
	authCmd.AddCommand(authPrintAccessTokenCmd)
}
