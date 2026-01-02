package gcpartifact

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var (
	ErrRepoCheck  = apperr.E("C-AR-001", "Failed to check Artifact Registry repository")
	ErrRepoCreate = apperr.E("C-AR-002", "Failed to create Artifact Registry repository")
)

func EnsureDockerRepo(ctx context.Context, accessToken, projectID, region, repoID string) error {
	exists, err := repoExists(ctx, accessToken, projectID, region, repoID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return createRepo(ctx, accessToken, projectID, region, repoID)
}

func repoExists(ctx context.Context, accessToken, projectID, region, repoID string) (bool, error) {
	u := fmt.Sprintf("https://artifactregistry.googleapis.com/v1/projects/%s/locations/%s/repositories/%s", projectID, region, repoID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false, apperr.New(ErrRepoCheck).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return false, apperr.New(ErrRepoCheck).WithCause(err).
			WithFix("Check your internet connection and try again.")
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)

	if res.StatusCode == 404 {
		return false, nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return false, apperr.New(ErrRepoCheck).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(raw)).
			WithFix("Ensure Artifact Registry API is enabled and you have permission to view repositories.")
	}

	return true, nil
}

func createRepo(ctx context.Context, accessToken, projectID, region, repoID string) error {
	u := fmt.Sprintf("https://artifactregistry.googleapis.com/v1/projects/%s/locations/%s/repositories?repositoryId=%s", projectID, region, repoID)

	body := map[string]any{
		"format":      "DOCKER",
		"description": "Advncd images",
	}
	b, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return apperr.New(ErrRepoCreate).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return apperr.New(ErrRepoCreate).WithCause(err).
			WithFix("Check your internet connection and try again.")
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)

	// 409 = already exists (race / parallel)
	if res.StatusCode == 409 {
		return nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return apperr.New(ErrRepoCreate).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(raw)).
			WithFix("Ensure you have permission to create Artifact Registry repositories (roles/artifactregistry.admin or owner in dev).")
	}

	return nil
}