package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpbilling"
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
	apisProject        string
	apisBillingAccount string
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
		failedErrByService := map[string]error{}
		billingLinked := false

		for _, svc := range services {
			state, err := gcpserviceusage.GetServiceState(ctx, tb.AccessToken, project.ProjectNumber, svc)
			if err == nil && state == "ENABLED" {
				fmt.Printf("%s: already enabled\n", svc)
				alreadyEnabled = append(alreadyEnabled, svc)
				continue
			}

			fmt.Printf("%s: enabling...\n", svc)
			if err := gcpserviceusage.EnableService(ctx, tb.AccessToken, project.ProjectNumber, svc); err != nil {
				if isBillingPreconditionError(err) {
					if !billingLinked {
						fmt.Println("billing: Cloud API enable requires a billing account linked to this project.")
						linkedAccount, linkErr := ensureProjectBilling(ctx, tb.AccessToken, projectID, strings.TrimSpace(apisBillingAccount))
						if linkErr != nil {
							fmt.Printf("billing: setup failed: %v\n", linkErr)
						} else {
							billingLinked = true
							fmt.Printf("billing: linked %s\n", linkedAccount)
						}
					}

					if billingLinked {
						fmt.Printf("%s: retrying after billing link...\n", svc)
						if retryErr := gcpserviceusage.EnableService(ctx, tb.AccessToken, project.ProjectNumber, svc); retryErr == nil {
							fmt.Printf("%s: enabled\n", svc)
							enabledNow = append(enabledNow, svc)
							continue
						} else {
							err = retryErr
						}
					}
				}

				fmt.Printf("%s: failed (%v)\n", svc, err)
				failed = append(failed, svc)
				failedErrByService[svc] = err
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
				if err := failedErrByService[svc]; err != nil {
					fmt.Printf("  - %s: %v\n", svc, err)
				} else {
					fmt.Printf("  - %s\n", svc)
				}
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
	apisEnableCmd.Flags().StringVar(&apisBillingAccount, "billing-account", "", "Billing account id or full resource name (optional)")
	apisCmd.AddCommand(apisEnableCmd)
}

func isBillingPreconditionError(err error) bool {
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		return false
	}

	text := strings.ToLower(err.Error())
	if appErr.Meta != nil {
		text += " " + strings.ToLower(appErr.Meta["raw_body"])
		text += " " + strings.ToLower(appErr.Meta["operation_error"])
	}

	return strings.Contains(text, "billing-enabled") ||
		strings.Contains(text, "billing account for project") ||
		strings.Contains(text, "ureq_project_billing_not_found")
}

func ensureProjectBilling(ctx context.Context, accessToken, projectID, preferred string) (string, error) {
	preferred = strings.TrimSpace(preferred)
	if preferred != "" {
		if err := gcpbilling.LinkProjectBilling(ctx, accessToken, projectID, preferred); err != nil {
			return "", err
		}
		return normalizeBillingAccountRef(preferred), nil
	}

	accounts, err := gcpbilling.ListOpenBillingAccounts(ctx, accessToken)
	if err != nil {
		fmt.Printf("billing: could not list billing accounts automatically (%v)\n", err)
		fmt.Println("billing: run `gcloud billing accounts list` if you need to discover account ids.")
		manual, promptErr := promptLine("Enter billing account id (XXXXXX-XXXXXX-XXXXXX) or full billingAccounts/... (blank to cancel)")
		if promptErr != nil {
			return "", promptErr
		}
		manual = strings.TrimSpace(manual)
		if manual == "" {
			return "", err
		}
		if linkErr := gcpbilling.LinkProjectBilling(ctx, accessToken, projectID, manual); linkErr != nil {
			return "", linkErr
		}
		return normalizeBillingAccountRef(manual), nil
	}
	if len(accounts) == 0 {
		manual, promptErr := promptLine("No open billing accounts visible. Enter billing account id manually (blank to cancel)")
		if promptErr != nil {
			return "", promptErr
		}
		manual = strings.TrimSpace(manual)
		if manual == "" {
			return "", fmt.Errorf("no open billing accounts found")
		}
		if linkErr := gcpbilling.LinkProjectBilling(ctx, accessToken, projectID, manual); linkErr != nil {
			return "", linkErr
		}
		return normalizeBillingAccountRef(manual), nil
	}

	sort.Slice(accounts, func(i, j int) bool {
		left := strings.TrimSpace(strings.ToLower(accounts[i].DisplayName + " " + accounts[i].Name))
		right := strings.TrimSpace(strings.ToLower(accounts[j].DisplayName + " " + accounts[j].Name))
		return left < right
	})

	fmt.Println("Select billing account:")
	for i, acc := range accounts {
		label := strings.TrimSpace(acc.DisplayName)
		if label == "" {
			label = acc.Name
		}
		fmt.Printf("  [%d] %s (%s)\n", i+1, label, acc.Name)
	}

	choice, err := readChoice(1, len(accounts))
	if err != nil {
		return "", err
	}
	selected := accounts[choice-1].Name

	if err := gcpbilling.LinkProjectBilling(ctx, accessToken, projectID, selected); err != nil {
		return "", err
	}
	return selected, nil
}

func normalizeBillingAccountRef(v string) string {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "billingAccounts/") {
		return v
	}
	return "billingAccounts/" + v
}
