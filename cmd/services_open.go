package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
)

var servicesOpenCmd = &cobra.Command{
	Use:   "open <service>",
	Short: "Open Cloud Run service in browser (URL + Console)",
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

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		tb, err := auth.GetAccessToken(ctx)
		if err != nil {
			return err
		}

		svc, err := gcprun.GetService(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region, name)
		if err != nil {
			return err
		}

		// 1) open public URL (if present)
		if svc.URL != "" {
			_ = openBrowser(svc.URL)
		} else {
			fmt.Println("Service has no public URL (maybe not ready yet).")
		}

		// 2) open Cloud Run console page
		consoleURL := cloudRunConsoleURL(cfg.ProjectID, cfg.Region, svc.Name)
		_ = openBrowser(consoleURL)

		fmt.Println("Opened in browser:")
		if svc.URL != "" {
			fmt.Printf("  URL:     %s\n", svc.URL)
		}
		fmt.Printf("  Console: %s\n", consoleURL)
		return nil
	},
}

func cloudRunConsoleURL(projectID, region, serviceName string) string {
	// Works in modern GCP console: cloud run service details
	return "https://console.cloud.google.com/run/detail/" + region + "/" + serviceName + "?project=" + projectID
}
