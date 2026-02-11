package llm

import (
	"context"
	"encoding/json"
	"strings"
)

type ServiceCondition struct {
	Type  string `json:"type"`
	State string `json:"state"`
}

type ExplainServiceInput struct {
	Project    string            `json:"project"`
	Region     string            `json:"region"`
	Service    string            `json:"service"`
	Status     string            `json:"status"`
	Image      string            `json:"image,omitempty"`
	Conditions []ServiceCondition `json:"conditions"`
}

// ExplainService returns a concise explanation text.
func ExplainService(ctx context.Context, c Client, model string, in ExplainServiceInput) (string, error) {
	payload, _ := json.MarshalIndent(in, "", "  ")

	system := strings.TrimSpace(`
You are a Google Cloud Run expert.

Explain the following Cloud Run service state to a developer:
- What each condition means (in plain language)
- Which condition is blocking readiness (if any)
- Common reasons for this situation
- What a developer typically checks next (high-level, no step-by-step commands)

Constraints:
- Do NOT invent missing data.
- Be concise, structured (bullets are ok).
- Do NOT include credentials or secrets.
`)

	user := "Service state:\n\n```json\n" + string(payload) + "\n```\n"

	req := ChatRequest{
		Model: model,
		Messages: []Message{
			{Role: RoleSystem, Content: system},
			{Role: RoleUser, Content: user},
		},
	}

	resp, err := c.Chat(ctx, req)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Text), nil
}

func ExplainServiceStream(
	ctx context.Context,
	c Client,
	model string,
	in ExplainServiceInput,
	onDelta func(string) error,
) error {
	if onDelta == nil {
		return New(ErrConfig, "onDelta is nil")
	}

	payload, _ := json.MarshalIndent(in, "", "  ")

	system := strings.TrimSpace(`
You are a Google Cloud Run expert.

Explain the following Cloud Run service state to a developer:
- What each condition means (in plain language)
- Which condition is blocking readiness (if any)
- Common reasons for this situation
- What a developer typically checks next (high-level, no step-by-step commands)

Constraints:
- Do NOT invent missing data.
- Be concise, structured (bullets are ok).
- Do NOT include credentials or secrets.
`)

	user := "Service state:\n\n```json\n" + string(payload) + "\n```\n"

	req := ChatRequest{
		Model: model,
		Messages: []Message{
			{Role: RoleSystem, Content: system},
			{Role: RoleUser, Content: user},
		},
	}

	// If client supports streaming, use it.
	type streamer interface {
		ChatStream(context.Context, ChatRequest, func(string) error) error
	}
	if s, ok := c.(streamer); ok {
		return s.ChatStream(ctx, req, onDelta)
	}

	// Fallback: non-stream provider -> one chunk.
	resp, err := c.Chat(ctx, req)
	if err != nil {
		return err
	}
	return onDelta(strings.TrimSpace(resp.Text))
}	