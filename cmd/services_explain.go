package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/creds"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/llm"
)

var servicesExplainCmd = &cobra.Command{
	Use:   "explain <service>",
	Short: "Explain Cloud Run service conditions using LLM (local Ollama by default)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svcName := args[0]

		// Load config
		cfgStore, err := config.DefaultStore()
		if err != nil {
			return err
		}
		cfg, err := cfgStore.Load()
		if err != nil || cfg == nil || cfg.ProjectID == "" || cfg.Region == "" {
			return apperr.New(config.StoreReadFailed).
				WithFix("Run: advncd init")
		}

		// Load creds
		credStore, err := creds.DefaultStore()
		if err != nil {
			return err
		}
		cr, err := credStore.Load()
		if err != nil || cr == nil || cr.AccessToken == "" {
			return apperr.New(apperr.AuthMissingClientID).
				WithFix("Run: advncd login")
		}

		if !cr.Expiry.IsZero() && time.Until(cr.Expiry) <= 0 {
			return apperr.New(apperr.AuthAuthTimeout).
				WithFix("Run: advncd login")
		}

		// Fetch service detail (the same data we already show in dashboard)
		client, llmCfg, err := llm.NewFromEnv()
		if err != nil {
			return err
		}

		// Один общий timeout: сетевые вызовы + LLM
		ctxTimeout := llmCfg.Timeout
		if ctxTimeout < 90*time.Second {
			ctxTimeout = 90 * time.Second // комфортный минимум для локальной модели
		}

		ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
		defer cancel()

		detail, err := gcprun.GetServiceForExplain(ctx, cr.AccessToken, cfg.ProjectID, cfg.Region, svcName)
		if err != nil {
			return err
		}

		// // Build LLM client from env
		// client, llmCfg, err := llm.NewFromEnv()
		// if err != nil {
		// 	return err
		// }

		// Convert conditions
		conds := make([]llm.ServiceCondition, 0, len(detail.Conditions))
		for _, c := range detail.Conditions {
			conds = append(conds, llm.ServiceCondition{Type: c.Type, State: c.State})
		}

		fmt.Println("Explaining service state with LLM...")
		fmt.Printf("  provider: %s\n", llmCfg.Provider)
		fmt.Printf("  model:    %s\n", llmCfg.Model)
		fmt.Println()

		// Warmup: first chat on Ollama can take time (model load).
		_, _ = client.Chat(ctx, llm.ChatRequest{
			Model: llmCfg.Model,
			Messages: []llm.Message{
				{Role: llm.RoleUser, Content: "Reply with OK."},
			},
		})

		text, err := llm.ExplainService(ctx, client, llmCfg.Model, llm.ExplainServiceInput{
			Project:    cfg.ProjectID,
			Region:     cfg.Region,
			Service:    svcName,
			Status:     detail.Status,
			Image:      detail.Image,
			Conditions: conds,
		})

		if err != nil {
			return err
		}

		fmt.Println(text)
		return nil
	},
}