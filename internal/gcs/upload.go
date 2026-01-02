package gcs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var ErrUpload = apperr.E("C-GCS-001", "Failed to upload source to Cloud Storage")

type uploadResp struct {
	Bucket string `json:"bucket"`
	Name   string `json:"name"`
	Size   string `json:"size"`
}

// UploadObjectMedia uploads raw bytes to GCS.
// Returns httpStatus (0 on success).
func UploadObjectMedia(ctx context.Context, accessToken, bucket, objectName string, content []byte) (int, error) {
	u, _ := url.Parse(fmt.Sprintf("https://storage.googleapis.com/upload/storage/v1/b/%s/o", bucket))
	q := u.Query()
	q.Set("uploadType", "media")
	q.Set("name", objectName)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(content))
	if err != nil {
		return 0, apperr.New(ErrUpload).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/gzip")

	client := &http.Client{Timeout: 60 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return 0, apperr.New(ErrUpload).WithCause(err).
			WithFix("Check your internet connection and try again.")
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return res.StatusCode, apperr.New(ErrUpload).
			WithMeta("http_status", res.Status).
			WithMeta("bucket", bucket).
			WithMeta("object", objectName).
			WithMeta("raw_body", string(raw)).
			WithFix("Ensure Cloud Storage API is enabled and you have permission to write objects.")
	}

	// optional parse (for debug)
	var out uploadResp
	_ = json.Unmarshal(raw, &out)
	return 0, nil
}