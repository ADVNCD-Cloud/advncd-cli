package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type ollamaClient struct {
	baseURL string
	model   string
	http    *http.Client
}

type ollamaChatStreamResp struct {
	Message Message `json:"message"`
	Done    bool    `json:"done,omitempty"`
	Error   string  `json:"error,omitempty"`
}

func newOllamaClient(baseURL, model string, httpClient *http.Client) *ollamaClient {
	baseURL = strings.TrimRight(baseURL, "/")
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &ollamaClient{baseURL: baseURL, model: model, http: httpClient}
}

// Ollama /api/chat request/response (stream=false).
type ollamaChatReq struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Options  map[string]any `json:"options,omitempty"`
}

type ollamaChatResp struct {
	Message Message `json:"message"`
	Error   string  `json:"error,omitempty"`
}

func (c *ollamaClient) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	model := c.model
	if strings.TrimSpace(req.Model) != "" {
		model = strings.TrimSpace(req.Model)
	}
	if model == "" {
		return ChatResponse{}, New(ErrConfig, "ollama model is empty")
	}
	if len(req.Messages) == 0 {
		return ChatResponse{}, New(ErrConfig, "messages are empty")
	}

	creq := ollamaChatReq{
		Model:    model,
		Messages: req.Messages,
		Stream:   false,
	}

	// Optional options mapping
	if req.Temperature != nil || req.MaxTokens != nil {
		creq.Options = map[string]any{}
		if req.Temperature != nil {
			creq.Options["temperature"] = *req.Temperature
		}
		if req.MaxTokens != nil {
			creq.Options["num_predict"] = *req.MaxTokens
		}
	}

	body, _ := json.Marshal(creq)
	url := c.baseURL + "/api/chat"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return ChatResponse{}, Wrap(ErrHTTP, "build request failed", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(httpReq)
	if err != nil {
		return ChatResponse{}, Wrap(ErrHTTP, "request failed", err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ChatResponse{}, New(ErrHTTP, "ollama HTTP "+res.Status+": "+string(raw))
	}

	var out ollamaChatResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return ChatResponse{}, Wrap(ErrDecode, "decode response failed", err)
	}
	if out.Error != "" {
		return ChatResponse{}, New(ErrProvider, "ollama error: "+out.Error)
	}

	return ChatResponse{Text: out.Message.Content}, nil
}

func (c *ollamaClient) ChatStream(ctx context.Context, req ChatRequest, onDelta func(string) error) error {
	model := c.model
	if strings.TrimSpace(req.Model) != "" {
		model = strings.TrimSpace(req.Model)
	}
	if model == "" {
		return New(ErrConfig, "ollama model is empty")
	}
	if len(req.Messages) == 0 {
		return New(ErrConfig, "messages are empty")
	}
	if onDelta == nil {
		return New(ErrConfig, "onDelta is nil")
	}

	creq := ollamaChatReq{
		Model:    model,
		Messages: req.Messages,
		Stream:   true,
	}

	// Optional options mapping
	if req.Temperature != nil || req.MaxTokens != nil {
		creq.Options = map[string]any{}
		if req.Temperature != nil {
			creq.Options["temperature"] = *req.Temperature
		}
		if req.MaxTokens != nil {
			creq.Options["num_predict"] = *req.MaxTokens
		}
	}

	body, _ := json.Marshal(creq)
	url := c.baseURL + "/api/chat"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Wrap(ErrHTTP, "build request failed", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(httpReq)
	if err != nil {
		return Wrap(ErrHTTP, "request failed", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		raw, _ := io.ReadAll(res.Body)
		return New(ErrHTTP, "ollama HTTP "+res.Status+": "+string(raw))
	}

	sc := bufio.NewScanner(res.Body)
	// NB: long chunks -> increase buffer
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		var out ollamaChatStreamResp
		if err := json.Unmarshal([]byte(line), &out); err != nil {
			return Wrap(ErrDecode, "decode stream chunk failed", err)
		}
		if out.Error != "" {
			return New(ErrProvider, "ollama error: "+out.Error)
		}

		// delta text (ollama sends incremental message content)
		if out.Message.Content != "" {
			if err := onDelta(out.Message.Content); err != nil {
				return err
			}
		}

		if out.Done {
			return nil
		}
	}

	if err := sc.Err(); err != nil {
		return Wrap(ErrHTTP, "stream read failed", err)
	}
	return nil
}