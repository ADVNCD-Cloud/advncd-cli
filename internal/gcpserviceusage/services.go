package gcpserviceusage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var ErrServiceGet = apperr.E("B-SU-001", "Failed to check API status")

type ServiceGet struct {
	Name  string `json:"name"`  // projects/{number}/services/{service}
	State string `json:"state"` // ENABLED / DISABLED
}

func GetServiceState(ctx context.Context, accessToken, projectNumber, serviceName string) (string, error) {
	u, _ := url.Parse("https://serviceusage.googleapis.com/v1/projects/" + projectNumber + "/services/" + serviceName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", apperr.New(ErrServiceGet).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", apperr.New(ErrServiceGet).WithCause(err).
			WithFix("Check your internet connection and try again.")
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", apperr.New(ErrServiceGet).
			WithMeta("http_status", res.Status).
			WithMeta("service", serviceName).
			WithMeta("raw_body", string(body)).
			WithFix("Ensure Service Usage API is available for this project, and you have permission to view service states.")
	}

	var out ServiceGet
	if err := json.Unmarshal(body, &out); err != nil {
		return "", apperr.New(ErrServiceGet).WithCause(err).
			WithMeta("service", serviceName).
			WithMeta("raw_body", string(body))
	}

	if out.State == "" {
		return "", apperr.New(ErrServiceGet).
			WithMeta("service", serviceName).
			WithMeta("raw_body", string(body)).
			WithFix("Google returned no state for this service.")
	}

	return out.State, nil
}