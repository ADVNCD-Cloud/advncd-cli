package llm

import (
	"os"
	"strings"
	"time"
)

type Provider string

const (
	ProviderOllama Provider = "ollama"
)

type Config struct {
	Provider Provider
	Model    string
	BaseURL  string
	Timeout  time.Duration
}

func LoadConfigFromEnv() Config {
	p := strings.TrimSpace(os.Getenv("ADVNCD_LLM_PROVIDER"))
	if p == "" {
		p = string(ProviderOllama)
	}

	model := strings.TrimSpace(os.Getenv("ADVNCD_LLM_MODEL"))
	if model == "" {
		model = "llama3.2"
	}

	base := strings.TrimSpace(os.Getenv("ADVNCD_LLM_BASE_URL"))
	if base == "" {
		base = "http://localhost:11434"
	}

	// Keep it conservative; L1 is short.
	timeout := 20 * time.Second
	if v := strings.TrimSpace(os.Getenv("ADVNCD_LLM_TIMEOUT")); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	}

	return Config{
		Provider: Provider(p),
		Model:    model,
		BaseURL:  base,
		Timeout:  timeout,
	}
}
