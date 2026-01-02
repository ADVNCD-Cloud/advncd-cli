package cloudbuild

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

type buildGetResp struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	LogURL string `json:"logUrl"`
}

func WaitBuild(ctx context.Context, req WaitRequest) (*Build, error) {
	if req.PollEvery <= 0 {
		req.PollEvery = 3 * time.Second
	}

	region := req.Region
	if region == "" {
		region = "global"
	}

	url := fmt.Sprintf("https://cloudbuild.googleapis.com/v1/projects/%s/locations/%s/builds/%s", req.ProjectID, region, req.BuildID)
	client := &http.Client{Timeout: 20 * time.Second}

	ticker := time.NewTicker(req.PollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, apperr.New(ErrBuildPoll).WithCause(ctx.Err()).
				WithFix("Build is still running; open Cloud Build logs URL to monitor.")
		case <-ticker.C:
			httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return nil, apperr.New(ErrBuildPoll).WithCause(err)
			}
			httpReq.Header.Set("Authorization", "Bearer "+req.AccessToken)

			res, err := client.Do(httpReq)
			if err != nil {
				return nil, apperr.New(ErrBuildPoll).WithCause(err).
					WithFix("Check your internet connection and try again.")
			}
			raw, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()

			if res.StatusCode < 200 || res.StatusCode >= 300 {
				return nil, apperr.New(ErrBuildPoll).
					WithMeta("http_status", res.Status).
					WithMeta("raw_body", string(raw))
			}

			var out buildGetResp
			if err := json.Unmarshal(raw, &out); err != nil {
				return nil, apperr.New(ErrBuildPoll).WithCause(err).
					WithMeta("raw_body", string(raw))
			}

			switch out.Status {
			case "SUCCESS", "FAILURE", "CANCELLED", "TIMEOUT", "INTERNAL_ERROR":
				return &Build{ID: out.ID, Status: out.Status, LogURL: out.LogURL}, nil
			default:
				// QUEUED, WORKING, etc.
			}
		}
	}
}