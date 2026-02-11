package llm

import "net/http"

func NewFromEnv() (Client, Config, error) {
	cfg := LoadConfigFromEnv()

	switch cfg.Provider {
	case ProviderOllama:
		hc := &http.Client{Timeout: cfg.Timeout}
		return newOllamaClient(cfg.BaseURL, cfg.Model, hc), cfg, nil
	default:
		return nil, cfg, New(ErrProvider, "unknown LLM provider: "+string(cfg.Provider))
	}
}