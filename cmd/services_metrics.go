package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
)

var servicesMetricsCmd = &cobra.Command{
	Use:   "metrics <service>",
	Short: "Open Cloud Run service metrics in Cloud Monitoring",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		service := args[0]

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

		metricsURL := cloudRunMetricsURL(cfg.ProjectID, cfg.Region, service)
		_ = openBrowser(metricsURL)

		fmt.Println("Opened metrics in browser:")
		fmt.Printf("  Metrics: %s\n", metricsURL)
		return nil
	},
}

func cloudRunMetricsURL(projectID, region, serviceName string) string {
	// This opens Metrics Explorer pre-filtered for the service.
	// Users can switch metrics (requests, latency, errors) in UI.
	return "https://console.cloud.google.com/monitoring/metrics-explorer" +
		"?project=" + projectID +
		"&resource=cloud_run_revision" +
		"&resource.label.service_name=" + serviceName +
		"&resource.label.location=" + region
}