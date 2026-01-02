package cloudbuild

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var (
	ErrBuildSubmit = apperr.E("C-BUILD-001", "Failed to submit Cloud Build")
	ErrBuildPoll   = apperr.E("C-BUILD-002", "Failed to poll Cloud Build")
)

type submitResp struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	LogURL string `json:"logUrl"`
}

type buildRequest struct {
	Steps []struct {
		Name string   `json:"name"`
		Args []string `json:"args"`
	} `json:"steps"`
	Images  []string `json:"images,omitempty"`
	Timeout string   `json:"timeout,omitempty"`
	Options struct {
		Logging string `json:"logging,omitempty"`
	} `json:"options,omitempty"`
}

func SubmitBuildpacksBuild(ctx context.Context, req SubmitRequest) (*Build, error) {
	// Build config: pack build --publish
	breq := buildRequest{
		Timeout: "1200s",
		Images:  []string{req.Image},
	}
	breq.Options.Logging = "CLOUD_LOGGING_ONLY"
	breq.Steps = append(breq.Steps, struct {
		Name string   `json:"name"`
		Args []string `json:"args"`
	}{
		Name: "gcr.io/k8s-skaffold/pack",
		Args: []string{
			"build", req.Image,
			"--builder", "gcr.io/buildpacks/builder:v1",
			"--path", ".",
			"--publish",
		},
	})

	meta, err := json.Marshal(breq)
	if err != nil {
		return nil, apperr.New(ErrBuildSubmit).WithCause(err)
	}

	// Cloud Build submit endpoint (supports multipart upload of source)
	url := fmt.Sprintf("https://cloudbuild.googleapis.com/v1/projects/%s/builds:submit", req.ProjectID)

	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	// Part 1: metadata JSON
	mh := make(textprotoMIMEHeader)
	mh.Set("Content-Disposition", `form-data; name="metadata"`)
	mh.Set("Content-Type", "application/json; charset=utf-8")
	metaPart, err := w.CreatePart(mh.std())
	if err != nil {
		return nil, apperr.New(ErrBuildSubmit).WithCause(err)
	}
	if _, err := metaPart.Write(meta); err != nil {
		return nil, apperr.New(ErrBuildSubmit).WithCause(err)
	}

	// Part 2: source tar.gz
	fh := make(textprotoMIMEHeader)
	fh.Set("Content-Disposition", `form-data; name="source"; filename="source.tar.gz"`)
	fh.Set("Content-Type", "application/gzip")
	srcPart, err := w.CreatePart(fh.std())
	if err != nil {
		return nil, apperr.New(ErrBuildSubmit).WithCause(err)
	}
	if err := writeTarGz(srcPart, req.SourceDir); err != nil {
		return nil, apperr.New(ErrBuildSubmit).WithCause(err).
			WithFix("Ensure the current directory is readable and not huge.")
	}

	_ = w.Close()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return nil, apperr.New(ErrBuildSubmit).WithCause(err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.AccessToken)
	httpReq.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{Timeout: 5 * time.Minute}
	res, err := client.Do(httpReq)
	if err != nil {
		return nil, apperr.New(ErrBuildSubmit).WithCause(err).
			WithFix("Check your internet connection and try again.")
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		ae := apperr.New(ErrBuildSubmit).
			WithMeta("http_status", res.Status).
			WithMeta("raw_body", string(raw))

		// Common fix for Artifact Registry repo missing
		ae = ae.WithFix("Ensure Artifact Registry repo 'advncd' exists in the selected region.").
			WithFix("Ensure Cloud Build has permission to push images (Project Editor/Owner in dev).")

		return nil, ae
	}

	var out submitResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, apperr.New(ErrBuildSubmit).WithCause(err).
			WithMeta("raw_body", string(raw))
	}
	if out.ID == "" {
		return nil, apperr.New(ErrBuildSubmit).
			WithMeta("raw_body", string(raw)).
			WithFix("Cloud Build returned no build id.")
	}

	return &Build{ID: out.ID, Status: out.Status, LogURL: out.LogURL}, nil
}