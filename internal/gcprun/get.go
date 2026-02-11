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

var ErrRunGetService = apperr.E("D-RUN-002", "Failed to get Cloud Run service")

type ServiceDetail struct {
	Name       string
	URL        string
	Image      string
	Conditions []Condition
}

type getResp struct {
	Name       string      `json:"name"`
	URI        string      `json:"uri,omitempty"`
	Conditions []Condition `json:"conditions,omitempty"`
	Template struct {
		Containers []struct {
			Image string `json:"image"`
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
	if len(out.Template.Containers) > 0 {
		img = out.Template.Containers[0].Image
	}

	return &ServiceDetail{
		Name:       shortNameOr(serviceName, out.Name),
		URL:        out.URI,
		Image:      img,
		Conditions: out.Conditions,
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