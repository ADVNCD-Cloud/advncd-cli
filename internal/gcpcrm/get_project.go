package gcpcrm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var ErrProjectGet = apperr.E("B-CRM-002", "Failed to fetch GCP project info")

type ProjectGet struct {
	ProjectNumber string `json:"projectNumber"`
	ProjectID     string `json:"projectId"`
	Name          string `json:"name"`
	LifecycleState string `json:"lifecycleState"`
}

func GetProject(ctx context.Context, accessToken, projectID string) (*ProjectGet, error) {
	u, _ := url.Parse("https://cloudresourcemanager.googleapis.com/v1/projects/" + projectID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, apperr.New(ErrProjectGet).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, apperr.New(ErrProjectGet).WithCause(err).
			WithFix("Check your internet connection and try again.")
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, apperr.New(ErrProjectGet).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(body)).
			WithFix("Ensure you have access to this project.")
	}

	var out ProjectGet
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, apperr.New(ErrProjectGet).WithCause(err).
			WithMeta("raw_body", string(body))
	}

	if out.ProjectNumber == "" {
		return nil, apperr.New(ErrProjectGet).
			WithMeta("raw_body", string(body)).
			WithFix("Google returned no projectNumber.")
	}

	return &out, nil
}