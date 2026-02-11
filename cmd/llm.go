package cmd

import (
	"github.com/spf13/cobra"
)

var llmCmd = &cobra.Command{
	Use:   "llm",
	Short: "LLM helpers (local Ollama / API providers)",
}