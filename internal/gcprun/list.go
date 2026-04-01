package gcprun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var ErrRunList = apperr.E("D-RUN-001", "Failed to list Cloud Run services")

type Service struct {
	Name      string
	Status    string
	URL       string
	UpdatedAt time.Time
}

// type Condition struct {
// 	Type  string `json:"type"`
// 	State string `json:"state"`
// }

type listResp struct {
	Services []struct {
		Name       string      `json:"name"`
		URI        string      `json:"uri,omitempty"`
		Conditions []Condition `json:"conditions,omitempty"`
		UpdateTime string      `json:"updateTime,omitempty"`
	} `json:"services"`
}

func ListServices(ctx context.Context, accessToken, projectID, region string) ([]Service, error) {
	u := fmt.Sprintf(
		"https://run.googleapis.com/v2/projects/%s/locations/%s/services",
		projectID,
		region,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, apperr.New(ErrRunList).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, apperr.New(ErrRunList).WithCause(err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, apperr.New(ErrRunList).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(raw)).
			WithFix("Ensure Cloud Run API is enabled and you have access to list services.")
	}

	var out listResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, apperr.New(ErrRunList).WithCause(err).
			WithMeta("raw_body", string(raw))
	}

	var services []Service
	for _, s := range out.Services {
		name := shortName(s.Name)
		status := DeriveStatus(s.Conditions)

		services = append(services, Service{
			Name:      name,
			Status:    status,
			URL:       s.URI,
			UpdatedAt: parseRFC3339OrZero(s.UpdateTime),
		})
	}

	return services, nil
}

func parseRFC3339OrZero(v string) time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return time.Time{}
	}
	return t
}

func shortName(full string) string {
	// projects/*/locations/*/services/{name}
	parts := strings.Split(full, "/")
	if len(parts) == 0 {
		return full
	}
	return parts[len(parts)-1]
}

// func deriveStatus(conds []Condition) string {
// 	// Cloud Run v2 often reports RoutesReady + ConfigurationsReady instead of Ready.
// 	var routes, config string

// 	for _, c := range conds {
// 		switch c.Type {
// 		case "Ready":
// 			switch c.State {
// 			case "CONDITION_SUCCEEDED":
// 				return "READY"
// 			case "CONDITION_FAILED":
// 				return "ERROR"
// 			default:
// 				return "UNKNOWN"
// 			}
// 		case "RoutesReady":
// 			routes = c.State
// 		case "ConfigurationsReady":
// 			config = c.State
// 		}
// 	}

// 	if routes == "CONDITION_FAILED" || config == "CONDITION_FAILED" {
// 		return "ERROR"
// 	}
// 	if routes == "CONDITION_SUCCEEDED" && config == "CONDITION_SUCCEEDED" {
// 		return "READY"
// 	}

// 	return "UNKNOWN"
// }
