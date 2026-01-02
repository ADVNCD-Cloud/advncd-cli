package gcs

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

var ErrBucketCreate = apperr.E("C-GCS-002", "Failed to create Cloud Storage bucket")

type bucketCreateReq struct {
	Name         string `json:"name"`
	Location     string `json:"location,omitempty"`
	StorageClass string `json:"storageClass,omitempty"`
}

func CreateBucket(ctx context.Context, accessToken, projectID, bucketName, location string) error {
	u, _ := url.Parse("https://storage.googleapis.com/storage/v1/b")
	q := u.Query()
	q.Set("project", projectID)
	u.RawQuery = q.Encode()

	payload := bucketCreateReq{
		Name:         bucketName,
		Location:     location,
		StorageClass: "STANDARD",
	}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(b))
	if err != nil {
		return apperr.New(ErrBucketCreate).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return apperr.New(ErrBucketCreate).WithCause(err).
			WithFix("Check your internet connection and try again.")
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)

	// 409 = already exists (ok for our case)
	if res.StatusCode == 409 {
		return nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return apperr.New(ErrBucketCreate).
			WithMeta("http_status", res.Status).
			WithMeta("bucket", bucketName).
			WithMeta("location", location).
			WithMeta("raw_body", string(raw)).
			WithFix("Ensure you have permissions to create buckets in this project.")
	}

	return nil
}