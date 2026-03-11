package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpcrm"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpserviceusage"
)

var requiredGoogleAPIs = []string{
	"run.googleapis.com",
	"cloudbuild.googleapis.com",
	"artifactregistry.googleapis.com",
	"monitoring.googleapis.com",
}

var (
	apisProject string
)

var apisCmd = &cobra.Command{
	Use:   "apis",
	Short: "Manage required Google APIs",
}

var apisEnableCmd = &cobra.Command{
	Use:   "enable [api ...]",
	Short: "Enable required Google APIs (or specific APIs)",
	Long:  "Enable Google APIs for the current project.\nIf no API list is provided, enables the default required set.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		tb, err := auth.GetAccessToken(ctx)
		if err != nil {
			return err
		}

		projectID, err := resolveProjectID(strings.TrimSpace(apisProject))
		if err != nil {
			return err
		}

		project, err := gcpcrm.GetProject(ctx, tb.AccessToken, projectID)
		if err != nil {
			return err
		}

		services := normalizeAPIList(args)
		if len(services) == 0 {
			services = append([]string(nil), requiredGoogleAPIs...)
		}

		fmt.Printf("project: %s\n", projectID)
		fmt.Printf("project_number: %s\n", project.ProjectNumber)
		fmt.Println("enabling apis:")
		for _, s := range services {
			fmt.Printf("  - %s\n", s)
		}
		fmt.Println()

		enabledNow := make([]string, 0, len(services))
		alreadyEnabled := make([]string, 0)
		failed := make([]string, 0)

		for _, svc := range services {
			state, err := gcpserviceusage.GetServiceState(ctx, tb.AccessToken, project.ProjectNumber, svc)
			if err == nil && state == "ENABLED" {
				fmt.Printf("%s: already enabled\n", svc)
				alreadyEnabled = append(alreadyEnabled, svc)
				continue
			}

			fmt.Printf("%s: enabling...\n", svc)
			if err := gcpserviceusage.EnableService(ctx, tb.AccessToken, project.ProjectNumber, svc); err != nil {
				fmt.Printf("%s: failed\n", svc)
				failed = append(failed, svc)
				continue
			}

			fmt.Printf("%s: enabled\n", svc)
			enabledNow = append(enabledNow, svc)
		}

		fmt.Println()
		fmt.Printf("summary: enabled_now=%d already_enabled=%d failed=%d\n", len(enabledNow), len(alreadyEnabled), len(failed))
		if len(failed) > 0 {
			fmt.Println("failed:")
			for _, svc := range failed {
				fmt.Printf("  - %s\n", svc)
			}
			return fmt.Errorf("failed to enable one or more APIs")
		}

		return nil
	},
}

func resolveProjectID(override string) (string, error) {
	if override != "" {
		return override, nil
	}

	cfgStore, err := config.DefaultStore()
	if err != nil {
		return "", err
	}

	cfg, err := cfgStore.Load()
	if err != nil {
		return "", err
	}

	if cfg == nil || strings.TrimSpace(cfg.ProjectID) == "" {
		return "", fmt.Errorf("project is not set; run `advncd init` or pass --project")
	}

	return strings.TrimSpace(cfg.ProjectID), nil
}

func normalizeAPIList(items []string) []string {
	if len(items) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, raw := range items {
		svc := normalizeAPIName(raw)
		if svc == "" {
			continue
		}
		if _, ok := seen[svc]; ok {
			continue
		}
		seen[svc] = struct{}{}
		out = append(out, svc)
	}
	return out
}

func normalizeAPIName(raw string) string {
	svc := strings.ToLower(strings.TrimSpace(raw))
	if svc == "" {
		return ""
	}

	if !strings.Contains(svc, ".") {
		return svc + ".googleapis.com"
	}

	return svc
}

func init() {
	apisEnableCmd.Flags().StringVar(&apisProject, "project", "", "GCP project id override (optional)")
	apisCmd.AddCommand(apisEnableCmd)
}
