package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/creds"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/llm"
)

type sseMsg struct {
	Type string `json:"type"` // "status" | "delta" | "done" | "error"
	Text string `json:"text"`
}

func writeSSE(w http.ResponseWriter, msg sseMsg) error {
	b, _ := json.Marshal(msg)
	_, err := fmt.Fprintf(w, "data: %s\n\n", b)
	return err
}

func serviceExplainStreamHandler(w http.ResponseWriter, r *http.Request) {
	name, ok := serviceNameFromPath(r) // должен уметь резать /explain/stream
	if !ok {
		http.NotFound(w, r)
		return
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// initial ping
	_ = writeSSE(w, sseMsg{Type: "status", Text: "Starting…"})
	fl.Flush()

	// Load config
	cfgStore, err := config.DefaultStore()
	if err != nil {
		_ = writeSSE(w, sseMsg{Type: "error", Text: "Failed to init config store: " + err.Error()})
		fl.Flush()
		return
	}
	cfg, err := cfgStore.Load()
	if err != nil || cfg == nil || cfg.ProjectID == "" || cfg.Region == "" {
		_ = writeSSE(w, sseMsg{Type: "error", Text: "Project not initialized. Run: advncd init"})
		fl.Flush()
		return
	}

	// Load creds
	credStore, err := creds.DefaultStore()
	if err != nil {
		_ = writeSSE(w, sseMsg{Type: "error", Text: "Failed to init creds store: " + err.Error()})
		fl.Flush()
		return
	}
	cr, err := credStore.Load()
	if err != nil || cr == nil || cr.AccessToken == "" {
		_ = writeSSE(w, sseMsg{Type: "error", Text: "Not authenticated. Run: advncd login"})
		fl.Flush()
		return
	}

	_ = writeSSE(w, sseMsg{Type: "status", Text: "Fetching service…"})
	fl.Flush()

	// Fetch service
	ctxRun, cancelRun := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancelRun()

	svc, err := gcprun.GetService(ctxRun, cr.AccessToken, cfg.ProjectID, cfg.Region, name)
	if err != nil {
		_ = writeSSE(w, sseMsg{Type: "error", Text: err.Error()})
		fl.Flush()
		return
	}

	_ = writeSSE(w, sseMsg{Type: "status", Text: "Generating explanation…"})
	fl.Flush()

	// LLM explain
	client, llmCfg, err := llm.NewFromEnv()
	if err != nil {
		_ = writeSSE(w, sseMsg{Type: "error", Text: err.Error()})
		fl.Flush()
		return
	}

	llmTimeout := llmCfg.Timeout
	if llmTimeout < 90*time.Second {
		llmTimeout = 90 * time.Second
	}
	ctxLLM, cancelLLM := context.WithTimeout(r.Context(), llmTimeout)
	defer cancelLLM()

	conds := make([]llm.ServiceCondition, 0, len(svc.Conditions))
	for _, c := range svc.Conditions {
		conds = append(conds, llm.ServiceCondition{Type: c.Type, State: c.State})
	}

	status := gcprun.DeriveStatus(svc.Conditions)

	err = llm.ExplainServiceStream(ctxLLM, client, llmCfg.Model, llm.ExplainServiceInput{
		Project:    cfg.ProjectID,
		Region:     cfg.Region,
		Service:    svc.Name,
		Status:     status,
		Image:      svc.Image,
		Conditions: conds,
	}, func(delta string) error {
		_ = writeSSE(w, sseMsg{Type: "delta", Text: delta})
		fl.Flush()
		return nil
	})
	if err != nil {
		_ = writeSSE(w, sseMsg{Type: "error", Text: err.Error()})
		fl.Flush()
		return
	}

	_ = writeSSE(w, sseMsg{Type: "done", Text: ""})
	fl.Flush()
}