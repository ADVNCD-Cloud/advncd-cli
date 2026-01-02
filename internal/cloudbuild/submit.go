package cloudbuild

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcs"
)

type createBuildReq struct {
	Source struct {
		StorageSource struct {
			Bucket string `json:"bucket"`
			Object string `json:"object"`
		} `json:"storageSource"`
	} `json:"source"`
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

type opResp struct {
	Name     string `json:"name"`
	Metadata struct {
		Build struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			LogURL string `json:"logUrl"`
		} `json:"build"`
	} `json:"metadata"`
}

func SubmitBuildpacksBuild(ctx context.Context, req SubmitRequest) (*Build, error) {
	// 1) archive source
	tgz, err := TarGzBytes(req.SourceDir)
	if err != nil {
		return nil, apperr.New(ErrBuildSubmit).WithCause(err).
			WithFix("Ensure the current directory is readable.")
	}

	// 2) upload to GCS bucket
	bucket := fmt.Sprintf("%s_cloudbuild", req.ProjectID)
	object := fmt.Sprintf("advncd/source-%d.tar.gz", time.Now().Unix())

	status, upErr := gcs.UploadObjectMedia(ctx, req.AccessToken, bucket, object, tgz)
	if upErr != nil {
		// Auto-create bucket if missing (404), then retry once
		if status == 404 {
			if err := gcs.CreateBucket(
				ctx,
				req.AccessToken,
				req.ProjectID,
				bucket,
				detectBuildRegionFromImage(req.Image),
			); err != nil {
				return nil, apperr.New(ErrBuildSubmit).WithCause(err).
					WithMeta("bucket", bucket).
					WithFix("Unable to auto-create the Cloud Build bucket; create it manually in Cloud Storage.")
			}

			// retry upload (IMPORTANT: use =, not :=)
			status, upErr = gcs.UploadObjectMedia(ctx, req.AccessToken, bucket, object, tgz)
			if upErr != nil {
				return nil, apperr.New(ErrBuildSubmit).WithCause(upErr).
					WithMeta("bucket", bucket).
					WithFix("Bucket was created, but upload still failed. Check IAM permissions for Cloud Storage.")
			}
		} else {
			return nil, apperr.New(ErrBuildSubmit).WithCause(upErr).
				WithMeta("bucket", bucket).
				WithFix("Check Cloud Storage permissions or API status.")
		}
	}

	// 3) create build in regional Cloud Build endpoint
	// Note: builds.create is regional: /v1/projects/{project}/locations/{region}/builds
	endpoint := fmt.Sprintf("https://cloudbuild.googleapis.com/v1/projects/%s/locations/%s/builds", req.ProjectID, detectBuildRegionFromImage(req.Image))

	cb := createBuildReq{}
	cb.Source.StorageSource.Bucket = bucket
	cb.Source.StorageSource.Object = object
	cb.Timeout = "1200s"
	cb.Images = []string{req.Image}
	cb.Options.Logging = "CLOUD_LOGGING_ONLY"

	cb.Steps = append(cb.Steps, struct {
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

	payload, _ := json.Marshal(cb)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, apperr.New(ErrBuildSubmit).WithCause(err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.AccessToken)
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{Timeout: 30 * time.Second}
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
			WithMeta("raw_body", string(raw)).
			WithFix("Ensure Cloud Build API is enabled and you have permission to create builds.").
			WithFix("Ensure Artifact Registry repo 'advncd' exists in the selected region.")
		return nil, ae
	}

	var op opResp
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, apperr.New(ErrBuildSubmit).WithCause(err).
			WithMeta("raw_body", string(raw))
	}

	id := op.Metadata.Build.ID
	if id == "" {
		return nil, apperr.New(ErrBuildSubmit).
			WithMeta("raw_body", string(raw)).
			WithFix("Cloud Build returned no build id (operation metadata missing).")
	}

	return &Build{ID: id, Status: op.Metadata.Build.Status, LogURL: op.Metadata.Build.LogURL}, nil
}

// MVP: derive build region from image prefix like "europe-west3-docker.pkg.dev/..."
// If parsing fails, fallback to "global".
func detectBuildRegionFromImage(image string) string {
	// image format: {region}-docker.pkg.dev/...
	const suf = "-docker.pkg.dev/"
	i := bytes.Index([]byte(image), []byte(suf))
	if i <= 0 {
		return "global"
	}
	return image[:i]
}