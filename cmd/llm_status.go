package cmd

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/llm"
)

var llmStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current LLM configuration and connectivity",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Read config from env (optionally loaded from .env if you enable autoload)
		cfg := llm.LoadConfigFromEnv()

		fmt.Println("llm:")
		fmt.Printf("  provider: %s\n", cfg.Provider)
		fmt.Printf("  model:    %s\n", cfg.Model)
		fmt.Printf("  base_url: %s\n", cfg.BaseURL)
		fmt.Printf("  timeout:  %s\n", cfg.Timeout)

		// Simple connectivity check for Ollama
		if cfg.Provider == llm.ProviderOllama {
			u := cfg.BaseURL
			if u == "" {
				u = "http://localhost:11434"
			}
			client := &http.Client{Timeout: 2 * time.Second}
			res, err := client.Get(u + "/api/tags")
			if err != nil {
				fmt.Printf("  reachable: no (%v)\n", err)
				return nil
			}
			_ = res.Body.Close()
			if res.StatusCode >= 200 && res.StatusCode < 300 {
				fmt.Printf("  reachable: yes (%s)\n", res.Status)
			} else {
				fmt.Printf("  reachable: no (%s)\n", res.Status)
			}
		}

		return nil
	},
}