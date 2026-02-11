package cmd

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
)

var servicesLogsCmd = &cobra.Command{
	Use:   "logs <service>",
	Short: "Open Cloud Run service logs in Cloud Logging",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfgStore, err := config.DefaultStore()
		if err != nil {
			return err
		}
		cfg, err := cfgStore.Load()
		if err != nil {
			return err
		}
		if cfg == nil || cfg.ProjectID == "" || cfg.Region == "" {
			return fmt.Errorf("missing project/region config; run: advncd init")
		}

		logsURL := cloudRunLogsURL(cfg.ProjectID, cfg.Region, name)
		_ = openBrowser(logsURL)

		fmt.Println("Opened logs in browser:")
		fmt.Printf("  Logs: %s\n", logsURL)
		return nil
	},
}

func cloudRunLogsURL(projectID, region, serviceName string) string {
	// Logging query:
	// resource.type="cloud_run_revision"
	// resource.labels.service_name="example"
	// resource.labels.location="europe-west3"
	query := `resource.type="cloud_run_revision"
			resource.labels.service_name="` + serviceName + `"
			resource.labels.location="` + region + `"`

	return "https://console.cloud.google.com/logs/query;query=" +
		urlQueryEscape(query) +
		"?project=" + projectID
}

func urlQueryEscape(s string) string {
	return url.QueryEscape(s)
}