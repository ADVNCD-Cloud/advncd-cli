package gcpserviceusage

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var (
	ErrServiceEnable = apperr.E("B-SU-002", "Failed to enable Google API")
)

type serviceUsageOperation struct {
	Name  string `json:"name"`
	Done  bool   `json:"done"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func EnableService(ctx context.Context, accessToken, projectNumber, serviceName string) error {
	u := "https://serviceusage.googleapis.com/v1/projects/" + projectNumber + "/services/" + serviceName + ":enable"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader([]byte("{}")))
	if err != nil {
		return apperr.New(ErrServiceEnable).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return apperr.New(ErrServiceEnable).WithCause(err).
			WithFix("Check your internet connection and try again.")
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return apperr.New(ErrServiceEnable).
			WithMeta("http_status", res.Status).
			WithMeta("service", serviceName).
			WithMeta("raw_body", string(body)).
			WithFix("Ensure you have permission to enable services in this project.")
	}

	var op serviceUsageOperation
	if err := json.Unmarshal(body, &op); err != nil {
		// If Google returned non-operation response, treat as success.
		return nil
	}
	if op.Name == "" {
		return nil
	}

	return waitServiceUsageOperation(ctx, accessToken, op.Name, serviceName)
}

func waitServiceUsageOperation(ctx context.Context, accessToken, opName, serviceName string) error {
	opURL := opName
	if u, err := url.Parse(opName); err != nil || u.Scheme == "" || u.Host == "" {
		opURL = "https://serviceusage.googleapis.com/v1/" + opName
	}

	client := &http.Client{Timeout: 15 * time.Second}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, opURL, nil)
		if err != nil {
			return apperr.New(ErrServiceEnable).WithCause(err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)

		res, err := client.Do(req)
		if err != nil {
			return apperr.New(ErrServiceEnable).WithCause(err)
		}

		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()

		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return apperr.New(ErrServiceEnable).
				WithMeta("http_status", res.Status).
				WithMeta("service", serviceName).
				WithMeta("operation", opName).
				WithMeta("raw_body", string(body))
		}

		var op serviceUsageOperation
		if err := json.Unmarshal(body, &op); err != nil {
			return apperr.New(ErrServiceEnable).WithCause(err).
				WithMeta("operation", opName).
				WithMeta("raw_body", string(body))
		}

		if op.Done {
			if op.Error != nil && op.Error.Message != "" {
				return apperr.New(ErrServiceEnable).
					WithMeta("service", serviceName).
					WithMeta("operation", opName).
					WithMeta("operation_error", op.Error.Message)
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return apperr.New(ErrServiceEnable).WithCause(ctx.Err()).
				WithMeta("service", serviceName).
				WithMeta("operation", opName)
		case <-ticker.C:
		}
	}
}
