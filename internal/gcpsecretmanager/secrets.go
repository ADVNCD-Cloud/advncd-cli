package gcpsecretmanager

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var (
	ErrSecretEnsure = apperr.E("S-SEC-001", "Failed to ensure Secret Manager secret")
	ErrSecretWrite  = apperr.E("S-SEC-002", "Failed to write Secret Manager secret version")
)

func EnsureSecretVersion(ctx context.Context, accessToken, projectID, secretID, value string) error {
	projectID = strings.TrimSpace(projectID)
	secretID = strings.TrimSpace(secretID)
	if projectID == "" || secretID == "" {
		return apperr.New(ErrSecretEnsure).
			WithMeta("project_id", projectID).
			WithMeta("secret_id", secretID)
	}

	if err := ensureSecret(ctx, accessToken, projectID, secretID); err != nil {
		return err
	}
	return addVersion(ctx, accessToken, projectID, secretID, value)
}

func ensureSecret(ctx context.Context, accessToken, projectID, secretID string) error {
	u, _ := url.Parse("https://secretmanager.googleapis.com/v1/projects/" + url.PathEscape(projectID) + "/secrets")
	q := u.Query()
	q.Set("secretId", secretID)
	u.RawQuery = q.Encode()

	body, _ := json.Marshal(map[string]any{
		"replication": map[string]any{
			"automatic": map[string]any{},
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return apperr.New(ErrSecretEnsure).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return apperr.New(ErrSecretEnsure).WithCause(err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	// Already exists is fine.
	if res.StatusCode == 409 {
		return nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return apperr.New(ErrSecretEnsure).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(raw)).
			WithMeta("project_id", projectID).
			WithMeta("secret_id", secretID)
	}
	return nil
}

func addVersion(ctx context.Context, accessToken, projectID, secretID, value string) error {
	u := "https://secretmanager.googleapis.com/v1/projects/" + url.PathEscape(projectID) +
		"/secrets/" + url.PathEscape(secretID) + ":addVersion"

	body, _ := json.Marshal(map[string]any{
		"payload": map[string]any{
			"data": base64.StdEncoding.EncodeToString([]byte(value)),
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return apperr.New(ErrSecretWrite).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return apperr.New(ErrSecretWrite).WithCause(err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return apperr.New(ErrSecretWrite).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(raw)).
			WithMeta("project_id", projectID).
			WithMeta("secret_id", secretID)
	}
	return nil
}
