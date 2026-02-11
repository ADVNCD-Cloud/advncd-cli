package smoke

import (
	"context"
	"fmt"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/llm"
)

func Run() error {
	client, cfg, err := llm.NewFromEnv()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	text, err := llm.ExplainService(ctx, client, cfg.Model, llm.ExplainServiceInput{
		Project: "demo",
		Region:  "europe-west3",
		Service: "example",
		Status:  "NOT_READY",
		Conditions: []llm.ServiceCondition{
			{Type: "RoutesReady", State: "CONDITION_FAILED"},
			{Type: "ConfigurationsReady", State: "CONDITION_SUCCEEDED"},
		},
	})
	if err != nil {
		return err
	}

	fmt.Println("LLM OK:", time.Now().Format(time.RFC3339))
	fmt.Println(text)
	return nil
}