package gcpcrm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var (
	ErrProjectCreate = apperr.E("B-CRM-003", "Failed to create GCP project")
	ErrProjectDelete = apperr.E("B-CRM-004", "Failed to delete GCP project")
)

type crmOperation struct {
	Name  string `json:"name"`
	Done  bool   `json:"done"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func CreateProject(ctx context.Context, accessToken, projectID, name string) error {
	payload := map[string]string{
		"projectId": strings.TrimSpace(projectID),
	}
	if strings.TrimSpace(name) != "" {
		payload["name"] = strings.TrimSpace(name)
	}

	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://cloudresourcemanager.googleapis.com/v1/projects", bytes.NewReader(b))
	if err != nil {
		return apperr.New(ErrProjectCreate).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return apperr.New(ErrProjectCreate).WithCause(err).
			WithFix("Check your internet connection and try again.")
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return apperr.New(ErrProjectCreate).
			WithMeta("http_status", res.Status).
			WithMeta("project_id", projectID).
			WithMeta("raw_body", string(body)).
			WithFix("Project ID must be globally unique and match Google naming rules.")
	}

	var op crmOperation
	if err := json.Unmarshal(body, &op); err != nil || op.Name == "" {
		// If Google returned project directly, treat as success.
		return nil
	}

	return waitCRMOperation(ctx, accessToken, op.Name, ErrProjectCreate)
}

func DeleteProject(ctx context.Context, accessToken, projectID string) error {
	u := "https://cloudresourcemanager.googleapis.com/v1/projects/" + strings.TrimSpace(projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return apperr.New(ErrProjectDelete).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return apperr.New(ErrProjectDelete).WithCause(err).
			WithFix("Check your internet connection and try again.")
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return apperr.New(ErrProjectDelete).
			WithMeta("http_status", res.Status).
			WithMeta("project_id", projectID).
			WithMeta("raw_body", string(body))
	}

	var op crmOperation
	if err := json.Unmarshal(body, &op); err != nil || op.Name == "" {
		return nil
	}

	return waitCRMOperation(ctx, accessToken, op.Name, ErrProjectDelete)
}

func waitCRMOperation(ctx context.Context, accessToken, opName string, entry apperr.Entry) error {
	opURL := opName
	if u, err := url.Parse(opName); err != nil || u.Scheme == "" || u.Host == "" {
		opURL = "https://cloudresourcemanager.googleapis.com/v1/" + opName
	}

	client := &http.Client{Timeout: 15 * time.Second}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, opURL, nil)
		if err != nil {
			return apperr.New(entry).WithCause(err)
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)

		res, err := client.Do(req)
		if err != nil {
			return apperr.New(entry).WithCause(err)
		}

		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()

		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return apperr.New(entry).
				WithMeta("http_status", res.Status).
				WithMeta("operation", opName).
				WithMeta("raw_body", string(body))
		}

		var op crmOperation
		if err := json.Unmarshal(body, &op); err != nil {
			return apperr.New(entry).WithCause(err).
				WithMeta("operation", opName).
				WithMeta("raw_body", string(body))
		}

		if op.Done {
			if op.Error != nil && op.Error.Message != "" {
				return apperr.New(entry).
					WithMeta("operation", opName).
					WithMeta("operation_error", op.Error.Message)
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return apperr.New(entry).WithCause(ctx.Err()).
				WithMeta("operation", opName)
		case <-ticker.C:
		}
	}
}
