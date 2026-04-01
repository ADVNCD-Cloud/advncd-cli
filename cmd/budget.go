package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/advncdyaml"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpbilling"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpcrm"
)

var (
	budgetAmountEUR   float64
	budgetThresholds  string
	budgetBillingAcct string
	budgetDisplayName string
)

var budgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Billing budget guardrails",
}

var budgetInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a billing budget for the active project",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgStore, err := config.DefaultStore()
		if err != nil {
			return err
		}
		cfg, err := cfgStore.Load()
		if err != nil {
			return err
		}
		if cfg == nil || strings.TrimSpace(cfg.ProjectID) == "" {
			return fmt.Errorf("missing project config; run: advncd init")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		tb, err := auth.GetAccessToken(ctx)
		if err != nil {
			return err
		}

		project, err := gcpcrm.GetProject(ctx, tb.AccessToken, cfg.ProjectID)
		if err != nil {
			return err
		}

		billingAccount := strings.TrimSpace(budgetBillingAcct)
		if billingAccount == "" {
			info, err := gcpbilling.GetProjectBillingInfo(ctx, tb.AccessToken, cfg.ProjectID)
			if err == nil && info != nil && strings.TrimSpace(info.BillingAccountName) != "" {
				billingAccount = strings.TrimSpace(info.BillingAccountName)
			}
		}
		if billingAccount == "" {
			accounts, err := gcpbilling.ListOpenBillingAccounts(ctx, tb.AccessToken)
			if err != nil {
				return err
			}
			if len(accounts) == 1 {
				billingAccount = accounts[0].Name
			} else if len(accounts) > 1 {
				return fmt.Errorf("multiple billing accounts found; pass --billing-account")
			}
		}
		if billingAccount == "" {
			return fmt.Errorf("no billing account found for project; pass --billing-account")
		}

		if yamlCfg, err := advncdyaml.ReadFromRoot("."); err == nil && yamlCfg != nil {
			if !cmd.Flags().Changed("amount-eur") && yamlCfg.Guardrails.Budget.AmountEUR > 0 {
				budgetAmountEUR = yamlCfg.Guardrails.Budget.AmountEUR
			}
			if !cmd.Flags().Changed("thresholds") && strings.TrimSpace(yamlCfg.Guardrails.Budget.ThresholdsCSV) != "" {
				budgetThresholds = strings.TrimSpace(yamlCfg.Guardrails.Budget.ThresholdsCSV)
			}
		}

		thresholds := parseThresholdsCSV(budgetThresholds, []float64{0.5, 0.9, 1.0})
		if len(thresholds) == 0 {
			return fmt.Errorf("invalid --thresholds; expected csv like 0.5,0.9,1.0")
		}

		displayName := strings.TrimSpace(budgetDisplayName)
		if displayName == "" {
			displayName = "advncd-safety-budget-" + cfg.ProjectID
		}

		if err := gcpbilling.CreateProjectBudget(ctx, tb.AccessToken, gcpbilling.BudgetCreateInput{
			BillingAccountName: billingAccount,
			ProjectNumber:      strings.TrimSpace(project.ProjectNumber),
			DisplayName:        displayName,
			AmountEUR:          budgetAmountEUR,
			Thresholds:         thresholds,
		}); err != nil {
			return err
		}

		fmt.Println("✓ Budget initialized")
		fmt.Printf("project: %s\n", cfg.ProjectID)
		fmt.Printf("billing_account: %s\n", billingAccount)
		fmt.Printf("amount_eur: %.2f\n", budgetAmountEUR)
		fmt.Printf("thresholds: %s\n", budgetThresholds)
		return nil
	},
}

func init() {
	budgetInitCmd.Flags().Float64Var(&budgetAmountEUR, "amount-eur", 10.0, "Budget amount in EUR")
	budgetInitCmd.Flags().StringVar(&budgetThresholds, "thresholds", "0.5,0.9,1.0", "Alert thresholds csv (fractions)")
	budgetInitCmd.Flags().StringVar(&budgetBillingAcct, "billing-account", "", "Billing account id or billingAccounts/... (optional)")
	budgetInitCmd.Flags().StringVar(&budgetDisplayName, "display-name", "", "Budget display name (optional)")
	budgetCmd.AddCommand(budgetInitCmd)
}
