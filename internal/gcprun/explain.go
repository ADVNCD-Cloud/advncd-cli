package gcprun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ExplainCondition struct {
	Type  string `json:"type"`
	State string `json:"state"`
}

type ExplainService struct {
	Name       string
	URL        string
	Status     string
	Image      string
	Conditions []ExplainCondition
}

type runServiceV2 struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
	// status.conditions in v2
	Conditions []struct {
		Type  string `json:"type"`
		State string `json:"state"`
	} `json:"conditions"`
	Template struct {
		Containers []struct {
			Image string `json:"image"`
		} `json:"containers"`
	} `json:"template"`
}

// GetServiceForExplain fetches minimal Cloud Run service data for LLM "Explain".
func GetServiceForExplain(ctx context.Context, accessToken, projectID, region, service string) (*ExplainService, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, fmt.Errorf("missing access token (run advncd login)")
	}
	if projectID == "" || region == "" || service == "" {
		return nil, fmt.Errorf("missing project/region/service")
	}

	u := fmt.Sprintf("https://run.googleapis.com/v2/projects/%s/locations/%s/services/%s", projectID, region, service)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("cloud run describe failed: %s: %s", res.Status, string(raw))
	}

	var v2 runServiceV2
	if err := json.Unmarshal(raw, &v2); err != nil {
		return nil, fmt.Errorf("failed to decode Cloud Run response: %w", err)
	}

	out := &ExplainService{
		Name:   service,
		URL:    v2.URI,
		Status: deriveExplainStatus(v2.Conditions),
	}

	if len(v2.Template.Containers) > 0 {
		out.Image = v2.Template.Containers[0].Image
	}

	out.Conditions = make([]ExplainCondition, 0, len(v2.Conditions))
	for _, c := range v2.Conditions {
		out.Conditions = append(out.Conditions, ExplainCondition{
			Type:  c.Type,
			State: c.State,
		})
	}

	return out, nil
}

// Minimal, opinionated status derivation for Explain.
func deriveExplainStatus(conds []struct {
	Type  string `json:"type"`
	State string `json:"state"`
}) string {
	// Prefer the Cloud Run readiness signal if present
	for _, c := range conds {
		if c.Type == "RoutesReady" || c.Type == "Ready" {
			switch c.State {
			case "CONDITION_SUCCEEDED":
				return "READY"
			case "CONDITION_FAILED":
				return "NOT_READY"
			default:
				return "UNKNOWN"
			}
		}
	}

	// Fallback: if any failed -> NOT_READY
	for _, c := range conds {
		if c.State == "CONDITION_FAILED" {
			return "NOT_READY"
		}
	}
	// If any succeeded -> maybe ok
	for _, c := range conds {
		if c.State == "CONDITION_SUCCEEDED" {
			return "READY"
		}
	}
	return "UNKNOWN"
}