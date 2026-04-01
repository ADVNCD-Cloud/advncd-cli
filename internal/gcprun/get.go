package gcprun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var ErrRunGetService = apperr.E("D-RUN-002", "Failed to get Cloud Run service")

type ServiceDetail struct {
	Name           string
	URL            string
	Image          string
	Env            map[string]string
	SecretEnvKeys  []string
	Memory         string
	TimeoutSeconds int
	MinInstances   int
	MaxInstances   int
	Conditions     []Condition
	Status         string
	LatestRevision string
}

type getResp struct {
	Name                  string      `json:"name"`
	URI                   string      `json:"uri,omitempty"`
	Conditions            []Condition `json:"conditions,omitempty"`
	LatestReadyRevision   string      `json:"latestReadyRevision,omitempty"`
	LatestCreatedRevision string      `json:"latestCreatedRevision,omitempty"`
	Scaling               struct {
		MinInstanceCount int `json:"minInstanceCount,omitempty"`
		MaxInstanceCount int `json:"maxInstanceCount,omitempty"`
	} `json:"scaling,omitempty"`
	Template struct {
		Timeout    string `json:"timeout,omitempty"`
		Containers []struct {
			Image     string `json:"image"`
			Resources struct {
				Limits map[string]string `json:"limits,omitempty"`
			} `json:"resources,omitempty"`
			Env []struct {
				Name        string `json:"name"`
				Value       string `json:"value,omitempty"`
				ValueSource *struct {
					SecretKeyRef *struct {
						Secret  string `json:"secret,omitempty"`
						Version string `json:"version,omitempty"`
					} `json:"secretKeyRef,omitempty"`
				} `json:"valueSource,omitempty"`
			} `json:"env,omitempty"`
		} `json:"containers,omitempty"`
	} `json:"template,omitempty"`
}

func GetService(ctx context.Context, accessToken, projectID, region, serviceName string) (*ServiceDetail, error) {
	u := fmt.Sprintf(
		"https://run.googleapis.com/v2/projects/%s/locations/%s/services/%s",
		projectID,
		region,
		serviceName,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, apperr.New(ErrRunGetService).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, apperr.New(ErrRunGetService).WithCause(err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)

	if res.StatusCode == 404 {
		return nil, apperr.New(ErrRunGetService).
			WithMeta("http_status", res.Status).
			WithMeta("service", serviceName).
			WithFix("Service not found in this project/region. Run: advncd services")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, apperr.New(ErrRunGetService).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(raw)).
			WithFix("Ensure Cloud Run API is enabled and you have permission to read services.")
	}

	var out getResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, apperr.New(ErrRunGetService).WithCause(err).
			WithMeta("raw_body", string(raw))
	}

	img := ""
	env := map[string]string{}
	secretEnv := make([]string, 0, 8)
	memory := ""
	if len(out.Template.Containers) > 0 {
		img = out.Template.Containers[0].Image
		if lim := out.Template.Containers[0].Resources.Limits; lim != nil {
			memory = strings.TrimSpace(lim["memory"])
		}
		for _, v := range out.Template.Containers[0].Env {
			if strings.TrimSpace(v.Name) == "" {
				continue
			}
			if v.ValueSource != nil && v.ValueSource.SecretKeyRef != nil {
				secretEnv = append(secretEnv, v.Name)
				continue
			}
			env[v.Name] = v.Value
		}
	}

	latestRevision := strings.TrimSpace(out.LatestReadyRevision)
	if latestRevision == "" {
		latestRevision = strings.TrimSpace(out.LatestCreatedRevision)
	}
	status := DeriveStatus(out.Conditions)
	timeoutSec := parseTimeoutSeconds(out.Template.Timeout)

	return &ServiceDetail{
		Name:           shortNameOr(serviceName, out.Name),
		URL:            out.URI,
		Image:          img,
		Env:            env,
		SecretEnvKeys:  secretEnv,
		Memory:         memory,
		TimeoutSeconds: timeoutSec,
		MinInstances:   out.Scaling.MinInstanceCount,
		MaxInstances:   out.Scaling.MaxInstanceCount,
		Conditions:     out.Conditions,
		Status:         status,
		LatestRevision: latestRevision,
	}, nil
}

func shortNameOr(fallback, full string) string {
	if full == "" {
		return fallback
	}
	parts := strings.Split(full, "/")
	if len(parts) == 0 {
		return fallback
	}
	return parts[len(parts)-1]
}

func parseTimeoutSeconds(raw string) int {
	v := strings.TrimSpace(strings.TrimSuffix(raw, "s"))
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}
