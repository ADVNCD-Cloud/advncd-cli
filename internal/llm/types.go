package llm

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the minimal portable request we support across providers.
type ChatRequest struct {
	Model    string    // optional; provider default if empty
	Messages []Message  // required
	Temperature *float32 // optional
	MaxTokens   *int     // optional
}

// ChatResponse is a provider-agnostic response.
type ChatResponse struct {
	Text string
}

type Client interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}
