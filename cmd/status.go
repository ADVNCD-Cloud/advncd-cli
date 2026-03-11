package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpcrm"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpserviceusage"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show local status (auth + config + API readiness)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		si, err := auth.GetSessionInfo()
		if err != nil {
			return err
		}

		// Probe broker + gcp access token
		tb, err := auth.GetAccessToken(ctx)
		if err != nil {
			return err
		}

		// Config (project/region)
		cfgStore, err := config.DefaultStore()
		if err != nil {
			return err
		}
		cfg, err := cfgStore.Load()
		if err != nil {
			return err
		}

		fmt.Println("auth: ok")
		fmt.Printf("email: %s\n", si.Email)
		fmt.Printf("app_access_expires_in: %s\n", time.Until(si.AppExpiry).Truncate(time.Second))
		fmt.Printf("auth_base_url: %s\n", si.AuthBaseURL)
		fmt.Println("gcp_access_token: ok")
		fmt.Printf("gcp_access_expires_in: %s\n", time.Until(tb.Expiry).Truncate(time.Second))
		fmt.Printf("creds: %s\n", si.CredsPath)

		fmt.Println()
		if cfg == nil || cfg.ProjectID == "" || cfg.Region == "" {
			fmt.Println("config: not set")
			fmt.Printf("config_path: %s\n", cfgStore.Path)
			fmt.Println("fix: run `advncd init`")
			return nil
		}

		fmt.Printf("project: %s\n", cfg.ProjectID)
		fmt.Printf("region: %s\n", cfg.Region)
		fmt.Printf("config: %s\n", cfgStore.Path)

		// ---- B4: API readiness checks ----
		fmt.Println()
		fmt.Println("apis:")

		// Service Usage prefers projectNumber in resource names
		p, err := gcpcrm.GetProject(ctx, tb.AccessToken, cfg.ProjectID)
		if err != nil {
			// Don't fail whole status for readiness; show hint and exit gracefully.
			fmt.Println("  (unable to resolve project number; skipping API checks)")
			fmt.Println("  fix: ensure you have access to this project")
			return nil
		}
		projectNumber := p.ProjectNumber

		missing := []string{}

		for _, svc := range requiredGoogleAPIs {
			state, err := gcpserviceusage.GetServiceState(ctx, tb.AccessToken, projectNumber, svc)
			if err != nil {
				// If we can't query one service, show unknown but continue.
				fmt.Printf("  %s: unknown\n", svc)
				continue
			}
			switch state {
			case "ENABLED":
				fmt.Printf("  %s: enabled\n", svc)
			default:
				fmt.Printf("  %s: disabled\n", svc)
				missing = append(missing, svc)
			}
		}

		if len(missing) > 0 {
			fmt.Println()
			fmt.Println("fix: enable missing APIs in Google Cloud Console → APIs & Services → Library")
			fmt.Println("missing:")
			for _, m := range missing {
				fmt.Printf("  - %s\n", m)
			}
			// (Optional hint; still no gcloud dependency)
			fmt.Println("fix: run `advncd apis enable` to enable missing APIs from CLI.")
		}

		return nil
	},
}
